package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"proyecto-cursos/services/users-api/internal/domain"
)

type mockUserRepository struct {
	createFn                           func(ctx context.Context, user domain.User) (*domain.User, error)
	createWithEmailVerificationTokenFn func(ctx context.Context, user domain.User, token domain.EmailVerificationToken) (*domain.User, error)
	getByEmailFn                       func(ctx context.Context, email string) (*domain.User, error)
	getByIDFn                          func(ctx context.Context, userID string) (*domain.User, error)
	listFn                             func(ctx context.Context) ([]domain.User, error)
	updateRoleFn                       func(ctx context.Context, userID string, role domain.Role) (*domain.User, error)
	createEmailVerificationTokenFn     func(ctx context.Context, token domain.EmailVerificationToken) error
	consumeEmailVerificationTokenFn    func(ctx context.Context, tokenHash string, now time.Time) error
	createPasswordResetTokenFn         func(ctx context.Context, token domain.PasswordResetToken) error
	consumePasswordResetTokenFn        func(ctx context.Context, tokenHash, passwordHash string, now time.Time) error
	updatePasswordByEmailFn            func(ctx context.Context, email, passwordHash string, now time.Time) error
	isEmailVerifiedFn                  func(ctx context.Context, userID string) (bool, error)
	markEmailVerifiedByEmailFn         func(ctx context.Context, email string, now time.Time) error
}

func (m *mockUserRepository) Create(ctx context.Context, user domain.User) (*domain.User, error) {
	return m.createFn(ctx, user)
}

func (m *mockUserRepository) CreateWithEmailVerificationToken(ctx context.Context, user domain.User, token domain.EmailVerificationToken) (*domain.User, error) {
	return m.createWithEmailVerificationTokenFn(ctx, user, token)
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return m.getByEmailFn(ctx, email)
}

func (m *mockUserRepository) GetByID(ctx context.Context, userID string) (*domain.User, error) {
	return m.getByIDFn(ctx, userID)
}

func (m *mockUserRepository) List(ctx context.Context) ([]domain.User, error) {
	return m.listFn(ctx)
}

func (m *mockUserRepository) UpdateRole(ctx context.Context, userID string, role domain.Role) (*domain.User, error) {
	return m.updateRoleFn(ctx, userID, role)
}

func (m *mockUserRepository) CreateEmailVerificationToken(ctx context.Context, token domain.EmailVerificationToken) error {
	return m.createEmailVerificationTokenFn(ctx, token)
}

func (m *mockUserRepository) ConsumeEmailVerificationToken(ctx context.Context, tokenHash string, now time.Time) error {
	return m.consumeEmailVerificationTokenFn(ctx, tokenHash, now)
}

func (m *mockUserRepository) CreatePasswordResetToken(ctx context.Context, token domain.PasswordResetToken) error {
	return m.createPasswordResetTokenFn(ctx, token)
}

func (m *mockUserRepository) ConsumePasswordResetToken(ctx context.Context, tokenHash, passwordHash string, now time.Time) error {
	return m.consumePasswordResetTokenFn(ctx, tokenHash, passwordHash, now)
}

func (m *mockUserRepository) UpdatePasswordByEmail(ctx context.Context, email, passwordHash string, now time.Time) error {
	if m.updatePasswordByEmailFn == nil {
		return nil
	}

	return m.updatePasswordByEmailFn(ctx, email, passwordHash, now)
}

func (m *mockUserRepository) IsEmailVerified(ctx context.Context, userID string) (bool, error) {
	return m.isEmailVerifiedFn(ctx, userID)
}

func (m *mockUserRepository) MarkEmailVerifiedByEmail(ctx context.Context, email string, now time.Time) error {
	if m.markEmailVerifiedByEmailFn == nil {
		return nil
	}

	return m.markEmailVerifiedByEmailFn(ctx, email, now)
}

type mockAuditLogRepository struct {
	createFn func(ctx context.Context, auditLog domain.AuditLog) error
}

func (m *mockAuditLogRepository) Create(ctx context.Context, auditLog domain.AuditLog) error {
	return m.createFn(ctx, auditLog)
}

type fakePasswordManager struct {
	hashFn    func(password string) (string, error)
	compareFn func(hash, password string) error
}

func (f fakePasswordManager) Hash(password string) (string, error) {
	return f.hashFn(password)
}

func (f fakePasswordManager) Compare(hash, password string) error {
	return f.compareFn(hash, password)
}

type fakeTokenIssuer struct {
	issueFn func(userID string, role domain.Role) (string, error)
}

func (f fakeTokenIssuer) Issue(userID string, role domain.Role) (string, error) {
	return f.issueFn(userID, role)
}

type fakeSecureTokenGenerator struct {
	generateFn func() (string, string, error)
}

func (f fakeSecureTokenGenerator) Generate() (string, string, error) {
	return f.generateFn()
}

type fakeVerificationCodeStore struct {
	setFn    func(ctx context.Context, email, code string, ttl time.Duration) error
	getFn    func(ctx context.Context, email string) (string, bool, error)
	deleteFn func(ctx context.Context, email string) error
}

func (f fakeVerificationCodeStore) Set(ctx context.Context, email, code string, ttl time.Duration) error {
	if f.setFn == nil {
		return nil
	}

	return f.setFn(ctx, email, code, ttl)
}

func (f fakeVerificationCodeStore) Get(ctx context.Context, email string) (string, bool, error) {
	if f.getFn == nil {
		return "", false, nil
	}

	return f.getFn(ctx, email)
}

func (f fakeVerificationCodeStore) Delete(ctx context.Context, email string) error {
	if f.deleteFn == nil {
		return nil
	}

	return f.deleteFn(ctx, email)
}

type fakePendingRegistrationStore struct {
	setFn    func(ctx context.Context, email string, pending PendingRegistration, ttl time.Duration) error
	getFn    func(ctx context.Context, email string) (PendingRegistration, bool, error)
	deleteFn func(ctx context.Context, email string) error
}

