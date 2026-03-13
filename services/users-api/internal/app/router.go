package app

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"proyecto-cursos/internal/platform/httpx"
	"proyecto-cursos/internal/platform/internalauth"
	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/internal/platform/requestid"
	"proyecto-cursos/internal/platform/server"
	platformstore "proyecto-cursos/internal/platform/store"
	"proyecto-cursos/services/users-api/internal/domain"
	"proyecto-cursos/services/users-api/internal/service"
)

type Handler struct {
	service    *service.UserService
	logger     *logger.Logger
	mailer     Mailer
	appBaseURL string
}

func NewHTTPHandler(log *logger.Logger, userService *service.UserService, jwtManager *JWTManager, mailer Mailer, appBaseURL string, db *sql.DB, redisClient *redis.Client, internalToken string) http.Handler {
	readyFn := func(ctx context.Context) error {
		if err := db.PingContext(ctx); err != nil {
			return err
		}

		return platformstore.PingRedis(ctx, redisClient)
	}

	router := server.NewRouter("users-api", log, readyFn)
	handler := &Handler{
		service:    userService,
		logger:     log,
		mailer:     mailer,
		appBaseURL: strings.TrimRight(strings.TrimSpace(appBaseURL), "/"),
	}

	router.Post("/auth/register", handler.handleRegister)
	router.Post("/auth/login", handler.handleLogin)
	router.Post("/register", handler.handleRegisterWithCode)
	router.Post("/verify", handler.handleVerifyWithCode)
	router.Get("/auth/verify-email", handler.handleVerifyEmail)
	router.Post("/auth/verify-email/request", handler.handleVerifyEmailRequest)
	router.Post("/auth/password/forgot", handler.handleForgotPassword)
	router.Post("/auth/password/reset", handler.handleResetPassword)
	router.Post("/auth/password/forgot/code", handler.handleForgotPasswordWithCode)
	router.Post("/auth/password/reset/code", handler.handleResetPasswordWithCode)
	router.Group(func(r chi.Router) {
		r.Use(internalauth.RequireToken(internalToken))
		r.Get("/internal/users/{id}", handler.handleInternalGetUser)
		r.Get("/internal/users/{id}/email-verified", handler.handleInternalEmailVerified)
	})

	router.Group(func(r chi.Router) {
		r.Use(AuthRequired(jwtManager))
		r.Get("/me", handler.handleMe)

		r.Route("/admin", func(r chi.Router) {
			r.Use(RequireRoles(domain.RoleAdmin))
			r.Get("/users", handler.handleAdminListUsers)
			r.Get("/users/{id}", handler.handleAdminGetUser)
			r.Patch("/users/{id}/role", handler.handleAdminChangeRole)
		})
	})

	return router
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Phone    string `json:"phone"`
	DNI      string `json:"dni"`
	Address  string `json:"address"`
}

type registerWithCodeRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Phone    string `json:"phone"`
	DNI      string `json:"dni"`
	Address  string `json:"address"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type emailRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

type resetPasswordWithCodeRequest struct {
	Email       string `json:"email"`
	Code        string `json:"code"`
	NewPassword string `json:"newPassword"`
}

type verifyCodeRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type changeRoleRequest struct {
	Role domain.Role `json:"role"`
}

type authUserResponse struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Email      string      `json:"email"`
	IsVerified bool        `json:"isVerified"`
	Role       domain.Role `json:"role"`
}

type adminUserListResponse struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Email     string      `json:"email"`
	Role      domain.Role `json:"role"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

