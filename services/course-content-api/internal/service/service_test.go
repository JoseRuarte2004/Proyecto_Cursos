package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"proyecto-cursos/internal/platform/auth"
	"proyecto-cursos/services/course-content-api/internal/domain"
)

type mockLessonRepository struct {
	createFn        func(ctx context.Context, lesson domain.Lesson) (*domain.Lesson, error)
	getByIDFn       func(ctx context.Context, courseID, lessonID string) (*domain.Lesson, error)
	getAttachmentFn func(ctx context.Context, courseID, lessonID, attachmentID string) (*domain.Attachment, error)
	listByCourseFn  func(ctx context.Context, courseID string) ([]domain.Lesson, error)
	updateFn        func(ctx context.Context, lesson domain.Lesson) (*domain.Lesson, error)
	deleteFn        func(ctx context.Context, courseID, lessonID string) error
}

func (m *mockLessonRepository) Create(ctx context.Context, lesson domain.Lesson) (*domain.Lesson, error) {
	return m.createFn(ctx, lesson)
}

func (m *mockLessonRepository) GetByID(ctx context.Context, courseID, lessonID string) (*domain.Lesson, error) {
	return m.getByIDFn(ctx, courseID, lessonID)
}

func (m *mockLessonRepository) GetAttachment(ctx context.Context, courseID, lessonID, attachmentID string) (*domain.Attachment, error) {
	return m.getAttachmentFn(ctx, courseID, lessonID, attachmentID)
}

func (m *mockLessonRepository) ListByCourse(ctx context.Context, courseID string) ([]domain.Lesson, error) {
	return m.listByCourseFn(ctx, courseID)
}

func (m *mockLessonRepository) Update(ctx context.Context, lesson domain.Lesson) (*domain.Lesson, error) {
	return m.updateFn(ctx, lesson)
}

func (m *mockLessonRepository) Delete(ctx context.Context, courseID, lessonID string) error {
	return m.deleteFn(ctx, courseID, lessonID)
}

type fakeAssignmentChecker struct {
	checkFn func(ctx context.Context, courseID, teacherID string) (bool, error)
}

func (f fakeAssignmentChecker) IsTeacherAssigned(ctx context.Context, courseID, teacherID string) (bool, error) {
	return f.checkFn(ctx, courseID, teacherID)
}

type fakeEnrollmentChecker struct {
	checkFn func(ctx context.Context, courseID, studentID string) (bool, error)
}

func (f fakeEnrollmentChecker) IsStudentEnrolled(ctx context.Context, courseID, studentID string) (bool, error) {
	return f.checkFn(ctx, courseID, studentID)
}

func TestCreateEditAndListLessons(t *testing.T) {
	t.Parallel()

	var created domain.Lesson
	repository := &mockLessonRepository{
		createFn: func(_ context.Context, lesson domain.Lesson) (*domain.Lesson, error) {
			created = lesson
			return &lesson, nil
		},
		getByIDFn: func(context.Context, string, string) (*domain.Lesson, error) {
			return &created, nil
		},
		getAttachmentFn: func(context.Context, string, string, string) (*domain.Attachment, error) {
			return nil, nil
		},
		listByCourseFn: func(context.Context, string) ([]domain.Lesson, error) {
			return []domain.Lesson{created}, nil
		},
		updateFn: func(_ context.Context, lesson domain.Lesson) (*domain.Lesson, error) {
			created = lesson
			return &lesson, nil
		},
		deleteFn: func(context.Context, string, string) error { return nil },
	}
	service := NewLessonService(
		repository,
		fakeAssignmentChecker{checkFn: func(context.Context, string, string) (bool, error) { return true, nil }},
		fakeEnrollmentChecker{checkFn: func(context.Context, string, string) (bool, error) { return true, nil }},
	)
	frozenNow := time.Date(2026, time.February, 28, 19, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return frozenNow }

	lesson, err := service.CreateLesson(context.Background(), auth.Session{
		Token: "admin-token",
		Claims: auth.Claims{
			UserID: "admin-1",
			Role:   auth.RoleAdmin,
		},
	}, "course-1", CreateLessonInput{
		Title:       "Introduccion",
		Description: "Arranque del curso",
		OrderIndex:  1,
		VideoURL:    "https://video.test/1",
		Attachments: []AttachmentInput{{
			FileName:    "guia.pdf",
			ContentType: "application/pdf",
			Data:        []byte("pdf"),
		}},
	})
	require.NoError(t, err)
	require.NotEmpty(t, lesson.ID)
	require.Equal(t, frozenNow, created.CreatedAt)
	require.Len(t, created.Attachments, 1)
	require.Equal(t, "file", created.Attachments[0].Kind)

	newTitle := "Introduccion a Go"
	updated, err := service.UpdateLesson(context.Background(), auth.Session{
		Token: "admin-token",
		Claims: auth.Claims{
			UserID: "admin-1",
			Role:   auth.RoleAdmin,
		},
	}, "course-1", lesson.ID, UpdateLessonInput{
		Title: &newTitle,
		Attachments: []AttachmentInput{{
			FileName:    "captura.png",
			ContentType: "image/png",
			Data:        []byte("png"),
		}},
	})
	require.NoError(t, err)
	require.Equal(t, newTitle, updated.Title)
	require.Len(t, updated.Attachments, 2)

	listed, err := service.ListLessons(context.Background(), auth.Session{
		Token: "admin-token",
		Claims: auth.Claims{
			UserID: "admin-1",
			Role:   auth.RoleAdmin,
		},
	}, "course-1")
	require.NoError(t, err)
	require.Len(t, listed, 1)
}

