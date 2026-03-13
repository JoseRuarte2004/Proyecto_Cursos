package app

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"proyecto-cursos/internal/platform/auth"
	"proyecto-cursos/internal/platform/httpx"
	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/internal/platform/server"
	"proyecto-cursos/services/course-content-api/internal/domain"
	"proyecto-cursos/services/course-content-api/internal/service"
)

const maxLessonUploadBytes = 25 << 20

type Handler struct {
	service    *service.LessonService
	logger     *logger.Logger
	jwtManager *auth.JWTManager
}

func NewHTTPHandler(log *logger.Logger, lessonService *service.LessonService, jwtManager *auth.JWTManager, db *sql.DB) http.Handler {
	readyFn := func(ctx context.Context) error {
		return db.PingContext(ctx)
	}

	router := server.NewRouter("course-content-api", log, readyFn)
	handler := &Handler{
		service:    lessonService,
		logger:     log,
		jwtManager: jwtManager,
	}

	router.Get("/courses/{courseId}/lessons/{lessonId}/attachments/{attachmentId}", handler.handleDownloadAttachment)

	router.Group(func(r chi.Router) {
		r.Use(auth.AuthRequired(jwtManager))
		r.Get("/courses/{courseId}/lessons", handler.handleListLessons)

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRoles(auth.RoleAdmin, auth.RoleTeacher))
			r.Post("/courses/{courseId}/lessons", handler.handleCreateLesson)
			r.Patch("/courses/{courseId}/lessons/{lessonId}", handler.handleUpdateLesson)
			r.Delete("/courses/{courseId}/lessons/{lessonId}", handler.handleDeleteLesson)
		})
	})

	return router
}

type createLessonRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	OrderIndex  int    `json:"orderIndex"`
	VideoURL    string `json:"videoUrl"`
}

type updateLessonRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	OrderIndex  *int    `json:"orderIndex"`
	VideoURL    *string `json:"videoUrl"`
}

type attachmentResponse struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	URL         string `json:"url"`
	CreatedAt   string `json:"createdAt"`
}

