package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"proyecto-cursos/services/enrollments-api/internal/domain"
)

var (
	ErrCourseIDRequired         = errors.New("courseId is required")
	ErrCourseNotFound           = errors.New("course not found")
	ErrCourseNotPublished       = errors.New("course is not published")
	ErrEnrollmentNotFound       = errors.New("enrollment not found")
	ErrEnrollmentAlreadyExists  = errors.New("enrollment already exists")
	ErrCourseFull               = errors.New("course has no available seats")
	ErrPendingEnrollmentMissing = errors.New("pending enrollment not found")
	ErrEmailNotVerified         = errors.New("email not verified")
	ErrForbidden                = errors.New("forbidden")
)

type EnrollmentRepository interface {
	ReservePending(ctx context.Context, enrollment domain.Enrollment, capacity int) (*domain.Enrollment, error)
	ConfirmPending(ctx context.Context, userID, courseID string, capacity int) (*domain.Enrollment, error)
	CancelPending(ctx context.Context, userID, courseID string) error
	ListByUserStatuses(ctx context.Context, userID string, statuses []domain.Status) ([]domain.Enrollment, error)
	ListPaginated(ctx context.Context, limit, offset int) ([]domain.Enrollment, error)
	ListByCourseStatuses(ctx context.Context, courseID string, statuses []domain.Status) ([]domain.Enrollment, error)
	CountActiveByCourse(ctx context.Context, courseID string) (int, error)
	CountReservedByCourse(ctx context.Context, courseID string) (int, error)
	IsStudentEnrolled(ctx context.Context, courseID, studentID string) (bool, error)
	GetByUserCourse(ctx context.Context, userID, courseID string) (*domain.Enrollment, error)
	DeleteByCourse(ctx context.Context, courseID string) error
}

type CourseInfo struct {
	ID       string
	Title    string
	Category string
	ImageURL *string
	Price    float64
	Currency string
	Status   string
	Capacity int
}

type CoursesClient interface {
	GetCourse(ctx context.Context, courseID string) (*CourseInfo, error)
	IsTeacherAssigned(ctx context.Context, courseID, teacherID string) (bool, error)
}

type UsersClient interface {
	IsEmailVerified(ctx context.Context, userID string) (bool, error)
	GetUser(ctx context.Context, userID string) (*UserInfo, error)
}

type ReserveInput struct {
	CourseID string
}

type EnrollmentWithCourse struct {
	Enrollment domain.Enrollment
	Course     *CourseInfo
}

type UserInfo struct {
	ID    string
	Name  string
	Email string
	Role  string
}

type TeacherCourseEnrollmentView struct {
	StudentName string
	Status      domain.Status
	CreatedAt   time.Time
}

type TeacherCourseEnrollmentsView struct {
	CourseID    string
	CourseTitle string
	Enrollments []TeacherCourseEnrollmentView
}

type AdminEnrollmentView struct {
	StudentName string
	CourseTitle string
	Status      domain.Status
	CreatedAt   time.Time
}

type EnrollmentService struct {
	repo    EnrollmentRepository
	courses CoursesClient
	users   UsersClient
	now     func() time.Time
}

