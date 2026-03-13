package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"proyecto-cursos/services/enrollments-api/internal/domain"
)

type mockEnrollmentRepository struct {
	reservePendingFn    func(ctx context.Context, enrollment domain.Enrollment, capacity int) (*domain.Enrollment, error)
	confirmPendingFn    func(ctx context.Context, userID, courseID string) (*domain.Enrollment, error)
	cancelPendingFn     func(ctx context.Context, userID, courseID string) error
	listByUserFn        func(ctx context.Context, userID string, statuses []domain.Status) ([]domain.Enrollment, error)
	listPaginatedFn     func(ctx context.Context, limit, offset int) ([]domain.Enrollment, error)
	listByCourseFn      func(ctx context.Context, courseID string, statuses []domain.Status) ([]domain.Enrollment, error)
	countActiveByCourse func(ctx context.Context, courseID string) (int, error)
	countReservedByCourse func(ctx context.Context, courseID string) (int, error)
	isStudentEnrolledFn func(ctx context.Context, courseID, studentID string) (bool, error)
	getByUserCourseFn   func(ctx context.Context, userID, courseID string) (*domain.Enrollment, error)
	deleteByCourseFn    func(ctx context.Context, courseID string) error
}

func (m *mockEnrollmentRepository) ReservePending(ctx context.Context, enrollment domain.Enrollment, capacity int) (*domain.Enrollment, error) {
	return m.reservePendingFn(ctx, enrollment, capacity)
}

func (m *mockEnrollmentRepository) ConfirmPending(ctx context.Context, userID, courseID string, _ int) (*domain.Enrollment, error) {
	return m.confirmPendingFn(ctx, userID, courseID)
}

func (m *mockEnrollmentRepository) CancelPending(ctx context.Context, userID, courseID string) error {
	if m.cancelPendingFn == nil {
		return nil
	}
	return m.cancelPendingFn(ctx, userID, courseID)
}

func (m *mockEnrollmentRepository) ListByUserStatuses(ctx context.Context, userID string, statuses []domain.Status) ([]domain.Enrollment, error) {
	return m.listByUserFn(ctx, userID, statuses)
}

func (m *mockEnrollmentRepository) ListPaginated(ctx context.Context, limit, offset int) ([]domain.Enrollment, error) {
	return m.listPaginatedFn(ctx, limit, offset)
}

func (m *mockEnrollmentRepository) ListByCourseStatuses(ctx context.Context, courseID string, statuses []domain.Status) ([]domain.Enrollment, error) {
	return m.listByCourseFn(ctx, courseID, statuses)
}

func (m *mockEnrollmentRepository) CountActiveByCourse(ctx context.Context, courseID string) (int, error) {
	return m.countActiveByCourse(ctx, courseID)
}

func (m *mockEnrollmentRepository) CountReservedByCourse(ctx context.Context, courseID string) (int, error) {
	if m.countReservedByCourse != nil {
		return m.countReservedByCourse(ctx, courseID)
	}
	if m.countActiveByCourse != nil {
		return m.countActiveByCourse(ctx, courseID)
	}
	return 0, nil
}

func (m *mockEnrollmentRepository) IsStudentEnrolled(ctx context.Context, courseID, studentID string) (bool, error) {
	return m.isStudentEnrolledFn(ctx, courseID, studentID)
}

func (m *mockEnrollmentRepository) GetByUserCourse(ctx context.Context, userID, courseID string) (*domain.Enrollment, error) {
	return m.getByUserCourseFn(ctx, userID, courseID)
}

func (m *mockEnrollmentRepository) DeleteByCourse(ctx context.Context, courseID string) error {
	if m.deleteByCourseFn == nil {
		return nil
	}

	return m.deleteByCourseFn(ctx, courseID)
}

type fakeCoursesClient struct {
	getCourseFn         func(ctx context.Context, courseID string) (*CourseInfo, error)
	isTeacherAssignedFn func(ctx context.Context, courseID, teacherID string) (bool, error)
}