func (f fakePendingRegistrationStore) Set(ctx context.Context, email string, pending PendingRegistration, ttl time.Duration) error {
	if f.setFn == nil {
		return nil
	}

	return f.setFn(ctx, email, pending, ttl)
}

func (f fakePendingRegistrationStore) Get(ctx context.Context, email string) (PendingRegistration, bool, error) {
	if f.getFn == nil {
		return PendingRegistration{}, false, nil
	}

	return f.getFn(ctx, email)
}

func (f fakePendingRegistrationStore) Delete(ctx context.Context, email string) error {
	if f.deleteFn == nil {
		return nil
	}

	return f.deleteFn(ctx, email)
}

func TestRegisterCreatesVerificationToken(t *testing.T) {
	t.Parallel()

	var createdUser domain.User
	var createdToken domain.EmailVerificationToken
	service := NewUserService(
		&mockUserRepository{
			createFn: func(context.Context, domain.User) (*domain.User, error) { return nil, nil },
			createWithEmailVerificationTokenFn: func(_ context.Context, user domain.User, token domain.EmailVerificationToken) (*domain.User, error) {
				createdUser = user
				createdToken = token
				return &user, nil
			},
			getByEmailFn:                    func(context.Context, string) (*domain.User, error) { return nil, nil },
			getByIDFn:                       func(context.Context, string) (*domain.User, error) { return nil, nil },
			listFn:                          func(context.Context) ([]domain.User, error) { return nil, nil },
			updateRoleFn:                    func(context.Context, string, domain.Role) (*domain.User, error) { return nil, nil },
			createEmailVerificationTokenFn:  func(context.Context, domain.EmailVerificationToken) error { return nil },
			consumeEmailVerificationTokenFn: func(context.Context, string, time.Time) error { return nil },
			createPasswordResetTokenFn:      func(context.Context, domain.PasswordResetToken) error { return nil },
			consumePasswordResetTokenFn:     func(context.Context, string, string, time.Time) error { return nil },
			isEmailVerifiedFn:               func(context.Context, string) (bool, error) { return false, nil },
		},
		&mockAuditLogRepository{
			createFn: func(context.Context, domain.AuditLog) error { return nil },
		},
		fakePasswordManager{
			hashFn: func(password string) (string, error) {
				return "hashed-" + password, nil
			},
			compareFn: func(string, string) error { return nil },
		},
		fakeTokenIssuer{
			issueFn: func(string, domain.Role) (string, error) { return "token", nil },
		},
		24*time.Hour,
		60*time.Minute,
		false,
	)
	service.now = func() time.Time { return time.Date(2026, time.February, 28, 10, 0, 0, 0, time.UTC) }
	service.tokenGenerator = fakeSecureTokenGenerator{
		generateFn: func() (string, string, error) {
			return "raw-verify-token", "hashed-verify-token", nil
		},
	}

	result, err := service.Register(context.Background(), RegisterInput{
		Name:     "Ada Lovelace",
		Email:    "ADA@example.com",
		Password: "superpass",
		Phone:    "12345",
		DNI:      "99887766",
		Address:  "Main St",
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.User.ID)
	require.Equal(t, "ada@example.com", result.User.Email)
	require.Equal(t, domain.RoleStudent, result.User.Role)
	require.Equal(t, "raw-verify-token", result.VerificationToken)
	require.Equal(t, "hashed-superpass", createdUser.PasswordHash)
	require.False(t, createdUser.EmailVerified)
	require.Equal(t, createdUser.ID, createdToken.UserID)
	require.Equal(t, "hashed-verify-token", createdToken.TokenHash)
	require.Equal(t, service.now().Add(24*time.Hour), createdToken.ExpiresAt)
}

func TestRegisterReturnsConflictWhenEmailAlreadyExists(t *testing.T) {
	t.Parallel()

	service := NewUserService(
		&mockUserRepository{
			createFn: func(context.Context, domain.User) (*domain.User, error) { return nil, nil },
			createWithEmailVerificationTokenFn: func(context.Context, domain.User, domain.EmailVerificationToken) (*domain.User, error) {
				return nil, ErrEmailAlreadyExists
			},
			getByEmailFn:                    func(context.Context, string) (*domain.User, error) { return nil, nil },
			getByIDFn:                       func(context.Context, string) (*domain.User, error) { return nil, nil },
			listFn:                          func(context.Context) ([]domain.User, error) { return nil, nil },
			updateRoleFn:                    func(context.Context, string, domain.Role) (*domain.User, error) { return nil, nil },
			createEmailVerificationTokenFn:  func(context.Context, domain.EmailVerificationToken) error { return nil },
			consumeEmailVerificationTokenFn: func(context.Context, string, time.Time) error { return nil },
			createPasswordResetTokenFn:      func(context.Context, domain.PasswordResetToken) error { return nil },
			consumePasswordResetTokenFn:     func(context.Context, string, string, time.Time) error { return nil },
			isEmailVerifiedFn:               func(context.Context, string) (bool, error) { return false, nil },
		},
		&mockAuditLogRepository{
			createFn: func(context.Context, domain.AuditLog) error { return nil },
		},
		fakePasswordManager{
			hashFn:    func(password string) (string, error) { return "hashed-" + password, nil },
			compareFn: func(string, string) error { return nil },
		},
		fakeTokenIssuer{
			issueFn: func(string, domain.Role) (string, error) { return "", nil },
		},
		24*time.Hour,
		60*time.Minute,
		false,
	)
	service.tokenGenerator = fakeSecureTokenGenerator{
		generateFn: func() (string, string, error) { return "token", "token-hash", nil },
	}

	_, err := service.Register(context.Background(), RegisterInput{
		Name:     "Ada Lovelace",
		Email:    "ada@example.com",
		Password: "superpass",
	})

	require.ErrorIs(t, err, ErrEmailAlreadyExists)
}

func TestVerifyEmailMarksUserVerifiedAndTokenUsed(t *testing.T) {
	t.Parallel()

	var consumedHash string
	var consumedAt time.Time
	service := NewUserService(
		&mockUserRepository{
			createFn: func(context.Context, domain.User) (*domain.User, error) { return nil, nil },
			createWithEmailVerificationTokenFn: func(context.Context, domain.User, domain.EmailVerificationToken) (*domain.User, error) {
				return nil, nil
			},
			getByEmailFn:                   func(context.Context, string) (*domain.User, error) { return nil, nil },
			getByIDFn:                      func(context.Context, string) (*domain.User, error) { return nil, nil },
			listFn:                         func(context.Context) ([]domain.User, error) { return nil, nil },
			updateRoleFn:                   func(context.Context, string, domain.Role) (*domain.User, error) { return nil, nil },
			createEmailVerificationTokenFn: func(context.Context, domain.EmailVerificationToken) error { return nil },
			consumeEmailVerificationTokenFn: func(_ context.Context, tokenHash string, now time.Time) error {
				consumedHash, consumedAt = tokenHash, now
				return nil
			},
			createPasswordResetTokenFn:  func(context.Context, domain.PasswordResetToken) error { return nil },
			consumePasswordResetTokenFn: func(context.Context, string, string, time.Time) error { return nil },
			isEmailVerifiedFn:           func(context.Context, string) (bool, error) { return false, nil },
		},
		&mockAuditLogRepository{createFn: func(context.Context, domain.AuditLog) error { return nil }},
		fakePasswordManager{
			hashFn:    func(string) (string, error) { return "", nil },
			compareFn: func(string, string) error { return nil },
		},
		fakeTokenIssuer{issueFn: func(string, domain.Role) (string, error) { return "", nil }},
		24*time.Hour,
		60*time.Minute,
		false,
	)
	frozenNow := time.Date(2026, time.February, 28, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return frozenNow }

	err := service.VerifyEmail(context.Background(), "verify-me")

	require.NoError(t, err)
	require.Equal(t, frozenNow, consumedAt)
	require.Equal(t, "d757afa8f14ef7434c81a75fd826537c6b13a8dadd65363b0fd10f1ec2cb211f", consumedHash)
}

func TestRegisterWithVerificationCodeStoresCodeInRedis(t *testing.T) {
	t.Parallel()

	var storedCodeEmail string
	var storedCode string
	var storedCodeTTL time.Duration
	var pendingEmail string
	var pendingData PendingRegistration
	var pendingTTL time.Duration

	service := NewUserService(
		&mockUserRepository{
			createFn: func(_ context.Context, user domain.User) (*domain.User, error) { return &user, nil },
			createWithEmailVerificationTokenFn: func(context.Context, domain.User, domain.EmailVerificationToken) (*domain.User, error) {
				return nil, nil
			},
			getByEmailFn:                    func(context.Context, string) (*domain.User, error) { return nil, ErrUserNotFound },
			getByIDFn:                       func(context.Context, string) (*domain.User, error) { return nil, nil },
			listFn:                          func(context.Context) ([]domain.User, error) { return nil, nil },
			updateRoleFn:                    func(context.Context, string, domain.Role) (*domain.User, error) { return nil, nil },
			createEmailVerificationTokenFn:  func(context.Context, domain.EmailVerificationToken) error { return nil },
			consumeEmailVerificationTokenFn: func(context.Context, string, time.Time) error { return nil },
			createPasswordResetTokenFn:      func(context.Context, domain.PasswordResetToken) error { return nil },
			consumePasswordResetTokenFn:     func(context.Context, string, string, time.Time) error { return nil },
			isEmailVerifiedFn:               func(context.Context, string) (bool, error) { return false, nil },
		},
		&mockAuditLogRepository{createFn: func(context.Context, domain.AuditLog) error { return nil }},
		fakePasswordManager{
			hashFn:    func(password string) (string, error) { return "hashed-" + password, nil },
			compareFn: func(string, string) error { return nil },
		},
		fakeTokenIssuer{issueFn: func(string, domain.Role) (string, error) { return "", nil }},
		24*time.Hour,
		60*time.Minute,
		false,
		WithPendingRegistrationStore(fakePendingRegistrationStore{
			setFn: func(_ context.Context, email string, pending PendingRegistration, ttl time.Duration) error {
				pendingEmail = email
				pendingData = pending
				pendingTTL = ttl
				return nil
			},
		}, 15*time.Minute),
		WithVerificationCodeStore(fakeVerificationCodeStore{
			setFn: func(_ context.Context, email, code string, ttl time.Duration) error {
				storedCodeEmail = email
				storedCode = code
				storedCodeTTL = ttl
				return nil
			},
		}, 15*time.Minute),
	)
	service.now = func() time.Time { return time.Date(2026, time.February, 28, 14, 0, 0, 0, time.UTC) }

	result, err := service.RegisterWithVerificationCode(context.Background(), RegisterInput{
		Email:    "CodeUser@example.com",
		Password: "superpass",
	})

	require.NoError(t, err)
	require.NotEmpty(t, result.User.ID)
	require.Equal(t, "codeuser@example.com", result.User.Email)
	require.False(t, result.User.IsVerified)
	require.Equal(t, "codeuser@example.com", storedCodeEmail)
	require.Len(t, storedCode, 6)
	require.Equal(t, 15*time.Minute, storedCodeTTL)
	require.Equal(t, "codeuser@example.com", pendingEmail)
	require.Equal(t, "codeuser", pendingData.Name)
	require.Equal(t, "hashed-superpass", pendingData.PasswordHash)
	require.Equal(t, 15*time.Minute, pendingTTL)
}

func TestVerifyEmailCodeMarksUserAndDeletesKey(t *testing.T) {
	t.Parallel()

	var createdUser domain.User
	var deletedCodeEmail string
	var deletedPendingEmail string

	service := NewUserService(
		&mockUserRepository{
			createFn: func(_ context.Context, user domain.User) (*domain.User, error) {
				createdUser = user
				return &user, nil
			},
			createWithEmailVerificationTokenFn: func(context.Context, domain.User, domain.EmailVerificationToken) (*domain.User, error) {
				return nil, nil
			},
			getByEmailFn:                    func(context.Context, string) (*domain.User, error) { return nil, nil },
			getByIDFn:                       func(context.Context, string) (*domain.User, error) { return nil, nil },
			listFn:                          func(context.Context) ([]domain.User, error) { return nil, nil },
			updateRoleFn:                    func(context.Context, string, domain.Role) (*domain.User, error) { return nil, nil },
			createEmailVerificationTokenFn:  func(context.Context, domain.EmailVerificationToken) error { return nil },
			consumeEmailVerificationTokenFn: func(context.Context, string, time.Time) error { return nil },
			createPasswordResetTokenFn:      func(context.Context, domain.PasswordResetToken) error { return nil },
			consumePasswordResetTokenFn:     func(context.Context, string, string, time.Time) error { return nil },
			isEmailVerifiedFn:               func(context.Context, string) (bool, error) { return false, nil },
			markEmailVerifiedByEmailFn:      func(context.Context, string, time.Time) error { return nil },
		},
		&mockAuditLogRepository{createFn: func(context.Context, domain.AuditLog) error { return nil }},
		fakePasswordManager{
			hashFn:    func(string) (string, error) { return "", nil },
			compareFn: func(string, string) error { return nil },
		},
		fakeTokenIssuer{issueFn: func(string, domain.Role) (string, error) { return "", nil }},
		24*time.Hour,
		60*time.Minute,
		false,
		WithPendingRegistrationStore(fakePendingRegistrationStore{
			getFn: func(_ context.Context, email string) (PendingRegistration, bool, error) {
				require.Equal(t, "codeuser@example.com", email)
				return PendingRegistration{
					Name:         "Code User",
					PasswordHash: "hashed-pass",
					Phone:        "123",
					DNI:          "999",
					Address:      "Street 1",
				}, true, nil
			},
			deleteFn: func(_ context.Context, email string) error {
				deletedPendingEmail = email
				return nil
			},
		}, 15*time.Minute),
		WithVerificationCodeStore(fakeVerificationCodeStore{
			getFn: func(_ context.Context, email string) (string, bool, error) {
				require.Equal(t, "codeuser@example.com", email)
				return "459812", true, nil
			},
			deleteFn: func(_ context.Context, email string) error {
				deletedCodeEmail = email
				return nil
			},
		}, 15*time.Minute),
	)
	frozenNow := time.Date(2026, time.March, 10, 19, 5, 0, 0, time.UTC)
	service.now = func() time.Time { return frozenNow }

	err := service.VerifyEmailCode(context.Background(), "codeuser@example.com", "459812")

	require.NoError(t, err)
	require.Equal(t, "codeuser@example.com", deletedCodeEmail)
	require.Equal(t, "codeuser@example.com", deletedPendingEmail)
	require.Equal(t, "codeuser@example.com", createdUser.Email)
	require.Equal(t, "Code User", createdUser.Name)
	require.True(t, createdUser.EmailVerified)
	require.True(t, createdUser.IsVerified)
	require.Equal(t, domain.RoleStudent, createdUser.Role)
	require.Equal(t, "hashed-pass", createdUser.PasswordHash)
	require.Equal(t, frozenNow, createdUser.CreatedAt)
	require.Equal(t, frozenNow, createdUser.UpdatedAt)
	require.NotNil(t, createdUser.EmailVerifiedAt)
	require.Equal(t, frozenNow, *createdUser.EmailVerifiedAt)
}

func TestVerifyEmailCodeReturnsInvalidForMismatchedCode(t *testing.T) {
	t.Parallel()

	service := NewUserService(
		&mockUserRepository{
			createFn: func(context.Context, domain.User) (*domain.User, error) { return nil, nil },
			createWithEmailVerificationTokenFn: func(context.Context, domain.User, domain.EmailVerificationToken) (*domain.User, error) {
				return nil, nil
			},
			getByEmailFn:                    func(context.Context, string) (*domain.User, error) { return nil, nil },
			getByIDFn:                       func(context.Context, string) (*domain.User, error) { return nil, nil },
			listFn:                          func(context.Context) ([]domain.User, error) { return nil, nil },
			updateRoleFn:                    func(context.Context, string, domain.Role) (*domain.User, error) { return nil, nil },
			createEmailVerificationTokenFn:  func(context.Context, domain.EmailVerificationToken) error { return nil },
			consumeEmailVerificationTokenFn: func(context.Context, string, time.Time) error { return nil },
			createPasswordResetTokenFn:      func(context.Context, domain.PasswordResetToken) error { return nil },
			consumePasswordResetTokenFn:     func(context.Context, string, string, time.Time) error { return nil },
			isEmailVerifiedFn:               func(context.Context, string) (bool, error) { return false, nil },
			markEmailVerifiedByEmailFn: func(context.Context, string, time.Time) error {
				t.Fatal("user should not be marked as verified when code is wrong")
				return nil
			},
		},
		&mockAuditLogRepository{createFn: func(context.Context, domain.AuditLog) error { return nil }},
		fakePasswordManager{
			hashFn:    func(string) (string, error) { return "", nil },
			compareFn: func(string, string) error { return nil },
		},
		fakeTokenIssuer{issueFn: func(string, domain.Role) (string, error) { return "", nil }},
		24*time.Hour,
		60*time.Minute,
		false,
		WithVerificationCodeStore(fakeVerificationCodeStore{
			getFn: func(context.Context, string) (string, bool, error) {
				return "111111", true, nil
			},
			deleteFn: func(context.Context, string) error {
				t.Fatal("redis key should not be deleted when code is wrong")
				return nil
			},
		}, 15*time.Minute),
	)

	err := service.VerifyEmailCode(context.Background(), "codeuser@example.com", "222222")
	require.ErrorIs(t, err, ErrVerificationCodeInvalid)
}

func TestForgotPasswordDoesNotLeakExistence(t *testing.T) {
	t.Parallel()

	var createdTokens int
	service := NewUserService(
		&mockUserRepository{
			createFn: func(context.Context, domain.User) (*domain.User, error) { return nil, nil },
			createWithEmailVerificationTokenFn: func(context.Context, domain.User, domain.EmailVerificationToken) (*domain.User, error) {
				return nil, nil
			},
			getByEmailFn:                    func(context.Context, string) (*domain.User, error) { return nil, ErrUserNotFound },
			getByIDFn:                       func(context.Context, string) (*domain.User, error) { return nil, nil },
			listFn:                          func(context.Context) ([]domain.User, error) { return nil, nil },
			updateRoleFn:                    func(context.Context, string, domain.Role) (*domain.User, error) { return nil, nil },
			createEmailVerificationTokenFn:  func(context.Context, domain.EmailVerificationToken) error { return nil },
			consumeEmailVerificationTokenFn: func(context.Context, string, time.Time) error { return nil },
			createPasswordResetTokenFn:      func(context.Context, domain.PasswordResetToken) error { createdTokens++; return nil },
			consumePasswordResetTokenFn:     func(context.Context, string, string, time.Time) error { return nil },
			isEmailVerifiedFn:               func(context.Context, string) (bool, error) { return false, nil },
		},
		&mockAuditLogRepository{createFn: func(context.Context, domain.AuditLog) error { return nil }},
		fakePasswordManager{
			hashFn:    func(string) (string, error) { return "", nil },
			compareFn: func(string, string) error { return nil },
		},
		fakeTokenIssuer{issueFn: func(string, domain.Role) (string, error) { return "", nil }},
		24*time.Hour,
		60*time.Minute,
		false,
	)

	result, err := service.ForgotPassword(context.Background(), "missing@example.com")

	require.NoError(t, err)
	require.Nil(t, result)
	require.Zero(t, createdTokens)
}

func TestRequestPasswordResetCodeStoresCodeInRedis(t *testing.T) {
	t.Parallel()

	var storedEmail string
	var storedCode string
	var storedTTL time.Duration

	service := NewUserService(
		&mockUserRepository{
			createFn: func(context.Context, domain.User) (*domain.User, error) { return nil, nil },
			createWithEmailVerificationTokenFn: func(context.Context, domain.User, domain.EmailVerificationToken) (*domain.User, error) {
				return nil, nil
			},
			getByEmailFn: func(_ context.Context, email string) (*domain.User, error) {
				return &domain.User{
					ID:    "user-1",
					Email: email,
				}, nil
			},
			getByIDFn:                       func(context.Context, string) (*domain.User, error) { return nil, nil },
			listFn:                          func(context.Context) ([]domain.User, error) { return nil, nil },
			updateRoleFn:                    func(context.Context, string, domain.Role) (*domain.User, error) { return nil, nil },
			createEmailVerificationTokenFn:  func(context.Context, domain.EmailVerificationToken) error { return nil },
			consumeEmailVerificationTokenFn: func(context.Context, string, time.Time) error { return nil },
			createPasswordResetTokenFn:      func(context.Context, domain.PasswordResetToken) error { return nil },
			consumePasswordResetTokenFn:     func(context.Context, string, string, time.Time) error { return nil },
			isEmailVerifiedFn:               func(context.Context, string) (bool, error) { return false, nil },
		},
		&mockAuditLogRepository{createFn: func(context.Context, domain.AuditLog) error { return nil }},
		fakePasswordManager{
			hashFn:    func(string) (string, error) { return "", nil },
			compareFn: func(string, string) error { return nil },
		},
		fakeTokenIssuer{issueFn: func(string, domain.Role) (string, error) { return "", nil }},
		24*time.Hour,
		60*time.Minute,
		false,
		WithPasswordResetCodeStore(fakeVerificationCodeStore{
			setFn: func(_ context.Context, email, code string, ttl time.Duration) error {
				storedEmail = email
				storedCode = code
				storedTTL = ttl
				return nil
			},
		}, 15*time.Minute),
	)

	result, err := service.RequestPasswordResetCode(context.Background(), "reset@example.com")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "reset@example.com", result.Email)
	require.Equal(t, "reset@example.com", storedEmail)
	require.Len(t, storedCode, 6)
	require.Equal(t, 15*time.Minute, storedTTL)
}