func NewEnrollmentService(repo EnrollmentRepository, courses CoursesClient, users UsersClient) *EnrollmentService {
	return &EnrollmentService{
		repo:    repo,
		courses: courses,
		users:   users,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *EnrollmentService) Reserve(ctx context.Context, studentID string, input ReserveInput) (*domain.Enrollment, error) {
	courseID := strings.TrimSpace(input.CourseID)
	if courseID == "" {
		return nil, ErrCourseIDRequired
	}

	if s.users != nil {
		emailVerified, err := s.users.IsEmailVerified(ctx, studentID)
		if err != nil {
			return nil, err
		}
		if !emailVerified {
			return nil, ErrEmailNotVerified
		}
	}

	course, err := s.courses.GetCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if course.Status != "published" {
		return nil, ErrCourseNotPublished
	}

	return s.repo.ReservePending(ctx, domain.Enrollment{
		ID:        uuid.NewString(),
		UserID:    studentID,
		CourseID:  courseID,
		Status:    domain.StatusPending,
		CreatedAt: s.now(),
	}, course.Capacity)
}

func (s *EnrollmentService) Confirm(ctx context.Context, userID, courseID string) (*domain.Enrollment, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(courseID) == "" {
		return nil, ErrCourseIDRequired
	}

	course, err := s.courses.GetCourse(ctx, strings.TrimSpace(courseID))
	if err != nil {
		return nil, err
	}

	return s.repo.ConfirmPending(ctx, strings.TrimSpace(userID), strings.TrimSpace(courseID), course.Capacity)
}

func (s *EnrollmentService) ConfirmPaidEnrollment(ctx context.Context, userID, courseID string) (*domain.Enrollment, error) {
	userID = strings.TrimSpace(userID)
	courseID = strings.TrimSpace(courseID)
	if userID == "" || courseID == "" {
		return nil, ErrCourseIDRequired
	}

	enrollment, err := s.repo.GetByUserCourse(ctx, userID, courseID)
	if err != nil {
		if errors.Is(err, ErrEnrollmentNotFound) {
			return nil, ErrPendingEnrollmentMissing
		}
		return nil, err
	}

	if enrollment.Status == domain.StatusActive {
		return enrollment, nil
	}
	if enrollment.Status != domain.StatusPending {
		return nil, ErrPendingEnrollmentMissing
	}

	course, err := s.courses.GetCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}

	return s.repo.ConfirmPending(ctx, userID, courseID, course.Capacity)
}

func (s *EnrollmentService) ListMyEnrollments(ctx context.Context, studentID string) ([]EnrollmentWithCourse, error) {
	enrollments, err := s.repo.ListByUserStatuses(ctx, studentID, []domain.Status{domain.StatusPending, domain.StatusActive})
	if err != nil {
		return nil, err
	}

	response := make([]EnrollmentWithCourse, 0, len(enrollments))
	for _, enrollment := range enrollments {
		course, err := s.courses.GetCourse(ctx, enrollment.CourseID)
		if err != nil {
			return nil, err
		}

		response = append(response, EnrollmentWithCourse{
			Enrollment: enrollment,
			Course:     course,
		})
	}

	return response, nil
}

func (s *EnrollmentService) ListAdminEnrollments(ctx context.Context, limit, offset int) ([]domain.Enrollment, error) {
	return s.repo.ListPaginated(ctx, limit, offset)
}

func (s *EnrollmentService) ListAdminEnrollmentsView(ctx context.Context, limit, offset int) ([]AdminEnrollmentView, error) {
	enrollments, err := s.ListAdminEnrollments(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	userNameCache := make(map[string]string)
	courseTitleCache := make(map[string]string)
	view := make([]AdminEnrollmentView, 0, len(enrollments))

	for _, enrollment := range enrollments {
		studentName := "Alumno sin nombre"
		if s.users != nil {
			if cached, ok := userNameCache[enrollment.UserID]; ok {
				studentName = cached
			} else {
				user, err := s.users.GetUser(ctx, enrollment.UserID)
				if err != nil {
					return nil, err
				}
				if user != nil && strings.TrimSpace(user.Name) != "" {
					studentName = strings.TrimSpace(user.Name)
				}
				userNameCache[enrollment.UserID] = studentName
			}
		}

		courseTitle := "Curso sin titulo"
		if cached, ok := courseTitleCache[enrollment.CourseID]; ok {
			courseTitle = cached
		} else {
			course, err := s.courses.GetCourse(ctx, enrollment.CourseID)
			if err != nil {
				return nil, err
			}
			if course != nil && strings.TrimSpace(course.Title) != "" {
				courseTitle = strings.TrimSpace(course.Title)
			}
			courseTitleCache[enrollment.CourseID] = courseTitle
		}

		view = append(view, AdminEnrollmentView{
			StudentName: studentName,
			CourseTitle: courseTitle,
			Status:      enrollment.Status,
			CreatedAt:   enrollment.CreatedAt,
		})
	}

	return view, nil
}

func (s *EnrollmentService) ListTeacherCourseEnrollments(ctx context.Context, teacherID, courseID string) ([]domain.Enrollment, error) {
	assigned, err := s.courses.IsTeacherAssigned(ctx, courseID, teacherID)
	if err != nil {
		return nil, err
	}
	if !assigned {
		return nil, ErrForbidden
	}

	return s.repo.ListByCourseStatuses(ctx, courseID, []domain.Status{domain.StatusPending, domain.StatusActive})
}

func (s *EnrollmentService) ListTeacherCourseEnrollmentsView(ctx context.Context, teacherID, courseID string) (*TeacherCourseEnrollmentsView, error) {
	enrollments, err := s.ListTeacherCourseEnrollments(ctx, teacherID, courseID)
	if err != nil {
		return nil, err
	}

	course, err := s.courses.GetCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}

	view := &TeacherCourseEnrollmentsView{
		CourseID:    courseID,
		CourseTitle: strings.TrimSpace(course.Title),
		Enrollments: make([]TeacherCourseEnrollmentView, 0, len(enrollments)),
	}

	for _, enrollment := range enrollments {
		studentName := "Alumno sin nombre"
		if s.users != nil {
			user, err := s.users.GetUser(ctx, enrollment.UserID)
			if err != nil {
				return nil, err
			}
			if user != nil && strings.TrimSpace(user.Name) != "" {
				studentName = strings.TrimSpace(user.Name)
			}
		}

		view.Enrollments = append(view.Enrollments, TeacherCourseEnrollmentView{
			StudentName: studentName,
			Status:      enrollment.Status,
			CreatedAt:   enrollment.CreatedAt,
		})
	}

	return view, nil
}

