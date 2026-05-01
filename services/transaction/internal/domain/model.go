package domain

import "time"

const (
	RoleCustomer = "customer"
	RoleAdmin    = "admin"
)

type Actor struct {
	UserID string
	Role   string
}

func (a Actor) CanAccess(account Account) bool {
	return a.Role == RoleAdmin || account.OwnerUserID == a.UserID
}

type Account struct {
	ID           string    `json:"id"`
	OwnerUserID  string    `json:"owner_user_id"`
	Currency     string    `json:"currency"`
	BalanceCents int64     `json:"balance_cents"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type TransactionType string

const (
	TransactionDeposit     TransactionType = "deposit"
	TransactionWithdrawal  TransactionType = "withdrawal"
	TransactionTransferIn  TransactionType = "transfer_in"
	TransactionTransferOut TransactionType = "transfer_out"
)

type Transaction struct {
	ID                    string          `json:"id"`
	AccountID             string          `json:"account_id"`
	CounterpartyAccountID *string         `json:"counterparty_account_id,omitempty"`
	Type                  TransactionType `json:"type"`
	AmountCents           int64           `json:"amount_cents"`
	BalanceAfterCents     int64           `json:"balance_after_cents"`
	Description           string          `json:"description"`
	RequestedByUserID     string          `json:"requested_by_user_id"`
	CreatedAt             time.Time       `json:"created_at"`
}

type MonthlyBalance struct {
	Month        time.Time `json:"month"`
	BalanceCents int64     `json:"balance_cents"`
}

type TransactionEvent struct {
	OwnerUserID string      `json:"owner_user_id"`
	Account     Account     `json:"account"`
	Transaction Transaction `json:"transaction"`
	Operation   string      `json:"operation"`
	PublishedAt time.Time   `json:"published_at"`
}
