package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/services/courses-api/internal/domain"
)

type fakeBootstrapCatalogRepository struct {
	countCoursesFn func(ctx context.Context) (int, error)
	createFn       func(ctx context.Context, course domain.Course) (*domain.Course, error)
}

func (f fakeBootstrapCatalogRepository) CountCourses(ctx context.Context) (int, error) {
	return f.countCoursesFn(ctx)
}

func (f fakeBootstrapCatalogRepository) Create(ctx context.Context, course domain.Course) (*domain.Course, error) {
	return f.createFn(ctx, course)
}

func TestEnsureBootstrapCatalogSeedsWhenDatabaseIsEmpty(t *testing.T) {
	t.Parallel()

	created := make([]domain.Course, 0, len(defaultBootstrapCatalog))
	err := EnsureBootstrapCatalog(
		context.Background(),
		fakeBootstrapCatalogRepository{
			countCoursesFn: func(context.Context) (int, error) {
				return 0, nil
			},
			createFn: func(_ context.Context, course domain.Course) (*domain.Course, error) {
				created = append(created, course)
				return &course, nil
			},
		},
		logger.New("test"),
	)

	require.NoError(t, err)
	require.Len(t, created, len(defaultBootstrapCatalog))
	for index, course := range created {
		require.NotEmpty(t, course.ID)
		require.Equal(t, bootstrapCatalogOwnerID, course.CreatedBy)
		require.Equal(t, domain.StatusPublished, course.Status)
		require.Equal(t, defaultBootstrapCatalog[index].Title, course.Title)
	}
}

func TestEnsureBootstrapCatalogSkipsWhenCoursesAlreadyExist(t *testing.T) {
	t.Parallel()

	err := EnsureBootstrapCatalog(
		context.Background(),
		fakeBootstrapCatalogRepository{
			countCoursesFn: func(context.Context) (int, error) {
				return 3, nil
			},
			createFn: func(context.Context, domain.Course) (*domain.Course, error) {
				t.Fatal("create should not run when catalog already exists")
				return nil, nil
			},
		},
		logger.New("test"),
	)

	require.NoError(t, err)
}
