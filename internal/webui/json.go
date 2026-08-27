package webui

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"oralhistory/internal/domain"
)

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, domain.ErrInvalidInput, "Content-Type 必须为 application/json")
		return false
	}
	reader := http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, domain.ErrInvalidInput, "JSON 请求无效: "+err.Error())
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, domain.ErrInvalidInput, "请求只能包含一个 JSON 对象")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var envelope errorEnvelope
	envelope.Error.Code = code
	envelope.Error.Message = message
	writeJSON(w, status, envelope)
}

func mapError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := domain.ErrorCode(err)
	switch code {
	case domain.ErrInvalidInput:
		status = http.StatusBadRequest
	case domain.ErrNotFound:
		status = http.StatusNotFound
	case domain.ErrForbidden:
		status = http.StatusForbidden
	case domain.ErrRevisionConflict, domain.ErrIdempotencyConflict, domain.ErrInvalidState:
		status = http.StatusConflict
	case domain.ErrIntegrity:
		status = http.StatusUnprocessableEntity
	}
	writeError(w, status, code, err.Error())
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不受支持")
}
