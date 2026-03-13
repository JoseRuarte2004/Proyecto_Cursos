package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"

	"proyecto-cursos/services/users-api/internal/domain"
	"proyecto-cursos/services/users-api/internal/service"
)

type PostgresUserRepository struct {
	db *sql.DB
}

type PostgresAuditLogRepository struct {
	db *sql.DB
}

type scanner interface {
	Scan(dest ...any) error
}

func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func NewPostgresAuditLogRepository(db *sql.DB) *PostgresAuditLogRepository {
	return &PostgresAuditLogRepository{db: db}
}

func (r *PostgresUserRepository) Create(ctx context.Context, user domain.User) (*domain.User, error) {
	return r.createUser(ctx, r.db, user)
}

func (r *PostgresUserRepository) CreateWithEmailVerificationToken(ctx context.Context, user domain.User, token domain.EmailVerificationToken) (*domain.User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer txRollback(tx)

	createdUser, err := r.createUser(ctx, tx, user)
	if err != nil {
		return nil, err
	}

	if err := r.insertEmailVerificationToken(ctx, tx, token); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return createdUser, nil
}

func (r *PostgresUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	const query = `
		SELECT id, name, email, password_hash, email_verified, email_verified_at, role, phone, dni, address, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	user, err := scanUser(r.db.QueryRowContext(ctx, query, email))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (r *PostgresUserRepository) GetByID(ctx context.Context, userID string) (*domain.User, error) {
	const query = `
		SELECT id, name, email, password_hash, email_verified, email_verified_at, role, phone, dni, address, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	user, err := scanUser(r.db.QueryRowContext(ctx, query, userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (r *PostgresUserRepository) List(ctx context.Context) ([]domain.User, error) {
	const query = `
		SELECT id, name, email, password_hash, email_verified, email_verified_at, role, phone, dni, address, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]domain.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}

		users = append(users, *user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (r *PostgresUserRepository) UpdateRole(ctx context.Context, userID string, role domain.Role) (*domain.User, error) {
	const query = `
		UPDATE users
		SET role = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, email, password_hash, email_verified, email_verified_at, role, phone, dni, address, created_at, updated_at
	`

	user, err := scanUser(r.db.QueryRowContext(ctx, query, userID, role))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (r *PostgresUserRepository) CreateEmailVerificationToken(ctx context.Context, token domain.EmailVerificationToken) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at, created_at, used_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		token.ID,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
		token.CreatedAt,
		token.UsedAt,
	)
	return err
}

func (r *PostgresUserRepository) ConsumeEmailVerificationToken(ctx context.Context, tokenHash string, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer txRollback(tx)

	var tokenID string
	var userID string
	if err := tx.QueryRowContext(
		ctx,
		`SELECT id, user_id
		 FROM email_verification_tokens
		 WHERE token_hash = $1
		   AND used_at IS NULL
		   AND expires_at > $2
		 FOR UPDATE`,
		tokenHash,
		now,
	).Scan(&tokenID, &userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrEmailVerificationTokenInvalid
		}
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE users
		 SET email_verified = TRUE,
		     email_verified_at = COALESCE(email_verified_at, $2),
		     updated_at = $2
		 WHERE id = $1`,
		userID,
		now,
	); err != nil {
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE email_verification_tokens
		 SET used_at = $2
		 WHERE id = $1`,
		tokenID,
		now,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *PostgresUserRepository) CreatePasswordResetToken(ctx context.Context, token domain.PasswordResetToken) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, created_at, used_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		token.ID,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
		token.CreatedAt,
		token.UsedAt,
	)
	return err
}

func (r *PostgresUserRepository) ConsumePasswordResetToken(ctx context.Context, tokenHash, passwordHash string, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer txRollback(tx)

	var tokenID string
	var userID string
	if err := tx.QueryRowContext(
		ctx,
		`SELECT id, user_id
		 FROM password_reset_tokens
		 WHERE token_hash = $1
		   AND used_at IS NULL
		   AND expires_at > $2
		 FOR UPDATE`,
		tokenHash,
		now,
	).Scan(&tokenID, &userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrPasswordResetTokenInvalid
		}
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE users
		 SET password_hash = $2,
		     updated_at = $3
		 WHERE id = $1`,
		userID,
		passwordHash,
		now,
	); err != nil {
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE password_reset_tokens
		 SET used_at = $2
		 WHERE id = $1`,
		tokenID,
		now,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *PostgresUserRepository) UpdatePasswordByEmail(ctx context.Context, email, passwordHash string, now time.Time) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE users
		 SET password_hash = $2,
		     updated_at = $3
		 WHERE email = $1`,
		email,
		passwordHash,
		now,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return service.ErrUserNotFound
	}

	return nil
}

func (r *PostgresUserRepository) IsEmailVerified(ctx context.Context, userID string) (bool, error) {
	var emailVerified bool
	if err := r.db.QueryRowContext(
		ctx,
		`SELECT email_verified FROM users WHERE id = $1`,
		userID,
	).Scan(&emailVerified); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, service.ErrUserNotFound
		}
		return false, err
	}

	return emailVerified, nil
}

