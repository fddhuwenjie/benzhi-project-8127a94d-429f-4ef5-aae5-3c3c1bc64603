package persistence

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"oralhistory/internal/domain"
)

func TestCommitAndIdempotency(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c := &domain.OralHistoryCase{CaseID: "case-a", Title: "测试", Status: domain.StatusDraft, CreatedAt: time.Now()}
	response, _ := json.Marshal(map[string]any{"case": c})
	event := domain.AuditEvent{CaseID: c.CaseID, Type: "created", ActorID: "a", Revision: 1, RequestID: "r1", RequestSHA256: "fp", OccurredAt: time.Now()}
	record := &IdempotencyRecord{CaseID: c.CaseID, RequestID: "r1", Fingerprint: "fp", Response: response}
	if err := store.Commit(Commit{Case: c, ExpectedRevision: 0, Event: event, Idempotency: record}); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadCase(c.CaseID)
	if err != nil || loaded.Revision != 1 {
		t.Fatalf("快照提交失败: %#v, %v", loaded, err)
	}
	replayed, err := store.LookupIdempotency(c.CaseID, "r1", "fp")
	if err != nil || replayed == nil {
		t.Fatalf("幂等记录不存在: %#v, %v", replayed, err)
	}
	if _, err := store.LookupIdempotency(c.CaseID, "r1", "other"); domain.ErrorCode(err) != domain.ErrIdempotencyConflict {
		t.Fatalf("相同 request_id 异载荷应冲突: %v", err)
	}
}

func TestOpenRejectsTruncatedEventFrame(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.eventPath("damaged"), []byte(`{"sequence":1`), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root); err == nil {
		t.Fatal("应拒绝截断事件尾帧")
	}
}
