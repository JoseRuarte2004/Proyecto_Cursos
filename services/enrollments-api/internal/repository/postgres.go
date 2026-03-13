package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"

	"proyecto-cursos/services/enrollments-api/internal/domain"
	"proyecto-cursos/services/enrollments-api/internal/service"
)

type PostgresEnrollmentRepository struct {
	db *sql.DB
}

type scanner interface {
	Scan(dest ...any) error
}

func NewPostgresEnrollmentRepository(db *sql.DB) *PostgresEnrollmentRepository {
	return &PostgresEnrollmentRepository{db: db}
}

func (r *PostgresEnrollmentRepository) ReservePending(ctx context.Context, enrollment domain.Enrollment, capacity int) (*domain.Enrollment, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, enrollment.CourseID); err != nil {
		return nil, err
	}

	var existingStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT status
		FROM enrollments
		WHERE user_id = $1 AND course_id = $2
	`, enrollment.UserID, enrollment.CourseID).Scan(&existingStatus)
	switch {
	case err == nil:
		switch domain.Status(existingStatus) {
		case domain.StatusCancelled, domain.StatusRefunded:
			// Allow a terminal enrollment to be reserved again without creating a duplicate row.
		default:
			return nil, service.ErrEnrollmentAlreadyExists
		}
	case errors.Is(err, sql.ErrNoRows):
		err = nil
	default:
		return nil, err
	}

	var reservedCount int
	if err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM enrollments
		WHERE course_id = $1 AND status IN ('active', 'pending')
	`, enrollment.CourseID).Scan(&reservedCount); err != nil {
		return nil, err
	}

	if reservedCount >= capacity {
		return nil, service.ErrCourseFull
	}

	var created *domain.Enrollment
	if existingStatus == string(domain.StatusCancelled) || existingStatus == string(domain.StatusRefunded) {
		created, err = scanEnrollment(tx.QueryRowContext(ctx, `
			UPDATE enrollments
			SET status = 'pending',
				created_at = $3
			WHERE user_id = $1
			  AND course_id = $2
			  AND status IN ('cancelled', 'refunded')
			RETURNING id, user_id, course_id, status, created_at
		`, enrollment.UserID, enrollment.CourseID, enrollment.CreatedAt))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, service.ErrEnrollmentAlreadyExists
			}
			return nil, err
		}
	} else {
		const query = `
			INSERT INTO enrollments (id, user_id, course_id, status, created_at)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, user_id, course_id, status, created_at
		`
		created, err = scanEnrollment(tx.QueryRowContext(
			ctx,
			query,
			enrollment.ID,
			enrollment.UserID,
			enrollment.CourseID,
			enrollment.Status,
			enrollment.CreatedAt,
		))
		if err != nil {
			var pqErr *pq.Error
			if errors.As(err, &pqErr) && pqErr.Code == "23505" {
				return nil, service.ErrEnrollmentAlreadyExists
			}
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return created, nil
}

func (r *PostgresEnrollmentRepository) ConfirmPending(ctx context.Context, userID, courseID string, capacity int) (*domain.Enrollment, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, courseID); err != nil {
		return nil, err
	}

	enrollment, err := scanEnrollment(tx.QueryRowContext(ctx, `
		SELECT id, user_id, course_id, status, created_at
		FROM enrollments
		WHERE user_id = $1 AND course_id = $2
		FOR UPDATE
	`, userID, courseID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrPendingEnrollmentMissing
		}
		return nil, err
	}
	if enrollment.Status == domain.StatusActive {
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return enrollment, nil
	}
	if enrollment.Status != domain.StatusPending {
		return nil, service.ErrPendingEnrollmentMissing
	}

	var activeCount int
	if err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM enrollments
		WHERE course_id = $1 AND status = 'active'
	`, courseID).Scan(&activeCount); err != nil {
		return nil, err
	}
	if activeCount >= capacity {
		return nil, service.ErrCourseFull
	}

	confirmed, err := scanEnrollment(tx.QueryRowContext(ctx, `
		UPDATE enrollments
		SET status = 'active'
		WHERE user_id = $1 AND course_id = $2 AND status = 'pending'
		RETURNING id, user_id, course_id, status, created_at
	`, userID, courseID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrPendingEnrollmentMissing
		}
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return confirmed, nil
}

func (r *PostgresEnrollmentRepository) CancelPending(ctx context.Context, userID, courseID string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE enrollments
		SET status = 'cancelled'
		WHERE user_id = $1 AND course_id = $2 AND status = 'pending'
	`, userID, courseID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return service.ErrPendingEnrollmentMissing
	}

	return nil
}

