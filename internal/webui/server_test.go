package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oralhistory/internal/application"
	"oralhistory/internal/persistence"
)

func TestIndexAndJSONValidation(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := NewServer(application.NewService(store)).Handler()
	index := httptest.NewRecorder()
	handler.ServeHTTP(index, httptest.NewRequest(http.MethodGet, "/", nil))
	if index.Code != http.StatusOK || !strings.Contains(index.Body.String(), "<body>") {
		t.Fatalf("首页响应无效: %d %s", index.Code, index.Body.String())
	}
	bad := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/cases", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "text/plain")
	handler.ServeHTTP(bad, request)
	if bad.Code != http.StatusUnsupportedMediaType || !strings.Contains(bad.Body.String(), "invalid_input") {
		t.Fatalf("JSON 错误映射无效: %d %s", bad.Code, bad.Body.String())
	}
}
