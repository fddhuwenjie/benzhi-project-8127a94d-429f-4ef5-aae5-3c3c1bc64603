package wrappedintegrityerrorchain_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oralhistory/internal/application"
	"oralhistory/internal/persistence"
	"oralhistory/internal/webui"
)

func TestWrappedIntegrityErrorKeepsHTTPClassification(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := application.NewService(store)
	created, err := app.CreateCase(application.CreateCaseCommand{
		CommandMeta: application.CommandMeta{CaseID: "wrapped-error-case", ActorID: "archivist", RequestID: "create-wrapped-error", ExpectedRevision: 0},
		Title:       "错误链复现", CollectionDate: "2026-08-27", CustodyReference: "ERR-1", SourceAudioURI: "archive://wrapped-error.wav",
		SourceSHA256:          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ConsentDocumentSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ArchivistID:           "archivist", ReviewerID: "reviewer",
	})
	if err != nil {
		t.Fatal(err)
	}
	casePath := filepath.Join(store.Root(), "cases", created.CaseID+".json")
	if err := os.WriteFile(casePath, []byte("{broken-json"), 0o640); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/cases/"+created.CaseID, nil)
	response := httptest.NewRecorder()
	webui.NewServer(app).Handler().ServeHTTP(response, request)

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("错误响应不是 JSON: %v", err)
	}
	for _, contextText := range []string{"组装案件工作台失败", "读取案件快照失败", "integrity_error: 案件快照损坏"} {
		if !strings.Contains(body.Error.Message, contextText) {
			t.Fatalf("错误链缺少运行上下文 %q: %s", contextText, body.Error.Message)
		}
	}
	if response.Code != http.StatusUnprocessableEntity || body.Error.Code != "integrity_error" {
		t.Fatalf("包装后的完整性错误丢失 HTTP 分类: status=%d code=%q body=%s", response.Code, body.Error.Code, response.Body.String())
	}
}
