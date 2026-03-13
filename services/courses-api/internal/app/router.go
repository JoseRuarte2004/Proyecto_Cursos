package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"proyecto-cursos/internal/platform/auth"
	"proyecto-cursos/internal/platform/httpx"
	"proyecto-cursos/internal/platform/internalauth"
	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/internal/platform/server"
	platformstore "proyecto-cursos/internal/platform/store"
	"proyecto-cursos/services/courses-api/internal/domain"
	"proyecto-cursos/services/courses-api/internal/service"
)

type Handler struct {
	service         *service.CourseService
	recommendations *service.RecommendationService
	logger          *logger.Logger
	cache           *CourseCache
}

func NewHTTPHandler(log *logger.Logger, courseService *service.CourseService, recommendationService *service.RecommendationService, jwtManager *auth.JWTManager, db *sql.DB, redisClient *redis.Client, cache *CourseCache, internalToken string) http.Handler {
	readyFn := func(ctx context.Context) error {
		if err := db.PingContext(ctx); err != nil {
			return err
		}

		return platformstore.PingRedis(ctx, redisClient)
	}

	router := server.NewRouter("courses-api", log, readyFn)
	handler := &Handler{
		service:         courseService,
		recommendations: recommendationService,
		logger:          log,
		cache:           cache,
	}

	router.Get("/courses", handler.handleListPublishedCourses)
	router.Get("/courses/{id}", handler.handleGetPublishedCourse)
	router.Post("/recommend", handler.handleRecommendCourse)

	router.Group(func(r chi.Router) {
		r.Use(internalauth.RequireToken(internalToken))
		r.Get("/internal/courses/{id}", handler.handleInternalGetCourse)
		r.Get("/internal/courses/{id}/teachers/{teacherId}/assigned", handler.handleInternalTeacherAssigned)
		r.Get("/internal/courses/{id}/teachers", handler.handleInternalCourseTeachers)
	})

	router.Group(func(r chi.Router) {
		r.Use(auth.AuthRequired(jwtManager))

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles(auth.RoleAdmin))
			r.Post("/courses", handler.handleCreateCourse)
			r.Patch("/courses/{id}", handler.handleUpdateCourse)
			r.Delete("/courses/{id}", handler.handleDeleteCourse)
			r.Post("/courses/{id}/teachers", handler.handleAssignTeacher)
			r.Get("/courses/{id}/teachers", handler.handleListCourseTeachers)
			r.Delete("/courses/{id}/teachers/{teacherId}", handler.handleRemoveTeacher)
		})

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles(auth.RoleTeacher))
			r.Get("/teacher/me/courses", handler.handleTeacherCourses)
		})
	})

	return router
}

type createCourseRequest struct {
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Category    string        `json:"category"`
	ImageURL    *string       `json:"imageUrl"`
	Price       float64       `json:"price"`
	Currency    string        `json:"currency"`
	Capacity    int           `json:"capacity"`
	Status      domain.Status `json:"status"`
}

type updateCourseRequest struct {
	Title       *string        `json:"title"`
	Description *string        `json:"description"`
	Category    *string        `json:"category"`
	ImageURL    *string        `json:"imageUrl"`
	Price       *float64       `json:"price"`
	Currency    *string        `json:"currency"`
	Capacity    *int           `json:"capacity"`
	Status      *domain.Status `json:"status"`
}

type assignTeacherRequest struct {
	TeacherID string `json:"teacherId"`
}

type recommendCourseRequest struct {
	Question string                          `json:"question"`
	Name     string                          `json:"name"`
	History  []service.RecommendationMessage `json:"history"`
}

type courseResponse struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Category    string        `json:"category"`
	ImageURL    *string       `json:"imageUrl,omitempty"`
	Price       float64       `json:"price"`
	Currency    string        `json:"currency"`
	Capacity    int           `json:"capacity"`
	Status      domain.Status `json:"status"`
	CreatedBy   string        `json:"createdBy"`
	CreatedAt   string        `json:"createdAt"`
	UpdatedAt   string        `json:"updatedAt"`
}

type recommendCourseResponse struct {
	Answer string `json:"answer"`
}

func (h *Handler) handleListPublishedCourses(w http.ResponseWriter, r *http.Request) {
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

	cacheKey := fmt.Sprintf("courses:public:list:limit=%d:offset=%d", limit, offset)
	if payload, err := h.cache.Get(r.Context(), cacheKey); err == nil && payload != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
		return
	}

	courses, err := h.service.ListPublished(r.Context(), limit, offset)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	response := make([]courseResponse, 0, len(courses))
	for _, course := range courses {
		response = append(response, toCourseResponse(course))
	}

	payload, err := json.Marshal(response)
	if err == nil {
		_ = h.cache.Set(r.Context(), cacheKey, payload)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) handleGetPublishedCourse(w http.ResponseWriter, r *http.Request) {
	courseID := chi.URLParam(r, "id")
	cacheKey := "courses:public:detail:" + courseID
	if payload, err := h.cache.Get(r.Context(), cacheKey); err == nil && payload != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
		return
	}

	course, err := h.service.GetPublishedCourse(r.Context(), courseID)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	response := toCourseResponse(*course)
	payload, err := json.Marshal(response)
	if err == nil {
		_ = h.cache.Set(r.Context(), cacheKey, payload)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) handleRecommendCourse(w http.ResponseWriter, r *http.Request) {
	var req recommendCourseRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if h.recommendations == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, service.ErrRecommendationsDisabled.Error())
		return
	}

	result, err := h.recommendations.Recommend(r.Context(), req.Name, req.Question, req.History)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, recommendCourseResponse{
		Answer: result.Answer,
	})
}

