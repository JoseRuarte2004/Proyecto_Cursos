package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"proyecto-cursos/internal/platform/auth"
	"proyecto-cursos/services/courses-api/internal/domain"
)

var (
	ErrTitleRequired          = errors.New("title is required")
	ErrDescriptionRequired    = errors.New("description is required")
	ErrCategoryRequired       = errors.New("category is required")
	ErrCurrencyRequired       = errors.New("currency is required")
	ErrInvalidPrice           = errors.New("price must be greater than or equal to 0")
	ErrInvalidCapacity        = errors.New("capacity must be greater than 0")
	ErrInvalidStatus          = errors.New("invalid status")
	ErrCourseNotFound         = errors.New("course not found")
	ErrTeacherNotFound        = errors.New("teacher not found")
	ErrInvalidTeacher         = errors.New("user is not a teacher")
	ErrTeacherAlreadyAssigned = errors.New("teacher already assigned to course")
)

type CourseRepository interface {
	Create(ctx context.Context, course domain.Course) (*domain.Course, error)
	GetByID(ctx context.Context, courseID string) (*domain.Course, error)
	ListPublished(ctx context.Context, limit, offset int) ([]domain.Course, error)
	ListRecommendationCatalog(ctx context.Context) ([]RecommendationCatalogItem, error)
	Update(ctx context.Context, course domain.Course) (*domain.Course, error)
	Delete(ctx context.Context, courseID string) error
	AssignTeacher(ctx context.Context, courseID, teacherID string) error
	ListTeachers(ctx context.Context, courseID string) ([]string, error)
	RemoveTeacher(ctx context.Context, courseID, teacherID string) error
	ListByTeacher(ctx context.Context, teacherID string) ([]domain.Course, error)
	IsTeacherAssigned(ctx context.Context, courseID, teacherID string) (bool, error)
}

type TeacherInfo struct {
	ID    string
	Name  string
	Email string
	Role  auth.Role
}

type TeacherVerifier interface {
	GetTeacher(ctx context.Context, teacherID string) (*TeacherInfo, error)
}

type EnrollmentCleaner interface {
	DeleteCourseEnrollments(ctx context.Context, courseID string) error
}

type CreateCourseInput struct {
	Title       string
	Description string
	Category    string
	ImageURL    *string
	Price       float64
	Currency    string
	Capacity    int
	Status      domain.Status
}

type UpdateCourseInput struct {
	Title       *string
	Description *string
	Category    *string
	ImageURL    *string
	Price       *float64
	Currency    *string
	Capacity    *int
	Status      *domain.Status
}

type CourseService struct {
	repo        CourseRepository
	teachers    TeacherVerifier
	enrollments EnrollmentCleaner
	now         func() time.Time
}

func NewCourseService(repo CourseRepository, teachers TeacherVerifier, enrollments EnrollmentCleaner) *CourseService {
	if enrollments == nil {
		enrollments = noopEnrollmentCleaner{}
	}

	return &CourseService{
		repo:        repo,
		teachers:    teachers,
		enrollments: enrollments,
		now:         time.Now().UTC,
	}
}