func (s *EnrollmentService) ListActiveStudentIDs(ctx context.Context, courseID string) ([]string, error) {
	courseID = strings.TrimSpace(courseID)
	if courseID == "" {
		return nil, ErrCourseIDRequired
	}

	enrollments, err := s.repo.ListByCourseStatuses(ctx, courseID, []domain.Status{domain.StatusActive})
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(enrollments))
	studentIDs := make([]string, 0, len(enrollments))
	for _, enrollment := range enrollments {
		userID := strings.TrimSpace(enrollment.UserID)
		if userID == "" {
			continue
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		studentIDs = append(studentIDs, userID)
	}

	return studentIDs, nil
}

func (s *EnrollmentService) GetAvailability(ctx context.Context, courseID string) (*CourseAvailability, error) {
	courseID = strings.TrimSpace(courseID)
	if courseID == "" {
		return nil, ErrCourseIDRequired
	}

	course, err := s.courses.GetCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}

	activeCount, err := s.repo.CountActiveByCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}

	reservedCount, err := s.repo.CountReservedByCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}

	available := course.Capacity - reservedCount
	if available < 0 {
		available = 0
	}

	return &CourseAvailability{
		CourseID:    courseID,
		Capacity:    course.Capacity,
		ActiveCount: activeCount,
		Available:   available,
	}, nil
}

func (s *EnrollmentService) CancelPendingEnrollment(ctx context.Context, userID, courseID string) error {
	userID = strings.TrimSpace(userID)
	courseID = strings.TrimSpace(courseID)
	if userID == "" || courseID == "" {
		return ErrCourseIDRequired
	}

	return s.repo.CancelPending(ctx, userID, courseID)
}

func (s *EnrollmentService) IsStudentEnrolled(ctx context.Context, courseID, studentID string) (bool, error) {
	return s.repo.IsStudentEnrolled(ctx, courseID, studentID)
}

func (s *EnrollmentService) HasPendingEnrollment(ctx context.Context, userID, courseID string) (bool, error) {
	enrollment, err := s.repo.GetByUserCourse(ctx, strings.TrimSpace(userID), strings.TrimSpace(courseID))
	if err != nil {
		if errors.Is(err, ErrEnrollmentNotFound) {
			return false, nil
		}
		return false, err
	}

	return enrollment.Status == domain.StatusPending, nil
}

func (s *EnrollmentService) DeleteCourseEnrollments(ctx context.Context, courseID string) error {
	courseID = strings.TrimSpace(courseID)
	if courseID == "" {
		return ErrCourseIDRequired
	}

	return s.repo.DeleteByCourse(ctx, courseID)
}

type CourseAvailability struct {
	CourseID    string
	Capacity    int
	ActiveCount int
	Available   int
}
