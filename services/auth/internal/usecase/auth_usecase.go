package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/example/fintech-core-api/services/auth/internal/domain"
	"github.com/example/fintech-core-api/services/auth/internal/security"
)

var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrValidation         = errors.New("validation error")
)

type AuthUsecase struct {
	users     domain.UserRepository
	crypto    *security.Encryptor
	jwt       *security.JWTManager
	clockFunc func() time.Time
}

type RegisterInput struct {
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type PublicUser struct {
	ID        string      `json:"id"`
	Email     string      `json:"email"`
	FullName  string      `json:"full_name"`
	Role      domain.Role `json:"role"`
	CreatedAt time.Time   `json:"created_at"`
}

type AuthResult struct {
	User      PublicUser `json:"user"`
	Token     string     `json:"token"`
	ExpiresAt time.Time  `json:"expires_at"`
}

func NewAuthUsecase(users domain.UserRepository, crypto *security.Encryptor, jwt *security.JWTManager) *AuthUsecase {
	return &AuthUsecase{
		users:     users,
		crypto:    crypto,
		jwt:       jwt,
		clockFunc: time.Now,
	}
}

func (u *AuthUsecase) Register(ctx context.Context, input RegisterInput) (*AuthResult, error) {
	if err := validateRegister(input); err != nil {
		return nil, err
	}

	role := domain.RoleCustomer
	if input.Role != "" {
		parsed, ok := domain.ParseRole(input.Role)
		if !ok {
			return nil, fmt.Errorf("%w: invalid role", ErrValidation)
		}
		role = parsed
	}

	emailHash := u.crypto.LookupHash(input.Email)
	if _, err := u.users.GetByEmailHash(ctx, emailHash); err == nil {
		return nil, ErrEmailTaken
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	passwordHash, err := security.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}
	encryptedEmail, err := u.crypto.Encrypt(security.NormalizeEmail(input.Email))
	if err != nil {
		return nil, err
	}
	encryptedFullName, err := u.crypto.Encrypt(strings.TrimSpace(input.FullName))
	if err != nil {
		return nil, err
	}

	now := u.clockFunc().UTC()
	user := &domain.User{
		ID:                domain.NewID(),
		EmailHash:         emailHash,
		EncryptedEmail:    encryptedEmail,
		EncryptedFullName: encryptedFullName,
		PasswordHash:      passwordHash,
		Role:              role,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := u.users.Create(ctx, user); err != nil {
		return nil, err
	}

	return u.issueToken(user)
}

func (u *AuthUsecase) Login(ctx context.Context, input LoginInput) (*AuthResult, error) {
	if _, err := mail.ParseAddress(input.Email); err != nil {
		return nil, fmt.Errorf("%w: invalid email", ErrValidation)
	}
	if input.Password == "" {
		return nil, fmt.Errorf("%w: password required", ErrValidation)
	}

	user, err := u.users.GetByEmailHash(ctx, u.crypto.LookupHash(input.Email))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if !security.CheckPassword(user.PasswordHash, input.Password) {
		return nil, ErrInvalidCredentials
	}
	return u.issueToken(user)
}

func (u *AuthUsecase) ValidateToken(ctx context.Context, token string) (*security.Claims, error) {
	claims, err := u.jwt.Validate(token)
	if err != nil {
		return nil, err
	}
	if _, err := u.users.GetByID(ctx, claims.UserID); err != nil {
		return nil, err
	}
	return claims, nil
}

func (u *AuthUsecase) GetProfile(ctx context.Context, userID string) (*PublicUser, error) {
	user, err := u.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	public, err := u.publicUser(user)
	if err != nil {
		return nil, err
	}
	return &public, nil
}

func (u *AuthUsecase) issueToken(user *domain.User) (*AuthResult, error) {
	token, expiresAt, err := u.jwt.Generate(user.ID, string(user.Role))
	if err != nil {
		return nil, err
	}
	public, err := u.publicUser(user)
	if err != nil {
		return nil, err
	}
	return &AuthResult{
		User:      public,
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func (u *AuthUsecase) publicUser(user *domain.User) (PublicUser, error) {
	email, err := u.crypto.Decrypt(user.EncryptedEmail)
	if err != nil {
		return PublicUser{}, err
	}
	fullName, err := u.crypto.Decrypt(user.EncryptedFullName)
	if err != nil {
		return PublicUser{}, err
	}
	return PublicUser{
		ID:        user.ID,
		Email:     email,
		FullName:  fullName,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
	}, nil
}

func validateRegister(input RegisterInput) error {
	if _, err := mail.ParseAddress(input.Email); err != nil {
		return fmt.Errorf("%w: invalid email", ErrValidation)
	}
	if len(strings.TrimSpace(input.FullName)) < 2 {
		return fmt.Errorf("%w: full_name must be at least 2 characters", ErrValidation)
	}
	if len(input.Password) < 12 {
		return fmt.Errorf("%w: password must be at least 12 characters", ErrValidation)
	}
	return nil
}
