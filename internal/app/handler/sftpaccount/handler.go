// Package sftpaccount is the HTTP adapter for SFTPGo account management.
package sftpaccount

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/app/httpx"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/port"
)

const (
	maxBodyBytes = 1 << 20

	unconfiguredMessage = "sftpgo is not configured: set SFTPGO_URL and SFTPGO_API_KEY"
)

// Handler serves the SFTP account routes.
type Handler struct {
	accounts port.SFTPAccounts
}

// NewHandler returns the HTTP handler.
func NewHandler(accounts port.SFTPAccounts) *Handler {
	return &Handler{accounts: accounts}
}

// Config handles GET /api/v1/sftp-accounts/config.
func (h *Handler) Config(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, h.accounts.Config())
}

// Get handles GET /api/v1/sftp-accounts/{username}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	view, err := h.accounts.Get(r.Context(), r.PathValue("username"))
	if err != nil {
		writeError(w, err)

		return
	}

	httpx.WriteJSON(w, http.StatusOK, view)
}

// Preview handles POST /api/v1/sftp-accounts/preview.
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	var req model.SFTPAccountRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	plan, err := h.accounts.Preview(r.Context(), req)
	if err != nil {
		writeError(w, err)

		return
	}

	httpx.WriteJSON(w, http.StatusOK, plan)
}

// Create handles POST /api/v1/sftp-accounts.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.SFTPAccountRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.accounts.Apply(r.Context(), req)
	if err != nil {
		writeError(w, err)

		return
	}

	status := http.StatusOK
	if result.UserAction == model.SFTPActionCreated {
		status = http.StatusCreated
	}

	httpx.WriteJSON(w, status, result)
}

func writeError(w http.ResponseWriter, err error) {
	var validation *model.ValidationError

	var mismatch *model.BucketMismatchError

	var upstream *model.SFTPGoError

	switch {
	case errors.Is(err, model.ErrSFTPGoUnconfigured):
		httpx.WriteError(w, http.StatusServiceUnavailable, "sftpgo_unconfigured", unconfiguredMessage, nil)
	case errors.As(err, &validation):
		message := "validation failed"
		if len(validation.Issues) == 1 && validation.Issues[0].Message != "" {
			message = validation.Issues[0].Message
		}

		httpx.WriteError(w, http.StatusUnprocessableEntity, "validation_error", message, validation.Issues)
	case errors.As(err, &mismatch):
		httpx.WriteError(w, http.StatusConflict, "bucket_mismatch", mismatch.Error(), nil)
	case errors.As(err, &upstream):
		httpx.WriteError(w, http.StatusBadGateway, "sftpgo_error", upstream.Error(), nil)
	default:
		slog.Error("sftp request failed", "error", err)
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