func (f fakeCoursesClient) GetCourse(ctx context.Context, courseID string) (*CourseInfo, error) {
	return f.getCourseFn(ctx, courseID)
}

func (f fakeCoursesClient) IsTeacherAssigned(ctx context.Context, courseID, teacherID string) (bool, error) {
	return f.isTeacherAssignedFn(ctx, courseID, teacherID)
}

type fakeUsersClient struct {
	isEmailVerifiedFn func(ctx context.Context, userID string) (bool, error)
	getUserFn         func(ctx context.Context, userID string) (*UserInfo, error)
}

func (f fakeUsersClient) IsEmailVerified(ctx context.Context, userID string) (bool, error) {
	return f.isEmailVerifiedFn(ctx, userID)
}

func (f fakeUsersClient) GetUser(ctx context.Context, userID string) (*UserInfo, error) {
	if f.getUserFn == nil {
		return &UserInfo{ID: userID}, nil
	}

	return f.getUserFn(ctx, userID)
}

func TestReserveWithCapacityCreatesPending(t *testing.T) {
	t.Parallel()

	var reserved domain.Enrollment
	svc := NewEnrollmentService(
		&mockEnrollmentRepository{
			reservePendingFn: func(_ context.Context, enrollment domain.Enrollment, capacity int) (*domain.Enrollment, error) {
				require.Equal(t, 20, capacity)
				reserved = enrollment
				return &enrollment, nil
			},
			confirmPendingFn: func(context.Context, string, string) (*domain.Enrollment, error) { return nil, nil },
			listByUserFn:     func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) { return nil, nil },
			listPaginatedFn:  func(context.Context, int, int) ([]domain.Enrollment, error) { return nil, nil },
			listByCourseFn:   func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) { return nil, nil },
			countActiveByCourse: func(context.Context, string) (int, error) {
				return 0, nil
			},
			isStudentEnrolledFn: func(context.Context, string, string) (bool, error) { return false, nil },
			getByUserCourseFn:   func(context.Context, string, string) (*domain.Enrollment, error) { return nil, ErrEnrollmentNotFound },
		},
		fakeCoursesClient{
			getCourseFn: func(context.Context, string) (*CourseInfo, error) {
				return &CourseInfo{ID: "course-1", Status: "published", Capacity: 20}, nil
			},
			isTeacherAssignedFn: func(context.Context, string, string) (bool, error) { return false, nil },
		},
		fakeUsersClient{
			isEmailVerifiedFn: func(context.Context, string) (bool, error) { return true, nil },
		},
	)
	frozenNow := time.Date(2026, time.February, 28, 20, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return frozenNow }

	enrollment, err := svc.Reserve(context.Background(), "student-1", ReserveInput{CourseID: "course-1"})
	require.NoError(t, err)
	require.NotEmpty(t, enrollment.ID)
	require.Equal(t, domain.StatusPending, reserved.Status)
	require.Equal(t, frozenNow, reserved.CreatedAt)
}

func TestReserveWithoutCapacityReturnsConflict(t *testing.T) {
	t.Parallel()

	svc := NewEnrollmentService(
		&mockEnrollmentRepository{
			reservePendingFn: func(context.Context, domain.Enrollment, int) (*domain.Enrollment, error) {
				return nil, ErrCourseFull
			},
			confirmPendingFn: func(context.Context, string, string) (*domain.Enrollment, error) { return nil, nil },
			listByUserFn:     func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) { return nil, nil },
			listPaginatedFn:  func(context.Context, int, int) ([]domain.Enrollment, error) { return nil, nil },
			listByCourseFn:   func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) { return nil, nil },
			countActiveByCourse: func(context.Context, string) (int, error) {
				return 0, nil
			},
			isStudentEnrolledFn: func(context.Context, string, string) (bool, error) { return false, nil },
			getByUserCourseFn:   func(context.Context, string, string) (*domain.Enrollment, error) { return nil, ErrEnrollmentNotFound },
		},
		fakeCoursesClient{
			getCourseFn: func(context.Context, string) (*CourseInfo, error) {
				return &CourseInfo{ID: "course-1", Status: "published", Capacity: 1}, nil
			},
			isTeacherAssignedFn: func(context.Context, string, string) (bool, error) { return false, nil },
		},
		fakeUsersClient{
			isEmailVerifiedFn: func(context.Context, string) (bool, error) { return true, nil },
		},
	)

	_, err := svc.Reserve(context.Background(), "student-1", ReserveInput{CourseID: "course-1"})
	require.ErrorIs(t, err, ErrCourseFull)
}