func (r *PostgresUserRepository) MarkEmailVerifiedByEmail(ctx context.Context, email string, now time.Time) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE users
		 SET email_verified = TRUE,
		     email_verified_at = COALESCE(email_verified_at, $2),
		     updated_at = $2
		 WHERE email = $1`,
		email,
		now,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return service.ErrUserNotFound
	}

	return nil
}

func (r *PostgresAuditLogRepository) Create(ctx context.Context, auditLog domain.AuditLog) error {
	const query = `
		INSERT INTO audit_logs (id, admin_id, action, target_user_id, "timestamp", request_id, ip)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		auditLog.ID,
		auditLog.AdminID,
		auditLog.Action,
		auditLog.TargetUserID,
		auditLog.Timestamp,
		auditLog.RequestID,
		auditLog.IP,
	)

	return err
}

type userQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type userExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (r *PostgresUserRepository) createUser(ctx context.Context, db userQuerier, user domain.User) (*domain.User, error) {
	const query = `
		INSERT INTO users (id, name, email, password_hash, email_verified, email_verified_at, role, phone, dni, address, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, name, email, password_hash, email_verified, email_verified_at, role, phone, dni, address, created_at, updated_at
	`

	row := db.QueryRowContext(
		ctx,
		query,
		user.ID,
		user.Name,
		user.Email,
		user.PasswordHash,
		user.EmailVerified || user.IsVerified,
		user.EmailVerifiedAt,
		user.Role,
		user.Phone,
		user.DNI,
		user.Address,
		user.CreatedAt,
		user.UpdatedAt,
	)

	createdUser, err := scanUser(row)
	if err != nil {
		return nil, translateCreateUserErr(err)
	}

	return createdUser, nil
}

func (r *PostgresUserRepository) insertEmailVerificationToken(ctx context.Context, db userExecer, token domain.EmailVerificationToken) error {
	_, err := db.ExecContext(
		ctx,
		`INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at, created_at, used_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		token.ID,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
		token.CreatedAt,
		token.UsedAt,
	)
	return err
}

func translateCreateUserErr(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return service.ErrEmailAlreadyExists
	}
	return err
}

func scanUser(row scanner) (*domain.User, error) {
	var user domain.User
	var emailVerifiedAt sql.NullTime
	if err := row.Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.PasswordHash,
		&user.EmailVerified,
		&emailVerifiedAt,
		&user.Role,
		&user.Phone,
		&user.DNI,
		&user.Address,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if emailVerifiedAt.Valid {
		timestamp := emailVerifiedAt.Time
		user.EmailVerifiedAt = &timestamp
	}
	user.IsVerified = user.EmailVerified

	return &user, nil
}

func txRollback(tx *sql.Tx) {
	_ = tx.Rollback()
}