func (r *PostgresEnrollmentRepository) UpsertPaymentActivationIssue(ctx context.Context, issue domain.PaymentActivationIssue) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO payment_activation_issues (
			id,
			order_id,
			user_id,
			course_id,
			reason_code,
			status,
			last_error,
			attempts,
			next_attempt_at,
			created_at,
			updated_at,
			resolved_at,
			locked_at,
			locked_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NULL, NULL)
		ON CONFLICT (order_id) DO UPDATE
		SET reason_code = EXCLUDED.reason_code,
			status = EXCLUDED.status,
			last_error = EXCLUDED.last_error,
			attempts = GREATEST(payment_activation_issues.attempts, EXCLUDED.attempts),
			next_attempt_at = EXCLUDED.next_attempt_at,
			updated_at = EXCLUDED.updated_at,
			resolved_at = EXCLUDED.resolved_at,
			locked_at = NULL,
			locked_by = NULL
	`, issue.ID, issue.OrderID, issue.UserID, issue.CourseID, issue.ReasonCode, issue.Status, issue.LastError, issue.Attempts, issue.NextAttemptAt, issue.CreatedAt, issue.UpdatedAt, issue.ResolvedAt)
	return err
}

func (r *PostgresEnrollmentRepository) ClaimPaymentActivationIssues(ctx context.Context, workerID string, now time.Time, limit int) ([]domain.PaymentActivationIssue, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	rows, err := tx.QueryContext(ctx, `
		WITH candidate AS (
			SELECT id
			FROM payment_activation_issues
			WHERE status = 'retryable'
			  AND resolved_at IS NULL
			  AND COALESCE(next_attempt_at, created_at) <= $1
			  AND (locked_at IS NULL OR locked_at <= $2)
			ORDER BY COALESCE(next_attempt_at, created_at) ASC, created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $4
		)
		UPDATE payment_activation_issues issues
		SET locked_at = $1,
			locked_by = $3
		FROM candidate
		WHERE issues.id = candidate.id
		RETURNING issues.id, issues.order_id, issues.user_id, issues.course_id, issues.reason_code, issues.status, issues.last_error, issues.attempts, issues.next_attempt_at, issues.created_at, issues.updated_at, issues.resolved_at, issues.locked_at, issues.locked_by
	`, now, now.Add(-2*time.Minute), workerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	issues, err := collectPaymentActivationIssues(rows)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return issues, nil
}

func (r *PostgresEnrollmentRepository) ReleasePaymentActivationIssue(ctx context.Context, issueID string, status domain.PaymentActivationIssueStatus, nextAttemptAt *time.Time, lastError string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE payment_activation_issues
		SET status = $2,
			last_error = $3,
			attempts = attempts + 1,
			next_attempt_at = $4,
			updated_at = NOW(),
			locked_at = NULL,
			locked_by = NULL
		WHERE id = $1
	`, issueID, status, lastError, nextAttemptAt)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return service.ErrEnrollmentNotFound
	}

	return nil
}

func (r *PostgresEnrollmentRepository) ResolvePaymentActivationIssue(ctx context.Context, orderID string, resolvedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE payment_activation_issues
		SET status = 'resolved',
			resolved_at = $2,
			updated_at = $2,
			locked_at = NULL,
			locked_by = NULL
		WHERE order_id = $1
		  AND resolved_at IS NULL
	`, orderID, resolvedAt)
	return err
}

func (r *PostgresEnrollmentRepository) ListByUserStatuses(ctx context.Context, userID string, statuses []domain.Status) ([]domain.Enrollment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, course_id, status, created_at
		FROM enrollments
		WHERE user_id = $1 AND status = ANY($2)
		ORDER BY created_at DESC
	`, userID, pq.Array(statuses))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectEnrollments(rows)
}

