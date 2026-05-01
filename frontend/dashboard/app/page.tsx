"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import {
  Activity,
  ArrowDownToLine,
  ArrowRightLeft,
  ArrowUpFromLine,
  Landmark,
  LogIn,
  LogOut,
  PlusCircle,
  RefreshCw,
  Send
} from "lucide-react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis
} from "recharts";

const AUTH_API = process.env.NEXT_PUBLIC_AUTH_API_URL ?? "http://localhost:8080";
const TRANSACTION_API = process.env.NEXT_PUBLIC_TRANSACTION_API_URL ?? "http://localhost:8081";

type User = {
  id: string;
  email: string;
  full_name: string;
  role: string;
};

type AuthResult = {
  user: User;
  token: string;
  expires_at: string;
};

type Account = {
  id: string;
  owner_user_id: string;
  currency: string;
  balance_cents: number;
  created_at: string;
};

type Transaction = {
  id: string;
  account_id: string;
  counterparty_account_id?: string;
  type: string;
  amount_cents: number;
  balance_after_cents: number;
  description: string;
  created_at: string;
};

type MonthlyBalance = {
  month: string;
  balance_cents: number;
};

type TransactionEvent = {
  operation: string;
  account: Account;
  transaction: Transaction;
  published_at: string;
};

type ActionKind = "deposit" | "withdraw" | "transfer";
type AuthMode = "login" | "register";

