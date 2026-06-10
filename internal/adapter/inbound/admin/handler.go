// Package admin exposes a read-only HTTP API for operators to inspect jobs,
// users and deliveries. It returns DTOs (never raw domain structs) and supports
// pagination and filtering on the jobs listing. It performs no mutations.
package admin

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// Config holds admin API settings.
type Config struct {
	// Token, when non-empty, must be presented in the X-Admin-Token header.
	Token string
}

// Deps are the repositories the admin API reads from.
type Deps struct {
	Jobs       domain.JobRepository
	Users      domain.UserRepository
	Deliveries domain.DeliveryRepository
	// Billing is optional; when set, user responses include the credit balance.
	Billing domain.BillingRepository
}

// Handler serves the admin endpoints.
type Handler struct {
	cfg  Config
	deps Deps
}

// NewHandler builds an admin Handler.
func NewHandler(cfg Config, deps Deps) *Handler {
	return &Handler{cfg: cfg, deps: deps}
}

// Routes returns an http.Handler with the admin routes registered.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/jobs", h.auth(h.listJobs))
	mux.HandleFunc("GET /admin/jobs/{id}", h.auth(h.getJob))
	mux.HandleFunc("GET /admin/users/{id}", h.auth(h.getUser))
	mux.HandleFunc("GET /admin/deliveries/{id}", h.auth(h.getDelivery))
	return mux
}

// auth wraps a handler with the optional admin-token check.
func (h *Handler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.cfg.Token != "" && !adminTokenEqual(r.Header.Get("X-Admin-Token"), h.cfg.Token) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func adminTokenEqual(got, want string) bool {
	if want == "" || got == "" || len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	var filter domain.JobFilter
	if raw := r.URL.Query().Get("user_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid user_id")
			return
		}
		filter.UserID = &id
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = domain.JobStatus(status)
	}
	if op := r.URL.Query().Get("operation"); op != "" {
		filter.Operation = domain.OperationType(op)
	}

	// Fetch one extra row to determine whether more pages exist.
	jobs, err := h.deps.Jobs.List(r.Context(), filter, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list jobs failed")
		return
	}
	hasMore := len(jobs) > limit
	if hasMore {
		jobs = jobs[:limit]
	}

	items := make([]JobDTO, 0, len(jobs))
	for _, j := range jobs {
		items = append(items, newJobDTO(j))
	}
	writeJSON(w, http.StatusOK, listResponse[JobDTO]{
		Items:      items,
		Pagination: pagination{Limit: limit, Offset: offset, Count: len(items), HasMore: hasMore},
	})
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	job, err := h.deps.Jobs.GetByID(r.Context(), id)
	if err != nil {
		writeNotFoundOr500(w, err, "get job failed")
		return
	}
	writeJSON(w, http.StatusOK, newJobDTO(job))
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	user, err := h.deps.Users.GetByID(r.Context(), id)
	if err != nil {
		writeNotFoundOr500(w, err, "get user failed")
		return
	}
	dto := newUserDTO(user)
	if h.deps.Billing != nil {
		if acc, aerr := h.deps.Billing.GetAccountByUser(r.Context(), user.ID, domain.CurrencyCredits); aerr == nil {
			dto.BalanceCredits = &acc.BalanceCached
		}
	}
	writeJSON(w, http.StatusOK, dto)
}

func (h *Handler) getDelivery(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	del, err := h.deps.Deliveries.GetByID(r.Context(), id)
	if err != nil {
		writeNotFoundOr500(w, err, "get delivery failed")
		return
	}
	writeJSON(w, http.StatusOK, newDeliveryDTO(del))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parsePagination(r *http.Request) (limit, offset int) {
	limit = defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}
	return limit, offset
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func writeNotFoundOr500(w http.ResponseWriter, err error, msg string) {
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeError(w, http.StatusInternalServerError, msg)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
