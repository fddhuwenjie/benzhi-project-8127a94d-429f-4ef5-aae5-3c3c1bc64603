package commitfailurepartialstate_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"oralhistory/internal/domain"
	"oralhistory/internal/persistence"
)

func TestCommitFailureDoesNotAdvanceState(t *testing.T) {
	root := t.TempDir()
	store, err := persistence.Open(root)
	if err != nil {
		t.Fatal(err)
	}

	caseValue := &domain.OralHistoryCase{
		CaseID: "case-tx", Title: "事务测试", Status: domain.StatusDraft,
		CreatedAt: time.Unix(1, 0).UTC(), Segments: []domain.TranscriptSegment{},
		Constraints: []domain.ConsentConstraint{}, Conflicts: []domain.DisclosureConflict{},
		Reviews: []domain.ReviewDecision{},
	}
	response, err := json.Marshal(map[string]any{"case": caseValue})
	if err != nil {
		t.Fatal(err)
	}
	first := persistence.Commit{
		Case: caseValue, ExpectedRevision: 0,
		Event: domain.AuditEvent{CaseID: caseValue.CaseID, Type: "created", ActorID: "archivist", Revision: 1, RequestID: "req-1", RequestSHA256: "fp-1", OccurredAt: time.Unix(2, 0).UTC()},
		Idempotency: &persistence.IdempotencyRecord{CaseID: caseValue.CaseID, RequestID: "req-1", Fingerprint: "fp-1", Response: response},
	}
	if err := store.Commit(first); err != nil {
		t.Fatal(err)
	}

	manifestPath := filepath.Join(root, "manifests", "case-tx.json")
	if err := os.WriteFile(manifestPath, []byte("{}\n"), 0o440); err != nil {
		t.Fatal(err)
	}
	working, err := store.LoadCase(caseValue.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	working.Title = "不应落库的标题"
	second := persistence.Commit{
		Case: working, ExpectedRevision: 1,
		Event: domain.AuditEvent{CaseID: working.CaseID, Type: "updated", ActorID: "archivist", Revision: 2, RequestID: "req-2", RequestSHA256: "fp-2", OccurredAt: time.Unix(3, 0).UTC()},
		Idempotency: &persistence.IdempotencyRecord{CaseID: working.CaseID, RequestID: "req-2", Fingerprint: "fp-2", Response: response},
		Manifest: &domain.ReleaseManifest{ManifestID: "already-sealed", CaseID: working.CaseID},
	}
	if err := store.Commit(second); err == nil {
		t.Fatal("预期重复清单提交失败")
	}

	loaded, err := store.LoadCase(caseValue.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	events, err := store.LoadEvents(caseValue.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Revision != 1 || loaded.Title != "事务测试" || len(events) != 1 {
		t.Fatalf("失败提交推进了持久状态: revision=%d title=%q events=%d", loaded.Revision, loaded.Title, len(events))
	}
}
