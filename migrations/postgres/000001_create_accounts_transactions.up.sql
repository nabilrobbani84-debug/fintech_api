CREATE TABLE IF NOT EXISTS accounts (
  id UUID PRIMARY KEY,
  owner_user_id CHAR(36) NOT NULL,
  currency CHAR(3) NOT NULL DEFAULT 'IDR',
  balance_cents BIGINT NOT NULL DEFAULT 0 CHECK (balance_cents >= 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_accounts_owner_user_id ON accounts(owner_user_id);

CREATE TABLE IF NOT EXISTS transactions (
  id UUID PRIMARY KEY,
  account_id UUID NOT NULL REFERENCES accounts(id),
  counterparty_account_id UUID NULL REFERENCES accounts(id),
  type VARCHAR(16) NOT NULL CHECK (type IN ('deposit', 'withdrawal', 'transfer_in', 'transfer_out')),
  amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
  balance_after_cents BIGINT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  requested_by_user_id CHAR(36) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_transactions_account_created_at ON transactions(account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_requested_by ON transactions(requested_by_user_id);

