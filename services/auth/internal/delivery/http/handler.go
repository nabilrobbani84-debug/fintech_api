package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/example/fintech-core-api/services/auth/internal/domain"
	"github.com/example/fintech-core-api/services/auth/internal/usecase"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"go.uber.org/zap"
)

type Handler struct {
	mux         *http.ServeMux
	auth        *usecase.AuthUsecase
	logger      *zap.Logger
	swaggerFile string
	origin      string
}

func NewHandler(auth *usecase.AuthUsecase, logger *zap.Logger, swaggerFile string, origin string) http.Handler {
	h := &Handler{
		mux:         http.NewServeMux(),
		auth:        auth,
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
	h.mux.HandleFunc("GET /healthz", h.health)
	h.mux.HandleFunc("GET /swagger/doc.yaml", h.swaggerDoc)
	h.mux.Handle("GET /swagger/", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.yaml")))
	h.mux.HandleFunc("POST /api/v1/auth/register", h.register)
	h.mux.HandleFunc("POST /api/v1/auth/login", h.login)
	h.mux.Handle("GET /api/v1/auth/me", authMiddleware(h.auth)(http.HandlerFunc(h.me)))
	h.mux.Handle("GET /api/v1/admin/users/{id}", authMiddleware(h.auth)(requireRoles(domain.RoleAdmin)(http.HandlerFunc(h.adminGetUser))))
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "auth"})
}

func (h *Handler) swaggerDoc(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, h.swaggerFile)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var input usecase.RegisterInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.auth.Register(r.Context(), input)
	if err != nil {
		h.handleUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input usecase.LoginInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.auth.Login(r.Context(), input)
	if err != nil {
		h.handleUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user context")
		return
	}
	user, err := h.auth.GetProfile(r.Context(), userID)
	if err != nil {
		h.handleUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) adminGetUser(w http.ResponseWriter, r *http.Request) {
	user, err := h.auth.GetProfile(r.Context(), r.PathValue("id"))
	if err != nil {
		h.handleUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) handleUsecaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, usecase.ErrValidation):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, usecase.ErrEmailTaken):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, usecase.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "resource not found")
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
