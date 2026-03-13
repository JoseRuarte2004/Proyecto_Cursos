package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"proyecto-cursos/internal/platform/api"
	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/internal/platform/server"
	"proyecto-cursos/services/chat-api/internal/domain"
	"proyecto-cursos/services/chat-api/internal/service"
	"proyecto-cursos/services/chat-api/internal/ws"
)

const maxChatUploadBytes = 25 << 20

type Handler struct {
	log      *logger.Logger
	service  *service.ChatService
	access   *CourseAccessService
	users    UsersAccess
	hub      *ws.Hub
	validate TokenValidator
}

type createMessageRequest struct {
	Content string `json:"content"`
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

type messageResponse struct {
	ID          string               `json:"id"`
	RoomID      string               `json:"roomId,omitempty"`
	CourseID    string               `json:"courseId,omitempty"`
	SenderID    string               `json:"senderId"`
	SenderName  string               `json:"senderName,omitempty"`
	SenderRole  string               `json:"senderRole"`
	Content     string               `json:"content"`
	Attachments []attachmentResponse `json:"attachments,omitempty"`
	CreatedAt   string               `json:"createdAt"`
}

type privateContactResponse struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}

func NewHTTPHandler(
	log *logger.Logger,
	db *sql.DB,
	chatService *service.ChatService,
	accessService *CourseAccessService,
	users UsersAccess,
	hub *ws.Hub,
	tokenValidator TokenValidator,
) http.Handler {
	readyFn := func(ctx context.Context) error {
		return db.PingContext(ctx)
	}

	router := server.NewRouter("chat-api", log, readyFn)
	handler := &Handler{
		log:      log,
		service:  chatService,
		access:   accessService,
		users:    users,
		hub:      hub,
		validate: tokenValidator,
	}

	router.Get("/attachments/{attachmentId}", handler.handleDownloadAttachment)

	// Endpoints legacy.
	router.Get("/history/{room_id}", handler.handleHistoryByRoom)
	router.Get("/api/chat/history/{room_id}", handler.handleHistoryByRoom)
	router.Get("/ws", handler.handleWebSocketByRoom)
	router.Get("/api/chat/ws", handler.handleWebSocketByRoom)

	// Chat grupal por curso.
	router.Get("/courses/{courseId}/messages", handler.handleHistoryByCourse)
	router.Post("/courses/{courseId}/messages", handler.handleCreateMessageByCourse)
	router.Get("/ws/courses/{courseId}", handler.handleWebSocketByCourse)

	// Chat privado teacher <-> student por curso.
	router.Get("/courses/{courseId}/private/contacts", handler.handlePrivateContactsByCourse)
	router.Get("/courses/{courseId}/private/{otherUserId}/messages", handler.handleHistoryByPrivateCourse)
	router.Post("/courses/{courseId}/private/{otherUserId}/messages", handler.handleCreateMessageByPrivateCourse)
	router.Get("/ws/courses/{courseId}/private/{otherUserId}", handler.handleWebSocketByPrivateCourse)

	return router
}