type adminUserDetailResponse struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Email     string      `json:"email"`
	Role      domain.Role `json:"role"`
	Phone     string      `json:"phone"`
	DNI       string      `json:"dni"`
	Address   string      `json:"address"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.Register(r.Context(), service.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		Phone:    req.Phone,
		DNI:      req.DNI,
		Address:  req.Address,
	})
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	h.sendVerificationEmail(r.Context(), result.User.Email, result.VerificationToken)
	httpx.WriteJSON(w, http.StatusCreated, toAuthUserResponse(result.User))
}

func (h *Handler) handleRegisterWithCode(w http.ResponseWriter, r *http.Request) {
	var req registerWithCodeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.RegisterWithVerificationCode(r.Context(), service.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		Phone:    req.Phone,
		DNI:      req.DNI,
		Address:  req.Address,
	})
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	h.sendVerificationCodeEmail(r.Context(), result.User.Email, result.VerificationCode)
	httpx.WriteJSON(w, http.StatusCreated, toAuthUserResponse(result.User))
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	token, user, err := h.service.Login(r.Context(), service.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  toAuthUserResponse(user),
	})
}

func (h *Handler) handleVerifyWithCode(w http.ResponseWriter, r *http.Request) {
	var req verifyCodeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.VerifyEmailCode(r.Context(), req.Email, req.Code); err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}

func (h *Handler) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	if err := h.service.VerifyEmail(r.Context(), r.URL.Query().Get("token")); err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}

func (h *Handler) handleVerifyEmailRequest(w http.ResponseWriter, r *http.Request) {
	var req emailRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.RequestEmailVerification(r.Context(), req.Email)
	if err != nil {
		h.logger.Error(r.Context(), "verification email request failed", map[string]any{
			"error": err.Error(),
		})
	}
	if result != nil {
		h.sendVerificationEmail(r.Context(), result.Email, result.VerificationToken)
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (h *Handler) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req emailRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.ForgotPassword(r.Context(), req.Email)
	if err != nil {
		h.logger.Error(r.Context(), "password forgot failed", map[string]any{
			"error": err.Error(),
		})
	}
	if result != nil {
		h.sendPasswordResetEmail(r.Context(), result.Email, result.ResetToken)
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (h *Handler) handleForgotPasswordWithCode(w http.ResponseWriter, r *http.Request) {
	var req emailRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.RequestPasswordResetCode(r.Context(), req.Email)
	if err != nil {
		h.logger.Error(r.Context(), "password forgot code failed", map[string]any{
			"error": err.Error(),
		})
	}
	if result != nil {
		h.sendPasswordResetCodeEmail(r.Context(), result.Email, result.ResetCode)
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (h *Handler) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "password_updated"})
}

func (h *Handler) handleResetPasswordWithCode(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordWithCodeRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.ResetPasswordWithCode(r.Context(), req.Email, req.Code, req.NewPassword); err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "password_updated"})
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing auth context")
		return
	}

	user, err := h.service.GetProfile(r.Context(), claims.UserID)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, toAuthUserResponse(user))
}

func (h *Handler) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.ListUsers(r.Context())
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	response := make([]adminUserListResponse, 0, len(users))
	for _, user := range users {
		response = append(response, toAdminUserListResponse(user))
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

func (h *Handler) handleAdminGetUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing auth context")
		return
	}

	userID := chi.URLParam(r, "id")
	user, err := h.service.GetSensitiveUser(r.Context(), claims.UserID, userID, auditMetaFromRequest(r))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, toAdminUserDetailResponse(user))
}

func (h *Handler) handleInternalGetUser(w http.ResponseWriter, r *http.Request) {
	user, err := h.service.GetProfile(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, toAuthUserResponse(user))
}

func (h *Handler) handleInternalEmailVerified(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	emailVerified, err := h.service.IsEmailVerified(r.Context(), userID)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"userId":        userID,
		"emailVerified": emailVerified,
	})
}

func (h *Handler) handleAdminChangeRole(w http.ResponseWriter, r *http.Request) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing auth context")
		return
	}

	var req changeRoleRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	userID := chi.URLParam(r, "id")
	user, err := h.service.ChangeRole(r.Context(), claims.UserID, userID, req.Role, auditMetaFromRequest(r))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, toAuthUserResponse(user))
}

func (h *Handler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrNameRequired),
		errors.Is(err, service.ErrInvalidEmail),
		errors.Is(err, service.ErrPasswordTooShort),
		errors.Is(err, service.ErrInvalidRole),
		errors.Is(err, service.ErrTokenRequired),
		errors.Is(err, service.ErrEmailVerificationTokenInvalid),
		errors.Is(err, service.ErrVerificationCodeInvalid),
		errors.Is(err, service.ErrPasswordResetCodeInvalid),
		errors.Is(err, service.ErrPasswordResetTokenInvalid):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrEmailAlreadyExists):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrEmailNotVerified):
		httpx.WriteError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrInvalidCredentials):
		httpx.WriteError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, service.ErrUserNotFound):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	default:
		h.logger.Error(r.Context(), "request failed", map[string]any{
			"error": err.Error(),
		})
		httpx.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}

func (h *Handler) sendVerificationEmail(ctx context.Context, email, token string) {
	if h.mailer == nil {
		return
	}

	link := h.appBaseURL + "/auth/verify-email?token=" + url.QueryEscape(token)
	message := MailMessage{
		To:       email,
		Subject:  "Verify your email",
		TextBody: "Use this link to verify your email: " + link,
		HTMLBody: "<p>Use this link to verify your email:</p><p><a href=\"" + link + "\">" + link + "</a></p>",
		Link:     link,
	}
	if err := h.mailer.Send(ctx, message); err != nil {
		h.logger.Error(ctx, "verification email send failed", map[string]any{
			"email": email,
			"error": err.Error(),
		})
	}
}

func (h *Handler) sendPasswordResetEmail(ctx context.Context, email, token string) {
	if h.mailer == nil {
		return
	}

	link := h.appBaseURL + "/reset-password?token=" + url.QueryEscape(token)
	message := MailMessage{
		To:       email,
		Subject:  "Reset your password",
		TextBody: "Use this link to reset your password: " + link,
		HTMLBody: "<p>Use this link to reset your password:</p><p><a href=\"" + link + "\">" + link + "</a></p>",
		Link:     link,
	}
	if err := h.mailer.Send(ctx, message); err != nil {
		h.logger.Error(ctx, "password reset email send failed", map[string]any{
			"email": email,
			"error": err.Error(),
		})
	}
}

func (h *Handler) sendVerificationCodeEmail(ctx context.Context, email, code string) {
	if h.mailer == nil {
		return
	}

	message := MailMessage{
		To:      email,
		Subject: "Your verification code",
		TextBody: "Use this code to verify your email: " + strings.TrimSpace(code) +
			". It expires in 15 minutes.",
		HTMLBody: "<p>Use this code to verify your email:</p><p><strong>" +
			strings.TrimSpace(code) +
			"</strong></p><p>It expires in 15 minutes.</p>",
	}
	if err := h.mailer.Send(ctx, message); err != nil {
		h.logger.Error(ctx, "verification code email send failed", map[string]any{
			"email": email,
			"error": err.Error(),
		})
	}
}

func (h *Handler) sendPasswordResetCodeEmail(ctx context.Context, email, code string) {
	if h.mailer == nil {
		return
	}

	message := MailMessage{
		To:      email,
		Subject: "Your password reset code",
		TextBody: "Use this code to reset your password: " + strings.TrimSpace(code) +
			". It expires in 15 minutes.",
		HTMLBody: "<p>Use this code to reset your password:</p><p><strong>" +
			strings.TrimSpace(code) +
			"</strong></p><p>It expires in 15 minutes.</p>",
	}
	if err := h.mailer.Send(ctx, message); err != nil {
		h.logger.Error(ctx, "password reset code email send failed", map[string]any{
			"email": email,
			"error": err.Error(),
		})
	}
}

func auditMetaFromRequest(r *http.Request) domain.AuditMeta {
	return domain.AuditMeta{
		RequestID: requestid.FromContext(r.Context()),
		IP:        clientIP(r),
	}
}

func clientIP(r *http.Request) string {
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		return forwardedFor
	}

	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

func toAuthUserResponse(user *domain.User) authUserResponse {
	return authUserResponse{
		ID:         user.ID,
		Name:       user.Name,
		Email:      user.Email,
		IsVerified: user.IsVerified || user.EmailVerified,
		Role:       user.Role,
	}
}

func toAdminUserListResponse(user domain.User) adminUserListResponse {
	return adminUserListResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Role:      user.Role,
		CreatedAt: user.CreatedAt.Format(timeLayout),
		UpdatedAt: user.UpdatedAt.Format(timeLayout),
	}
}

func toAdminUserDetailResponse(user *domain.User) adminUserDetailResponse {
	return adminUserDetailResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Role:      user.Role,
		Phone:     user.Phone,
		DNI:       user.DNI,
		Address:   user.Address,
		CreatedAt: user.CreatedAt.Format(timeLayout),
		UpdatedAt: user.UpdatedAt.Format(timeLayout),
	}
}

const timeLayout = time.RFC3339
