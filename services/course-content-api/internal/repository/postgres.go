package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"

	"proyecto-cursos/services/course-content-api/internal/domain"
	"proyecto-cursos/services/course-content-api/internal/service"
)

type PostgresLessonRepository struct {
	db *sql.DB
}

type scanner interface {
	Scan(dest ...any) error
}

type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func NewPostgresLessonRepository(db *sql.DB) *PostgresLessonRepository {
	return &PostgresLessonRepository{db: db}
}

func (r *PostgresLessonRepository) Create(ctx context.Context, lesson domain.Lesson) (*domain.Lesson, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	const query = `
		INSERT INTO lessons (id, course_id, title, description, order_index, video_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, course_id, title, description, order_index, video_url, created_at, updated_at
	`

	createdLesson, err := scanLesson(tx.QueryRowContext(
		ctx,
		query,
		lesson.ID,
		lesson.CourseID,
		lesson.Title,
		lesson.Description,
		lesson.OrderIndex,
		lesson.VideoURL,
		lesson.CreatedAt,
		lesson.UpdatedAt,
	))
	if err != nil {
		return nil, mapPQError(err)
	}

	if err := insertLessonAttachments(ctx, tx, lesson.Attachments); err != nil {
		return nil, mapPQError(err)
	}
	createdLesson.Attachments = cloneAttachmentsWithoutData(lesson.Attachments)

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return createdLesson, nil
}

func (r *PostgresLessonRepository) GetByID(ctx context.Context, courseID, lessonID string) (*domain.Lesson, error) {
	const query = `
		SELECT id, course_id, title, description, order_index, video_url, created_at, updated_at
		FROM lessons
		WHERE course_id = $1 AND id = $2
	`

	lesson, err := scanLesson(r.db.QueryRowContext(ctx, query, courseID, lessonID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrLessonNotFound
		}
		return nil, mapPQError(err)
	}

	attachmentsByLesson, err := listLessonAttachments(ctx, r.db, []string{lesson.ID})
	if err != nil {
		return nil, err
	}
	lesson.Attachments = attachmentsByLesson[lesson.ID]

	return lesson, nil
}

func (r *PostgresLessonRepository) GetAttachment(ctx context.Context, courseID, lessonID, attachmentID string) (*domain.Attachment, error) {
	const query = `
		SELECT id, lesson_id, course_id, kind, file_name, content_type, size_bytes, data, created_at
		FROM lesson_attachments
		WHERE course_id = $1 AND lesson_id = $2 AND id = $3
	`

	attachment, err := scanLessonAttachment(r.db.QueryRowContext(ctx, query, courseID, lessonID, attachmentID), true)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAttachmentNotFound
		}
		return nil, err
	}

	return attachment, nil
}

func (r *PostgresLessonRepository) ListByCourse(ctx context.Context, courseID string) ([]domain.Lesson, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, course_id, title, description, order_index, video_url, created_at, updated_at
		FROM lessons
		WHERE course_id = $1
		ORDER BY order_index ASC, created_at ASC
	`, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lessons := make([]domain.Lesson, 0)
	lessonIDs := make([]string, 0)
	for rows.Next() {
		lesson, err := scanLesson(rows)
		if err != nil {
			return nil, err
		}

		lessons = append(lessons, *lesson)
		lessonIDs = append(lessonIDs, lesson.ID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	attachmentsByLesson, err := listLessonAttachments(ctx, r.db, lessonIDs)
	if err != nil {
		return nil, err
	}
	for index := range lessons {
		lessons[index].Attachments = attachmentsByLesson[lessons[index].ID]
	}

	return lessons, nil
}

func (r *PostgresLessonRepository) Update(ctx context.Context, lesson domain.Lesson) (*domain.Lesson, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	const query = `
		UPDATE lessons
		SET title = $3,
			description = $4,
			order_index = $5,
			video_url = $6,
			updated_at = $7
		WHERE course_id = $1 AND id = $2
		RETURNING id, course_id, title, description, order_index, video_url, created_at, updated_at
	`

	updatedLesson, err := scanLesson(tx.QueryRowContext(
		ctx,
		query,
		lesson.CourseID,
		lesson.ID,
		lesson.Title,
		lesson.Description,
		lesson.OrderIndex,
		lesson.VideoURL,
		lesson.UpdatedAt,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrLessonNotFound
		}
		return nil, mapPQError(err)
	}

	if err := insertLessonAttachments(ctx, tx, lesson.Attachments); err != nil {
		return nil, mapPQError(err)
	}

	attachmentsByLesson, err := listLessonAttachments(ctx, tx, []string{lesson.ID})
	if err != nil {
		return nil, err
	}
	updatedLesson.Attachments = attachmentsByLesson[lesson.ID]

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return updatedLesson, nil
}

func (r *PostgresLessonRepository) Delete(ctx context.Context, courseID, lessonID string) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM lessons
		WHERE course_id = $1 AND id = $2
	`, courseID, lessonID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return service.ErrLessonNotFound
	}

	return nil
}