func (r *PostgresEnrollmentRepository) ListPaginated(ctx context.Context, limit, offset int) ([]domain.Enrollment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, course_id, status, created_at
		FROM enrollments
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectEnrollments(rows)
}

func (r *PostgresEnrollmentRepository) ListByCourseStatuses(ctx context.Context, courseID string, statuses []domain.Status) ([]domain.Enrollment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, course_id, status, created_at
		FROM enrollments
		WHERE course_id = $1 AND status = ANY($2)
		ORDER BY created_at DESC
	`, courseID, pq.Array(statuses))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectEnrollments(rows)
}

func (r *PostgresEnrollmentRepository) CountActiveByCourse(ctx context.Context, courseID string) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM enrollments
		WHERE course_id = $1 AND status = 'active'
	`, courseID).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

func (r *PostgresEnrollmentRepository) CountReservedByCourse(ctx context.Context, courseID string) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM enrollments
		WHERE course_id = $1 AND status IN ('active', 'pending')
	`, courseID).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

func (r *PostgresEnrollmentRepository) IsStudentEnrolled(ctx context.Context, courseID, studentID string) (bool, error) {
	var exists bool
	if err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM enrollments
			WHERE course_id = $1 AND user_id = $2 AND status = 'active'
		)
	`, courseID, studentID).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func (r *PostgresEnrollmentRepository) GetByUserCourse(ctx context.Context, userID, courseID string) (*domain.Enrollment, error) {
	enrollment, err := scanEnrollment(r.db.QueryRowContext(ctx, `
		SELECT id, user_id, course_id, status, created_at
		FROM enrollments
		WHERE user_id = $1 AND course_id = $2
	`, userID, courseID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrEnrollmentNotFound
		}
		return nil, err
	}

	return enrollment, nil
}

func (r *PostgresEnrollmentRepository) DeleteByCourse(ctx context.Context, courseID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM enrollments
		WHERE course_id = $1
	`, courseID)
	return err
}

func collectEnrollments(rows *sql.Rows) ([]domain.Enrollment, error) {
	enrollments := make([]domain.Enrollment, 0)
	for rows.Next() {
		enrollment, err := scanEnrollment(rows)
		if err != nil {
			return nil, err
		}

		enrollments = append(enrollments, *enrollment)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return enrollments, nil
}

func collectPaymentActivationIssues(rows *sql.Rows) ([]domain.PaymentActivationIssue, error) {
	issues := make([]domain.PaymentActivationIssue, 0)
	for rows.Next() {
		issue, err := scanPaymentActivationIssue(rows)
		if err != nil {
			return nil, err
		}
		issues = append(issues, *issue)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return issues, nil
}

func scanEnrollment(row scanner) (*domain.Enrollment, error) {
	var enrollment domain.Enrollment
	if err := row.Scan(
		&enrollment.ID,
		&enrollment.UserID,
		&enrollment.CourseID,
		&enrollment.Status,
		&enrollment.CreatedAt,
	); err != nil {
		return nil, err
	}

	return &enrollment, nil
}

func scanPaymentActivationIssue(row scanner) (*domain.PaymentActivationIssue, error) {
	var (
		issue         domain.PaymentActivationIssue
		nextAttemptAt sql.NullTime
		resolvedAt    sql.NullTime
		lockedAt      sql.NullTime
		lockedBy      sql.NullString
	)

	if err := row.Scan(
		&issue.ID,
		&issue.OrderID,
		&issue.UserID,
		&issue.CourseID,
		&issue.ReasonCode,
		&issue.Status,
		&issue.LastError,
		&issue.Attempts,
		&nextAttemptAt,
		&issue.CreatedAt,
		&issue.UpdatedAt,
		&resolvedAt,
		&lockedAt,
		&lockedBy,
	); err != nil {
		return nil, err
	}

	if nextAttemptAt.Valid {
		issue.NextAttemptAt = &nextAttemptAt.Time
	}
	if resolvedAt.Valid {
		issue.ResolvedAt = &resolvedAt.Time
	}
	if lockedAt.Valid {
		issue.LockedAt = &lockedAt.Time
	}
	if lockedBy.Valid {
		issue.LockedBy = &lockedBy.String
	}

	return &issue, nil
}
