package websession

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/service/accounthistory"
)

type AccountHistoryService interface {
	List(context.Context, uuid.UUID, int, string) (accounthistory.Page, error)
}

func (h *Handler) accountHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	query := r.URL.Query()
	limit := 20
	cursor := ""
	for key, values := range query {
		if (key != "limit" && key != "cursor") || len(values) != 1 {
			writeError(w, 400, "invalid history query")
			return
		}
	}
	if values, ok := query["limit"]; ok {
		var err error
		limit, err = strconv.Atoi(values[0])
		if err != nil || limit < 1 || limit > 50 {
			writeError(w, 400, "invalid history query")
			return
		}
	}
	if values, ok := query["cursor"]; ok {
		cursor = values[0]
		if len(cursor) > 512 {
			writeError(w, 400, "invalid history query")
			return
		}
	}
	if h.deps.AccountHistory == nil {
		writeError(w, 503, "account history unavailable")
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	page, err := h.deps.AccountHistory.List(r.Context(), principal.AccountID, limit, cursor)
	if err != nil {
		if errors.Is(err, accounthistory.ErrInvalidQuery) {
			writeError(w, 400, "invalid history query")
		} else {
			writeError(w, 503, "account history unavailable")
		}
		return
	}
	writeJSON(w, http.StatusOK, page)
}