func (h *Handler) handleCreateCourse(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing auth context")
		return
	}

	var req createCourseRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	course, err := h.service.CreateCourse(r.Context(), session.Claims.UserID, service.CreateCourseInput{
		Title:       req.Title,
		Description: req.Description,
		Category:    req.Category,
		ImageURL:    req.ImageURL,
		Price:       req.Price,
		Currency:    req.Currency,
		Capacity:    req.Capacity,
		Status:      req.Status,
	})
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	_ = h.cache.InvalidatePublic(r.Context())
	httpx.WriteJSON(w, http.StatusCreated, toCourseResponse(*course))
}

func (h *Handler) handleUpdateCourse(w http.ResponseWriter, r *http.Request) {
	var req updateCourseRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	course, err := h.service.UpdateCourse(r.Context(), chi.URLParam(r, "id"), service.UpdateCourseInput{
		Title:       req.Title,
		Description: req.Description,
		Category:    req.Category,
		ImageURL:    req.ImageURL,
		Price:       req.Price,
		Currency:    req.Currency,
		Capacity:    req.Capacity,
		Status:      req.Status,
	})
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	_ = h.cache.InvalidatePublic(r.Context())
	httpx.WriteJSON(w, http.StatusOK, toCourseResponse(*course))
}

func (h *Handler) handleDeleteCourse(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteCourse(r.Context(), chi.URLParam(r, "id")); err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	_ = h.cache.InvalidatePublic(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleAssignTeacher(w http.ResponseWriter, r *http.Request) {
	var req assignTeacherRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.AssignTeacher(r.Context(), chi.URLParam(r, "id"), req.TeacherID); err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	_ = h.cache.InvalidatePublic(r.Context())
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "assigned"})
}

func (h *Handler) handleListCourseTeachers(w http.ResponseWriter, r *http.Request) {
	teacherIDs, err := h.service.ListCourseTeachers(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"teacherIds": teacherIDs,
	})
}

func (h *Handler) handleRemoveTeacher(w http.ResponseWriter, r *http.Request) {
	if err := h.service.RemoveTeacher(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "teacherId")); err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	_ = h.cache.InvalidatePublic(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleTeacherCourses(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing auth context")
		return
	}

	courses, err := h.service.ListTeacherCourses(r.Context(), session.Claims.UserID)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	response := make([]courseResponse, 0, len(courses))
	for _, course := range courses {
		response = append(response, toCourseResponse(course))
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) handleInternalGetCourse(w http.ResponseWriter, r *http.Request) {
	course, err := h.service.GetCourse(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, toCourseResponse(*course))
}

func (h *Handler) handleInternalTeacherAssigned(w http.ResponseWriter, r *http.Request) {
	teacherID := chi.URLParam(r, "teacherId")
	assigned, err := h.service.IsTeacherAssigned(r.Context(), chi.URLParam(r, "id"), teacherID)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"assigned": assigned})
}

func (h *Handler) handleInternalCourseTeachers(w http.ResponseWriter, r *http.Request) {
	teacherIDs, err := h.service.ListCourseTeachers(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"teacherIds": teacherIDs,
	})
}

func (h *Handler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrTitleRequired),
		errors.Is(err, service.ErrDescriptionRequired),
		errors.Is(err, service.ErrCategoryRequired),
		errors.Is(err, service.ErrCurrencyRequired),
		errors.Is(err, service.ErrInvalidPrice),
		errors.Is(err, service.ErrInvalidCapacity),
		errors.Is(err, service.ErrInvalidStatus),
		errors.Is(err, service.ErrInvalidTeacher),
		errors.Is(err, service.ErrQuestionRequired):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrTeacherAlreadyAssigned):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrCourseNotFound),
		errors.Is(err, service.ErrTeacherNotFound),
		errors.Is(err, service.ErrRecommendationCatalogEmpty):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrRecommendationsDisabled):
		httpx.WriteError(w, http.StatusServiceUnavailable, err.Error())
	default:
		h.logger.Error(r.Context(), "request failed", map[string]any{
			"error": err.Error(),
		})
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}

func toCourseResponse(course domain.Course) courseResponse {
	return courseResponse{
		ID:          course.ID,
		Title:       course.Title,
		Description: course.Description,
		Category:    course.Category,
		ImageURL:    course.ImageURL,
		Price:       course.Price,
		Currency:    course.Currency,
		Capacity:    course.Capacity,
		Status:      course.Status,
		CreatedBy:   course.CreatedBy,
		CreatedAt:   course.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   course.UpdatedAt.Format(time.RFC3339),
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