func (s *CourseService) CreateCourse(ctx context.Context, adminID string, input CreateCourseInput) (*domain.Course, error) {
	now := s.now()
	course := domain.Course{
		ID:          uuid.NewString(),
		Title:       strings.TrimSpace(input.Title),
		Description: strings.TrimSpace(input.Description),
		Category:    strings.TrimSpace(input.Category),
		ImageURL:    normalizeOptionalString(input.ImageURL),
		Price:       input.Price,
		Currency:    strings.ToUpper(strings.TrimSpace(input.Currency)),
		Capacity:    input.Capacity,
		Status:      input.Status,
		CreatedBy:   adminID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := validateCourse(course); err != nil {
		return nil, err
	}

	return s.repo.Create(ctx, course)
}

func (s *CourseService) UpdateCourse(ctx context.Context, courseID string, input UpdateCourseInput) (*domain.Course, error) {
	course, err := s.repo.GetByID(ctx, courseID)
	if err != nil {
		return nil, err
	}

	if input.Title != nil {
		course.Title = strings.TrimSpace(*input.Title)
	}
	if input.Description != nil {
		course.Description = strings.TrimSpace(*input.Description)
	}
	if input.Category != nil {
		course.Category = strings.TrimSpace(*input.Category)
	}
	if input.ImageURL != nil {
		course.ImageURL = normalizeOptionalString(input.ImageURL)
	}
	if input.Price != nil {
		course.Price = *input.Price
	}
	if input.Currency != nil {
		course.Currency = strings.ToUpper(strings.TrimSpace(*input.Currency))
	}
	if input.Capacity != nil {
		course.Capacity = *input.Capacity
	}
	if input.Status != nil {
		course.Status = *input.Status
	}
	course.UpdatedAt = s.now()

	if err := validateCourse(*course); err != nil {
		return nil, err
	}

	return s.repo.Update(ctx, *course)
}

func (s *CourseService) DeleteCourse(ctx context.Context, courseID string) error {
	if _, err := s.repo.GetByID(ctx, courseID); err != nil {
		return err
	}

	if err := s.enrollments.DeleteCourseEnrollments(ctx, courseID); err != nil {
		return err
	}

	return s.repo.Delete(ctx, courseID)
}

func (s *CourseService) ListPublished(ctx context.Context, limit, offset int) ([]domain.Course, error) {
	return s.repo.ListPublished(ctx, limit, offset)
}

func (s *CourseService) GetPublishedCourse(ctx context.Context, courseID string) (*domain.Course, error) {
	course, err := s.repo.GetByID(ctx, courseID)
	if err != nil {
		return nil, err
	}

	if course.Status != domain.StatusPublished {
		return nil, ErrCourseNotFound
	}

	return course, nil
}

func (s *CourseService) GetCourse(ctx context.Context, courseID string) (*domain.Course, error) {
	return s.repo.GetByID(ctx, courseID)
}

func (s *CourseService) AssignTeacher(ctx context.Context, courseID, teacherID string) error {
	if _, err := s.repo.GetByID(ctx, courseID); err != nil {
		return err
	}

	teacher, err := s.teachers.GetTeacher(ctx, teacherID)
	if err != nil {
		return err
	}

	if teacher.Role != auth.RoleTeacher {
		return ErrInvalidTeacher
	}

	return s.repo.AssignTeacher(ctx, courseID, teacherID)
}

func (s *CourseService) ListCourseTeachers(ctx context.Context, courseID string) ([]string, error) {
	if _, err := s.repo.GetByID(ctx, courseID); err != nil {
		return nil, err
	}

	return s.repo.ListTeachers(ctx, courseID)
}

func (s *CourseService) RemoveTeacher(ctx context.Context, courseID, teacherID string) error {
	if _, err := s.repo.GetByID(ctx, courseID); err != nil {
		return err
	}

	return s.repo.RemoveTeacher(ctx, courseID, teacherID)
}

func (s *CourseService) ListTeacherCourses(ctx context.Context, teacherID string) ([]domain.Course, error) {
	return s.repo.ListByTeacher(ctx, teacherID)
}

func (s *CourseService) IsTeacherAssigned(ctx context.Context, courseID, teacherID string) (bool, error) {
	return s.repo.IsTeacherAssigned(ctx, courseID, teacherID)
}

func validateCourse(course domain.Course) error {
	if course.Title == "" {
		return ErrTitleRequired
	}
	if course.Description == "" {
		return ErrDescriptionRequired
	}
	if course.Category == "" {
		return ErrCategoryRequired
	}
	if course.Currency == "" {
		return ErrCurrencyRequired
	}
	if course.Price < 0 {
		return ErrInvalidPrice
	}
	if course.Capacity <= 0 {
		return ErrInvalidCapacity
	}
	if !course.Status.IsValid() {
		return ErrInvalidStatus
	}

	return nil
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

type noopEnrollmentCleaner struct{}

func (noopEnrollmentCleaner) DeleteCourseEnrollments(context.Context, string) error {
	return nil
}