func TestReserveBlockedWhenEmailNotVerified(t *testing.T) {
	t.Parallel()

	svc := NewEnrollmentService(
		&mockEnrollmentRepository{
			reservePendingFn: func(context.Context, domain.Enrollment, int) (*domain.Enrollment, error) {
				t.Fatal("reserve should not be called when email is not verified")
				return nil, nil
			},
			confirmPendingFn:    func(context.Context, string, string) (*domain.Enrollment, error) { return nil, nil },
			listByUserFn:        func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) { return nil, nil },
			listPaginatedFn:     func(context.Context, int, int) ([]domain.Enrollment, error) { return nil, nil },
			listByCourseFn:      func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) { return nil, nil },
			countActiveByCourse: func(context.Context, string) (int, error) { return 0, nil },
			isStudentEnrolledFn: func(context.Context, string, string) (bool, error) { return false, nil },
			getByUserCourseFn:   func(context.Context, string, string) (*domain.Enrollment, error) { return nil, ErrEnrollmentNotFound },
		},
		fakeCoursesClient{
			getCourseFn: func(context.Context, string) (*CourseInfo, error) {
				return &CourseInfo{ID: "course-1", Status: "published", Capacity: 1}, nil
			},
			isTeacherAssignedFn: func(context.Context, string, string) (bool, error) { return false, nil },
		},
		fakeUsersClient{
			isEmailVerifiedFn: func(context.Context, string) (bool, error) { return false, nil },
		},
	)

	_, err := svc.Reserve(context.Background(), "student-1", ReserveInput{CourseID: "course-1"})

	require.ErrorIs(t, err, ErrEmailNotVerified)
}

func TestDuplicateEnrollmentReturnsConflict(t *testing.T) {
	t.Parallel()

	svc := NewEnrollmentService(
		&mockEnrollmentRepository{
			reservePendingFn: func(context.Context, domain.Enrollment, int) (*domain.Enrollment, error) {
				return nil, ErrEnrollmentAlreadyExists
			},
			confirmPendingFn: func(context.Context, string, string) (*domain.Enrollment, error) { return nil, nil },
			listByUserFn:     func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) { return nil, nil },
			listPaginatedFn:  func(context.Context, int, int) ([]domain.Enrollment, error) { return nil, nil },
			listByCourseFn:   func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) { return nil, nil },
			countActiveByCourse: func(context.Context, string) (int, error) {
				return 0, nil
			},
			isStudentEnrolledFn: func(context.Context, string, string) (bool, error) { return false, nil },
			getByUserCourseFn:   func(context.Context, string, string) (*domain.Enrollment, error) { return nil, ErrEnrollmentNotFound },
		},
		fakeCoursesClient{
			getCourseFn: func(context.Context, string) (*CourseInfo, error) {
				return &CourseInfo{ID: "course-1", Status: "published", Capacity: 10}, nil
			},
			isTeacherAssignedFn: func(context.Context, string, string) (bool, error) { return false, nil },
		},
		fakeUsersClient{
			isEmailVerifiedFn: func(context.Context, string) (bool, error) { return true, nil },
		},
	)

	_, err := svc.Reserve(context.Background(), "student-1", ReserveInput{CourseID: "course-1"})
	require.ErrorIs(t, err, ErrEnrollmentAlreadyExists)
}