type lessonResponse struct {
	ID          string               `json:"id"`
	CourseID    string               `json:"courseId"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	OrderIndex  int                  `json:"orderIndex"`
	VideoURL    string               `json:"videoUrl"`
	Attachments []attachmentResponse `json:"attachments,omitempty"`
	CreatedAt   string               `json:"createdAt"`
	UpdatedAt   string               `json:"updatedAt"`
}

func (h *Handler) handleListLessons(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing auth context")
		return
	}

	lessons, err := h.service.ListLessons(r.Context(), session, chi.URLParam(r, "courseId"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	response := make([]lessonResponse, 0, len(lessons))
	for _, lesson := range lessons {
		response = append(response, toLessonResponse(lesson))
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) handleCreateLesson(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing auth context")
		return
	}

	req, attachments, err := decodeCreateLessonRequest(w, r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	lesson, err := h.service.CreateLesson(r.Context(), session, chi.URLParam(r, "courseId"), service.CreateLessonInput{
		Title:       req.Title,
		Description: req.Description,
		OrderIndex:  req.OrderIndex,
		VideoURL:    req.VideoURL,
		Attachments: attachments,
	})
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, toLessonResponse(*lesson))
}

func (h *Handler) handleUpdateLesson(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing auth context")
		return
	}

	req, attachments, err := decodeUpdateLessonRequest(w, r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	lesson, err := h.service.UpdateLesson(r.Context(), session, chi.URLParam(r, "courseId"), chi.URLParam(r, "lessonId"), service.UpdateLessonInput{
		Title:       req.Title,
		Description: req.Description,
		OrderIndex:  req.OrderIndex,
		VideoURL:    req.VideoURL,
		Attachments: attachments,
	})
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, toLessonResponse(*lesson))
}

func (h *Handler) handleDeleteLesson(w http.ResponseWriter, r *http.Request) {
	session, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing auth context")
		return
	}

	if err := h.service.DeleteLesson(r.Context(), session, chi.URLParam(r, "courseId"), chi.URLParam(r, "lessonId")); err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleDownloadAttachment(w http.ResponseWriter, r *http.Request) {
	session, ok := h.sessionFromRequest(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	attachment, err := h.service.GetLessonAttachment(
		r.Context(),
		session,
		chi.URLParam(r, "courseId"),
		chi.URLParam(r, "lessonId"),
		chi.URLParam(r, "attachmentId"),
	)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	dispositionType := "attachment"
	if attachment.Kind == "image" || attachment.Kind == "video" {
		dispositionType = "inline"
	}

	w.Header().Set("Content-Type", attachment.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(attachment.SizeBytes, 10))
	w.Header().Set("Content-Disposition", mime.FormatMediaType(dispositionType, map[string]string{
		"filename": attachment.FileName,
	}))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(attachment.Data)
}

func (h *Handler) sessionFromRequest(r *http.Request) (auth.Session, bool) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			token = strings.TrimSpace(authHeader[len("Bearer "):])
		}
	}
	if token == "" || h.jwtManager == nil {
		return auth.Session{}, false
	}

	claims, err := h.jwtManager.Parse(token)
	if err != nil {
		return auth.Session{}, false
	}

	return auth.Session{
		Token:  token,
		Claims: *claims,
	}, true
}

func (h *Handler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrTitleRequired),
		errors.Is(err, service.ErrDescriptionRequired),
		errors.Is(err, service.ErrVideoURLRequired),
		errors.Is(err, service.ErrInvalidOrderIndex),
		errors.Is(err, service.ErrTooManyAttachments),
		errors.Is(err, service.ErrAttachmentTooLarge),
		errors.Is(err, service.ErrAttachmentNameRequired),
		errors.Is(err, service.ErrAttachmentDataRequired):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrOrderIndexAlreadyUsed):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrLessonNotFound), errors.Is(err, service.ErrAttachmentNotFound):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrForbidden):
		httpx.WriteError(w, http.StatusForbidden, err.Error())
	default:
		h.logger.Error(r.Context(), "request failed", map[string]any{
			"error": err.Error(),
		})
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}

func toLessonResponse(lesson domain.Lesson) lessonResponse {
	attachments := make([]attachmentResponse, 0, len(lesson.Attachments))
	for _, attachment := range lesson.Attachments {
		attachments = append(attachments, attachmentResponse{
			ID:          attachment.ID,
			Kind:        attachment.Kind,
			FileName:    attachment.FileName,
			ContentType: attachment.ContentType,
			SizeBytes:   attachment.SizeBytes,
			URL: "/content/courses/" +
				url.PathEscape(lesson.CourseID) +
				"/lessons/" +
				url.PathEscape(lesson.ID) +
				"/attachments/" +
				url.PathEscape(attachment.ID),
			CreatedAt: attachment.CreatedAt.Format(time.RFC3339),
		})
	}

	return lessonResponse{
		ID:          lesson.ID,
		CourseID:    lesson.CourseID,
		Title:       lesson.Title,
		Description: lesson.Description,
		OrderIndex:  lesson.OrderIndex,
		VideoURL:    lesson.VideoURL,
		Attachments: attachments,
		CreatedAt:   lesson.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   lesson.UpdatedAt.Format(time.RFC3339),
	}
}

func decodeCreateLessonRequest(w http.ResponseWriter, r *http.Request) (createLessonRequest, []service.AttachmentInput, error) {
	if !isMultipartRequest(r) {
		var req createLessonRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			return createLessonRequest{}, nil, err
		}
		return req, nil, nil
	}

	fields, attachments, err := parseMultipartFields(w, r)
	if err != nil {
		return createLessonRequest{}, nil, err
	}

	orderIndex, err := strconv.Atoi(strings.TrimSpace(fields.Get("orderIndex")))
	if err != nil {
		return createLessonRequest{}, nil, errors.New("orderIndex must be a number")
	}

	return createLessonRequest{
		Title:       strings.TrimSpace(fields.Get("title")),
		Description: strings.TrimSpace(fields.Get("description")),
		OrderIndex:  orderIndex,
		VideoURL:    strings.TrimSpace(fields.Get("videoUrl")),
	}, attachments, nil
}

func decodeUpdateLessonRequest(w http.ResponseWriter, r *http.Request) (updateLessonRequest, []service.AttachmentInput, error) {
	if !isMultipartRequest(r) {
		var req updateLessonRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			return updateLessonRequest{}, nil, err
		}
		return req, nil, nil
	}

	fields, attachments, err := parseMultipartFields(w, r)
	if err != nil {
		return updateLessonRequest{}, nil, err
	}

	req := updateLessonRequest{}
	if _, ok := fields["title"]; ok {
		value := strings.TrimSpace(fields.Get("title"))
		req.Title = &value
	}
	if _, ok := fields["description"]; ok {
		value := strings.TrimSpace(fields.Get("description"))
		req.Description = &value
	}
	if _, ok := fields["videoUrl"]; ok {
		value := strings.TrimSpace(fields.Get("videoUrl"))
		req.VideoURL = &value
	}
	if _, ok := fields["orderIndex"]; ok {
		value, err := strconv.Atoi(strings.TrimSpace(fields.Get("orderIndex")))
		if err != nil {
			return updateLessonRequest{}, nil, errors.New("orderIndex must be a number")
		}
		req.OrderIndex = &value
	}

	return req, attachments, nil
}

func parseMultipartFields(w http.ResponseWriter, r *http.Request) (url.Values, []service.AttachmentInput, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLessonUploadBytes)
	if err := r.ParseMultipartForm(maxLessonUploadBytes); err != nil {
		return nil, nil, errors.New("invalid multipart body")
	}

	attachments, err := readUploadedAttachments(r.MultipartForm.File["attachments"])
	if err != nil {
		return nil, nil, err
	}

	return r.MultipartForm.Value, attachments, nil
}

func readUploadedAttachments(files []*multipart.FileHeader) ([]service.AttachmentInput, error) {
	if len(files) == 0 {
		return nil, nil
	}

	attachments := make([]service.AttachmentInput, 0, len(files))
	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			return nil, err
		}

		data, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}

		contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
		if contentType == "" && len(data) > 0 {
			contentType = http.DetectContentType(data)
		}
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		attachments = append(attachments, service.AttachmentInput{
			FileName:    header.Filename,
			ContentType: contentType,
			Data:        data,
		})
	}

	return attachments, nil
}

func isMultipartRequest(r *http.Request) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type"))), "multipart/form-data")
}
