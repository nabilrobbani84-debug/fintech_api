from __future__ import annotations

from dataclasses import dataclass
from datetime import date

import streamlit as st


st.set_page_config(
    page_title="FinTech Core API Demo",
    page_icon=":credit_card:",
    layout="wide",
    initial_sidebar_state="expanded",
)


@dataclass(frozen=True)
class Transaction:
    trx_id: str
    source: str
    destination: str
    trx_type: str
    amount: int
    status: str


accounts = {
    "AC-1001": {"owner": "Nabil Robbani", "balance": 24_750_000},
    "AC-1002": {"owner": "Warkop Digital", "balance": 12_400_000},
    "AC-1003": {"owner": "Ruqyah Center", "balance": 8_950_000},
}

transactions = [
    Transaction("TRX-2401", "External", "AC-1001", "Deposit", 4_500_000, "Settled"),
    Transaction("TRX-2402", "AC-1001", "AC-1002", "Transfer", 1_250_000, "Settled"),
    Transaction("TRX-2403", "AC-1002", "External", "Withdraw", 750_000, "Settled"),
    Transaction("TRX-2404", "AC-1001", "AC-1003", "Transfer", 2_000_000, "Settled"),
]


def rupiah(value: int) -> str:
    return f"Rp {value:,.0f}".replace(",", ".")


with st.sidebar:
    st.title("FinTech Core API")
    selected_page = st.radio(
        "Demo view",
        ["Overview", "API Explorer", "Transaction Simulator", "Architecture"],
    )
    st.caption("Streamlit demo for the Go microservices portfolio project.")


st.title("FinTech Core API")
st.write(
    "Microservices demo for a financial transaction platform with Go, Clean Architecture, "
    "JWT auth, encrypted user data, PostgreSQL ledger locking, gRPC service communication, "
    "and a Next.js operational dashboard."
)


if selected_page == "Overview":
    total_balance = sum(account["balance"] for account in accounts.values())
    total_volume = sum(transaction.amount for transaction in transactions)

    metric_a, metric_b, metric_c, metric_d = st.columns(4)
    metric_a.metric("Accounts", len(accounts))
    metric_b.metric("Ledger Balance", rupiah(total_balance))
    metric_c.metric("Transaction Volume", rupiah(total_volume))
    metric_d.metric("Services", "Auth + Ledger")

    st.subheader("Operational Snapshot")
    st.dataframe(
        [
            {
                "Account ID": account_id,
                "Owner": data["owner"],
                "Balance": rupiah(data["balance"]),
            }
            for account_id, data in accounts.items()
        ],
        use_container_width=True,
        hide_index=True,
    )

    st.subheader("Recent Ledger Events")
    st.dataframe(
        [
            {
                "ID": transaction.trx_id,
                "Type": transaction.trx_type,
                "Source": transaction.source,
                "Destination": transaction.destination,
                "Amount": rupiah(transaction.amount),
                "Status": transaction.status,
            }
            for transaction in transactions
        ],
        use_container_width=True,
        hide_index=True,
    )

    st.subheader("Monthly Balance Trend")
    st.line_chart(
        {
            "Balance": [12_500_000, 14_250_000, 17_000_000, 21_350_000, 24_750_000],
            "Transaction Volume": [2_250_000, 3_800_000, 5_200_000, 6_100_000, 8_500_000],
        }
    )


elif selected_page == "API Explorer":
    st.subheader("Public REST Endpoints")
    endpoint = st.selectbox(
        "Choose endpoint",
        [
            "POST /api/v1/auth/register",
            "POST /api/v1/auth/login",
            "GET /api/v1/auth/me",
            "POST /api/v1/accounts",
            "GET /api/v1/accounts",
            "POST /api/v1/transactions/deposit",
            "POST /api/v1/transactions/withdraw",
            "POST /api/v1/transactions/transfer",
            "GET /api/v1/reports/monthly-balance",
            "GET /api/v1/events",
        ],
    )

    auth_required = not endpoint.startswith("POST /api/v1/auth/register") and not endpoint.startswith(
        "POST /api/v1/auth/login"
    )
    service = "Auth Service" if "/auth/" in endpoint else "Transaction Service"

    info_a, info_b, info_c = st.columns(3)
    info_a.metric("Service", service)
    info_b.metric("Auth", "JWT required" if auth_required else "Public")
    info_c.metric("Transport", "REST API")

    st.code(
        f"""curl -X {endpoint.split()[0]} \\
  http://localhost:{'8080' if service == 'Auth Service' else '8081'}{endpoint.split()[1]} \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer <jwt>" """,
        language="bash",
    )

    st.info(
        "This Streamlit demo is intentionally safe: it documents and simulates the API flow without "
        "connecting to a real banking backend or moving money."
    )


elif selected_page == "Transaction Simulator":
    st.subheader("Ledger-safe Transfer Simulation")

    source = st.selectbox("Source account", list(accounts.keys()))
    destination = st.selectbox(
        "Destination account",
        [account_id for account_id in accounts.keys() if account_id != source],
    )
    amount = st.number_input("Amount", min_value=10_000, max_value=5_000_000, value=250_000, step=10_000)
    submitted = st.button("Simulate transfer", type="primary")

    if submitted:
        source_balance = accounts[source]["balance"]
        if amount > source_balance:
            st.error("Transfer rejected: insufficient balance.")
        else:
            st.success("Transfer accepted and committed atomically.")
            st.write(
                {
                    "transaction_id": f"TRX-{date.today().strftime('%y%m%d')}-SIM",
                    "source": source,
                    "destination": destination,
                    "amount_cents": amount * 100,
                    "locking_strategy": "SELECT ... FOR UPDATE with deterministic account ordering",
                    "status": "Settled",
                }
            )

    st.caption(
        "The Go transaction service uses integer amount_cents and PostgreSQL row locks to avoid "
        "floating-point drift and race conditions."
    )


else:
    st.subheader("Service Architecture")
    st.markdown(
        """
        ```mermaid
        flowchart LR
          Dashboard[Streamlit / Next.js Dashboard] -->|REST + SSE| Transaction[Transaction Service]
          Dashboard -->|REST| Auth[Auth Service]
          Transaction -->|gRPC ValidateToken| Auth
          Auth -->|SQL| MySQL[(MySQL users)]
          Transaction -->|SQL + row locks| Postgres[(PostgreSQL ledger)]
        ```
        """
    )
    st.write(
        "- Auth Service handles JWT, RBAC, bcrypt password hashing, AES-GCM encrypted profile data, and HMAC email lookup.\n"
        "- Transaction Service handles account creation, deposit, withdrawal, transfer, monthly reports, and Server-Sent Events.\n"
        "- gRPC is used for service-to-service token validation.\n"
        "- Docker Compose wires the local development stack across MySQL, PostgreSQL, Go services, and dashboard."
    )
