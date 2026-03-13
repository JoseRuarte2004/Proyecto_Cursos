package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"

	"proyecto-cursos/internal/platform/validation"
	"proyecto-cursos/services/users-api/internal/domain"
)

var (
	ErrNameRequired                  = errors.New("name is required")
	ErrInvalidEmail                  = errors.New("invalid email")
	ErrPasswordTooShort              = errors.New("password must be at least 8 characters")
	ErrEmailAlreadyExists            = errors.New("email already exists")
	ErrInvalidCredentials            = errors.New("invalid credentials")
	ErrEmailNotVerified              = errors.New("email not verified")
	ErrUserNotFound                  = errors.New("user not found")
	ErrInvalidRole                   = errors.New("invalid role")
	ErrTokenRequired                 = errors.New("token is required")
	ErrEmailVerificationTokenInvalid = errors.New("email verification token is invalid or expired")
	ErrVerificationCodeInvalid       = errors.New("verification code is invalid or expired")
	ErrPasswordResetCodeInvalid      = errors.New("password reset code is invalid or expired")
	ErrPasswordResetTokenInvalid     = errors.New("password reset token is invalid or expired")
	defaultEmailVerifyTTL            = 24 * time.Hour
	defaultVerificationCodeTTL       = 15 * time.Minute
	defaultPasswordResetTTL          = 60 * time.Minute
)

type UserRepository interface {
	Create(ctx context.Context, user domain.User) (*domain.User, error)
	CreateWithEmailVerificationToken(ctx context.Context, user domain.User, token domain.EmailVerificationToken) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, userID string) (*domain.User, error)
	List(ctx context.Context) ([]domain.User, error)
	UpdateRole(ctx context.Context, userID string, role domain.Role) (*domain.User, error)
	CreateEmailVerificationToken(ctx context.Context, token domain.EmailVerificationToken) error
	ConsumeEmailVerificationToken(ctx context.Context, tokenHash string, now time.Time) error
	CreatePasswordResetToken(ctx context.Context, token domain.PasswordResetToken) error
	ConsumePasswordResetToken(ctx context.Context, tokenHash, passwordHash string, now time.Time) error
	UpdatePasswordByEmail(ctx context.Context, email, passwordHash string, now time.Time) error
	IsEmailVerified(ctx context.Context, userID string) (bool, error)
	MarkEmailVerifiedByEmail(ctx context.Context, email string, now time.Time) error
}

type AuditLogRepository interface {
	Create(ctx context.Context, auditLog domain.AuditLog) error
}

type PasswordManager interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type TokenIssuer interface {
	Issue(userID string, role domain.Role) (string, error)
}

type SecureTokenGenerator interface {
	Generate() (string, string, error)
}

type VerificationCodeStore interface {
	Set(ctx context.Context, email, code string, ttl time.Duration) error
	Get(ctx context.Context, email string) (code string, found bool, err error)
	Delete(ctx context.Context, email string) error
}

type PendingRegistrationStore interface {
	Set(ctx context.Context, email string, pending PendingRegistration, ttl time.Duration) error
	Get(ctx context.Context, email string) (PendingRegistration, bool, error)
	Delete(ctx context.Context, email string) error
}

type PendingRegistration struct {
	Name         string
	PasswordHash string
	Phone        string
	DNI          string
	Address      string
}

type RegisterInput struct {
	Name     string
	Email    string
	Password string
	Phone    string
	DNI      string
	Address  string
}

type LoginInput struct {
	Email    string
	Password string
}

type RegisterResult struct {
	User              *domain.User
	VerificationToken string
}

type RegisterWithCodeResult struct {
	User             *domain.User
	VerificationCode string
}

type EmailVerificationRequestResult struct {
	Email             string
	VerificationToken string
}

type PasswordResetRequestResult struct {
	Email      string
	ResetToken string
}

type PasswordResetCodeRequestResult struct {
	Email     string
	ResetCode string
}

type UserService struct {
	users                UserRepository
	audits               AuditLogRepository
	password             PasswordManager
	tokens               TokenIssuer
	verificationCodes    VerificationCodeStore
	pendingRegistrations PendingRegistrationStore
	passwordResetCodes   VerificationCodeStore
	emailVerifyTTL       time.Duration
	verificationCodeTTL  time.Duration
	passwordResetCodeTTL time.Duration
	resetTTL             time.Duration
	requireVerifiedLogin bool
	tokenGenerator       SecureTokenGenerator
	now                  func() time.Time
}

type UserServiceOption func(*UserService)

func WithVerificationCodeStore(store VerificationCodeStore, ttl time.Duration) UserServiceOption {
	return func(s *UserService) {
		if store != nil {
			s.verificationCodes = store
		}
		if ttl > 0 {
			s.verificationCodeTTL = ttl
		}
	}
}

