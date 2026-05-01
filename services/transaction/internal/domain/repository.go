package domain

import (
	"context"
	"errors"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrForbidden         = errors.New("forbidden")
	ErrValidation        = errors.New("validation error")
)

type LedgerRepository interface {
	CreateAccount(ctx context.Context, account *Account) error
	GetAccount(ctx context.Context, accountID string) (*Account, error)
	ListAccountsByOwner(ctx context.Context, ownerUserID string) ([]Account, error)
	ListTransactions(ctx context.Context, accountID string, limit int) ([]Transaction, error)
	MonthlyBalances(ctx context.Context, accountID string, months int) ([]MonthlyBalance, error)
	WithinTx(ctx context.Context, fn func(tx LedgerTx) error) error
}

type LedgerTx interface {
	LockAccount(ctx context.Context, accountID string) (*Account, error)
	UpdateAccountBalance(ctx context.Context, accountID string, balanceCents int64) error
	CreateTransaction(ctx context.Context, transaction *Transaction) error
}
