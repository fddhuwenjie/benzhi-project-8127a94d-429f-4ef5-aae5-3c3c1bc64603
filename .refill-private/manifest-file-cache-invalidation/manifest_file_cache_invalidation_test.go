package manifestfilecacheinvalidation_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oralhistory/internal/application"
	"oralhistory/internal/domain"
	"oralhistory/internal/persistence"
	"oralhistory/internal/webui"
)

func TestManifestFileReplacementInvalidatesCachedContent(t *testing.T) {
	root := t.TempDir()
	store, err := persistence.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	caseID := "case-resource-cache"
	caseContent, err := json.Marshal(domain.OralHistoryCase{CaseID: caseID, Status: domain.StatusSealed})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cases", caseID+".json"), caseContent, 0o640); err != nil {
		t.Fatal(err)
	}
	manifestContent, err := json.Marshal(domain.ReleaseManifest{
		ManifestID: "manifest-resource-cache",
		CaseID:     caseID,
		PublicSegments: []domain.PublicSegment{{
			SegmentID: "segment-original", StartMS: 0, EndMS: 100, Text: "首次加载的公开内容",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "manifests", caseID+".json")
	if err := os.WriteFile(manifestPath, manifestContent, 0o440); err != nil {
		t.Fatal(err)
	}

	handler := webui.NewServer(application.NewService(store)).Handler()
	first := queryManifest(t, handler, caseID)
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), "首次加载的公开内容") {
		t.Fatalf("首次清单查询失败: status=%d body=%s", first.Code, first.Body.String())
	}
	if err := os.Chmod(manifestPath, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte("{broken manifest\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	second := queryManifest(t, handler, caseID)
	if second.Code != http.StatusUnprocessableEntity || !strings.Contains(second.Body.String(), domain.ErrIntegrity) {
		t.Fatalf("清单文件失效后仍返回缓存内容: status=%d body=%s", second.Code, second.Body.String())
	}
}

func queryManifest(t *testing.T, handler http.Handler, caseID string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/cases/"+caseID+"/manifest-query", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
