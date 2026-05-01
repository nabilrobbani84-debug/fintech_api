package mysql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/example/fintech-core-api/services/auth/internal/domain"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO users (id, email_hash, encrypted_email, encrypted_full_name, password_hash, role, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID,
		user.EmailHash,
		user.EncryptedEmail,
		user.EncryptedFullName,
		user.PasswordHash,
		user.Role,
		user.CreatedAt,
		user.UpdatedAt,
	)
	return err
}

func (r *UserRepository) GetByEmailHash(ctx context.Context, emailHash string) (*domain.User, error) {
	return r.get(ctx, `SELECT id, email_hash, encrypted_email, encrypted_full_name, password_hash, role, created_at, updated_at FROM users WHERE email_hash = ? LIMIT 1`, emailHash)
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return r.get(ctx, `SELECT id, email_hash, encrypted_email, encrypted_full_name, password_hash, role, created_at, updated_at FROM users WHERE id = ? LIMIT 1`, id)
}

func (r *UserRepository) get(ctx context.Context, query string, arg string) (*domain.User, error) {
	var user domain.User
	var role string
	err := r.db.QueryRowContext(ctx, query, arg).Scan(
		&user.ID,
		&user.EmailHash,
		&user.EncryptedEmail,
		&user.EncryptedFullName,
		&user.PasswordHash,
		&role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	parsed, ok := domain.ParseRole(role)
	if !ok {
		return nil, errors.New("invalid role stored in database")
	}
	user.Role = parsed
	return &user, nil
}