func WithPasswordResetCodeStore(store VerificationCodeStore, ttl time.Duration) UserServiceOption {
	return func(s *UserService) {
		if store != nil {
			s.passwordResetCodes = store
		}
		if ttl > 0 {
			s.passwordResetCodeTTL = ttl
		}
	}
}

func WithPendingRegistrationStore(store PendingRegistrationStore, ttl time.Duration) UserServiceOption {
	return func(s *UserService) {
		if store != nil {
			s.pendingRegistrations = store
		}
		if ttl > 0 {
			s.verificationCodeTTL = ttl
		}
	}
}

func NewUserService(users UserRepository, audits AuditLogRepository, password PasswordManager, tokens TokenIssuer, emailVerifyTTL, resetTTL time.Duration, requireVerifiedLogin bool, opts ...UserServiceOption) *UserService {
	if emailVerifyTTL <= 0 {
		emailVerifyTTL = defaultEmailVerifyTTL
	}
	if resetTTL <= 0 {
		resetTTL = defaultPasswordResetTTL
	}

	svc := &UserService{
		users:                users,
		audits:               audits,
		password:             password,
		tokens:               tokens,
		verificationCodes:    noopVerificationCodeStore{},
		pendingRegistrations: noopPendingRegistrationStore{},
		passwordResetCodes:   noopVerificationCodeStore{},
		emailVerifyTTL:       emailVerifyTTL,
		verificationCodeTTL:  defaultVerificationCodeTTL,
		passwordResetCodeTTL: defaultVerificationCodeTTL,
		resetTTL:             resetTTL,
		requireVerifiedLogin: requireVerifiedLogin,
		tokenGenerator:       randomTokenGenerator{},
		now:                  time.Now().UTC,
	}

	for _, opt := range opts {
		opt(svc)
	}

	return svc
}

func (s *UserService) Register(ctx context.Context, input RegisterInput) (*RegisterResult, error) {
	name := strings.TrimSpace(input.Name)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := strings.TrimSpace(input.Password)

	if name == "" {
		return nil, ErrNameRequired
	}

	if !validation.IsEmail(email) {
		return nil, ErrInvalidEmail
	}

	if len(password) < 8 {
		return nil, ErrPasswordTooShort
	}

	passwordHash, err := s.password.Hash(password)
	if err != nil {
		return nil, err
	}

	now := s.now()
	rawToken, tokenHash, err := s.tokenGenerator.Generate()
	if err != nil {
		return nil, err
	}

	user := domain.User{
		ID:            uuid.NewString(),
		Name:          name,
		Email:         email,
		PasswordHash:  passwordHash,
		IsVerified:    false,
		EmailVerified: false,
		Role:          domain.RoleStudent,
		Phone:         strings.TrimSpace(input.Phone),
		DNI:           strings.TrimSpace(input.DNI),
		Address:       strings.TrimSpace(input.Address),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	verificationToken := domain.EmailVerificationToken{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(s.emailVerifyTTL),
		CreatedAt: now,
	}

	createdUser, err := s.users.CreateWithEmailVerificationToken(ctx, user, verificationToken)
	if err != nil {
		return nil, err
	}

	return &RegisterResult{
		User:              createdUser,
		VerificationToken: rawToken,
	}, nil
}

func (s *UserService) RegisterWithVerificationCode(ctx context.Context, input RegisterInput) (*RegisterWithCodeResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := strings.TrimSpace(input.Password)

	if !validation.IsEmail(email) {
		return nil, ErrInvalidEmail
	}
	if len(password) < 8 {
		return nil, ErrPasswordTooShort
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = inferNameFromEmail(email)
	}
	if name == "" {
		return nil, ErrNameRequired
	}

	existingUser, err := s.users.GetByEmail(ctx, email)
	switch {
	case err == nil && existingUser != nil:
		return nil, ErrEmailAlreadyExists
	case err != nil && !errors.Is(err, ErrUserNotFound):
		return nil, err
	}

	passwordHash, err := s.password.Hash(password)
	if err != nil {
		return nil, err
	}

	pending := PendingRegistration{
		Name:         name,
		PasswordHash: passwordHash,
		Phone:        strings.TrimSpace(input.Phone),
		DNI:          strings.TrimSpace(input.DNI),
		Address:      strings.TrimSpace(input.Address),
	}

	if err := s.pendingRegistrations.Set(ctx, email, pending, s.verificationCodeTTL); err != nil {
		return nil, err
	}

	code, err := generateVerificationCode()
	if err != nil {
		return nil, err
	}
	if err := s.verificationCodes.Set(ctx, email, code, s.verificationCodeTTL); err != nil {
		_ = s.pendingRegistrations.Delete(ctx, email)
		return nil, err
	}

	now := s.now()
	user := &domain.User{
		ID:            uuid.NewString(),
		Name:          name,
		Email:         email,
		PasswordHash:  passwordHash,
		IsVerified:    false,
		EmailVerified: false,
		Role:          domain.RoleStudent,
		Phone:         pending.Phone,
		DNI:           pending.DNI,
		Address:       pending.Address,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	return &RegisterWithCodeResult{
		User:             user,
		VerificationCode: code,
	}, nil
}

func (s *UserService) Login(ctx context.Context, input LoginInput) (string, *domain.User, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return "", nil, ErrInvalidCredentials
		}
		return "", nil, err
	}

	if err := s.password.Compare(user.PasswordHash, input.Password); err != nil {
		return "", nil, ErrInvalidCredentials
	}
	if s.requireVerifiedLogin && !user.EmailVerified {
		return "", nil, ErrEmailNotVerified
	}

	token, err := s.tokens.Issue(user.ID, user.Role)
	if err != nil {
		return "", nil, err
	}

	return token, user, nil
}

