package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"proyecto-cursos/internal/platform/auth"
	"proyecto-cursos/services/course-content-api/internal/domain"
)

var (
	ErrTitleRequired          = errors.New("title is required")
	ErrDescriptionRequired    = errors.New("description is required")
	ErrVideoURLRequired       = errors.New("videoUrl is required")
	ErrInvalidOrderIndex      = errors.New("orderIndex must be greater than 0")
	ErrLessonNotFound         = errors.New("lesson not found")
	ErrAttachmentNotFound     = errors.New("attachment not found")
	ErrForbidden              = errors.New("forbidden")
	ErrOrderIndexAlreadyUsed  = errors.New("orderIndex already exists for this course")
	ErrTooManyAttachments     = errors.New("too many attachments")
	ErrAttachmentTooLarge     = errors.New("attachment size must be at most 25MB")
	ErrAttachmentNameRequired = errors.New("attachment fileName is required")
	ErrAttachmentDataRequired = errors.New("attachment data is required")
)

type LessonRepository interface {
	Create(ctx context.Context, lesson domain.Lesson) (*domain.Lesson, error)
	GetByID(ctx context.Context, courseID, lessonID string) (*domain.Lesson, error)
	GetAttachment(ctx context.Context, courseID, lessonID, attachmentID string) (*domain.Attachment, error)
	ListByCourse(ctx context.Context, courseID string) ([]domain.Lesson, error)
	Update(ctx context.Context, lesson domain.Lesson) (*domain.Lesson, error)
	Delete(ctx context.Context, courseID, lessonID string) error
}

type CourseAssignmentChecker interface {
	IsTeacherAssigned(ctx context.Context, courseID, teacherID string) (bool, error)
}

type EnrollmentChecker interface {
	IsStudentEnrolled(ctx context.Context, courseID, studentID string) (bool, error)
}

type AttachmentInput struct {
	FileName    string
	ContentType string
	Data        []byte
}

type CreateLessonInput struct {
	Title       string
	Description string
	OrderIndex  int
	VideoURL    string
	Attachments []AttachmentInput
}

type UpdateLessonInput struct {
	Title       *string
	Description *string
	OrderIndex  *int
	VideoURL    *string
	Attachments []AttachmentInput
}

type LessonService struct {
	repo              LessonRepository
	assignments       CourseAssignmentChecker
	enrollmentChecker EnrollmentChecker
	now               func() time.Time
}

const (
	maxLessonAttachments   = 8
	maxAttachmentSizeBytes = 25 << 20
)

func NewLessonService(repo LessonRepository, assignments CourseAssignmentChecker, enrollmentChecker EnrollmentChecker) *LessonService {
	return &LessonService{
		repo:              repo,
		assignments:       assignments,
		enrollmentChecker: enrollmentChecker,
		now:               time.Now().UTC,
	}
}

