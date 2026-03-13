package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"proyecto-cursos/internal/platform/auth"
	"proyecto-cursos/services/courses-api/internal/domain"
)

type mockCourseRepository struct {
	createFn                    func(ctx context.Context, course domain.Course) (*domain.Course, error)
	getByIDFn                   func(ctx context.Context, courseID string) (*domain.Course, error)
	listPublishedFn             func(ctx context.Context, limit, offset int) ([]domain.Course, error)
	listRecommendationCatalogFn func(ctx context.Context) ([]RecommendationCatalogItem, error)
	updateFn                    func(ctx context.Context, course domain.Course) (*domain.Course, error)
	deleteFn                    func(ctx context.Context, courseID string) error
	assignTeacherFn             func(ctx context.Context, courseID, teacherID string) error
	listTeachersFn              func(ctx context.Context, courseID string) ([]string, error)
	removeTeacherFn             func(ctx context.Context, courseID, teacherID string) error
	listByTeacherFn             func(ctx context.Context, teacherID string) ([]domain.Course, error)
	isTeacherAssignedFn         func(ctx context.Context, courseID, teacherID string) (bool, error)
}

func (m *mockCourseRepository) Create(ctx context.Context, course domain.Course) (*domain.Course, error) {
	return m.createFn(ctx, course)
}

func (m *mockCourseRepository) GetByID(ctx context.Context, courseID string) (*domain.Course, error) {
	return m.getByIDFn(ctx, courseID)
}

func (m *mockCourseRepository) ListPublished(ctx context.Context, limit, offset int) ([]domain.Course, error) {
	return m.listPublishedFn(ctx, limit, offset)
}

func (m *mockCourseRepository) ListRecommendationCatalog(ctx context.Context) ([]RecommendationCatalogItem, error) {
	if m.listRecommendationCatalogFn == nil {
		return nil, nil
	}

	return m.listRecommendationCatalogFn(ctx)
}

func (m *mockCourseRepository) Update(ctx context.Context, course domain.Course) (*domain.Course, error) {
	return m.updateFn(ctx, course)
}

func (m *mockCourseRepository) Delete(ctx context.Context, courseID string) error {
	return m.deleteFn(ctx, courseID)
}

func (m *mockCourseRepository) AssignTeacher(ctx context.Context, courseID, teacherID string) error {
	return m.assignTeacherFn(ctx, courseID, teacherID)
}

func (m *mockCourseRepository) ListTeachers(ctx context.Context, courseID string) ([]string, error) {
	return m.listTeachersFn(ctx, courseID)
}

func (m *mockCourseRepository) RemoveTeacher(ctx context.Context, courseID, teacherID string) error {
	return m.removeTeacherFn(ctx, courseID, teacherID)
}

func (m *mockCourseRepository) ListByTeacher(ctx context.Context, teacherID string) ([]domain.Course, error) {
	return m.listByTeacherFn(ctx, teacherID)
}

func (m *mockCourseRepository) IsTeacherAssigned(ctx context.Context, courseID, teacherID string) (bool, error) {
	return m.isTeacherAssignedFn(ctx, courseID, teacherID)
}

type fakeTeacherVerifier struct {
	getTeacherFn func(ctx context.Context, teacherID string) (*TeacherInfo, error)
}

func (f fakeTeacherVerifier) GetTeacher(ctx context.Context, teacherID string) (*TeacherInfo, error) {
	return f.getTeacherFn(ctx, teacherID)
}

type fakeEnrollmentCleaner struct {
	deleteCourseEnrollmentsFn func(ctx context.Context, courseID string) error
}

func (f fakeEnrollmentCleaner) DeleteCourseEnrollments(ctx context.Context, courseID string) error {
	if f.deleteCourseEnrollmentsFn == nil {
		return nil
	}

	return f.deleteCourseEnrollmentsFn(ctx, courseID)
}