func TestTeacherCanManageLessonsOnlyWhenAssigned(t *testing.T) {
	t.Parallel()

	var created domain.Lesson
	repository := &mockLessonRepository{
		createFn: func(_ context.Context, lesson domain.Lesson) (*domain.Lesson, error) {
			created = lesson
			return &lesson, nil
		},
		getByIDFn: func(context.Context, string, string) (*domain.Lesson, error) {
			return &created, nil
		},
		getAttachmentFn: func(context.Context, string, string, string) (*domain.Attachment, error) {
			return nil, nil
		},
		listByCourseFn: func(context.Context, string) ([]domain.Lesson, error) {
			return []domain.Lesson{created}, nil
		},
		updateFn: func(_ context.Context, lesson domain.Lesson) (*domain.Lesson, error) {
			created = lesson
			return &lesson, nil
		},
		deleteFn: func(context.Context, string, string) error { return nil },
	}

	service := NewLessonService(
		repository,
		fakeAssignmentChecker{checkFn: func(_ context.Context, courseID, teacherID string) (bool, error) {
			return teacherID == "teacher-1", nil
		}},
		fakeEnrollmentChecker{checkFn: func(context.Context, string, string) (bool, error) { return false, nil }},
	)

	teacherSession := auth.Session{
		Token: "teacher-token",
		Claims: auth.Claims{
			UserID: "teacher-1",
			Role:   auth.RoleTeacher,
		},
	}

	lesson, err := service.CreateLesson(context.Background(), teacherSession, "course-1", CreateLessonInput{
		Title:       "Video 1",
		Description: "Clase inicial",
		OrderIndex:  1,
		VideoURL:    "https://video.test/teacher-1",
	})
	require.NoError(t, err)

	newVideoURL := "https://video.test/teacher-1-updated"
	updated, err := service.UpdateLesson(context.Background(), teacherSession, "course-1", lesson.ID, UpdateLessonInput{
		VideoURL: &newVideoURL,
	})
	require.NoError(t, err)
	require.Equal(t, newVideoURL, updated.VideoURL)

	require.NoError(t, service.DeleteLesson(context.Background(), teacherSession, "course-1", lesson.ID))

	_, err = service.CreateLesson(context.Background(), auth.Session{
		Token: "teacher-token",
		Claims: auth.Claims{
			UserID: "teacher-2",
			Role:   auth.RoleTeacher,
		},
	}, "course-1", CreateLessonInput{
		Title:       "Video 2",
		Description: "Clase bloqueada",
		OrderIndex:  2,
		VideoURL:    "https://video.test/teacher-2",
	})
	require.ErrorIs(t, err, ErrForbidden)

	err = service.DeleteLesson(context.Background(), auth.Session{
		Token: "teacher-token",
		Claims: auth.Claims{
			UserID: "teacher-2",
			Role:   auth.RoleTeacher,
		},
	}, "course-1", lesson.ID)
	require.ErrorIs(t, err, ErrForbidden)
}

func TestTeacherPermissionsAssignedVsNotAssigned(t *testing.T) {
	t.Parallel()

	repository := &mockLessonRepository{
		createFn:  func(context.Context, domain.Lesson) (*domain.Lesson, error) { return nil, nil },
		getByIDFn: func(context.Context, string, string) (*domain.Lesson, error) { return nil, nil },
		getAttachmentFn: func(context.Context, string, string, string) (*domain.Attachment, error) {
			return nil, nil
		},
		listByCourseFn: func(context.Context, string) ([]domain.Lesson, error) {
			return []domain.Lesson{{ID: "lesson-1"}}, nil
		},
		updateFn: func(context.Context, domain.Lesson) (*domain.Lesson, error) { return nil, nil },
		deleteFn: func(context.Context, string, string) error { return nil },
	}

	service := NewLessonService(
		repository,
		fakeAssignmentChecker{checkFn: func(_ context.Context, courseID, teacherID string) (bool, error) {
			return teacherID == "teacher-1", nil
		}},
		fakeEnrollmentChecker{checkFn: func(context.Context, string, string) (bool, error) { return false, nil }},
	)

	lessons, err := service.ListLessons(context.Background(), auth.Session{
		Token: "teacher-token",
		Claims: auth.Claims{
			UserID: "teacher-1",
			Role:   auth.RoleTeacher,
		},
	}, "course-1")
	require.NoError(t, err)
	require.Len(t, lessons, 1)

	_, err = service.ListLessons(context.Background(), auth.Session{
		Token: "teacher-token",
		Claims: auth.Claims{
			UserID: "teacher-2",
			Role:   auth.RoleTeacher,
		},
	}, "course-1")
	require.ErrorIs(t, err, ErrForbidden)
}