func mapPQError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return service.ErrOrderIndexAlreadyUsed
	}

	return err
}

func scanLesson(row scanner) (*domain.Lesson, error) {
	var lesson domain.Lesson
	if err := row.Scan(
		&lesson.ID,
		&lesson.CourseID,
		&lesson.Title,
		&lesson.Description,
		&lesson.OrderIndex,
		&lesson.VideoURL,
		&lesson.CreatedAt,
		&lesson.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return &lesson, nil
}

func scanLessonAttachment(row scanner, includeData bool) (*domain.Attachment, error) {
	var attachment domain.Attachment
	if includeData {
		if err := row.Scan(
			&attachment.ID,
			&attachment.LessonID,
			&attachment.CourseID,
			&attachment.Kind,
			&attachment.FileName,
			&attachment.ContentType,
			&attachment.SizeBytes,
			&attachment.Data,
			&attachment.CreatedAt,
		); err != nil {
			return nil, err
		}
		return &attachment, nil
	}

	if err := row.Scan(
		&attachment.ID,
		&attachment.LessonID,
		&attachment.CourseID,
		&attachment.Kind,
		&attachment.FileName,
		&attachment.ContentType,
		&attachment.SizeBytes,
		&attachment.CreatedAt,
	); err != nil {
		return nil, err
	}

	return &attachment, nil
}

func listLessonAttachments(ctx context.Context, queryable dbtx, lessonIDs []string) (map[string][]domain.Attachment, error) {
	attachmentsByLesson := make(map[string][]domain.Attachment, len(lessonIDs))
	if len(lessonIDs) == 0 {
		return attachmentsByLesson, nil
	}

	rows, err := queryable.QueryContext(ctx, `
		SELECT id, lesson_id, course_id, kind, file_name, content_type, size_bytes, created_at
		FROM lesson_attachments
		WHERE lesson_id = ANY($1)
		ORDER BY created_at ASC, id ASC
	`, pq.Array(lessonIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		attachment, err := scanLessonAttachment(rows, false)
		if err != nil {
			return nil, err
		}
		attachmentsByLesson[attachment.LessonID] = append(
			attachmentsByLesson[attachment.LessonID],
			*attachment,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return attachmentsByLesson, nil
}

func insertLessonAttachments(ctx context.Context, execer dbtx, attachments []domain.Attachment) error {
	for _, attachment := range attachments {
		if len(attachment.Data) == 0 {
			continue
		}

		if _, err := execer.ExecContext(ctx, `
			INSERT INTO lesson_attachments (
				id, lesson_id, course_id, kind, file_name, content_type, size_bytes, data, created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`,
			attachment.ID,
			attachment.LessonID,
			attachment.CourseID,
			attachment.Kind,
			attachment.FileName,
			attachment.ContentType,
			attachment.SizeBytes,
			attachment.Data,
			attachment.CreatedAt,
		); err != nil {
			return err
		}
	}

	return nil
}

func cloneAttachmentsWithoutData(attachments []domain.Attachment) []domain.Attachment {
	if len(attachments) == 0 {
		return nil
	}

	cloned := make([]domain.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		cloned = append(cloned, domain.Attachment{
			ID:          attachment.ID,
			LessonID:    attachment.LessonID,
			CourseID:    attachment.CourseID,
			Kind:        attachment.Kind,
			FileName:    attachment.FileName,
			ContentType: attachment.ContentType,
			SizeBytes:   attachment.SizeBytes,
			CreatedAt:   attachment.CreatedAt,
		})
	}

	return cloned
}