func TestCreateCourse(t *testing.T) {
	t.Parallel()

	var created domain.Course
	svc := NewCourseService(
		&mockCourseRepository{
			createFn: func(_ context.Context, course domain.Course) (*domain.Course, error) {
				created = course
				return &course, nil
			},
			getByIDFn:           func(context.Context, string) (*domain.Course, error) { return nil, nil },
			listPublishedFn:     func(context.Context, int, int) ([]domain.Course, error) { return nil, nil },
			updateFn:            func(context.Context, domain.Course) (*domain.Course, error) { return nil, nil },
			deleteFn:            func(context.Context, string) error { return nil },
			assignTeacherFn:     func(context.Context, string, string) error { return nil },
			listTeachersFn:      func(context.Context, string) ([]string, error) { return nil, nil },
			removeTeacherFn:     func(context.Context, string, string) error { return nil },
			listByTeacherFn:     func(context.Context, string) ([]domain.Course, error) { return nil, nil },
			isTeacherAssignedFn: func(context.Context, string, string) (bool, error) { return false, nil },
		},
		fakeTeacherVerifier{getTeacherFn: func(context.Context, string) (*TeacherInfo, error) { return nil, nil }},
		fakeEnrollmentCleaner{},
	)
	frozenNow := time.Date(2026, time.February, 28, 18, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return frozenNow }

	course, err := svc.CreateCourse(context.Background(), "admin-1", CreateCourseInput{
		Title:       "Go Avanzado",
		Description: "Testing y concurrencia",
		Category:    "backend",
		Price:       9999.50,
		Currency:    "ars",
		Capacity:    25,
		Status:      domain.StatusDraft,
	})

	require.NoError(t, err)
	require.NotEmpty(t, course.ID)
	require.Equal(t, "admin-1", created.CreatedBy)
	require.Equal(t, "ARS", created.Currency)
	require.Equal(t, frozenNow, created.CreatedAt)
}

func TestPublishUnpublishCourse(t *testing.T) {
	t.Parallel()

	current := &domain.Course{
		ID:          "course-1",
		Title:       "Go",
		Description: "Desc",
		Category:    "backend",
		Price:       10,
		Currency:    "USD",
		Capacity:    10,
		Status:      domain.StatusDraft,
		CreatedBy:   "admin-1",
	}
	svc := NewCourseService(
		&mockCourseRepository{
			createFn: func(context.Context, domain.Course) (*domain.Course, error) { return nil, nil },
			getByIDFn: func(context.Context, string) (*domain.Course, error) {
				copied := *current
				return &copied, nil
			},
			listPublishedFn: func(context.Context, int, int) ([]domain.Course, error) { return nil, nil },
			updateFn: func(_ context.Context, course domain.Course) (*domain.Course, error) {
				current = &course
				return &course, nil
			},
			deleteFn:            func(context.Context, string) error { return nil },
			assignTeacherFn:     func(context.Context, string, string) error { return nil },
			listTeachersFn:      func(context.Context, string) ([]string, error) { return nil, nil },
			removeTeacherFn:     func(context.Context, string, string) error { return nil },
			listByTeacherFn:     func(context.Context, string) ([]domain.Course, error) { return nil, nil },
			isTeacherAssignedFn: func(context.Context, string, string) (bool, error) { return false, nil },
		},
		fakeTeacherVerifier{getTeacherFn: func(context.Context, string) (*TeacherInfo, error) { return nil, nil }},
		fakeEnrollmentCleaner{},
	)

	published := domain.StatusPublished
	updated, err := svc.UpdateCourse(context.Background(), "course-1", UpdateCourseInput{
		Status: &published,
	})
	require.NoError(t, err)
	require.Equal(t, domain.StatusPublished, updated.Status)

	draft := domain.StatusDraft
	updated, err = svc.UpdateCourse(context.Background(), "course-1", UpdateCourseInput{
		Status: &draft,
	})
	require.NoError(t, err)
	require.Equal(t, domain.StatusDraft, updated.Status)
}

func TestAssignTeacherAvoidsDuplicates(t *testing.T) {
	t.Parallel()

	svc := NewCourseService(
		&mockCourseRepository{
			createFn: func(context.Context, domain.Course) (*domain.Course, error) { return nil, nil },
			getByIDFn: func(context.Context, string) (*domain.Course, error) {
				return &domain.Course{ID: "course-1", Status: domain.StatusDraft}, nil
			},
			listPublishedFn: func(context.Context, int, int) ([]domain.Course, error) { return nil, nil },
			updateFn:        func(context.Context, domain.Course) (*domain.Course, error) { return nil, nil },
			deleteFn:        func(context.Context, string) error { return nil },
			assignTeacherFn: func(context.Context, string, string) error { return ErrTeacherAlreadyAssigned },
			listTeachersFn:  func(context.Context, string) ([]string, error) { return nil, nil },
			removeTeacherFn: func(context.Context, string, string) error { return nil },
			listByTeacherFn: func(context.Context, string) ([]domain.Course, error) { return nil, nil },
			isTeacherAssignedFn: func(context.Context, string, string) (bool, error) {
				return false, nil
			},
		},
		fakeTeacherVerifier{
			getTeacherFn: func(context.Context, string) (*TeacherInfo, error) {
				return &TeacherInfo{ID: "teacher-1", Role: auth.RoleTeacher}, nil
			},
		},
		fakeEnrollmentCleaner{},
	)

	err := svc.AssignTeacher(context.Background(), "course-1", "teacher-1")
	require.ErrorIs(t, err, ErrTeacherAlreadyAssigned)
}