func (s *UserService) VerifyEmail(ctx context.Context, token string) error {
	tokenHash, err := hashOpaqueToken(token)
	if err != nil {
		return err
	}

	return s.users.ConsumeEmailVerificationToken(ctx, tokenHash, s.now())
}

func (s *UserService) VerifyEmailCode(ctx context.Context, email, code string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	code = strings.TrimSpace(code)

	if !validation.IsEmail(email) {
		return ErrInvalidEmail
	}
	if !isSixDigitCode(code) {
		return ErrVerificationCodeInvalid
	}

	storedCode, found, err := s.verificationCodes.Get(ctx, email)
	if err != nil {
		return err
	}
	if !found || subtle.ConstantTimeCompare([]byte(storedCode), []byte(code)) != 1 {
		return ErrVerificationCodeInvalid
	}

	pending, found, err := s.pendingRegistrations.Get(ctx, email)
	if err != nil {
		return err
	}
	if !found {
		return ErrVerificationCodeInvalid
	}

	now := s.now()
	emailVerifiedAt := now
	_, err = s.users.Create(ctx, domain.User{
		ID:              uuid.NewString(),
		Name:            strings.TrimSpace(pending.Name),
		Email:           email,
		PasswordHash:    pending.PasswordHash,
		IsVerified:      true,
		EmailVerified:   true,
		EmailVerifiedAt: &emailVerifiedAt,
		Role:            domain.RoleStudent,
		Phone:           strings.TrimSpace(pending.Phone),
		DNI:             strings.TrimSpace(pending.DNI),
		Address:         strings.TrimSpace(pending.Address),
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err != nil {
		return err
	}

	_ = s.verificationCodes.Delete(ctx, email)
	_ = s.pendingRegistrations.Delete(ctx, email)

	return nil
}

func (s *UserService) RequestEmailVerification(ctx context.Context, email string) (*EmailVerificationRequestResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validation.IsEmail(email) {
		return nil, nil
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if user.EmailVerified {
		return nil, nil
	}

	rawToken, tokenHash, err := s.tokenGenerator.Generate()
	if err != nil {
		return nil, err
	}

	now := s.now()
	if err := s.users.CreateEmailVerificationToken(ctx, domain.EmailVerificationToken{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(s.emailVerifyTTL),
		CreatedAt: now,
	}); err != nil {
		return nil, err
	}

	return &EmailVerificationRequestResult{
		Email:             user.Email,
		VerificationToken: rawToken,
	}, nil
}

func (s *UserService) ForgotPassword(ctx context.Context, email string) (*PasswordResetRequestResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validation.IsEmail(email) {
		return nil, nil
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, nil
		}
		return nil, err
	}

	rawToken, tokenHash, err := s.tokenGenerator.Generate()
	if err != nil {
		return nil, err
	}

	now := s.now()
	if err := s.users.CreatePasswordResetToken(ctx, domain.PasswordResetToken{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(s.resetTTL),
		CreatedAt: now,
	}); err != nil {
		return nil, err
	}

	return &PasswordResetRequestResult{
		Email:      user.Email,
		ResetToken: rawToken,
	}, nil
}

func (s *UserService) RequestPasswordResetCode(ctx context.Context, email string) (*PasswordResetCodeRequestResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validation.IsEmail(email) {
		return nil, nil
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, nil
		}
		return nil, err
	}

	code, err := generateVerificationCode()
	if err != nil {
		return nil, err
	}
	if err := s.passwordResetCodes.Set(ctx, user.Email, code, s.passwordResetCodeTTL); err != nil {
		return nil, err
	}

	return &PasswordResetCodeRequestResult{
		Email:     user.Email,
		ResetCode: code,
	}, nil
}

func (s *UserService) ResetPassword(ctx context.Context, token, newPassword string) error {
	newPassword = strings.TrimSpace(newPassword)
	if len(newPassword) < 8 {
		return ErrPasswordTooShort
	}

	tokenHash, err := hashOpaqueToken(token)
	if err != nil {
		return err
	}

	passwordHash, err := s.password.Hash(newPassword)
	if err != nil {
		return err
	}

	return s.users.ConsumePasswordResetToken(ctx, tokenHash, passwordHash, s.now())
}

func (s *UserService) ResetPasswordWithCode(ctx context.Context, email, code, newPassword string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	code = strings.TrimSpace(code)
	newPassword = strings.TrimSpace(newPassword)

	if !validation.IsEmail(email) {
		return ErrInvalidEmail
	}
	if !isSixDigitCode(code) {
		return ErrPasswordResetCodeInvalid
	}
	if len(newPassword) < 8 {
		return ErrPasswordTooShort
	}

	storedCode, found, err := s.passwordResetCodes.Get(ctx, email)
	if err != nil {
		return err
	}
	if !found || subtle.ConstantTimeCompare([]byte(storedCode), []byte(code)) != 1 {
		return ErrPasswordResetCodeInvalid
	}

	passwordHash, err := s.password.Hash(newPassword)
	if err != nil {
		return err
	}

	if err := s.users.UpdatePasswordByEmail(ctx, email, passwordHash, s.now()); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return ErrPasswordResetCodeInvalid
		}
		return err
	}
	if err := s.passwordResetCodes.Delete(ctx, email); err != nil {
		return err
	}

	return nil
}

