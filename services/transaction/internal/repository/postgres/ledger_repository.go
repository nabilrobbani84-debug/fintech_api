package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/example/fintech-core-api/services/transaction/internal/domain"
)

type LedgerRepository struct {
	db *sql.DB
}

type txRepository struct {
	tx *sql.Tx
}

func NewLedgerRepository(db *sql.DB) *LedgerRepository {
	return &LedgerRepository{db: db}
}

func (r *LedgerRepository) CreateAccount(ctx context.Context, account *domain.Account) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO accounts (id, owner_user_id, currency, balance_cents, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		account.ID,
		account.OwnerUserID,
		account.Currency,
		account.BalanceCents,
		account.CreatedAt,
		account.UpdatedAt,
	)
	return err
}

func (r *LedgerRepository) GetAccount(ctx context.Context, accountID string) (*domain.Account, error) {
	return scanAccount(r.db.QueryRowContext(ctx, accountSelectSQL+` WHERE id = $1`, accountID))
}

func (r *LedgerRepository) ListAccountsByOwner(ctx context.Context, ownerUserID string) ([]domain.Account, error) {
	rows, err := r.db.QueryContext(ctx, accountSelectSQL+` WHERE owner_user_id = $1 ORDER BY created_at DESC`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []domain.Account
	for rows.Next() {
		account, err := scanAccountRows(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (r *LedgerRepository) ListTransactions(ctx context.Context, accountID string, limit int) ([]domain.Transaction, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, account_id, counterparty_account_id, type, amount_cents, balance_after_cents, description, requested_by_user_id, created_at
		 FROM transactions
		 WHERE account_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`,
		accountID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []domain.Transaction
	for rows.Next() {
		transaction, err := scanTransactionRows(rows)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, transaction)
	}
	return transactions, rows.Err()
}

func (r *LedgerRepository) MonthlyBalances(ctx context.Context, accountID string, months int) ([]domain.MonthlyBalance, error) {
	if months <= 0 || months > 24 {
		months = 12
	}
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT DISTINCT ON (date_trunc('month', created_at))
		     date_trunc('month', created_at) AS month,
		     balance_after_cents
		   FROM transactions
		   WHERE account_id = $1
		     AND created_at >= date_trunc('month', NOW()) - (($2::int - 1) * INTERVAL '1 month')
		   ORDER BY date_trunc('month', created_at), created_at DESC`,
		accountID,
		months,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var balances []domain.MonthlyBalance
	for rows.Next() {
		var balance domain.MonthlyBalance
		if err := rows.Scan(&balance.Month, &balance.BalanceCents); err != nil {
			return nil, err
		}
		balances = append(balances, balance)
	}
	return balances, rows.Err()
}

func (r *LedgerRepository) WithinTx(ctx context.Context, fn func(tx domain.LedgerTx) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := fn(&txRepository{tx: tx}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *txRepository) LockAccount(ctx context.Context, accountID string) (*domain.Account, error) {
	return scanAccount(r.tx.QueryRowContext(ctx, accountSelectSQL+` WHERE id = $1 FOR UPDATE`, accountID))
}

func (r *txRepository) UpdateAccountBalance(ctx context.Context, accountID string, balanceCents int64) error {
	_, err := r.tx.ExecContext(
		ctx,
		`UPDATE accounts SET balance_cents = $2, updated_at = $3 WHERE id = $1`,
		accountID,
		balanceCents,
		time.Now().UTC(),
	)
	return err
}

func (r *txRepository) CreateTransaction(ctx context.Context, transaction *domain.Transaction) error {
	_, err := r.tx.ExecContext(
		ctx,
		`INSERT INTO transactions (id, account_id, counterparty_account_id, type, amount_cents, balance_after_cents, description, requested_by_user_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		transaction.ID,
		transaction.AccountID,
		transaction.CounterpartyAccountID,
		transaction.Type,
		transaction.AmountCents,
		transaction.BalanceAfterCents,
		transaction.Description,
		transaction.RequestedByUserID,
		transaction.CreatedAt,
	)
	return err
}

const accountSelectSQL = `SELECT id, owner_user_id, currency, balance_cents, created_at, updated_at FROM accounts`

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanAccount(row rowScanner) (*domain.Account, error) {
	account, err := scanAccountScanner(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func scanAccountRows(rows rowScanner) (domain.Account, error) {
	return scanAccountScanner(rows)
}

func scanAccountScanner(row rowScanner) (domain.Account, error) {
	var account domain.Account
	err := row.Scan(
		&account.ID,
		&account.OwnerUserID,
		&account.Currency,
		&account.BalanceCents,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
	return account, err
}

func scanTransactionRows(rows rowScanner) (domain.Transaction, error) {
	var transaction domain.Transaction
	var counterparty sql.NullString
	var txType string
	err := rows.Scan(
		&transaction.ID,
		&transaction.AccountID,
		&counterparty,
		&txType,
		&transaction.AmountCents,
		&transaction.BalanceAfterCents,
		&transaction.Description,
		&transaction.RequestedByUserID,
		&transaction.CreatedAt,
	)
	if err != nil {
		return domain.Transaction{}, err
	}
	if counterparty.Valid {
		value := counterparty.String
		transaction.CounterpartyAccountID = &value
	}
	transaction.Type = domain.TransactionType(txType)
	return transaction, nil
}
