package helloworld

import (
	"encoding/json"
	"net/http"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/port"
)

type Handler struct {
	usecase port.HelloWorldUsecase
}

func NewHandler(usecase port.HelloWorldUsecase) *Handler {
	return &Handler{usecase: usecase}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "World"
	}

	msg, err := h.usecase.SayHello(r.Context(), name)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(responseDTO{Message: msg}) //nolint:errchkjson
}