func (h *Handler) handleHistoryByRoom(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(r, true)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid token")
		return
	}

	roomID := strings.TrimSpace(chi.URLParam(r, "room_id"))
	if !canAccessRoom(principal.UserID, roomID) {
		api.WriteError(w, http.StatusForbidden, "FORBIDDEN", "forbidden room")
		return
	}

	messages, err := h.service.GetMessagesByRoom(r.Context(), service.ListMessagesInput{
		RoomID: roomID,
		Limit:  parseLimit(r),
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response := h.buildMessageResponses(r.Context(), messages)
	api.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) handleHistoryByCourse(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(r, true)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid token")
		return
	}

	courseID := strings.TrimSpace(chi.URLParam(r, "courseId"))
	if err := h.access.CheckCourseAccess(r.Context(), principal, courseID); err != nil {
		h.writeAccessError(w, err)
		return
	}

	messages, err := h.service.GetMessagesByRoom(r.Context(), service.ListMessagesInput{
		RoomID: roomFromCourseID(courseID),
		Limit:  parseLimit(r),
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response := h.buildMessageResponses(r.Context(), messages)
	api.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) handleCreateMessageByCourse(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(r, false)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid token")
		return
	}

	courseID := strings.TrimSpace(chi.URLParam(r, "courseId"))
	if err := h.access.CheckCourseAccess(r.Context(), principal, courseID); err != nil {
		h.writeAccessError(w, err)
		return
	}

	req, attachments, err := decodeCreateMessageRequest(w, r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	message, err := h.service.CreateMessage(r.Context(), service.CreateMessageInput{
		RoomID:      roomFromCourseID(courseID),
		SenderID:    principal.UserID,
		SenderRole:  principal.Role,
		Content:     req.Content,
		Attachments: attachments,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response := h.buildMessageResponse(r.Context(), *message)
	if payload, err := json.Marshal(withRealtimeType(response)); err == nil {
		h.hub.Broadcast(message.RoomID, payload)
	}

	api.WriteJSON(w, http.StatusCreated, response)
}

func (h *Handler) handlePrivateContactsByCourse(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(r, true)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid token")
		return
	}

	courseID := strings.TrimSpace(chi.URLParam(r, "courseId"))
	contacts, err := h.access.ListPrivateContacts(r.Context(), principal, courseID)
	if err != nil {
		h.writeAccessError(w, err)
		return
	}

	response := make([]privateContactResponse, 0, len(contacts))
	for _, contact := range contacts {
		response = append(response, privateContactResponse{
			UserID: contact.UserID,
			Name:   contact.Name,
			Role:   normalizeRole(contact.Role),
		})
	}

	api.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) handleHistoryByPrivateCourse(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(r, true)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid token")
		return
	}

	courseID := strings.TrimSpace(chi.URLParam(r, "courseId"))
	otherUserID := strings.TrimSpace(chi.URLParam(r, "otherUserId"))
	roomID, err := h.access.ResolvePrivateRoom(r.Context(), principal, courseID, otherUserID)
	if err != nil {
		h.writeAccessError(w, err)
		return
	}

	messages, err := h.service.GetMessagesByRoom(r.Context(), service.ListMessagesInput{
		RoomID: roomID,
		Limit:  parseLimit(r),
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response := h.buildMessageResponses(r.Context(), messages)
	api.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) handleCreateMessageByPrivateCourse(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(r, false)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid token")
		return
	}

	courseID := strings.TrimSpace(chi.URLParam(r, "courseId"))
	otherUserID := strings.TrimSpace(chi.URLParam(r, "otherUserId"))
	roomID, err := h.access.ResolvePrivateRoom(r.Context(), principal, courseID, otherUserID)
	if err != nil {
		h.writeAccessError(w, err)
		return
	}

	req, attachments, err := decodeCreateMessageRequest(w, r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	message, err := h.service.CreateMessage(r.Context(), service.CreateMessageInput{
		RoomID:      roomID,
		SenderID:    principal.UserID,
		SenderRole:  principal.Role,
		Content:     req.Content,
		Attachments: attachments,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	response := h.buildMessageResponse(r.Context(), *message)
	if payload, err := json.Marshal(withRealtimeType(response)); err == nil {
		h.hub.Broadcast(message.RoomID, payload)
	}

	api.WriteJSON(w, http.StatusCreated, response)
}

func (h *Handler) handleDownloadAttachment(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(r, true)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid token")
		return
	}

	roomID := strings.TrimSpace(r.URL.Query().Get("room"))
	if roomID == "" {
		api.WriteError(w, http.StatusBadRequest, "ROOM_ID_REQUIRED", "room query param is required")
		return
	}
	if !canAccessRoom(principal.UserID, roomID) {
		api.WriteError(w, http.StatusForbidden, "FORBIDDEN", "forbidden room")
		return
	}

	if strings.HasPrefix(roomID, "class_") {
		if err := h.access.CheckCourseAccess(r.Context(), principal, courseFromRoomID(roomID)); err != nil {
			h.writeAccessError(w, err)
			return
		}
	}

	attachment, err := h.service.GetAttachment(r.Context(), roomID, chi.URLParam(r, "attachmentId"))
	if err != nil {
		h.writeServiceError(w, err)
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

func (h *Handler) handleWebSocketByRoom(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(r, true)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid token")
		return
	}

	roomID := strings.TrimSpace(r.URL.Query().Get("room"))
	if roomID == "" {
		api.WriteError(w, http.StatusBadRequest, "ROOM_ID_REQUIRED", "room query param is required")
		return
	}
	if !canAccessRoom(principal.UserID, roomID) {
		api.WriteError(w, http.StatusForbidden, "FORBIDDEN", "forbidden room")
		return
	}

	courseID := courseFromRoomID(roomID)
	if strings.HasPrefix(roomID, "class_") {
		if err := h.access.CheckCourseAccess(r.Context(), principal, courseID); err != nil {
			h.writeAccessError(w, err)
			return
		}
	}

	h.startWebSocket(w, r, principal, roomID, courseID)
}

func (h *Handler) handleWebSocketByCourse(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(r, true)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid token")
		return
	}

	courseID := strings.TrimSpace(chi.URLParam(r, "courseId"))
	if err := h.access.CheckCourseAccess(r.Context(), principal, courseID); err != nil {
		h.writeAccessError(w, err)
		return
	}

	h.startWebSocket(w, r, principal, roomFromCourseID(courseID), courseID)
}

func (h *Handler) handleWebSocketByPrivateCourse(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalFromRequest(r, true)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid token")
		return
	}

	courseID := strings.TrimSpace(chi.URLParam(r, "courseId"))
	otherUserID := strings.TrimSpace(chi.URLParam(r, "otherUserId"))
	roomID, err := h.access.ResolvePrivateRoom(r.Context(), principal, courseID, otherUserID)
	if err != nil {
		h.writeAccessError(w, err)
		return
	}

	h.startWebSocket(w, r, principal, roomID, courseID)
}

func (h *Handler) startWebSocket(w http.ResponseWriter, r *http.Request, principal Principal, roomID, courseID string) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error(r.Context(), "websocket upgrade failed", map[string]any{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	client := ws.NewClient(
		conn,
		h.hub,
		h.service,
		roomID,
		courseID,
		principal.UserID,
		normalizeRole(principal.Role),
		h.resolveSenderName(r.Context(), principal.UserID),
		h.log,
	)
	h.hub.Register(client)

	go func() {
		defer cancel()
		client.WritePump(ctx)
	}()

	client.ReadPump(ctx)
	cancel()
}

func (h *Handler) principalFromRequest(r *http.Request, allowQuery bool) (Principal, bool) {
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" && allowQuery {
		token = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	if token == "" {
		return Principal{}, false
	}

	principal, err := h.validate.Validate(token)
	if err != nil {
		return Principal{}, false
	}
	return principal, true
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrRoomIDRequired),
		errors.Is(err, service.ErrContentRequired),
		errors.Is(err, service.ErrContentTooLong),
		errors.Is(err, service.ErrSenderIDRequired),
		errors.Is(err, service.ErrInvalidSenderID),
		errors.Is(err, service.ErrInvalidSenderRole),
		errors.Is(err, service.ErrInvalidLimit),
		errors.Is(err, service.ErrTooManyAttachments),
		errors.Is(err, service.ErrAttachmentTooLarge),
		errors.Is(err, service.ErrAttachmentNameRequired),
		errors.Is(err, service.ErrAttachmentDataRequired):
		api.WriteError(w, http.StatusBadRequest, api.CodeFromMessage(err.Error()), err.Error())
	case errors.Is(err, service.ErrAttachmentNotFound):
		api.WriteError(w, http.StatusNotFound, "ATTACHMENT_NOT_FOUND", err.Error())
	default:
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "internal server error")
	}
}

func (h *Handler) writeAccessError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrCourseIDRequired), errors.Is(err, ErrOtherUserIDRequired):
		api.WriteError(w, http.StatusBadRequest, api.CodeFromMessage(err.Error()), err.Error())
	case errors.Is(err, ErrUserNotFound):
		api.WriteError(w, http.StatusNotFound, "USER_NOT_FOUND", err.Error())
	case errors.Is(err, ErrPrivateChatNotAllowed):
		api.WriteError(w, http.StatusForbidden, "PRIVATE_CHAT_NOT_ALLOWED", err.Error())
	case errors.Is(err, ErrForbidden):
		api.WriteError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
	default:
		h.log.Error(context.Background(), "access check failed", map[string]any{"error": err.Error()})
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "internal server error")
	}
}