func TestResetPasswordWithCodeUpdatesPasswordAndDeletesCode(t *testing.T) {
	t.Parallel()

	frozenNow := time.Date(2026, time.March, 10, 20, 10, 0, 0, time.UTC)
	var updatedEmail string
	var updatedHash string
	var updatedAt time.Time
	var deletedEmail string

	service := NewUserService(
		&mockUserRepository{
			createFn: func(context.Context, domain.User) (*domain.User, error) { return nil, nil },
			createWithEmailVerificationTokenFn: func(context.Context, domain.User, domain.EmailVerificationToken) (*domain.User, error) {
				return nil, nil
			},
			getByEmailFn:                    func(context.Context, string) (*domain.User, error) { return nil, nil },
			getByIDFn:                       func(context.Context, string) (*domain.User, error) { return nil, nil },
			listFn:                          func(context.Context) ([]domain.User, error) { return nil, nil },
			updateRoleFn:                    func(context.Context, string, domain.Role) (*domain.User, error) { return nil, nil },
			createEmailVerificationTokenFn:  func(context.Context, domain.EmailVerificationToken) error { return nil },
			consumeEmailVerificationTokenFn: func(context.Context, string, time.Time) error { return nil },
			createPasswordResetTokenFn:      func(context.Context, domain.PasswordResetToken) error { return nil },
			consumePasswordResetTokenFn:     func(context.Context, string, string, time.Time) error { return nil },
			updatePasswordByEmailFn: func(_ context.Context, email, passwordHash string, now time.Time) error {
				updatedEmail = email
				updatedHash = passwordHash
				updatedAt = now
				return nil
			},
			isEmailVerifiedFn: func(context.Context, string) (bool, error) { return false, nil },
		},
		&mockAuditLogRepository{createFn: func(context.Context, domain.AuditLog) error { return nil }},
		fakePasswordManager{
			hashFn: func(password string) (string, error) {
				require.Equal(t, "new-password", password)
				return "hashed-new-password", nil
			},
			compareFn: func(string, string) error { return nil },
		},
		fakeTokenIssuer{issueFn: func(string, domain.Role) (string, error) { return "", nil }},
		24*time.Hour,
		60*time.Minute,
		false,
		WithPasswordResetCodeStore(fakeVerificationCodeStore{
			getFn: func(_ context.Context, email string) (string, bool, error) {
				require.Equal(t, "reset@example.com", email)
				return "459812", true, nil
			},
			deleteFn: func(_ context.Context, email string) error {
				deletedEmail = email
				return nil
			},
		}, 15*time.Minute),
	)
	service.now = func() time.Time { return frozenNow }

	err := service.ResetPasswordWithCode(context.Background(), "reset@example.com", "459812", "new-password")

	require.NoError(t, err)
	require.Equal(t, "reset@example.com", updatedEmail)
	require.Equal(t, "hashed-new-password", updatedHash)
	require.Equal(t, frozenNow, updatedAt)
	require.Equal(t, "reset@example.com", deletedEmail)
}

