package encoder_reuse_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oralhistory/internal/application"
	"oralhistory/internal/persistence"
	"oralhistory/internal/webui"
)

func TestInvalidInputEncoderDoesNotCaptureNextResponse(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := webui.NewServer(application.NewService(store)).Handler()

	invalid := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/cases", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "text/plain")
	handler.ServeHTTP(invalid, request)
	if invalid.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("无效请求应返回 415，实际为 %d", invalid.Code)
	}
	invalidBody := invalid.Body.String()

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK || !strings.Contains(health.Body.String(), `"status":"ok"`) {
		t.Fatalf("后续健康响应被旧编码器截获: status=%d body=%q", health.Code, health.Body.String())
	}
	if invalid.Body.String() != invalidBody {
		t.Fatalf("后续响应污染了已完成的错误响应: before=%q after=%q", invalidBody, invalid.Body.String())
	}
}