func parseLimit(r *http.Request) int {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return 50
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 50
	}
	return limit
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if len(header) < len("Bearer ")+1 || !strings.EqualFold(header[:len("Bearer ")], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(header[len("Bearer "):])
}

func canAccessRoom(userID, roomID string) bool {
	userID = strings.TrimSpace(userID)
	roomID = strings.TrimSpace(roomID)
	if userID == "" || roomID == "" {
		return false
	}

	if strings.HasPrefix(roomID, "private:") {
		parts := strings.Split(roomID, ":")
		if len(parts) != 4 {
			return false
		}
		return userID == strings.TrimSpace(parts[2]) || userID == strings.TrimSpace(parts[3])
	}

	if strings.HasPrefix(roomID, "private_") {
		parts := strings.Split(roomID, "_")
		if len(parts) < 3 {
			return false
		}
		left := strings.TrimSpace(parts[len(parts)-2])
		right := strings.TrimSpace(parts[len(parts)-1])
		return userID == left || userID == right
	}

	return true
}

func roomFromCourseID(courseID string) string {
	return "class_" + strings.TrimSpace(courseID)
}

func courseFromRoomID(roomID string) string {
	roomID = strings.TrimSpace(roomID)
	if strings.HasPrefix(roomID, "class_") {
		return strings.TrimPrefix(roomID, "class_")
	}
	if strings.HasPrefix(roomID, "private:") {
		parts := strings.Split(roomID, ":")
		if len(parts) == 4 {
			return strings.TrimSpace(parts[1])
		}
	}
	return roomID
}