export default function DashboardPage() {
  const [token, setToken] = useState("");
  const [user, setUser] = useState<User | null>(null);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [selectedAccountID, setSelectedAccountID] = useState("");
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [balances, setBalances] = useState<MonthlyBalance[]>([]);
  const [events, setEvents] = useState<TransactionEvent[]>([]);
  const [live, setLive] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [loading, setLoading] = useState(false);
  const [authMode, setAuthMode] = useState<AuthMode>("login");
  const [email, setEmail] = useState("customer@example.com");
  const [fullName, setFullName] = useState("FinTech Customer");
  const [password, setPassword] = useState("StrongPassword123");
  const [action, setAction] = useState<ActionKind>("deposit");
  const [amount, setAmount] = useState("250000");
  const [description, setDescription] = useState("Operational transaction");
  const [toAccountID, setToAccountID] = useState("");

  const selectedAccount = useMemo(
    () => accounts.find((account) => account.id === selectedAccountID) ?? accounts[0],
    [accounts, selectedAccountID]
  );

  const totalBalance = useMemo(
    () => accounts.reduce((sum, account) => sum + account.balance_cents, 0),
    [accounts]
  );

  const chartData = useMemo(
    () =>
      balances.map((point) => ({
        month: new Intl.DateTimeFormat("en", { month: "short" }).format(new Date(point.month)),
        balance: point.balance_cents / 100
      })),
    [balances]
  );

  const transactionCount = transactions.length;

  const api = useCallback(
    async <T,>(path: string, options: RequestInit = {}): Promise<T> => {
      const response = await fetch(`${TRANSACTION_API}${path}`, {
        ...options,
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
          ...(options.headers ?? {})
        }
      });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) {
        throw new Error(data.error ?? "request failed");
      }
      return data as T;
    },
    [token]
  );

  const loadAccounts = useCallback(async () => {
    if (!token) return;
    const data = await api<Account[]>("/api/v1/accounts");
    setAccounts(data);
    if (!selectedAccountID && data.length > 0) {
      setSelectedAccountID(data[0].id);
    }
  }, [api, selectedAccountID, token]);

  const loadAccountData = useCallback(async () => {
    if (!token || !selectedAccount) return;
    const [history, monthly] = await Promise.all([
      api<Transaction[]>(`/api/v1/transactions?account_id=${selectedAccount.id}&limit=20`),
      api<MonthlyBalance[]>(`/api/v1/reports/monthly-balance?account_id=${selectedAccount.id}&months=12`)
    ]);
    setTransactions(history);
    setBalances(monthly);
  }, [api, selectedAccount, token]);

  const refresh = useCallback(async () => {
    setError("");
    try {
      await loadAccounts();
      await loadAccountData();
    } catch (err) {
      setError(err instanceof Error ? err.message : "unable to refresh data");
    }
  }, [loadAccountData, loadAccounts]);

  useEffect(() => {
    queueMicrotask(() => {
      const savedToken = window.localStorage.getItem("fintech_token");
      const savedUser = window.localStorage.getItem("fintech_user");
      if (savedToken) {
        setToken(savedToken);
      }
      if (savedUser) {
        setUser(JSON.parse(savedUser) as User);
      }
    });
  }, []);

  useEffect(() => {
    if (!token) return;
    const timeout = window.setTimeout(() => {
      void refresh();
    }, 0);
    return () => window.clearTimeout(timeout);
  }, [token, refresh]);

  useEffect(() => {
    if (!selectedAccount) return;
    const timeout = window.setTimeout(() => {
      void loadAccountData();
    }, 0);
    return () => window.clearTimeout(timeout);
  }, [loadAccountData, selectedAccount]);

  useEffect(() => {
    if (!token) return;
    const source = new EventSource(`${TRANSACTION_API}/api/v1/events?token=${encodeURIComponent(token)}`);
    source.addEventListener("open", () => setLive(true));
    source.addEventListener("error", () => setLive(false));
    source.addEventListener("transaction", (message) => {
      const event = JSON.parse((message as MessageEvent).data) as TransactionEvent;
      setEvents((current) => [event, ...current].slice(0, 10));
      void refresh();
    });
    return () => {
      source.close();
      setLive(false);
    };
  }, [refresh, token]);

  async function submitAuth(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setLoading(true);
    setError("");
    setNotice("");
    try {
      const path = authMode === "login" ? "/api/v1/auth/login" : "/api/v1/auth/register";
      const body =
        authMode === "login"
          ? { email, password }
          : { email, full_name: fullName, password, role: "customer" };
      const response = await fetch(`${AUTH_API}${path}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body)
      });
      const data = (await response.json()) as AuthResult & { error?: string };
      if (!response.ok) {
        throw new Error(data.error ?? "authentication failed");
      }
      setToken(data.token);
      setUser(data.user);
      window.localStorage.setItem("fintech_token", data.token);
      window.localStorage.setItem("fintech_user", JSON.stringify(data.user));
      setNotice(`${authMode === "login" ? "Signed in" : "Registered"} as ${data.user.full_name}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "authentication failed");
    } finally {
      setLoading(false);
    }
  }

  async function createAccount() {
    setLoading(true);
    setError("");
    try {
      const account = await api<Account>("/api/v1/accounts", {
        method: "POST",
        body: JSON.stringify({ currency: "IDR" })
      });
      setSelectedAccountID(account.id);
      setNotice("Account created");
      await loadAccounts();
    } catch (err) {
      setError(err instanceof Error ? err.message : "unable to create account");
    } finally {
      setLoading(false);
    }
  }

  async function submitTransaction(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedAccount) return;
    setLoading(true);
    setError("");
    setNotice("");
    try {
      const amountCents = Math.round(Number(amount) * 100);
      if (!Number.isFinite(amountCents) || amountCents <= 0) {
        throw new Error("amount must be positive");
      }
      const path =
        action === "deposit"
          ? "/api/v1/transactions/deposit"
          : action === "withdraw"
            ? "/api/v1/transactions/withdraw"
            : "/api/v1/transactions/transfer";
      const body =
        action === "transfer"
          ? {
              from_account_id: selectedAccount.id,
              to_account_id: toAccountID,
              amount_cents: amountCents,
              description
            }
          : {
              account_id: selectedAccount.id,
              amount_cents: amountCents,
              description
            };
      await api(path, { method: "POST", body: JSON.stringify(body) });
      setNotice(`${actionLabel(action)} completed`);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "transaction failed");
    } finally {
      setLoading(false);
    }
  }

  function signOut() {
    setToken("");
    setUser(null);
    setAccounts([]);
    setTransactions([]);
    setBalances([]);
    setSelectedAccountID("");
    setEvents([]);
    window.localStorage.removeItem("fintech_token");
    window.localStorage.removeItem("fintech_user");
  }

  return (
    <main className="page">
      <header className="topbar">
        <div className="brand">
          <div className="brandMark" title="FinTech Core API">
            <Landmark size={20} />
          </div>
          <div>
            <h1>FinTech Core API</h1>
            <span>Secure ledger operations dashboard</span>
          </div>
        </div>
        <div className="session">
          <div className="status">
            <span className={live ? "dot live" : "dot"} />
            {live ? "Live stream" : "Stream idle"}
          </div>
          {user ? (
            <button className="button secondary" type="button" onClick={signOut} title="Sign out">
              <LogOut size={16} />
              Sign out
            </button>
          ) : null}
        </div>
      </header>

      <div className="shell">
        <aside className="panel">
          <div className="panelHeader">
            <h2>{user ? "Workspace" : "Access"}</h2>
            {user ? <span className="hint">{user.role}</span> : null}
          </div>

          {!user ? (
            <form className="stack" onSubmit={submitAuth}>
              <div className="segmented" aria-label="Authentication mode">
                <button
                  className={authMode === "login" ? "active" : ""}
                  type="button"
                  onClick={() => setAuthMode("login")}
                >
                  Login
                </button>
                <button
                  className={authMode === "register" ? "active" : ""}
                  type="button"
                  onClick={() => setAuthMode("register")}
                >
                  Register
                </button>
              </div>
              <div className="field">
                <label>Email</label>
                <input value={email} onChange={(event) => setEmail(event.target.value)} type="email" required />
              </div>
              {authMode === "register" ? (
                <div className="field">
                  <label>Full name</label>
                  <input value={fullName} onChange={(event) => setFullName(event.target.value)} required />
                </div>
              ) : null}
              <div className="field">
                <label>Password</label>
                <input
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  type="password"
                  minLength={12}
                  required
                />
              </div>
              <button className="button" type="submit" disabled={loading} title="Authenticate">
                <LogIn size={16} />
                {authMode === "login" ? "Login" : "Register"}
              </button>
            </form>
          ) : (
            <div className="stack">
              <div>
                <strong>{user.full_name}</strong>
                <div className="hint">{user.email}</div>
              </div>
              <button className="button" type="button" onClick={createAccount} disabled={loading} title="Create account">
                <PlusCircle size={16} />
                New account
              </button>
              <div className="accountList">
                {accounts.map((account) => (
                  <button
                    className={account.id === selectedAccount?.id ? "accountButton active" : "accountButton"}
                    key={account.id}
                    type="button"
                    onClick={() => setSelectedAccountID(account.id)}
                  >
                    <strong>{formatMoney(account.balance_cents, account.currency)}</strong>
                    <span>{shortID(account.id)}</span>
                  </button>
                ))}
                {accounts.length === 0 ? <div className="empty">Create an account to begin ledger operations.</div> : null}
              </div>
            </div>
          )}
        </aside>

        <section className="main">
          {error ? <div className="alert error">{error}</div> : null}
          {notice ? <div className="alert">{notice}</div> : null}

          <div className="metrics">
            <div className="card metric">
              <span>Total balance</span>
              <strong>{formatMoney(totalBalance, selectedAccount?.currency ?? "IDR")}</strong>
            </div>
            <div className="card metric">
              <span>Selected account</span>
              <strong>{selectedAccount ? shortID(selectedAccount.id) : "No account"}</strong>
            </div>
            <div className="card metric">
              <span>Loaded transactions</span>
              <strong>{transactionCount}</strong>
            </div>
          </div>

          <div className="grid">
            <section className="card">
              <div className="cardHeader">
                <h2>Monthly Balance</h2>
                <button className="button secondary" type="button" onClick={refresh} disabled={!token} title="Refresh data">
                  <RefreshCw size={16} />
                  Refresh
                </button>
              </div>
              {chartData.length > 0 ? (
                <div className="chartWrap">
                  <ResponsiveContainer>
                    <AreaChart data={chartData} margin={{ top: 12, right: 12, left: 0, bottom: 0 }}>
                      <defs>
                        <linearGradient id="balanceFill" x1="0" x2="0" y1="0" y2="1">
                          <stop offset="5%" stopColor="#167b68" stopOpacity={0.28} />
                          <stop offset="95%" stopColor="#167b68" stopOpacity={0.02} />
                        </linearGradient>
                      </defs>
                      <CartesianGrid stroke="#d9ded7" vertical={false} />
                      <XAxis dataKey="month" tickLine={false} axisLine={false} />
                      <YAxis tickLine={false} axisLine={false} tickFormatter={(value) => compactMoney(Number(value))} />
                      <Tooltip formatter={(value) => formatMoney(Number(value) * 100, selectedAccount?.currency ?? "IDR")} />
                      <Area
                        type="monotone"
                        dataKey="balance"
                        stroke="#167b68"
                        strokeWidth={2}
                        fill="url(#balanceFill)"
                      />
                    </AreaChart>
                  </ResponsiveContainer>
                </div>
              ) : (
                <div className="empty">Monthly balance data will appear after transactions are recorded.</div>
              )}
            </section>

            <section className="card">
              <div className="cardHeader">
                <h2>Move Money</h2>
                <span className="hint">{selectedAccount ? selectedAccount.currency : "IDR"}</span>
              </div>
              <form className="stack" onSubmit={submitTransaction}>
                <div className="actionGrid">
                  <button
                    className={action === "deposit" ? "button" : "button secondary"}
                    type="button"
                    onClick={() => setAction("deposit")}
                    title="Deposit"
                  >
                    <ArrowDownToLine size={16} />
                    Deposit
                  </button>
                  <button
                    className={action === "withdraw" ? "button warn" : "button secondary"}
                    type="button"
                    onClick={() => setAction("withdraw")}
                    title="Withdraw"
                  >
                    <ArrowUpFromLine size={16} />
                    Withdraw
                  </button>
                  <button
                    className={action === "transfer" ? "button" : "button secondary"}
                    type="button"
                    onClick={() => setAction("transfer")}
                    title="Transfer"
                  >
                    <ArrowRightLeft size={16} />
                    Transfer
                  </button>
                </div>
                {action === "transfer" ? (
                  <div className="field">
                    <label>Destination account ID</label>
                    <input value={toAccountID} onChange={(event) => setToAccountID(event.target.value)} required />
                  </div>
                ) : null}
                <div className="field">
                  <label>Amount</label>
                  <input value={amount} onChange={(event) => setAmount(event.target.value)} inputMode="decimal" required />
                </div>
                <div className="field">
                  <label>Description</label>
                  <textarea value={description} onChange={(event) => setDescription(event.target.value)} maxLength={280} />
                </div>
                <button className="button" type="submit" disabled={!selectedAccount || loading} title="Submit transaction">
                  <Send size={16} />
                  Submit
                </button>
              </form>
            </section>
          </div>

          <div className="grid">
            <section className="card">
              <div className="cardHeader">
                <h2>Transaction History</h2>
                <span className="hint">{selectedAccount ? shortID(selectedAccount.id) : "No account"}</span>
              </div>
              <div className="table">
                {transactions.map((transaction) => (
                  <div className="transactionRow" key={transaction.id}>
                    <div className="transactionPrimary">
                      <strong>{transaction.type.replace("_", " ")}</strong>
                      <span>
                        {transaction.description || "No description"} · {new Date(transaction.created_at).toLocaleString()}
                      </span>
                    </div>
                    <div className={isPositive(transaction.type) ? "amount positive" : "amount negative"}>
                      {isPositive(transaction.type) ? "+" : "-"}
                      {formatMoney(transaction.amount_cents, selectedAccount?.currency ?? "IDR")}
                    </div>
                  </div>
                ))}
                {transactions.length === 0 ? <div className="empty">No transactions loaded for this account.</div> : null}
              </div>
            </section>

            <section className="card">
              <div className="cardHeader">
                <h2>Live Activity</h2>
                <Activity size={17} color="#167b68" />
              </div>
              <div className="activityList">
                {events.map((event) => (
                  <div className="activity" key={`${event.transaction.id}-${event.published_at}`}>
                    <strong>{event.operation}</strong>
                    <span>
                      {" "}
                      {formatMoney(event.transaction.amount_cents, event.account.currency)} · {shortID(event.account.id)}
                    </span>
                  </div>
                ))}
                {events.length === 0 ? <div className="empty">Real-time events will stream here.</div> : null}
              </div>
            </section>
          </div>
        </section>
      </div>
    </main>
  );
}

function formatMoney(amountCents: number, currency: string) {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency,
    maximumFractionDigits: 2
  }).format(amountCents / 100);
}

function compactMoney(value: number) {
  return new Intl.NumberFormat("id-ID", {
    notation: "compact",
    maximumFractionDigits: 1
  }).format(value);
}

function shortID(id: string) {
  if (!id) return "";
  return `${id.slice(0, 8)}...${id.slice(-4)}`;
}

function isPositive(type: string) {
  return type === "deposit" || type === "transfer_in";
}

function actionLabel(action: ActionKind) {
  return action === "deposit" ? "Deposit" : action === "withdraw" ? "Withdrawal" : "Transfer";
}
