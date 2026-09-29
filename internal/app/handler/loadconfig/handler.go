// Package loadconfig is the HTTP adapter for load_config, templates, and meta.
package loadconfig

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/app/httpx"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/port"
)

const maxBodyBytes = 1 << 20

// Handler serves the load-config console API.
type Handler struct {
	usecase port.LoadConfigUsecase
}

// NewHandler returns the HTTP handler.
func NewHandler(usecase port.LoadConfigUsecase) *Handler {
	return &Handler{usecase: usecase}
}

type listResponse struct {
	Items []model.LoadConfig `json:"items"`
}

type templateListResponse struct {
	Items []model.Template `json:"items"`
}

type testPatternRequest struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

// List handles GET /api/v1/load-configs.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.usecase.List(r.Context(), model.ListFilter{
		PartnerType: r.URL.Query().Get("partnerType"),
		ImportType:  r.URL.Query().Get("importType"),
		Query:       r.URL.Query().Get("q"),
	})
	if err != nil {
		h.writeError(w, err)

		return
	}

	if items == nil {
		items = []model.LoadConfig{}
	}

	httpx.WriteJSON(w, http.StatusOK, listResponse{Items: items})
}

// Get handles GET /api/v1/load-configs/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.usecase.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeError(w, err)

		return
	}

	httpx.WriteJSON(w, http.StatusOK, cfg)
}

// Create handles POST /api/v1/load-configs.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var cfg model.LoadConfig
	if !decodeJSON(w, r, &cfg) {
		return
	}

	created, err := h.usecase.Create(r.Context(), cfg)
	if err != nil {
		h.writeError(w, err)

		return
	}

	w.Header().Set("Location", "/api/v1/load-configs/"+url.PathEscape(created.ID))
	httpx.WriteJSON(w, http.StatusCreated, created)
}

// Update handles PUT /api/v1/load-configs/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var cfg model.LoadConfig
	if !decodeJSON(w, r, &cfg) {
		return
	}

	id := r.PathValue("id")

	updated, err := h.usecase.Update(r.Context(), id, cfg)
	if err != nil {
		h.writeError(w, err)

		return
	}

	if updated.ID != id {
		w.Header().Set("Location", "/api/v1/load-configs/"+url.PathEscape(updated.ID))
	}

	httpx.WriteJSON(w, http.StatusOK, updated)
}

// Delete handles DELETE /api/v1/load-configs/{id}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.usecase.Delete(r.Context(), r.PathValue("id")); err != nil {
		h.writeError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Derive handles POST /api/v1/load-configs/derive.
func (h *Handler) Derive(w http.ResponseWriter, r *http.Request) {
	var identity model.Identity
	if !decodeJSON(w, r, &identity) {
		return
	}

	derived, err := h.usecase.Derive(r.Context(), identity)
	if err != nil {
		h.writeError(w, err)

		return
	}

	httpx.WriteJSON(w, http.StatusOK, derived)
}

// ListOrganizations handles GET /api/v1/organizations.
func (h *Handler) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if !model.ValidPartner(kind) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "type must be advertiser or publisher", nil)

		return
	}

	items, err := h.usecase.ListOrganizations(r.Context(), kind)
	if err != nil {
		h.writeError(w, err)

		return
	}

	httpx.WriteJSON(w, http.StatusOK, refsOrEmpty(items))
}

// ListAccounts handles GET /api/v1/organizations/{id}/accounts.
func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	items, err := h.usecase.ListAccounts(r.Context(), r.PathValue("id"))
	if err != nil {
		h.writeError(w, err)

		return
	}

	httpx.WriteJSON(w, http.StatusOK, refsOrEmpty(items))
}

// ListBases handles GET /api/v1/organizations/{slug}/bases.
func (h *Handler) ListBases(w http.ResponseWriter, r *http.Request) {
	kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if kind != model.PartnerPublisher {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "type must be publisher", nil)

		return
	}

	items, err := h.usecase.ListBases(r.Context(), r.PathValue("slug"))
	if err != nil {
		h.writeError(w, err)

		return
	}

	httpx.WriteJSON(w, http.StatusOK, refsOrEmpty(items))
}

func refsOrEmpty(items []model.NamedRef) []model.NamedRef {
	if items == nil {
		return []model.NamedRef{}
	}

	return items
}

// Validate handles POST /api/v1/load-configs/validate.
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	var cfg model.LoadConfig
	if !decodeJSON(w, r, &cfg) {
		return
	}

	report, err := h.usecase.Validate(r.Context(), cfg)
	if err != nil {
		h.writeError(w, err)

		return
	}

	httpx.WriteJSON(w, http.StatusOK, report)
}

// TestPattern handles POST /api/v1/load-configs/test-pattern.
func (h *Handler) TestPattern(w http.ResponseWriter, r *http.Request) {
	var body testPatternRequest
	if !decodeJSON(w, r, &body) {
		return
	}

	result, err := h.usecase.TestPattern(r.Context(), body.Pattern, body.Path)
	if err != nil {
		h.writeError(w, err)

		return
	}

	httpx.WriteJSON(w, http.StatusOK, result)
}

// Templates handles GET /api/v1/templates.
func (h *Handler) Templates(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, templateListResponse{Items: h.usecase.Templates(r.Context())})
}

// Meta handles GET /api/v1/meta.
func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, h.usecase.Meta(r.Context()))
}

func (h *Handler) writeError(w http.ResponseWriter, err error) {
	var validation *model.ValidationError
	switch {
	case errors.As(err, &validation):
		message := "validation failed"
		if len(validation.Issues) == 1 && validation.Issues[0].Message != "" {
			message = validation.Issues[0].Message
		}

		httpx.WriteError(w, http.StatusUnprocessableEntity, "validation_error", message, validation.Issues)
	case errors.Is(err, model.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not_found", "load config not found", nil)
	case errors.Is(err, model.ErrConflict):
		httpx.WriteError(w, http.StatusConflict, "conflict", "load config already exists", nil)
	default:
		slog.Error("request failed", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error", nil)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid JSON body", nil)

		return false
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "invalid JSON body", nil)

		return false
	}

	return true
}
