package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/example/fintech-core-api/services/transaction/internal/domain"
	"github.com/example/fintech-core-api/services/transaction/internal/events"
	"github.com/example/fintech-core-api/services/transaction/internal/usecase"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"go.uber.org/zap"
)

type Handler struct {
	mux         *http.ServeMux
	ledger      *usecase.TransactionUsecase
	auth        authClient
	broker      *events.Broker
	logger      *zap.Logger
	swaggerFile string
	origin      string
}

func NewHandler(ledger *usecase.TransactionUsecase, auth authClient, broker *events.Broker, logger *zap.Logger, swaggerFile string, origin string) http.Handler {
	h := &Handler{
		mux:         http.NewServeMux(),
		ledger:      ledger,
		auth:        auth,
		broker:      broker,
		logger:      logger,
		swaggerFile: swaggerFile,
		origin:      origin,
	}
	h.routes()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler := recoverMiddleware(h.logger)(
		requestLogger(h.logger)(
			cors(h.origin)(
				secureHeaders(h.mux),
			),
		),
	)
	handler.ServeHTTP(w, r)
}

func (h *Handler) routes() {
	protected := authMiddleware(h.auth)
	h.mux.HandleFunc("GET /healthz", h.health)
	h.mux.HandleFunc("GET /swagger/doc.yaml", h.swaggerDoc)
	h.mux.Handle("GET /swagger/", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.yaml")))
	h.mux.Handle("POST /api/v1/accounts", protected(http.HandlerFunc(h.createAccount)))
	h.mux.Handle("GET /api/v1/accounts", protected(http.HandlerFunc(h.listAccounts)))
	h.mux.Handle("POST /api/v1/transactions/deposit", protected(http.HandlerFunc(h.deposit)))
	h.mux.Handle("POST /api/v1/transactions/withdraw", protected(http.HandlerFunc(h.withdraw)))
	h.mux.Handle("POST /api/v1/transactions/transfer", protected(http.HandlerFunc(h.transfer)))
	h.mux.Handle("GET /api/v1/transactions", protected(http.HandlerFunc(h.listTransactions)))
	h.mux.Handle("GET /api/v1/reports/monthly-balance", protected(http.HandlerFunc(h.monthlyBalances)))
	h.mux.Handle("GET /api/v1/events", protected(http.HandlerFunc(h.events)))
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "transaction"})
}

func (h *Handler) swaggerDoc(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, h.swaggerFile)
}

func (h *Handler) createAccount(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing actor context")
		return
	}
	var input struct {
		Currency string `json:"currency"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	account, err := h.ledger.CreateAccount(r.Context(), actor, input.Currency)
	if err != nil {
		h.handleUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, account)
}

func (h *Handler) listAccounts(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing actor context")
		return
	}
	accounts, err := h.ledger.ListAccounts(r.Context(), actor)
	if err != nil {
		h.handleUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, accounts)
}

func (h *Handler) deposit(w http.ResponseWriter, r *http.Request) {
	h.moneyOperation(w, r, h.ledger.Deposit)
}

func (h *Handler) withdraw(w http.ResponseWriter, r *http.Request) {
	h.moneyOperation(w, r, h.ledger.Withdraw)
}

func (h *Handler) moneyOperation(w http.ResponseWriter, r *http.Request, operation func(context.Context, domain.Actor, usecase.MoneyInput) (*usecase.OperationResult, error)) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing actor context")
		return
	}
	var input usecase.MoneyInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := operation(r.Context(), actor, input)
	if err != nil {
		h.handleUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) transfer(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing actor context")
		return
	}
	var input usecase.TransferInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.ledger.Transfer(r.Context(), actor, input)
	if err != nil {
		h.handleUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listTransactions(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing actor context")
		return
	}
	accountID := r.URL.Query().Get("account_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	transactions, err := h.ledger.ListTransactions(r.Context(), actor, accountID, limit)
	if err != nil {
		h.handleUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, transactions)
}

func (h *Handler) monthlyBalances(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing actor context")
		return
	}
	accountID := r.URL.Query().Get("account_id")
	months, _ := strconv.Atoi(r.URL.Query().Get("months"))
	balances, err := h.ledger.MonthlyBalances(r.Context(), actor, accountID, months)
	if err != nil {
		h.handleUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, balances)
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing actor context")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	eventsCh := h.broker.Subscribe(r.Context())
	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-eventsCh:
			if !ok {
				return
			}
			if actor.Role != domain.RoleAdmin && event.OwnerUserID != actor.UserID {
				continue
			}
			payload, err := json.Marshal(event)
			if err != nil {
				h.logger.Error("marshal event", zap.Error(err))
				continue
			}
			_, _ = fmt.Fprintf(w, "event: transaction\ndata: %s\n\n", payload)
			flusher.Flush()
		}
	}
}

func (h *Handler) handleUsecaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "resource not found")
	case errors.Is(err, domain.ErrInsufficientFunds):
		writeError(w, http.StatusConflict, err.Error())
	default:
		h.logger.Error("request failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