func TestListPublished(t *testing.T) {
	t.Parallel()

	expected := []domain.Course{
		{ID: "published-1", Status: domain.StatusPublished},
		{ID: "published-2", Status: domain.StatusPublished},
	}
	svc := NewCourseService(
		&mockCourseRepository{
			createFn:        func(context.Context, domain.Course) (*domain.Course, error) { return nil, nil },
			getByIDFn:       func(context.Context, string) (*domain.Course, error) { return nil, nil },
			listPublishedFn: func(context.Context, int, int) ([]domain.Course, error) { return expected, nil },
			updateFn:        func(context.Context, domain.Course) (*domain.Course, error) { return nil, nil },
			deleteFn:        func(context.Context, string) error { return nil },
			assignTeacherFn: func(context.Context, string, string) error { return nil },
			listTeachersFn:  func(context.Context, string) ([]string, error) { return nil, nil },
			removeTeacherFn: func(context.Context, string, string) error { return nil },
			listByTeacherFn: func(context.Context, string) ([]domain.Course, error) { return nil, nil },
			isTeacherAssignedFn: func(context.Context, string, string) (bool, error) {
				return false, nil
			},
		},
		fakeTeacherVerifier{getTeacherFn: func(context.Context, string) (*TeacherInfo, error) { return nil, nil }},
		fakeEnrollmentCleaner{},
	)

	courses, err := svc.ListPublished(context.Background(), 20, 0)
	require.NoError(t, err)
	require.Equal(t, expected, courses)
}

func TestDeleteCourseDeletesEnrollments(t *testing.T) {
	t.Parallel()

	var cleanedCourseID string
	var deletedCourseID string

	svc := NewCourseService(
		&mockCourseRepository{
			createFn: func(context.Context, domain.Course) (*domain.Course, error) { return nil, nil },
			getByIDFn: func(context.Context, string) (*domain.Course, error) {
				return &domain.Course{ID: "course-1"}, nil
			},
			listPublishedFn: func(context.Context, int, int) ([]domain.Course, error) { return nil, nil },
			updateFn:        func(context.Context, domain.Course) (*domain.Course, error) { return nil, nil },
			deleteFn: func(_ context.Context, courseID string) error {
				deletedCourseID = courseID
				require.Equal(t, "course-1", cleanedCourseID)
				return nil
			},
			assignTeacherFn:     func(context.Context, string, string) error { return nil },
			listTeachersFn:      func(context.Context, string) ([]string, error) { return nil, nil },
			removeTeacherFn:     func(context.Context, string, string) error { return nil },
			listByTeacherFn:     func(context.Context, string) ([]domain.Course, error) { return nil, nil },
			isTeacherAssignedFn: func(context.Context, string, string) (bool, error) { return false, nil },
		},
		fakeTeacherVerifier{getTeacherFn: func(context.Context, string) (*TeacherInfo, error) { return nil, nil }},
		fakeEnrollmentCleaner{
			deleteCourseEnrollmentsFn: func(_ context.Context, courseID string) error {
				cleanedCourseID = courseID
				return nil
			},
		},
	)

	err := svc.DeleteCourse(context.Background(), "course-1")
	require.NoError(t, err)
	require.Equal(t, "course-1", deletedCourseID)
}

func TestDeleteCourseReturnsCleanupError(t *testing.T) {
	t.Parallel()

	svc := NewCourseService(
		&mockCourseRepository{
			createFn: func(context.Context, domain.Course) (*domain.Course, error) { return nil, nil },
			getByIDFn: func(context.Context, string) (*domain.Course, error) {
				return &domain.Course{ID: "course-1"}, nil
			},
			listPublishedFn:     func(context.Context, int, int) ([]domain.Course, error) { return nil, nil },
			updateFn:            func(context.Context, domain.Course) (*domain.Course, error) { return nil, nil },
			deleteFn:            func(context.Context, string) error { t.Fatal("delete should not run after cleanup error"); return nil },
			assignTeacherFn:     func(context.Context, string, string) error { return nil },
			listTeachersFn:      func(context.Context, string) ([]string, error) { return nil, nil },
			removeTeacherFn:     func(context.Context, string, string) error { return nil },
			listByTeacherFn:     func(context.Context, string) ([]domain.Course, error) { return nil, nil },
			isTeacherAssignedFn: func(context.Context, string, string) (bool, error) { return false, nil },
		},
		fakeTeacherVerifier{getTeacherFn: func(context.Context, string) (*TeacherInfo, error) { return nil, nil }},
		fakeEnrollmentCleaner{
			deleteCourseEnrollmentsFn: func(context.Context, string) error { return context.DeadlineExceeded },
		},
	)

	err := svc.DeleteCourse(context.Background(), "course-1")
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