func (h *Handler) buildMessageResponses(ctx context.Context, messages []domain.Message) []messageResponse {
	senderNames := h.resolveSenderNames(ctx, messages)
	response := make([]messageResponse, 0, len(messages))
	for _, msg := range messages {
		response = append(response, toMessageResponse(msg, senderNames[msg.SenderID]))
	}
	return response
}

func (h *Handler) buildMessageResponse(ctx context.Context, msg domain.Message) messageResponse {
	return toMessageResponse(msg, h.resolveSenderName(ctx, msg.SenderID))
}

func (h *Handler) resolveSenderNames(ctx context.Context, messages []domain.Message) map[string]string {
	senderNames := make(map[string]string, len(messages))
	seen := make(map[string]struct{}, len(messages))
	for _, msg := range messages {
		senderID := strings.TrimSpace(msg.SenderID)
		if senderID == "" {
			continue
		}
		if _, ok := seen[senderID]; ok {
			continue
		}
		seen[senderID] = struct{}{}
		senderNames[senderID] = h.resolveSenderName(ctx, senderID)
	}
	return senderNames
}

func (h *Handler) resolveSenderName(ctx context.Context, senderID string) string {
	senderID = strings.TrimSpace(senderID)
	if senderID == "" || h.users == nil {
		return ""
	}

	user, err := h.users.GetUser(ctx, senderID)
	if err != nil || user == nil {
		return ""
	}

	return strings.TrimSpace(user.Name)
}

func toMessageResponse(msg domain.Message, senderName string) messageResponse {
	attachments := make([]attachmentResponse, 0, len(msg.Attachments))
	for _, attachment := range msg.Attachments {
		attachments = append(attachments, attachmentResponse{
			ID:          attachment.ID,
			Kind:        attachment.Kind,
			FileName:    attachment.FileName,
			ContentType: attachment.ContentType,
			SizeBytes:   attachment.SizeBytes,
			URL: "/chat/attachments/" +
				url.PathEscape(attachment.ID) +
				"?room=" +
				url.QueryEscape(msg.RoomID),
			CreatedAt: attachment.CreatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"),
		})
	}

	return messageResponse{
		ID:          msg.ID,
		RoomID:      msg.RoomID,
		CourseID:    courseFromRoomID(msg.RoomID),
		SenderID:    msg.SenderID,
		SenderName:  strings.TrimSpace(senderName),
		SenderRole:  normalizeRole(msg.SenderRole),
		Content:     msg.Content,
		Attachments: attachments,
		CreatedAt:   msg.CreatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"),
	}
}

func withRealtimeType(message messageResponse) map[string]any {
	return map[string]any{
		"type":        "message",
		"id":          message.ID,
		"roomId":      message.RoomID,
		"courseId":    message.CourseID,
		"senderId":    message.SenderID,
		"senderName":  message.SenderName,
		"senderRole":  message.SenderRole,
		"content":     message.Content,
		"attachments": message.Attachments,
		"createdAt":   message.CreatedAt,
	}
}

func decodeCreateMessageRequest(w http.ResponseWriter, r *http.Request) (createMessageRequest, []service.AttachmentInput, error) {
	if !isMultipartRequest(r) {
		var req createMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return createMessageRequest{}, nil, errors.New("invalid json body")
		}
		return req, nil, nil
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxChatUploadBytes)
	if err := r.ParseMultipartForm(maxChatUploadBytes); err != nil {
		return createMessageRequest{}, nil, errors.New("invalid multipart body")
	}

	attachments, err := readUploadedAttachments(r.MultipartForm.File["attachments"])
	if err != nil {
		return createMessageRequest{}, nil, err
	}

	return createMessageRequest{
		Content: strings.TrimSpace(r.FormValue("content")),
	}, attachments, nil
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
