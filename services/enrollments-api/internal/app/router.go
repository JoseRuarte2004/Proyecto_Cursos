package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"proyecto-cursos/internal/platform/auth"
	"proyecto-cursos/internal/platform/httpx"
	"proyecto-cursos/internal/platform/internalauth"
	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/internal/platform/mq"
	"proyecto-cursos/internal/platform/server"
	"proyecto-cursos/services/enrollments-api/internal/domain"
	"proyecto-cursos/services/enrollments-api/internal/service"
)

type Handler struct {
	service *service.EnrollmentService
	logger  *logger.Logger
}

func NewHTTPHandler(log *logger.Logger, enrollmentService *service.EnrollmentService, jwtManager *auth.JWTManager, db *sql.DB, rabbit *mq.ConnectionManager, internalToken string) http.Handler {
	readyFn := func(ctx context.Context) error {
		if err := db.PingContext(ctx); err != nil {
			return err
		}

		if rabbit == nil {
			return nil
		}

		return rabbit.Ping(ctx)
	}

	router := server.NewRouter("enrollments-api", log, readyFn)
	handler := &Handler{
		service: enrollmentService,
		logger:  log,
	}

	router.Get("/courses/{courseId}/availability", handler.handleAvailability)
	router.Group(func(r chi.Router) {
		r.Use(internalauth.RequireToken(internalToken))
		r.Get("/internal/users/{userId}/courses/{courseId}/pending", handler.handleInternalPendingEnrollment)
		r.Delete("/internal/users/{userId}/courses/{courseId}/pending", handler.handleInternalCancelPendingEnrollment)
		r.Get("/internal/courses/{courseId}/students/{studentId}/enrolled", handler.handleInternalStudentEnrolled)
		r.Get("/internal/courses/{courseId}/students", handler.handleInternalCourseStudents)
		r.Delete("/internal/courses/{courseId}/enrollments", handler.handleInternalDeleteCourseEnrollments)
		r.Post("/enrollments/confirm", handler.handleConfirm)
	})

	router.Group(func(r chi.Router) {
		r.Use(auth.AuthRequired(jwtManager))

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles(auth.RoleStudent))
			r.Post("/enrollments/reserve", handler.handleReserve)
			r.Get("/me/enrollments", handler.handleMyEnrollments)
		})

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles(auth.RoleAdmin))
			r.Get("/admin/enrollments", handler.handleAdminEnrollments)
		})

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles(auth.RoleTeacher))
			r.Get("/teacher/courses/{courseId}/enrollments", handler.handleTeacherCourseEnrollments)
		})
	})

	return router
}

type reserveRequest struct {
	CourseID string `json:"courseId"`
}

type confirmRequest struct {
	UserID   string `json:"userId"`
	CourseID string `json:"courseId"`
}

type enrollmentResponse struct {
	ID        string        `json:"id"`
	UserID    string        `json:"userId"`
	CourseID  string        `json:"courseId"`
	Status    domain.Status `json:"status"`
	CreatedAt string        `json:"createdAt"`
}