func TestResetPasswordWithCodeRejectsInvalidCode(t *testing.T) {
	t.Parallel()

	service := NewUserService(
		&mockUserRepository{
			createFn: func(context.Context, domain.User) (*domain.User, error) { return nil, nil },
			createWithEmailVerificationTokenFn: func(context.Context, domain.User, domain.EmailVerificationToken) (*domain.User, error) {
				return nil, nil
			},
			getByEmailFn:                    func(context.Context, string) (*domain.User, error) { return nil, nil },
			getByIDFn:                       func(context.Context, string) (*domain.User, error) { return nil, nil },
			listFn:                          func(context.Context) ([]domain.User, error) { return nil, nil },
			updateRoleFn:                    func(context.Context, string, domain.Role) (*domain.User, error) { return nil, nil },
			createEmailVerificationTokenFn:  func(context.Context, domain.EmailVerificationToken) error { return nil },
			consumeEmailVerificationTokenFn: func(context.Context, string, time.Time) error { return nil },
			createPasswordResetTokenFn:      func(context.Context, domain.PasswordResetToken) error { return nil },
			consumePasswordResetTokenFn:     func(context.Context, string, string, time.Time) error { return nil },
			updatePasswordByEmailFn: func(context.Context, string, string, time.Time) error {
				t.Fatal("password should not be updated with invalid code")
				return nil
			},
			isEmailVerifiedFn: func(context.Context, string) (bool, error) { return false, nil },
		},
		&mockAuditLogRepository{createFn: func(context.Context, domain.AuditLog) error { return nil }},
		fakePasswordManager{
			hashFn:    func(string) (string, error) { return "", nil },
			compareFn: func(string, string) error { return nil },
		},
		fakeTokenIssuer{issueFn: func(string, domain.Role) (string, error) { return "", nil }},
		24*time.Hour,
		60*time.Minute,
		false,
		WithPasswordResetCodeStore(fakeVerificationCodeStore{
			getFn: func(context.Context, string) (string, bool, error) {
				return "111111", true, nil
			},
			deleteFn: func(context.Context, string) error {
				t.Fatal("reset code should not be deleted when code is invalid")
				return nil
			},
		}, 15*time.Minute),
	)

	err := service.ResetPasswordWithCode(context.Background(), "reset@example.com", "222222", "new-password")
	require.ErrorIs(t, err, ErrPasswordResetCodeInvalid)
}