func TestConfirmPendingToActive(t *testing.T) {
	t.Parallel()

	svc := NewEnrollmentService(
		&mockEnrollmentRepository{
			reservePendingFn: func(context.Context, domain.Enrollment, int) (*domain.Enrollment, error) { return nil, nil },
			confirmPendingFn: func(context.Context, string, string) (*domain.Enrollment, error) {
				return &domain.Enrollment{
					UserID:   "student-1",
					CourseID: "course-1",
					Status:   domain.StatusActive,
				}, nil
			},
			listByUserFn:        func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) { return nil, nil },
			listPaginatedFn:     func(context.Context, int, int) ([]domain.Enrollment, error) { return nil, nil },
			listByCourseFn:      func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) { return nil, nil },
			countActiveByCourse: func(context.Context, string) (int, error) { return 0, nil },
			isStudentEnrolledFn: func(context.Context, string, string) (bool, error) { return false, nil },
			getByUserCourseFn:   func(context.Context, string, string) (*domain.Enrollment, error) { return nil, ErrEnrollmentNotFound },
		},
		fakeCoursesClient{
			getCourseFn:         func(context.Context, string) (*CourseInfo, error) { return &CourseInfo{ID: "course-1", Capacity: 20}, nil },
			isTeacherAssignedFn: func(context.Context, string, string) (bool, error) { return false, nil },
		},
		fakeUsersClient{
			isEmailVerifiedFn: func(context.Context, string) (bool, error) { return true, nil },
		},
	)

	enrollment, err := svc.Confirm(context.Background(), "student-1", "course-1")
	require.NoError(t, err)
	require.Equal(t, domain.StatusActive, enrollment.Status)
}

func TestListByRole(t *testing.T) {
	t.Parallel()

	svc := NewEnrollmentService(
		&mockEnrollmentRepository{
			reservePendingFn: func(context.Context, domain.Enrollment, int) (*domain.Enrollment, error) { return nil, nil },
			confirmPendingFn: func(context.Context, string, string) (*domain.Enrollment, error) { return nil, nil },
			listByUserFn: func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) {
				return []domain.Enrollment{{ID: "enr-1", CourseID: "course-1", Status: domain.StatusActive}}, nil
			},
			listPaginatedFn: func(context.Context, int, int) ([]domain.Enrollment, error) {
				return []domain.Enrollment{{ID: "enr-2", CourseID: "course-2", Status: domain.StatusPending}}, nil
			},
			listByCourseFn: func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) {
				return []domain.Enrollment{{ID: "enr-3", CourseID: "course-1", Status: domain.StatusActive}}, nil
			},
			countActiveByCourse: func(context.Context, string) (int, error) { return 1, nil },
			isStudentEnrolledFn: func(context.Context, string, string) (bool, error) { return true, nil },
			getByUserCourseFn: func(context.Context, string, string) (*domain.Enrollment, error) {
				return &domain.Enrollment{ID: "enr-1", UserID: "student-1", CourseID: "course-1", Status: domain.StatusPending}, nil
			},
		},
		fakeCoursesClient{
			getCourseFn: func(context.Context, string) (*CourseInfo, error) {
				return &CourseInfo{ID: "course-1", Title: "Go", Category: "backend", Status: "published", Capacity: 10}, nil
			},
			isTeacherAssignedFn: func(_ context.Context, courseID, teacherID string) (bool, error) {
				return teacherID == "teacher-1", nil
			},
		},
		fakeUsersClient{
			isEmailVerifiedFn: func(context.Context, string) (bool, error) { return true, nil },
		},
	)

	me, err := svc.ListMyEnrollments(context.Background(), "student-1")
	require.NoError(t, err)
	require.Len(t, me, 1)
	require.Equal(t, "Go", me[0].Course.Title)

	adminList, err := svc.ListAdminEnrollments(context.Background(), 20, 0)
	require.NoError(t, err)
	require.Len(t, adminList, 1)

	teacherList, err := svc.ListTeacherCourseEnrollments(context.Background(), "teacher-1", "course-1")
	require.NoError(t, err)
	require.Len(t, teacherList, 1)

	_, err = svc.ListTeacherCourseEnrollments(context.Background(), "teacher-2", "course-1")
	require.ErrorIs(t, err, ErrForbidden)
}

