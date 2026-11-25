package websession

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

type PresentationHistoryReader interface {
	ListPresentationHistory(context.Context, uuid.UUID, int, *domain.JobCursor) ([]domain.PresentationHistoryRow, error)
}

type safePresentationHistoryItem struct {
	ConversationID uuid.UUID        `json:"conversation_id"`
	JobID          uuid.UUID        `json:"job_id"`
	Title          string           `json:"title"`
	Status         domain.JobStatus `json:"status"`
	CreatedAt      time.Time        `json:"created_at"`
	ModelName      string           `json:"model_name,omitempty"`
	FailureCode    string           `json:"failure_code,omitempty"`
}

type presentationHistoryPage struct {
	Items      []safePresentationHistoryItem `json:"items"`
	NextCursor string                        `json:"next_cursor,omitempty"`
}

type presentationHistoryCursor struct {
	AccountID uuid.UUID `json:"account_id"`
	CreatedAt time.Time `json:"created_at"`
	JobID     uuid.UUID `json:"job_id"`
}

func (h *Handler) listPresentations(w http.ResponseWriter, r *http.Request) {
	if !h.requirePresentationPreview(w, r) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	query := r.URL.Query()
	limit, err := imageJobHistoryLimit(query["limit"], query.Has("limit"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid presentation history limit")
		return
	}
	after, ok := readPresentationHistoryCursor(query["cursor"], query.Has("cursor"), principal.AccountID)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid presentation history cursor")
		return
	}
	if h.deps.PresentationHistory == nil {
		writeError(w, http.StatusServiceUnavailable, "presentation history unavailable")
		return
	}
	rows, err := h.deps.PresentationHistory.ListPresentationHistory(r.Context(), principal.AccountID, limit+1, after)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "presentation history unavailable")
		return
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	page := presentationHistoryPage{Items: make([]safePresentationHistoryItem, 0, len(rows))}
	for _, row := range rows {
		if row.AccountID != principal.AccountID || row.ConversationID == uuid.Nil || row.JobID == uuid.Nil || !row.Status.Valid() || row.CreatedAt.IsZero() {
			writeError(w, http.StatusServiceUnavailable, "presentation history unavailable")
			return
		}
		title := domain.BoundedWebPresentationTitle(row.Title)
		if title == "" || (row.TitleOrigin != domain.ConversationTitleOriginManual && !strings.HasPrefix(title, "Презентация · ")) {
			title = domain.WebPresentationTitle(row.TitlePrompt)
		}
		page.Items = append(page.Items, safePresentationHistoryItem{
			ConversationID: row.ConversationID, JobID: row.JobID, Title: title, Status: row.Status,
			CreatedAt: row.CreatedAt, ModelName: row.ModelName,
			FailureCode: presentationFailureCode(&domain.Job{Status: row.Status, ErrorCode: row.ErrorCode}),
		})
	}
	if hasMore {
		last := rows[len(rows)-1]
		raw, err := json.Marshal(presentationHistoryCursor{AccountID: principal.AccountID, CreatedAt: last.CreatedAt, JobID: last.JobID})
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "presentation history unavailable")
			return
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	writeJSON(w, http.StatusOK, page)
}

func readPresentationHistoryCursor(values []string, provided bool, accountID uuid.UUID) (*domain.JobCursor, bool) {
	if !provided {
		return nil, true
	}
	if len(values) != 1 || values[0] == "" || len(values[0]) > 256 {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(values[0])
	if err != nil {
		return nil, false
	}
	var cursor presentationHistoryCursor
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil || cursor.AccountID != accountID || cursor.JobID == uuid.Nil || cursor.CreatedAt.IsZero() {
		return nil, false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, false
	}
	return &domain.JobCursor{CreatedAt: cursor.CreatedAt, ID: cursor.JobID}, true
}
