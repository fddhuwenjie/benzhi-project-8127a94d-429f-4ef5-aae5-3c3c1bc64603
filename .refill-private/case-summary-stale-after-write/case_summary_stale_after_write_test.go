package case_summary_stale_after_write_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"oralhistory/internal/application"
	"oralhistory/internal/domain"
	"oralhistory/internal/persistence"
	"oralhistory/internal/webui"
)

func TestCaseSummaryRefreshesAfterWrite(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := webui.NewServer(application.NewService(store)).Handler()
	create := application.CreateCaseCommand{
		CommandMeta: application.CommandMeta{
			CaseID: "case-summary", ActorID: "archivist", RequestID: "create-summary", ExpectedRevision: 0,
		},
		Title: "待冻结案件", CollectionDate: "2026-08-27", CustodyReference: "OH-2026-21",
		SourceAudioURI:        "archive://oral-history/21.wav",
		SourceSHA256:          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ConsentDocumentSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ArchivistID:           "archivist", ReviewerID: "reviewer",
	}
	if response := postJSON(t, handler, "/api/cases", create); response.Code != http.StatusCreated {
		t.Fatalf("建档失败: status=%d body=%s", response.Code, response.Body.String())
	}
	assertSummary(t, listCases(t, handler), domain.StatusDraft, 1)

	freeze := application.CommandMeta{
		CaseID: "case-summary", ActorID: "archivist", RequestID: "freeze-summary", ExpectedRevision: 1,
	}
	if response := postJSON(t, handler, "/api/cases/case-summary/freeze", freeze); response.Code != http.StatusOK {
		t.Fatalf("冻结失败: status=%d body=%s", response.Code, response.Body.String())
	}
	assertSummary(t, listCases(t, handler), domain.StatusFrozen, 2)
}

func listCases(t *testing.T, handler http.Handler) []application.CaseSummary {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/cases", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("查询案件列表失败: status=%d body=%s", response.Code, response.Body.String())
	}
	var envelope struct {
		Cases []application.CaseSummary `json:"cases"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("解析案件列表失败: %v", err)
	}
	return envelope.Cases
}

func assertSummary(t *testing.T, cases []application.CaseSummary, status domain.CaseStatus, revision int64) {
	t.Helper()
	if len(cases) != 1 {
		t.Fatalf("案件列表应包含一项，实际为 %d", len(cases))
	}
	if cases[0].Status != status || cases[0].Revision != revision {
		t.Fatalf("案件摘要未反映已提交状态: want_status=%s want_revision=%d got_status=%s got_revision=%d", status, revision, cases[0].Status, cases[0].Revision)
	}
}

func postJSON(t *testing.T, handler http.Handler, path string, value any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
