package remediationpreviewrace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"oralhistory/internal/application"
	"oralhistory/internal/domain"
	"oralhistory/internal/persistence"
	"oralhistory/internal/webui"
)

func TestConcurrentRemediationPreviewsAreIsolated(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store)
	cases := make([]*domain.OralHistoryCase, 8)
	for i := range cases {
		cases[i] = prepareCase(t, service, fmt.Sprintf("preview-race-%d", i))
	}
	handler := webui.NewServer(service).Handler()

	start := make(chan struct{})
	ready := sync.WaitGroup{}
	ready.Add(len(cases))
	responses := make(chan *httptest.ResponseRecorder, len(cases)*64)
	var requests sync.WaitGroup
	for _, current := range cases {
		current := current
		requests.Add(1)
		go func() {
			defer requests.Done()
			ready.Done()
			<-start
			for attempt := 0; attempt < 64; attempt++ {
				command := application.RemediateCommand{
					CommandMeta: application.CommandMeta{
						CaseID: current.CaseID, ActorID: "archivist", RequestID: fmt.Sprintf("preview-%d", attempt), ExpectedRevision: current.Revision,
					},
					Preview: true, SegmentID: "segment-a", Disposition: domain.DispositionReplace,
					PublicText: "匿名后的某受访者", EvidenceRefs: []string{"consent#person-a"}, Reason: "落实匿名授权",
				}
				responses <- postJSON(t, handler, "/api/cases/"+current.CaseID+"/remediate", command)
			}
		}()
	}
	ready.Wait()
	close(start)
	requests.Wait()
	close(responses)
	for response := range responses {
		if response.Code != http.StatusOK {
			t.Fatalf("并发预演失败: %d %s", response.Code, response.Body.String())
		}
	}
}

func prepareCase(t *testing.T, service *application.Service, caseID string) *domain.OralHistoryCase {
	t.Helper()
	current, err := service.CreateCase(application.CreateCaseCommand{
		CommandMeta: application.CommandMeta{CaseID: caseID, ActorID: "archivist", RequestID: "create", ExpectedRevision: 0},
		Title:       "并发预演案件", CollectionDate: "2026-08-01", CustodyReference: "C-1", SourceAudioURI: "archive://audio/1",
		SourceSHA256:          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ConsentDocumentSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ArchivistID:           "archivist", ReviewerID: "reviewer",
	})
	if err != nil {
		t.Fatal(err)
	}
	current, err = service.FreezeBaseline(meta(current, "freeze"))
	if err != nil {
		t.Fatal(err)
	}
	current, err = service.AddSegment(application.AddSegmentCommand{
		CommandMeta: meta(current, "segment"),
		Segment: domain.TranscriptSegment{
			SegmentID: "segment-a", StartMS: 0, EndMS: 1000, SourceText: "受访者原话",
			SubjectIDs: []string{"person-a"}, Disposition: domain.DispositionOriginal,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	current, err = service.AddConstraint(application.AddConstraintCommand{
		CommandMeta: meta(current, "constraint"),
		Constraint: domain.ConsentConstraint{
			ConstraintID: "anonymous-person-a", ScopeType: "subject", ScopeValue: "person-a",
			Policy: domain.PolicyAnonymous, RequiredAlias: "某受访者", EvidenceReference: "consent#person-a",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	current, err = service.CheckConflicts(meta(current, "check"))
	if err != nil {
		t.Fatal(err)
	}
	return current
}

func meta(current *domain.OralHistoryCase, requestID string) application.CommandMeta {
	return application.CommandMeta{CaseID: current.CaseID, ActorID: "archivist", RequestID: requestID, ExpectedRevision: current.Revision}
}

func postJSON(t *testing.T, handler http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	content, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(content))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
