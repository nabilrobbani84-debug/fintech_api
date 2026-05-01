package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/example/fintech-core-api/services/transaction/internal/domain"
)

type EventPublisher interface {
	Publish(event domain.TransactionEvent)
}

type TransactionUsecase struct {
	ledger    domain.LedgerRepository
	publisher EventPublisher
	clockFunc func() time.Time
}

type MoneyInput struct {
	AccountID   string `json:"account_id"`
	AmountCents int64  `json:"amount_cents"`
	Description string `json:"description"`
}

type TransferInput struct {
	FromAccountID string `json:"from_account_id"`
	ToAccountID   string `json:"to_account_id"`
	AmountCents   int64  `json:"amount_cents"`
	Description   string `json:"description"`
}

type OperationResult struct {
	Accounts     []domain.Account     `json:"accounts"`
	Transactions []domain.Transaction `json:"transactions"`
}

func NewTransactionUsecase(ledger domain.LedgerRepository, publisher EventPublisher) *TransactionUsecase {
	return &TransactionUsecase{
		ledger:    ledger,
		publisher: publisher,
		clockFunc: time.Now,
	}
}

func (u *TransactionUsecase) CreateAccount(ctx context.Context, actor domain.Actor, currency string) (*domain.Account, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		currency = "IDR"
	}
	if len(currency) != 3 {
		return nil, fmt.Errorf("%w: currency must be ISO-4217 code", domain.ErrValidation)
	}
	now := u.clockFunc().UTC()
	account := &domain.Account{
		ID:           domain.NewID(),
		OwnerUserID:  actor.UserID,
		Currency:     currency,
		BalanceCents: 0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := u.ledger.CreateAccount(ctx, account); err != nil {
		return nil, err
	}
	return account, nil
}

func (u *TransactionUsecase) ListAccounts(ctx context.Context, actor domain.Actor) ([]domain.Account, error) {
	return u.ledger.ListAccountsByOwner(ctx, actor.UserID)
}

func (u *TransactionUsecase) GetAccount(ctx context.Context, actor domain.Actor, accountID string) (*domain.Account, error) {
	account, err := u.ledger.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccess(*account) {
		return nil, domain.ErrForbidden
	}
	return account, nil
}

func (u *TransactionUsecase) Deposit(ctx context.Context, actor domain.Actor, input MoneyInput) (*OperationResult, error) {
	if err := validateMoneyInput(input); err != nil {
		return nil, err
	}

	var result OperationResult
	if err := u.ledger.WithinTx(ctx, func(tx domain.LedgerTx) error {
		account, err := tx.LockAccount(ctx, input.AccountID)
		if err != nil {
			return err
		}
		if !actor.CanAccess(*account) {
			return domain.ErrForbidden
		}
		account.BalanceCents += input.AmountCents
		account.UpdatedAt = u.clockFunc().UTC()
		if err := tx.UpdateAccountBalance(ctx, account.ID, account.BalanceCents); err != nil {
			return err
		}
		transaction := u.newTransaction(account.ID, nil, domain.TransactionDeposit, input.AmountCents, account.BalanceCents, input.Description, actor.UserID)
		if err := tx.CreateTransaction(ctx, &transaction); err != nil {
			return err
		}
		result.Accounts = []domain.Account{*account}
		result.Transactions = []domain.Transaction{transaction}
		return nil
	}); err != nil {
		return nil, err
	}
	u.publish(result, "deposit")
	return &result, nil
}

func (u *TransactionUsecase) Withdraw(ctx context.Context, actor domain.Actor, input MoneyInput) (*OperationResult, error) {
	if err := validateMoneyInput(input); err != nil {
		return nil, err
	}

	var result OperationResult
	if err := u.ledger.WithinTx(ctx, func(tx domain.LedgerTx) error {
		account, err := tx.LockAccount(ctx, input.AccountID)
		if err != nil {
			return err
		}
		if !actor.CanAccess(*account) {
			return domain.ErrForbidden
		}
		if account.BalanceCents < input.AmountCents {
			return domain.ErrInsufficientFunds
		}
		account.BalanceCents -= input.AmountCents
		account.UpdatedAt = u.clockFunc().UTC()
		if err := tx.UpdateAccountBalance(ctx, account.ID, account.BalanceCents); err != nil {
			return err
		}
		transaction := u.newTransaction(account.ID, nil, domain.TransactionWithdrawal, input.AmountCents, account.BalanceCents, input.Description, actor.UserID)
		if err := tx.CreateTransaction(ctx, &transaction); err != nil {
			return err
		}
		result.Accounts = []domain.Account{*account}
		result.Transactions = []domain.Transaction{transaction}
		return nil
	}); err != nil {
		return nil, err
	}
	u.publish(result, "withdrawal")
	return &result, nil
}

