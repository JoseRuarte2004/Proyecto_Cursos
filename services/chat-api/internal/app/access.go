package app

import (
	"context"
	"errors"
	"sort"
	"strings"
)

var (
	ErrCourseIDRequired      = errors.New("courseId is required")
	ErrOtherUserIDRequired   = errors.New("otherUserId is required")
	ErrUserNotFound          = errors.New("user not found")
	ErrForbidden             = errors.New("forbidden")
	ErrPrivateChatNotAllowed = errors.New("private chat is allowed only between teacher and student")
)

type CoursesAccess interface {
	IsTeacherAssigned(ctx context.Context, courseID, teacherID string) (bool, error)
	ListTeacherIDs(ctx context.Context, courseID string) ([]string, error)
}

type EnrollmentsAccess interface {
	IsStudentEnrolled(ctx context.Context, courseID, studentID string) (bool, error)
	ListActiveStudentIDs(ctx context.Context, courseID string) ([]string, error)
}

type UsersAccess interface {
	GetUser(ctx context.Context, userID string) (*UserProfile, error)
}

type PrivateContact struct {
	UserID string
	Name   string
	Role   string
}

type CourseAccessService struct {
	courses     CoursesAccess
	enrollments EnrollmentsAccess
	users       UsersAccess
}

func NewCourseAccessService(courses CoursesAccess, enrollments EnrollmentsAccess, users UsersAccess) *CourseAccessService {
	return &CourseAccessService{
		courses:     courses,
		enrollments: enrollments,
		users:       users,
	}
}

func (s *CourseAccessService) CheckCourseAccess(ctx context.Context, principal Principal, courseID string) error {
	courseID = strings.TrimSpace(courseID)
	if courseID == "" {
		return ErrCourseIDRequired
	}

	switch normalizeRole(principal.Role) {
	case "admin":
		return nil
	case "teacher":
		assigned, err := s.courses.IsTeacherAssigned(ctx, courseID, strings.TrimSpace(principal.UserID))
		if err != nil {
			return err
		}
		if !assigned {
			return ErrForbidden
		}
		return nil
	default:
		enrolled, err := s.enrollments.IsStudentEnrolled(ctx, courseID, strings.TrimSpace(principal.UserID))
		if err != nil {
			return err
		}
		if !enrolled {
			return ErrForbidden
		}
		return nil
	}
}

func (s *CourseAccessService) ResolvePrivateRoom(ctx context.Context, principal Principal, courseID, otherUserID string) (string, error) {
	courseID = strings.TrimSpace(courseID)
	if courseID == "" {
		return "", ErrCourseIDRequired
	}

	otherUserID = strings.TrimSpace(otherUserID)
	if otherUserID == "" {
		return "", ErrOtherUserIDRequired
	}

	principalUserID := strings.TrimSpace(principal.UserID)
	if principalUserID == "" || principalUserID == otherUserID {
		return "", ErrPrivateChatNotAllowed
	}

	if err := s.CheckCourseAccess(ctx, principal, courseID); err != nil {
		return "", err
	}

	otherUser, err := s.users.GetUser(ctx, otherUserID)
	if err != nil {
		return "", err
	}

	currentRole := normalizeRole(principal.Role)
	otherRole := normalizeRole(otherUser.Role)
	switch currentRole {
	case "student":
		if otherRole != "teacher" {
			return "", ErrPrivateChatNotAllowed
		}
		assigned, err := s.courses.IsTeacherAssigned(ctx, courseID, otherUserID)
		if err != nil {
			return "", err
		}
		if !assigned {
			return "", ErrForbidden
		}
	case "teacher":
		if otherRole != "student" {
			return "", ErrPrivateChatNotAllowed
		}
		enrolled, err := s.enrollments.IsStudentEnrolled(ctx, courseID, otherUserID)
		if err != nil {
			return "", err
		}
		if !enrolled {
			return "", ErrForbidden
		}
	case "admin":
		switch otherRole {
		case "teacher":
			assigned, err := s.courses.IsTeacherAssigned(ctx, courseID, otherUserID)
			if err != nil {
				return "", err
			}
			if !assigned {
				return "", ErrForbidden
			}
		case "student":
			enrolled, err := s.enrollments.IsStudentEnrolled(ctx, courseID, otherUserID)
			if err != nil {
				return "", err
			}
			if !enrolled {
				return "", ErrForbidden
			}
		default:
			return "", ErrPrivateChatNotAllowed
		}
	default:
		return "", ErrForbidden
	}

	return privateRoomFromCourseAndUsers(courseID, principalUserID, otherUserID), nil
}