func TestStudentPermissionsUseEnrollmentChecker(t *testing.T) {
	t.Parallel()

	repository := &mockLessonRepository{
		createFn:  func(context.Context, domain.Lesson) (*domain.Lesson, error) { return nil, nil },
		getByIDFn: func(context.Context, string, string) (*domain.Lesson, error) { return nil, nil },
		getAttachmentFn: func(context.Context, string, string, string) (*domain.Attachment, error) {
			return nil, nil
		},
		listByCourseFn: func(context.Context, string) ([]domain.Lesson, error) {
			return []domain.Lesson{{ID: "lesson-1"}}, nil
		},
		updateFn: func(context.Context, domain.Lesson) (*domain.Lesson, error) { return nil, nil },
		deleteFn: func(context.Context, string, string) error { return nil },
	}

	service := NewLessonService(
		repository,
		fakeAssignmentChecker{checkFn: func(context.Context, string, string) (bool, error) { return false, nil }},
		fakeEnrollmentChecker{checkFn: func(_ context.Context, courseID, studentID string) (bool, error) {
			return studentID == "student-1", nil
		}},
	)

	lessons, err := service.ListLessons(context.Background(), auth.Session{
		Token: "student-token",
		Claims: auth.Claims{
			UserID: "student-1",
			Role:   auth.RoleStudent,
		},
	}, "course-1")
	require.NoError(t, err)
	require.Len(t, lessons, 1)

	_, err = service.ListLessons(context.Background(), auth.Session{
		Token: "student-token",
		Claims: auth.Claims{
			UserID: "student-2",
			Role:   auth.RoleStudent,
		},
	}, "course-1")
	require.ErrorIs(t, err, ErrForbidden)
}

func TestGetLessonAttachmentUsesSameReadAccessRules(t *testing.T) {
	t.Parallel()

	expected := &domain.Attachment{
		ID:          "att-1",
		LessonID:    "lesson-1",
		CourseID:    "course-1",
		Kind:        "image",
		FileName:    "captura.png",
		ContentType: "image/png",
		SizeBytes:   3,
		Data:        []byte("png"),
	}

	repository := &mockLessonRepository{
		createFn:  func(context.Context, domain.Lesson) (*domain.Lesson, error) { return nil, nil },
		getByIDFn: func(context.Context, string, string) (*domain.Lesson, error) { return nil, nil },
		getAttachmentFn: func(_ context.Context, courseID, lessonID, attachmentID string) (*domain.Attachment, error) {
			require.Equal(t, "course-1", courseID)
			require.Equal(t, "lesson-1", lessonID)
			require.Equal(t, "att-1", attachmentID)
			return expected, nil
		},
		listByCourseFn: func(context.Context, string) ([]domain.Lesson, error) { return nil, nil },
		updateFn:       func(context.Context, domain.Lesson) (*domain.Lesson, error) { return nil, nil },
		deleteFn:       func(context.Context, string, string) error { return nil },
	}

	service := NewLessonService(
		repository,
		fakeAssignmentChecker{checkFn: func(_ context.Context, courseID, teacherID string) (bool, error) {
			return teacherID == "teacher-1", nil
		}},
		fakeEnrollmentChecker{checkFn: func(_ context.Context, courseID, studentID string) (bool, error) {
			return studentID == "student-1", nil
		}},
	)

	attachment, err := service.GetLessonAttachment(context.Background(), auth.Session{
		Token: "teacher-token",
		Claims: auth.Claims{
			UserID: "teacher-1",
			Role:   auth.RoleTeacher,
		},
	}, "course-1", "lesson-1", "att-1")
	require.NoError(t, err)
	require.Equal(t, expected, attachment)

	_, err = service.GetLessonAttachment(context.Background(), auth.Session{
		Token: "teacher-token",
		Claims: auth.Claims{
			UserID: "teacher-2",
			Role:   auth.RoleTeacher,
		},
	}, "course-1", "lesson-1", "att-1")
	require.ErrorIs(t, err, ErrForbidden)
}