func TestResetPasswordUpdatesHashAndMarksTokenUsed(t *testing.T) {
	t.Parallel()

	var consumedHash string
	var consumedPasswordHash string
	var consumedAt time.Time
	service := NewUserService(
		&mockUserRepository{
			createFn: func(context.Context, domain.User) (*domain.User, error) { return nil, nil },
			createWithEmailVerificationTokenFn: func(context.Context, domain.User, domain.EmailVerificationToken) (*domain.User, error) {
				return nil, nil
			},
			getByEmailFn:                    func(context.Context, string) (*domain.User, error) { return nil, nil },
			getByIDFn:                       func(context.Context, string) (*domain.User, error) { return nil, nil },
			listFn:                          func(context.Context) ([]domain.User, error) { return nil, nil },
			updateRoleFn:                    func(context.Context, string, domain.Role) (*domain.User, error) { return nil, nil },
			createEmailVerificationTokenFn:  func(context.Context, domain.EmailVerificationToken) error { return nil },
			consumeEmailVerificationTokenFn: func(context.Context, string, time.Time) error { return nil },
			createPasswordResetTokenFn:      func(context.Context, domain.PasswordResetToken) error { return nil },
			consumePasswordResetTokenFn: func(_ context.Context, tokenHash, passwordHash string, now time.Time) error {
				consumedHash = tokenHash
				consumedPasswordHash = passwordHash
				consumedAt = now
				return nil
			},
			isEmailVerifiedFn: func(context.Context, string) (bool, error) { return false, nil },
		},
		&mockAuditLogRepository{createFn: func(context.Context, domain.AuditLog) error { return nil }},
		fakePasswordManager{
			hashFn: func(password string) (string, error) {
				require.Equal(t, "new-password", password)
				return "hashed-new-password", nil
			},
			compareFn: func(string, string) error { return nil },
		},
		fakeTokenIssuer{issueFn: func(string, domain.Role) (string, error) { return "", nil }},
		24*time.Hour,
		60*time.Minute,
		false,
	)
	frozenNow := time.Date(2026, time.February, 28, 13, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return frozenNow }

	err := service.ResetPassword(context.Background(), "reset-me", "new-password")

	require.NoError(t, err)
	require.Equal(t, "afdbf99cd2ad3f8574442283f2671d5dbcbacaf1135b294de0390b3729c518be", consumedHash)
	require.Equal(t, "hashed-new-password", consumedPasswordHash)
	require.Equal(t, frozenNow, consumedAt)
}

