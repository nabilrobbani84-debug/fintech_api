package domain

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("not found")

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByEmailHash(ctx context.Context, emailHash string) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
}