type courseSummaryResponse struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Category string  `json:"category"`
	ImageURL *string `json:"imageUrl,omitempty"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
	Status   string  `json:"status"`
}

type myEnrollmentResponse struct {
	CourseID  string                `json:"courseId"`
	Status    domain.Status         `json:"status"`
	CreatedAt string                `json:"createdAt"`
	Course    courseSummaryResponse `json:"course"`
}

type teacherCourseEnrollmentResponse struct {
	StudentName string        `json:"studentName"`
	Status      domain.Status `json:"status"`
	CreatedAt   string        `json:"createdAt"`
}

type teacherCourseEnrollmentsResponse struct {
	CourseID    string                            `json:"courseId"`
	CourseTitle string                            `json:"courseTitle"`
	Enrollments []teacherCourseEnrollmentResponse `json:"enrollments"`
}

type adminEnrollmentResponse struct {
	StudentName string        `json:"studentName"`
	CourseTitle string        `json:"courseTitle"`
	Status      domain.Status `json:"status"`
	CreatedAt   string        `json:"createdAt"`
}

func (h *Handler) handleReserve(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing auth context")
		return
	}

	var req reserveRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	enrollment, err := h.service.Reserve(r.Context(), session.Claims.UserID, service.ReserveInput{
		CourseID: req.CourseID,
	})
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, toEnrollmentResponse(*enrollment))
}

func (h *Handler) handleConfirm(w http.ResponseWriter, r *http.Request) {
	var req confirmRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	enrollment, err := h.service.Confirm(r.Context(), req.UserID, req.CourseID)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"userId":   enrollment.UserID,
		"courseId": enrollment.CourseID,
		"status":   enrollment.Status,
	})
}

func (h *Handler) handleMyEnrollments(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing auth context")
		return
	}

	items, err := h.service.ListMyEnrollments(r.Context(), session.Claims.UserID)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	response := make([]myEnrollmentResponse, 0, len(items))
	for _, item := range items {
		response = append(response, myEnrollmentResponse{
			CourseID:  item.Enrollment.CourseID,
			Status:    item.Enrollment.Status,
			CreatedAt: item.Enrollment.CreatedAt.Format(time.RFC3339),
			Course: courseSummaryResponse{
				ID:       item.Course.ID,
				Title:    item.Course.Title,
				Category: item.Course.Category,
				ImageURL: item.Course.ImageURL,
				Price:    item.Course.Price,
				Currency: item.Course.Currency,
				Status:   item.Course.Status,
			},
		})
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) handleAdminEnrollments(w http.ResponseWriter, r *http.Request) {
	limit := parseIntQuery(r, "limit", 20)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset := parseIntQuery(r, "offset", 0)
	if offset < 0 {
		offset = 0
	}

	enrollments, err := h.service.ListAdminEnrollmentsView(r.Context(), limit, offset)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	response := make([]adminEnrollmentResponse, 0, len(enrollments))
	for _, enrollment := range enrollments {
		response = append(response, adminEnrollmentResponse{
			StudentName: enrollment.StudentName,
			CourseTitle: enrollment.CourseTitle,
			Status:      enrollment.Status,
			CreatedAt:   enrollment.CreatedAt.Format(time.RFC3339),
		})
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) handleTeacherCourseEnrollments(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing auth context")
		return
	}

	view, err := h.service.ListTeacherCourseEnrollmentsView(r.Context(), session.Claims.UserID, chi.URLParam(r, "courseId"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	response := teacherCourseEnrollmentsResponse{
		CourseID:    view.CourseID,
		CourseTitle: view.CourseTitle,
		Enrollments: make([]teacherCourseEnrollmentResponse, 0, len(view.Enrollments)),
	}
	for _, enrollment := range view.Enrollments {
		response.Enrollments = append(response.Enrollments, teacherCourseEnrollmentResponse{
			StudentName: enrollment.StudentName,
			Status:      enrollment.Status,
			CreatedAt:   enrollment.CreatedAt.Format(time.RFC3339),
		})
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) handleAvailability(w http.ResponseWriter, r *http.Request) {
	availability, err := h.service.GetAvailability(r.Context(), chi.URLParam(r, "courseId"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"courseId":    availability.CourseID,
		"capacity":    availability.Capacity,
		"activeCount": availability.ActiveCount,
		"available":   availability.Available,
	})
}

func (h *Handler) handleInternalStudentEnrolled(w http.ResponseWriter, r *http.Request) {
	enrolled, err := h.service.IsStudentEnrolled(r.Context(), chi.URLParam(r, "courseId"), chi.URLParam(r, "studentId"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"enrolled": enrolled})
}

func (h *Handler) handleInternalCourseStudents(w http.ResponseWriter, r *http.Request) {
	studentIDs, err := h.service.ListActiveStudentIDs(r.Context(), chi.URLParam(r, "courseId"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"studentIds": studentIDs})
}

func (h *Handler) handleInternalPendingEnrollment(w http.ResponseWriter, r *http.Request) {
	pending, err := h.service.HasPendingEnrollment(r.Context(), chi.URLParam(r, "userId"), chi.URLParam(r, "courseId"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"pending": pending})
}

func (h *Handler) handleInternalCancelPendingEnrollment(w http.ResponseWriter, r *http.Request) {
	err := h.service.CancelPendingEnrollment(r.Context(), chi.URLParam(r, "userId"), chi.URLParam(r, "courseId"))
	if err != nil && !errors.Is(err, service.ErrPendingEnrollmentMissing) {
		h.writeServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleInternalDeleteCourseEnrollments(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteCourseEnrollments(r.Context(), chi.URLParam(r, "courseId")); err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrCourseIDRequired),
		errors.Is(err, service.ErrCourseNotPublished):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrEnrollmentAlreadyExists),
		errors.Is(err, service.ErrCourseFull),
		errors.Is(err, service.ErrPendingEnrollmentMissing):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrCourseNotFound):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrForbidden):
		httpx.WriteError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrEmailNotVerified):
		httpx.WriteError(w, http.StatusForbidden, err.Error())
	default:
		h.logger.Error(r.Context(), "request failed", map[string]any{
			"error": err.Error(),
		})
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}

func toEnrollmentResponse(enrollment domain.Enrollment) enrollmentResponse {
	return enrollmentResponse{
		ID:        enrollment.ID,
		UserID:    enrollment.UserID,
		CourseID:  enrollment.CourseID,
		Status:    enrollment.Status,
		CreatedAt: enrollment.CreatedAt.Format(time.RFC3339),
	}
}

func parseIntQuery(r *http.Request, key string, fallback int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}