func (s *CourseAccessService) ListPrivateContacts(ctx context.Context, principal Principal, courseID string) ([]PrivateContact, error) {
	courseID = strings.TrimSpace(courseID)
	if courseID == "" {
		return nil, ErrCourseIDRequired
	}

	if err := s.CheckCourseAccess(ctx, principal, courseID); err != nil {
		return nil, err
	}

	currentRole := normalizeRole(principal.Role)
	currentUserID := strings.TrimSpace(principal.UserID)

	candidateIDs := make([]string, 0)
	switch currentRole {
	case "student":
		teacherIDs, err := s.courses.ListTeacherIDs(ctx, courseID)
		if err != nil {
			return nil, err
		}
		candidateIDs = append(candidateIDs, teacherIDs...)
	case "teacher":
		studentIDs, err := s.enrollments.ListActiveStudentIDs(ctx, courseID)
		if err != nil {
			return nil, err
		}
		candidateIDs = append(candidateIDs, studentIDs...)
	case "admin":
		teacherIDs, err := s.courses.ListTeacherIDs(ctx, courseID)
		if err != nil {
			return nil, err
		}
		studentIDs, err := s.enrollments.ListActiveStudentIDs(ctx, courseID)
		if err != nil {
			return nil, err
		}
		candidateIDs = append(candidateIDs, teacherIDs...)
		candidateIDs = append(candidateIDs, studentIDs...)
	default:
		return nil, ErrForbidden
	}

	seen := map[string]struct{}{}
	contacts := make([]PrivateContact, 0, len(candidateIDs))
	for _, candidateID := range candidateIDs {
		candidateID = strings.TrimSpace(candidateID)
		if candidateID == "" || candidateID == currentUserID {
			continue
		}
		if _, exists := seen[candidateID]; exists {
			continue
		}
		seen[candidateID] = struct{}{}

		user, err := s.users.GetUser(ctx, candidateID)
		if err != nil {
			if errors.Is(err, ErrUserNotFound) {
				continue
			}
			return nil, err
		}

		role := normalizeRole(user.Role)
		if currentRole == "student" && role != "teacher" {
			continue
		}
		if currentRole == "teacher" && role != "student" {
			continue
		}
		if currentRole == "admin" && role != "teacher" && role != "student" {
			continue
		}

		contacts = append(contacts, PrivateContact{
			UserID: candidateID,
			Name:   strings.TrimSpace(user.Name),
			Role:   role,
		})
	}

	sort.Slice(contacts, func(i, j int) bool {
		leftName := strings.ToLower(strings.TrimSpace(contacts[i].Name))
		rightName := strings.ToLower(strings.TrimSpace(contacts[j].Name))
		if leftName == rightName {
			return contacts[i].UserID < contacts[j].UserID
		}
		return leftName < rightName
	})

	return contacts, nil
}

func privateRoomFromCourseAndUsers(courseID, leftUserID, rightUserID string) string {
	leftUserID = strings.TrimSpace(leftUserID)
	rightUserID = strings.TrimSpace(rightUserID)
	if leftUserID > rightUserID {
		leftUserID, rightUserID = rightUserID, leftUserID
	}
	return "private:" + strings.TrimSpace(courseID) + ":" + leftUserID + ":" + rightUserID
}