func TestListTeacherCourseEnrollmentsViewIncludesCourseAndStudentNames(t *testing.T) {
	t.Parallel()

	svc := NewEnrollmentService(
		&mockEnrollmentRepository{
			reservePendingFn: func(context.Context, domain.Enrollment, int) (*domain.Enrollment, error) { return nil, nil },
			confirmPendingFn: func(context.Context, string, string) (*domain.Enrollment, error) { return nil, nil },
			listByUserFn:     func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) { return nil, nil },
			listPaginatedFn:  func(context.Context, int, int) ([]domain.Enrollment, error) { return nil, nil },
			listByCourseFn: func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) {
				return []domain.Enrollment{
					{ID: "enr-1", UserID: "student-1", CourseID: "course-1", Status: domain.StatusActive, CreatedAt: time.Date(2026, time.March, 11, 21, 37, 43, 0, time.UTC)},
					{ID: "enr-2", UserID: "student-2", CourseID: "course-1", Status: domain.StatusPending, CreatedAt: time.Date(2026, time.March, 12, 10, 0, 0, 0, time.UTC)},
				}, nil
			},
			countActiveByCourse: func(context.Context, string) (int, error) { return 0, nil },
			isStudentEnrolledFn: func(context.Context, string, string) (bool, error) { return false, nil },
			getByUserCourseFn:   func(context.Context, string, string) (*domain.Enrollment, error) { return nil, ErrEnrollmentNotFound },
		},
		fakeCoursesClient{
			getCourseFn: func(context.Context, string) (*CourseInfo, error) {
				return &CourseInfo{ID: "course-1", Title: "Backend con Go", Status: "published", Capacity: 10}, nil
			},
			isTeacherAssignedFn: func(_ context.Context, courseID, teacherID string) (bool, error) {
				return teacherID == "teacher-1" && courseID == "course-1", nil
			},
		},
		fakeUsersClient{
			isEmailVerifiedFn: func(context.Context, string) (bool, error) { return true, nil },
			getUserFn: func(_ context.Context, userID string) (*UserInfo, error) {
				switch userID {
				case "student-1":
					return &UserInfo{ID: userID, Name: "Ana Perez"}, nil
				case "student-2":
					return &UserInfo{ID: userID, Name: "Jose Ruarte"}, nil
				default:
					return &UserInfo{ID: userID, Name: "Alumno"}, nil
				}
			},
		},
	)

	view, err := svc.ListTeacherCourseEnrollmentsView(context.Background(), "teacher-1", "course-1")
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Equal(t, "course-1", view.CourseID)
	require.Equal(t, "Backend con Go", view.CourseTitle)
	require.Len(t, view.Enrollments, 2)
	require.Equal(t, "Ana Perez", view.Enrollments[0].StudentName)
	require.Equal(t, domain.StatusActive, view.Enrollments[0].Status)
	require.Equal(t, "Jose Ruarte", view.Enrollments[1].StudentName)
}