func TestLogin(t *testing.T) {
	t.Parallel()

	expectedUser := &domain.User{
		ID:           "user-1",
		Name:         "Grace Hopper",
		Email:        "grace@example.com",
		PasswordHash: "hashed-password",
		Role:         domain.RoleTeacher,
	}
	service := NewUserService(
		&mockUserRepository{
			createFn: func(context.Context, domain.User) (*domain.User, error) { return nil, nil },
			createWithEmailVerificationTokenFn: func(context.Context, domain.User, domain.EmailVerificationToken) (*domain.User, error) {
				return nil, nil
			},
			getByEmailFn:                    func(_ context.Context, email string) (*domain.User, error) { return expectedUser, nil },
			getByIDFn:                       func(context.Context, string) (*domain.User, error) { return nil, nil },
			listFn:                          func(context.Context) ([]domain.User, error) { return nil, nil },
			updateRoleFn:                    func(context.Context, string, domain.Role) (*domain.User, error) { return nil, nil },
			createEmailVerificationTokenFn:  func(context.Context, domain.EmailVerificationToken) error { return nil },
			consumeEmailVerificationTokenFn: func(context.Context, string, time.Time) error { return nil },
			createPasswordResetTokenFn:      func(context.Context, domain.PasswordResetToken) error { return nil },
			consumePasswordResetTokenFn:     func(context.Context, string, string, time.Time) error { return nil },
			isEmailVerifiedFn:               func(context.Context, string) (bool, error) { return false, nil },
		},
		&mockAuditLogRepository{
			createFn: func(context.Context, domain.AuditLog) error { return nil },
		},
		fakePasswordManager{
			hashFn: func(string) (string, error) { return "", nil },
			compareFn: func(hash, password string) error {
				require.Equal(t, "hashed-password", hash)
				require.Equal(t, "correct-horse", password)
				return nil
			},
		},
		fakeTokenIssuer{
			issueFn: func(userID string, role domain.Role) (string, error) {
				require.Equal(t, "user-1", userID)
				require.Equal(t, domain.RoleTeacher, role)
				return "signed-token", nil
			},
		},
		24*time.Hour,
		60*time.Minute,
		false,
	)

	token, user, err := service.Login(context.Background(), LoginInput{
		Email:    "grace@example.com",
		Password: "correct-horse",
	})

	require.NoError(t, err)
	require.Equal(t, "signed-token", token)
	require.Equal(t, expectedUser, user)
}