func (s *LessonService) CreateLesson(ctx context.Context, session auth.Session, courseID string, input CreateLessonInput) (*domain.Lesson, error) {
	if err := s.ensureLessonWriteAllowed(ctx, session, courseID); err != nil {
		return nil, err
	}

	now := s.now()
	lesson := domain.Lesson{
		ID:          uuid.NewString(),
		CourseID:    courseID,
		Title:       strings.TrimSpace(input.Title),
		Description: strings.TrimSpace(input.Description),
		OrderIndex:  input.OrderIndex,
		VideoURL:    strings.TrimSpace(input.VideoURL),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	attachments, err := buildLessonAttachments(lesson.ID, courseID, input.Attachments, now)
	if err != nil {
		return nil, err
	}
	lesson.Attachments = attachments

	if err := validateLesson(lesson); err != nil {
		return nil, err
	}

	return s.repo.Create(ctx, lesson)
}

func (s *LessonService) UpdateLesson(ctx context.Context, session auth.Session, courseID, lessonID string, input UpdateLessonInput) (*domain.Lesson, error) {
	if err := s.ensureLessonWriteAllowed(ctx, session, courseID); err != nil {
		return nil, err
	}

	lesson, err := s.repo.GetByID(ctx, courseID, lessonID)
	if err != nil {
		return nil, err
	}

	if input.Title != nil {
		lesson.Title = strings.TrimSpace(*input.Title)
	}
	if input.Description != nil {
		lesson.Description = strings.TrimSpace(*input.Description)
	}
	if input.OrderIndex != nil {
		lesson.OrderIndex = *input.OrderIndex
	}
	if input.VideoURL != nil {
		lesson.VideoURL = strings.TrimSpace(*input.VideoURL)
	}
	if len(input.Attachments) > 0 {
		attachments, err := buildLessonAttachments(lesson.ID, courseID, input.Attachments, s.now())
		if err != nil {
			return nil, err
		}
		lesson.Attachments = append(lesson.Attachments, attachments...)
	}
	lesson.UpdatedAt = s.now()

	if err := validateLesson(*lesson); err != nil {
		return nil, err
	}

	return s.repo.Update(ctx, *lesson)
}

func (s *LessonService) DeleteLesson(ctx context.Context, session auth.Session, courseID, lessonID string) error {
	if err := s.ensureLessonWriteAllowed(ctx, session, courseID); err != nil {
		return err
	}

	return s.repo.Delete(ctx, courseID, lessonID)
}

func (s *LessonService) ListLessons(ctx context.Context, session auth.Session, courseID string) ([]domain.Lesson, error) {
	if err := s.ensureLessonReadAllowed(ctx, session, courseID); err != nil {
		return nil, err
	}

	return s.repo.ListByCourse(ctx, courseID)
}

func (s *LessonService) GetLessonAttachment(ctx context.Context, session auth.Session, courseID, lessonID, attachmentID string) (*domain.Attachment, error) {
	if err := s.ensureLessonReadAllowed(ctx, session, courseID); err != nil {
		return nil, err
	}

	return s.repo.GetAttachment(ctx, courseID, lessonID, attachmentID)
}

func validateLesson(lesson domain.Lesson) error {
	if lesson.Title == "" {
		return ErrTitleRequired
	}
	if lesson.Description == "" {
		return ErrDescriptionRequired
	}
	if lesson.VideoURL == "" {
		return ErrVideoURLRequired
	}
	if lesson.OrderIndex <= 0 {
		return ErrInvalidOrderIndex
	}

	return nil
}

func buildLessonAttachments(lessonID, courseID string, inputs []AttachmentInput, now time.Time) ([]domain.Attachment, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	if len(inputs) > maxLessonAttachments {
		return nil, ErrTooManyAttachments
	}

	attachments := make([]domain.Attachment, 0, len(inputs))
	for _, input := range inputs {
		fileName := strings.TrimSpace(input.FileName)
		if fileName == "" {
			return nil, ErrAttachmentNameRequired
		}
		if len(input.Data) == 0 {
			return nil, ErrAttachmentDataRequired
		}
		if len(input.Data) > maxAttachmentSizeBytes {
			return nil, ErrAttachmentTooLarge
		}

		attachments = append(attachments, domain.Attachment{
			ID:          uuid.NewString(),
			LessonID:    lessonID,
			CourseID:    courseID,
			Kind:        normalizeAttachmentKind(input.ContentType),
			FileName:    fileName,
			ContentType: strings.TrimSpace(input.ContentType),
			SizeBytes:   int64(len(input.Data)),
			CreatedAt:   now,
			Data:        append([]byte(nil), input.Data...),
		})
	}

	return attachments, nil
}

func normalizeAttachmentKind(contentType string) string {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "image"
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	default:
		return "file"
	}
}

func (s *LessonService) ensureLessonReadAllowed(ctx context.Context, session auth.Session, courseID string) error {
	switch session.Claims.Role {
	case auth.RoleAdmin:
		return nil
	case auth.RoleTeacher:
		assigned, err := s.assignments.IsTeacherAssigned(ctx, courseID, session.Claims.UserID)
		if err != nil {
			return err
		}
		if !assigned {
			return ErrForbidden
		}
		return nil
	case auth.RoleStudent:
		enrolled, err := s.enrollmentChecker.IsStudentEnrolled(ctx, courseID, session.Claims.UserID)
		if err != nil {
			return err
		}
		if !enrolled {
			return ErrForbidden
		}
		return nil
	default:
		return ErrForbidden
	}
}

func (s *LessonService) ensureLessonWriteAllowed(ctx context.Context, session auth.Session, courseID string) error {
	switch session.Claims.Role {
	case auth.RoleAdmin:
		return nil
	case auth.RoleTeacher:
		assigned, err := s.assignments.IsTeacherAssigned(ctx, courseID, session.Claims.UserID)
		if err != nil {
			return err
		}
		if !assigned {
			return ErrForbidden
		}
		return nil
	default:
		return ErrForbidden
	}
}