func TestListAdminEnrollmentsViewIncludesStudentAndCourseNames(t *testing.T) {
	t.Parallel()

	svc := NewEnrollmentService(
		&mockEnrollmentRepository{
			reservePendingFn: func(context.Context, domain.Enrollment, int) (*domain.Enrollment, error) { return nil, nil },
			confirmPendingFn: func(context.Context, string, string) (*domain.Enrollment, error) { return nil, nil },
			listByUserFn:     func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) { return nil, nil },
			listPaginatedFn: func(context.Context, int, int) ([]domain.Enrollment, error) {
				return []domain.Enrollment{
					{ID: "enr-1", UserID: "student-1", CourseID: "course-1", Status: domain.StatusActive, CreatedAt: time.Date(2026, time.March, 11, 21, 37, 43, 0, time.UTC)},
					{ID: "enr-2", UserID: "student-2", CourseID: "course-2", Status: domain.StatusPending, CreatedAt: time.Date(2026, time.March, 12, 10, 0, 0, 0, time.UTC)},
				}, nil
			},
			listByCourseFn:      func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) { return nil, nil },
			countActiveByCourse: func(context.Context, string) (int, error) { return 0, nil },
			isStudentEnrolledFn: func(context.Context, string, string) (bool, error) { return false, nil },
			getByUserCourseFn:   func(context.Context, string, string) (*domain.Enrollment, error) { return nil, ErrEnrollmentNotFound },
		},
		fakeCoursesClient{
			getCourseFn: func(_ context.Context, courseID string) (*CourseInfo, error) {
				switch courseID {
				case "course-1":
					return &CourseInfo{ID: courseID, Title: "Backend con Go", Status: "published", Capacity: 10}, nil
				case "course-2":
					return &CourseInfo{ID: courseID, Title: "Finanzas Personales y Presupuesto", Status: "published", Capacity: 10}, nil
				default:
					return &CourseInfo{ID: courseID, Title: "Curso"}, nil
				}
			},
			isTeacherAssignedFn: func(context.Context, string, string) (bool, error) { return false, nil },
		},
		fakeUsersClient{
			isEmailVerifiedFn: func(context.Context, string) (bool, error) { return true, nil },
			getUserFn: func(_ context.Context, userID string) (*UserInfo, error) {
				switch userID {
				case "student-1":
					return &UserInfo{ID: userID, Name: "Ana Perez"}, nil
				case "student-2":
					return &UserInfo{ID: userID, Name: "Jose Ruarte"}, nil
				default:
					return &UserInfo{ID: userID, Name: "Alumno"}, nil
				}
			},
		},
	)

	view, err := svc.ListAdminEnrollmentsView(context.Background(), 20, 0)
	require.NoError(t, err)
	require.Len(t, view, 2)
	require.Equal(t, "Ana Perez", view[0].StudentName)
	require.Equal(t, "Backend con Go", view[0].CourseTitle)
	require.Equal(t, domain.StatusActive, view[0].Status)
	require.Equal(t, "Jose Ruarte", view[1].StudentName)
	require.Equal(t, "Finanzas Personales y Presupuesto", view[1].CourseTitle)
	require.Equal(t, domain.StatusPending, view[1].Status)
}

func TestDeleteCourseEnrollments(t *testing.T) {
	t.Parallel()

	var deletedCourseID string
	svc := NewEnrollmentService(
		&mockEnrollmentRepository{
			reservePendingFn:    func(context.Context, domain.Enrollment, int) (*domain.Enrollment, error) { return nil, nil },
			confirmPendingFn:    func(context.Context, string, string) (*domain.Enrollment, error) { return nil, nil },
			listByUserFn:        func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) { return nil, nil },
			listPaginatedFn:     func(context.Context, int, int) ([]domain.Enrollment, error) { return nil, nil },
			listByCourseFn:      func(context.Context, string, []domain.Status) ([]domain.Enrollment, error) { return nil, nil },
			countActiveByCourse: func(context.Context, string) (int, error) { return 0, nil },
			isStudentEnrolledFn: func(context.Context, string, string) (bool, error) { return false, nil },
			getByUserCourseFn:   func(context.Context, string, string) (*domain.Enrollment, error) { return nil, ErrEnrollmentNotFound },
			deleteByCourseFn: func(_ context.Context, courseID string) error {
				deletedCourseID = courseID
				return nil
			},
		},
		fakeCoursesClient{
			getCourseFn:         func(context.Context, string) (*CourseInfo, error) { return nil, nil },
			isTeacherAssignedFn: func(context.Context, string, string) (bool, error) { return false, nil },
		},
		fakeUsersClient{
			isEmailVerifiedFn: func(context.Context, string) (bool, error) { return true, nil },
		},
	)

	err := svc.DeleteCourseEnrollments(context.Background(), "course-1")
	require.NoError(t, err)
	require.Equal(t, "course-1", deletedCourseID)
}
