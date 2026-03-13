package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"github.com/lib/pq"

	"proyecto-cursos/services/courses-api/internal/domain"
	"proyecto-cursos/services/courses-api/internal/service"
)

type PostgresCourseRepository struct {
	db *sql.DB
}

type scanner interface {
	Scan(dest ...any) error
}

func NewPostgresCourseRepository(db *sql.DB) *PostgresCourseRepository {
	return &PostgresCourseRepository{db: db}
}

func (r *PostgresCourseRepository) Create(ctx context.Context, course domain.Course) (*domain.Course, error) {
	const query = `
		INSERT INTO courses (id, title, description, category, image_url, price, currency, capacity, status, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, title, description, category, image_url, price::text, currency, capacity, status, created_by, created_at, updated_at
	`

	return scanCourse(r.db.QueryRowContext(
		ctx,
		query,
		course.ID,
		course.Title,
		course.Description,
		course.Category,
		course.ImageURL,
		course.Price,
		course.Currency,
		course.Capacity,
		course.Status,
		course.CreatedBy,
		course.CreatedAt,
		course.UpdatedAt,
	))
}

func (r *PostgresCourseRepository) GetByID(ctx context.Context, courseID string) (*domain.Course, error) {
	const query = `
		SELECT id, title, description, category, image_url, price::text, currency, capacity, status, created_by, created_at, updated_at
		FROM courses
		WHERE id = $1
	`

	course, err := scanCourse(r.db.QueryRowContext(ctx, query, courseID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrCourseNotFound
		}
		return nil, err
	}

	return course, nil
}

func (r *PostgresCourseRepository) ListPublished(ctx context.Context, limit, offset int) ([]domain.Course, error) {
	const query = `
		SELECT id, title, description, category, image_url, price::text, currency, capacity, status, created_by, created_at, updated_at
		FROM courses
		WHERE status = 'published'
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	courses := make([]domain.Course, 0)
	for rows.Next() {
		course, err := scanCourse(rows)
		if err != nil {
			return nil, err
		}

		courses = append(courses, *course)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return courses, nil
}

func (r *PostgresCourseRepository) CountCourses(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM courses`).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

func (r *PostgresCourseRepository) ListRecommendationCatalog(ctx context.Context) ([]service.RecommendationCatalogItem, error) {
	const query = `
		SELECT
			id,
			title,
			CASE
				WHEN length(trim(description)) > 220 THEN left(trim(description), 217) || '...'
				ELSE trim(description)
			END AS short_description,
			category,
			'no especificado' AS level,
			price::text,
			currency,
			capacity
		FROM courses
		WHERE status = 'published'
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	catalog := make([]service.RecommendationCatalogItem, 0)
	for rows.Next() {
		var item service.RecommendationCatalogItem
		var priceText string
		if err := rows.Scan(&item.ID, &item.Title, &item.ShortDescription, &item.Category, &item.Level, &priceText, &item.Currency, &item.Capacity); err != nil {
			return nil, err
		}
		price, err := strconv.ParseFloat(priceText, 64)
		if err != nil {
			return nil, err
		}
		item.Price = price

		catalog = append(catalog, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return catalog, nil
}

func (r *PostgresCourseRepository) Update(ctx context.Context, course domain.Course) (*domain.Course, error) {
	const query = `
		UPDATE courses
		SET title = $2,
			description = $3,
			category = $4,
			image_url = $5,
			price = $6,
			currency = $7,
			capacity = $8,
			status = $9,
			updated_at = $10
		WHERE id = $1
		RETURNING id, title, description, category, image_url, price::text, currency, capacity, status, created_by, created_at, updated_at
	`

	updatedCourse, err := scanCourse(r.db.QueryRowContext(
		ctx,
		query,
		course.ID,
		course.Title,
		course.Description,
		course.Category,
		course.ImageURL,
		course.Price,
		course.Currency,
		course.Capacity,
		course.Status,
		course.UpdatedAt,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrCourseNotFound
		}
		return nil, err
	}

	return updatedCourse, nil
}

func (r *PostgresCourseRepository) Delete(ctx context.Context, courseID string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM courses WHERE id = $1`, courseID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return service.ErrCourseNotFound
	}

	return nil
}

func (r *PostgresCourseRepository) AssignTeacher(ctx context.Context, courseID, teacherID string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO course_teachers (course_id, teacher_id)
		VALUES ($1, $2)
	`, courseID, teacherID)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return service.ErrTeacherAlreadyAssigned
		}
		return err
	}

	return nil
}

func (r *PostgresCourseRepository) ListTeachers(ctx context.Context, courseID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT teacher_id
		FROM course_teachers
		WHERE course_id = $1
		ORDER BY teacher_id
	`, courseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	teacherIDs := make([]string, 0)
	for rows.Next() {
		var teacherID string
		if err := rows.Scan(&teacherID); err != nil {
			return nil, err
		}

		teacherIDs = append(teacherIDs, teacherID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return teacherIDs, nil
}

func (r *PostgresCourseRepository) RemoveTeacher(ctx context.Context, courseID, teacherID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM course_teachers
		WHERE course_id = $1 AND teacher_id = $2
	`, courseID, teacherID)
	return err
}

func (r *PostgresCourseRepository) ListByTeacher(ctx context.Context, teacherID string) ([]domain.Course, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.title, c.description, c.category, c.image_url, c.price::text, c.currency, c.capacity, c.status, c.created_by, c.created_at, c.updated_at
		FROM courses c
		INNER JOIN course_teachers ct ON ct.course_id = c.id
		WHERE ct.teacher_id = $1
		ORDER BY c.created_at DESC
	`, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	courses := make([]domain.Course, 0)
	for rows.Next() {
		course, err := scanCourse(rows)
		if err != nil {
			return nil, err
		}

		courses = append(courses, *course)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return courses, nil
}

func (r *PostgresCourseRepository) IsTeacherAssigned(ctx context.Context, courseID, teacherID string) (bool, error) {
	var exists bool
	if err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM course_teachers
			WHERE course_id = $1 AND teacher_id = $2
		)
	`, courseID, teacherID).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func scanCourse(row scanner) (*domain.Course, error) {
	var course domain.Course
	var imageURL sql.NullString
	var priceText string

	if err := row.Scan(
		&course.ID,
		&course.Title,
		&course.Description,
		&course.Category,
		&imageURL,
		&priceText,
		&course.Currency,
		&course.Capacity,
		&course.Status,
		&course.CreatedBy,
		&course.CreatedAt,
		&course.UpdatedAt,
	); err != nil {
		return nil, err
	}

	price, err := strconv.ParseFloat(priceText, 64)
	if err != nil {
		return nil, err
	}
	course.Price = price

	if imageURL.Valid {
		course.ImageURL = &imageURL.String
	}

	return &course, nil
}