func (u *TransactionUsecase) Transfer(ctx context.Context, actor domain.Actor, input TransferInput) (*OperationResult, error) {
	if err := validateTransferInput(input); err != nil {
		return nil, err
	}

	var result OperationResult
	if err := u.ledger.WithinTx(ctx, func(tx domain.LedgerTx) error {
		locked, err := lockAccountsDeterministically(ctx, tx, input.FromAccountID, input.ToAccountID)
		if err != nil {
			return err
		}
		from := locked[input.FromAccountID]
		to := locked[input.ToAccountID]
		if !actor.CanAccess(*from) {
			return domain.ErrForbidden
		}
		if from.Currency != to.Currency {
			return fmt.Errorf("%w: currency mismatch", domain.ErrValidation)
		}
		if from.BalanceCents < input.AmountCents {
			return domain.ErrInsufficientFunds
		}

		now := u.clockFunc().UTC()
		from.BalanceCents -= input.AmountCents
		from.UpdatedAt = now
		to.BalanceCents += input.AmountCents
		to.UpdatedAt = now

		if err := tx.UpdateAccountBalance(ctx, from.ID, from.BalanceCents); err != nil {
			return err
		}
		if err := tx.UpdateAccountBalance(ctx, to.ID, to.BalanceCents); err != nil {
			return err
		}

		out := u.newTransaction(from.ID, &to.ID, domain.TransactionTransferOut, input.AmountCents, from.BalanceCents, input.Description, actor.UserID)
		in := u.newTransaction(to.ID, &from.ID, domain.TransactionTransferIn, input.AmountCents, to.BalanceCents, input.Description, actor.UserID)
		if err := tx.CreateTransaction(ctx, &out); err != nil {
			return err
		}
		if err := tx.CreateTransaction(ctx, &in); err != nil {
			return err
		}
		result.Accounts = []domain.Account{*from, *to}
		result.Transactions = []domain.Transaction{out, in}
		return nil
	}); err != nil {
		return nil, err
	}
	u.publish(result, "transfer")
	return &result, nil
}

func (u *TransactionUsecase) ListTransactions(ctx context.Context, actor domain.Actor, accountID string, limit int) ([]domain.Transaction, error) {
	account, err := u.GetAccount(ctx, actor, accountID)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccess(*account) {
		return nil, domain.ErrForbidden
	}
	return u.ledger.ListTransactions(ctx, accountID, limit)
}

func (u *TransactionUsecase) MonthlyBalances(ctx context.Context, actor domain.Actor, accountID string, months int) ([]domain.MonthlyBalance, error) {
	account, err := u.GetAccount(ctx, actor, accountID)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccess(*account) {
		return nil, domain.ErrForbidden
	}
	return u.ledger.MonthlyBalances(ctx, accountID, months)
}

func (u *TransactionUsecase) GetAccountInternal(ctx context.Context, accountID string) (*domain.Account, error) {
	return u.ledger.GetAccount(ctx, accountID)
}

func (u *TransactionUsecase) newTransaction(accountID string, counterpartyID *string, txType domain.TransactionType, amountCents int64, balanceAfter int64, description string, requestedBy string) domain.Transaction {
	return domain.Transaction{
		ID:                    domain.NewID(),
		AccountID:             accountID,
		CounterpartyAccountID: counterpartyID,
		Type:                  txType,
		AmountCents:           amountCents,
		BalanceAfterCents:     balanceAfter,
		Description:           strings.TrimSpace(description),
		RequestedByUserID:     requestedBy,
		CreatedAt:             u.clockFunc().UTC(),
	}
}

func (u *TransactionUsecase) publish(result OperationResult, operation string) {
	if u.publisher == nil {
		return
	}
	accounts := make(map[string]domain.Account, len(result.Accounts))
	for _, account := range result.Accounts {
		accounts[account.ID] = account
	}
	for _, transaction := range result.Transactions {
		account := accounts[transaction.AccountID]
		u.publisher.Publish(domain.TransactionEvent{
			OwnerUserID: account.OwnerUserID,
			Account:     account,
			Transaction: transaction,
			Operation:   operation,
			PublishedAt: u.clockFunc().UTC(),
		})
	}
}

func lockAccountsDeterministically(ctx context.Context, tx domain.LedgerTx, accountIDs ...string) (map[string]*domain.Account, error) {
	sortedIDs := append([]string(nil), accountIDs...)
	sort.Strings(sortedIDs)

	locked := make(map[string]*domain.Account, len(sortedIDs))
	for _, accountID := range sortedIDs {
		account, err := tx.LockAccount(ctx, accountID)
		if err != nil {
			return nil, err
		}
		locked[account.ID] = account
	}
	return locked, nil
}

func validateMoneyInput(input MoneyInput) error {
	if strings.TrimSpace(input.AccountID) == "" {
		return fmt.Errorf("%w: account_id required", domain.ErrValidation)
	}
	if input.AmountCents <= 0 {
		return fmt.Errorf("%w: amount_cents must be positive", domain.ErrValidation)
	}
	if len(input.Description) > 280 {
		return fmt.Errorf("%w: description too long", domain.ErrValidation)
	}
	return nil
}

func validateTransferInput(input TransferInput) error {
	if strings.TrimSpace(input.FromAccountID) == "" || strings.TrimSpace(input.ToAccountID) == "" {
		return fmt.Errorf("%w: from_account_id and to_account_id required", domain.ErrValidation)
	}
	if input.FromAccountID == input.ToAccountID {
		return fmt.Errorf("%w: source and destination accounts must be different", domain.ErrValidation)
	}
	if input.AmountCents <= 0 {
		return fmt.Errorf("%w: amount_cents must be positive", domain.ErrValidation)
	}
	if len(input.Description) > 280 {
		return fmt.Errorf("%w: description too long", domain.ErrValidation)
	}
	return nil
}