func TestLoginBlocksUnverifiedEmailWhenRequired(t *testing.T) {
	t.Parallel()

	service := NewUserService(
		&mockUserRepository{
			createFn: func(context.Context, domain.User) (*domain.User, error) { return nil, nil },
			createWithEmailVerificationTokenFn: func(context.Context, domain.User, domain.EmailVerificationToken) (*domain.User, error) {
				return nil, nil
			},
			getByEmailFn: func(context.Context, string) (*domain.User, error) {
				return &domain.User{
					ID:            "user-1",
					Email:         "user@example.com",
					PasswordHash:  "hash",
					EmailVerified: false,
					Role:          domain.RoleStudent,
				}, nil
			},
			getByIDFn:                       func(context.Context, string) (*domain.User, error) { return nil, nil },
			listFn:                          func(context.Context) ([]domain.User, error) { return nil, nil },
			updateRoleFn:                    func(context.Context, string, domain.Role) (*domain.User, error) { return nil, nil },
			createEmailVerificationTokenFn:  func(context.Context, domain.EmailVerificationToken) error { return nil },
			consumeEmailVerificationTokenFn: func(context.Context, string, time.Time) error { return nil },
			createPasswordResetTokenFn:      func(context.Context, domain.PasswordResetToken) error { return nil },
			consumePasswordResetTokenFn:     func(context.Context, string, string, time.Time) error { return nil },
			isEmailVerifiedFn:               func(context.Context, string) (bool, error) { return false, nil },
		},
		&mockAuditLogRepository{createFn: func(context.Context, domain.AuditLog) error { return nil }},
		fakePasswordManager{
			hashFn:    func(string) (string, error) { return "", nil },
			compareFn: func(string, string) error { return nil },
		},
		fakeTokenIssuer{
			issueFn: func(string, domain.Role) (string, error) {
				t.Fatal("token should not be issued when email verification is required")
				return "", nil
			},
		},
		24*time.Hour,
		60*time.Minute,
		true,
	)

	_, _, err := service.Login(context.Background(), LoginInput{
		Email:    "user@example.com",
		Password: "correct-horse",
	})

	require.ErrorIs(t, err, ErrEmailNotVerified)
}