func (s *UserService) IsEmailVerified(ctx context.Context, userID string) (bool, error) {
	return s.users.IsEmailVerified(ctx, strings.TrimSpace(userID))
}

func (s *UserService) GetProfile(ctx context.Context, userID string) (*domain.User, error) {
	return s.users.GetByID(ctx, userID)
}

func (s *UserService) ListUsers(ctx context.Context) ([]domain.User, error) {
	return s.users.List(ctx)
}

func (s *UserService) GetSensitiveUser(ctx context.Context, adminID, targetUserID string, meta domain.AuditMeta) (*domain.User, error) {
	user, err := s.users.GetByID(ctx, targetUserID)
	if err != nil {
		return nil, err
	}

	if err := s.audits.Create(ctx, s.newAuditLog(adminID, targetUserID, domain.ActionViewSensitive, meta)); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) ChangeRole(ctx context.Context, adminID, targetUserID string, role domain.Role, meta domain.AuditMeta) (*domain.User, error) {
	if !role.IsValid() {
		return nil, ErrInvalidRole
	}

	user, err := s.users.UpdateRole(ctx, targetUserID, role)
	if err != nil {
		return nil, err
	}

	if err := s.audits.Create(ctx, s.newAuditLog(adminID, targetUserID, domain.ActionChangeRole, meta)); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) newAuditLog(adminID, targetUserID string, action domain.AuditAction, meta domain.AuditMeta) domain.AuditLog {
	return domain.AuditLog{
		ID:           uuid.NewString(),
		AdminID:      adminID,
		Action:       action,
		TargetUserID: targetUserID,
		Timestamp:    s.now(),
		RequestID:    meta.RequestID,
		IP:           meta.IP,
	}
}

func generateVerificationCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%06d", n.Int64()), nil
}

func isSixDigitCode(value string) bool {
	if len(value) != 6 {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}

	return true
}

func inferNameFromEmail(email string) string {
	local, _, ok := strings.Cut(email, "@")
	if !ok {
		return ""
	}

	return strings.TrimSpace(local)
}

type noopVerificationCodeStore struct{}

func (noopVerificationCodeStore) Set(context.Context, string, string, time.Duration) error {
	return errors.New("verification code store not configured")
}

func (noopVerificationCodeStore) Get(context.Context, string) (string, bool, error) {
	return "", false, errors.New("verification code store not configured")
}

func (noopVerificationCodeStore) Delete(context.Context, string) error {
	return errors.New("verification code store not configured")
}

type noopPendingRegistrationStore struct{}

func (noopPendingRegistrationStore) Set(context.Context, string, PendingRegistration, time.Duration) error {
	return errors.New("pending registration store not configured")
}

func (noopPendingRegistrationStore) Get(context.Context, string) (PendingRegistration, bool, error) {
	return PendingRegistration{}, false, nil
}

func (noopPendingRegistrationStore) Delete(context.Context, string) error {
	return errors.New("pending registration store not configured")
}
