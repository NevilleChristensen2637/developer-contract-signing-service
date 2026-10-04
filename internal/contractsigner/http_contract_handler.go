package contractsigner

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/example/developer-contract-signing-service/internal/infrai"
)

type HTTPHandler struct {
	service *Service
}

func NewHTTPHandler(service *Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/contracts/sign" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req ContractSignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	resp, err := h.service.PrepareContract(r.Context(), req)
	if err != nil {
		var apiErr *infrai.APIError
		status := http.StatusInternalServerError
		if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