func TestLoginAllowsUnverifiedEmailWhenDisabled(t *testing.T) {
	t.Parallel()

	service := NewUserService(
		&mockUserRepository{
			createFn: func(context.Context, domain.User) (*domain.User, error) { return nil, nil },
			createWithEmailVerificationTokenFn: func(context.Context, domain.User, domain.EmailVerificationToken) (*domain.User, error) {
				return nil, nil
			},
			getByEmailFn: func(context.Context, string) (*domain.User, error) {
				return &domain.User{
					ID:            "user-1",
					Email:         "user@example.com",
					PasswordHash:  "hash",
					EmailVerified: false,
					Role:          domain.RoleStudent,
				}, nil
			},
			getByIDFn:                       func(context.Context, string) (*domain.User, error) { return nil, nil },
			listFn:                          func(context.Context) ([]domain.User, error) { return nil, nil },
			updateRoleFn:                    func(context.Context, string, domain.Role) (*domain.User, error) { return nil, nil },
			createEmailVerificationTokenFn:  func(context.Context, domain.EmailVerificationToken) error { return nil },
			consumeEmailVerificationTokenFn: func(context.Context, string, time.Time) error { return nil },
			createPasswordResetTokenFn:      func(context.Context, domain.PasswordResetToken) error { return nil },
			consumePasswordResetTokenFn:     func(context.Context, string, string, time.Time) error { return nil },
			isEmailVerifiedFn:               func(context.Context, string) (bool, error) { return false, nil },
		},
		&mockAuditLogRepository{createFn: func(context.Context, domain.AuditLog) error { return nil }},
		fakePasswordManager{
			hashFn:    func(string) (string, error) { return "", nil },
			compareFn: func(string, string) error { return nil },
		},
		fakeTokenIssuer{
			issueFn: func(userID string, role domain.Role) (string, error) {
				require.Equal(t, "user-1", userID)
				require.Equal(t, domain.RoleStudent, role)
				return "signed-token", nil
			},
		},
		24*time.Hour,
		60*time.Minute,
		false,
	)

	token, _, err := service.Login(context.Background(), LoginInput{
		Email:    "user@example.com",
		Password: "correct-horse",
	})

	require.NoError(t, err)
	require.Equal(t, "signed-token", token)
}

func TestChangeRole(t *testing.T) {
	t.Parallel()

	var audit domain.AuditLog
	service := NewUserService(
		&mockUserRepository{
			createFn: func(context.Context, domain.User) (*domain.User, error) { return nil, nil },
			createWithEmailVerificationTokenFn: func(context.Context, domain.User, domain.EmailVerificationToken) (*domain.User, error) {
				return nil, nil
			},
			getByEmailFn: func(context.Context, string) (*domain.User, error) { return nil, nil },
			getByIDFn:    func(context.Context, string) (*domain.User, error) { return nil, nil },
			listFn:       func(context.Context) ([]domain.User, error) { return nil, nil },
			updateRoleFn: func(_ context.Context, userID string, role domain.Role) (*domain.User, error) {
				require.Equal(t, "target-1", userID)
				require.Equal(t, domain.RoleAdmin, role)
				return &domain.User{
					ID:    "target-1",
					Name:  "Linus",
					Email: "linus@example.com",
					Role:  domain.RoleAdmin,
				}, nil
			},
			createEmailVerificationTokenFn:  func(context.Context, domain.EmailVerificationToken) error { return nil },
			consumeEmailVerificationTokenFn: func(context.Context, string, time.Time) error { return nil },
			createPasswordResetTokenFn:      func(context.Context, domain.PasswordResetToken) error { return nil },
			consumePasswordResetTokenFn:     func(context.Context, string, string, time.Time) error { return nil },
			isEmailVerifiedFn:               func(context.Context, string) (bool, error) { return false, nil },
		},
		&mockAuditLogRepository{
			createFn: func(_ context.Context, auditLog domain.AuditLog) error {
				audit = auditLog
				return nil
			},
		},
		fakePasswordManager{
			hashFn:    func(string) (string, error) { return "", nil },
			compareFn: func(string, string) error { return nil },
		},
		fakeTokenIssuer{
			issueFn: func(string, domain.Role) (string, error) { return "", nil },
		},
		24*time.Hour,
		60*time.Minute,
		false,
	)
	frozenNow := time.Date(2026, time.February, 28, 11, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return frozenNow }

	user, err := service.ChangeRole(context.Background(), "admin-1", "target-1", domain.RoleAdmin, domain.AuditMeta{
		RequestID: "req-123",
		IP:        "127.0.0.1",
	})

	require.NoError(t, err)
	require.Equal(t, domain.RoleAdmin, user.Role)
	require.Equal(t, "admin-1", audit.AdminID)
	require.Equal(t, "target-1", audit.TargetUserID)
	require.Equal(t, domain.ActionChangeRole, audit.Action)
	require.Equal(t, "req-123", audit.RequestID)
	require.Equal(t, "127.0.0.1", audit.IP)
	require.Equal(t, frozenNow, audit.Timestamp)
}

func TestLoginReturnsInvalidCredentialsWhenPasswordDoesNotMatch(t *testing.T) {
	t.Parallel()

	service := NewUserService(
		&mockUserRepository{
			createFn: func(context.Context, domain.User) (*domain.User, error) { return nil, nil },
			createWithEmailVerificationTokenFn: func(context.Context, domain.User, domain.EmailVerificationToken) (*domain.User, error) {
				return nil, nil
			},
			getByEmailFn:                    func(context.Context, string) (*domain.User, error) { return &domain.User{PasswordHash: "hash"}, nil },
			getByIDFn:                       func(context.Context, string) (*domain.User, error) { return nil, nil },
			listFn:                          func(context.Context) ([]domain.User, error) { return nil, nil },
			updateRoleFn:                    func(context.Context, string, domain.Role) (*domain.User, error) { return nil, nil },
			createEmailVerificationTokenFn:  func(context.Context, domain.EmailVerificationToken) error { return nil },
			consumeEmailVerificationTokenFn: func(context.Context, string, time.Time) error { return nil },
			createPasswordResetTokenFn:      func(context.Context, domain.PasswordResetToken) error { return nil },
			consumePasswordResetTokenFn:     func(context.Context, string, string, time.Time) error { return nil },
			isEmailVerifiedFn:               func(context.Context, string) (bool, error) { return false, nil },
		},
		&mockAuditLogRepository{
			createFn: func(context.Context, domain.AuditLog) error { return nil },
		},
		fakePasswordManager{
			hashFn: func(string) (string, error) { return "", nil },
			compareFn: func(string, string) error {
				return errors.New("mismatch")
			},
		},
		fakeTokenIssuer{
			issueFn: func(string, domain.Role) (string, error) { return "", nil },
		},
		24*time.Hour,
		60*time.Minute,
		false,
	)

	_, _, err := service.Login(context.Background(), LoginInput{
		Email:    "user@example.com",
		Password: "wrong",
	})

	require.ErrorIs(t, err, ErrInvalidCredentials)
}
