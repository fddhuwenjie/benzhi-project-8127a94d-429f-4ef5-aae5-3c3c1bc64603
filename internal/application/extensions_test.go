package application

import (
	"testing"

	"oralhistory/internal/domain"
	"oralhistory/internal/persistence"
)

func TestBatchCommitIsAtomicAndIdempotent(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(store)
	c := mustCreate(t, s)
	c, err = s.FreezeBaseline(CommandMeta{CaseID: c.CaseID, ActorID: "arch", RequestID: "freeze-batch", ExpectedRevision: c.Revision})
	if err != nil {
		t.Fatal(err)
	}
	cmd := BatchRegistrationCommand{CommandMeta: CommandMeta{CaseID: c.CaseID, ActorID: "arch", RequestID: "batch-ok", ExpectedRevision: c.Revision}, Segments: []domain.TranscriptSegment{{SegmentID: "s1", StartMS: 0, EndMS: 100, SourceText: "甲", SubjectIDs: []string{"p"}}, {SegmentID: "s2", StartMS: 100, EndMS: 200, SourceText: "乙", TopicCodes: []string{"t"}}}, Constraints: []domain.ConsentConstraint{{ConstraintID: "c1", ScopeType: "subject", ScopeValue: "p", Policy: domain.PolicyAllow, EvidenceReference: "doc#1"}, {ConstraintID: "c2", ScopeType: "topic", ScopeValue: "t", Policy: domain.PolicyAllow, EvidenceReference: "doc#2"}}}
	result, err := s.RegisterBatch(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if result.Case.Revision != c.Revision+1 || result.Summary.SegmentsAdded != 2 {
		t.Fatalf("批次统计或修订错误: %#v", result)
	}
	replayed, err := s.RegisterBatch(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Case.Revision != result.Case.Revision || replayed.Summary != result.Summary {
		t.Fatalf("幂等重放不同: %#v / %#v", result, replayed)
	}
	cmd.Segments[0].SourceText = "修改"
	if _, err := s.RegisterBatch(cmd); domain.ErrorCode(err) != domain.ErrIdempotencyConflict {
		t.Fatalf("应拒绝 request_id 载荷冲突: %v", err)
	}
	loaded, _ := store.LoadCase(c.CaseID)
	if loaded.Revision != result.Case.Revision || len(loaded.Segments) != 2 {
		t.Fatalf("冲突请求修改了案件: %#v", loaded)
	}
}

func TestCandidateRejectsStaleConflictCheck(t *testing.T) {
	store, _ := persistence.Open(t.TempDir())
	s := NewService(store)
	c := mustCreate(t, s)
	c, _ = s.FreezeBaseline(CommandMeta{CaseID: c.CaseID, ActorID: "arch", RequestID: "freeze-stale", ExpectedRevision: c.Revision})
	c, _ = s.AddSegment(AddSegmentCommand{CommandMeta: CommandMeta{CaseID: c.CaseID, ActorID: "arch", RequestID: "seg-stale", ExpectedRevision: c.Revision}, Segment: domain.TranscriptSegment{SegmentID: "s", StartMS: 0, EndMS: 100, SourceText: "甲", SubjectIDs: []string{"p"}}})
	c, _ = s.AddConstraint(AddConstraintCommand{CommandMeta: CommandMeta{CaseID: c.CaseID, ActorID: "arch", RequestID: "con-stale", ExpectedRevision: c.Revision}, Constraint: domain.ConsentConstraint{ConstraintID: "allow", ScopeType: "subject", ScopeValue: "p", Policy: domain.PolicyAllow, EvidenceReference: "doc"}})
	c, _ = s.CheckConflicts(CommandMeta{CaseID: c.CaseID, ActorID: "arch", RequestID: "check-stale", ExpectedRevision: c.Revision})
	c, _ = s.AddSegment(AddSegmentCommand{CommandMeta: CommandMeta{CaseID: c.CaseID, ActorID: "arch", RequestID: "edit-stale", ExpectedRevision: c.Revision}, Segment: domain.TranscriptSegment{SegmentID: "s", StartMS: 0, EndMS: 100, SourceText: "改写", SubjectIDs: []string{"p"}}})
	_, err := s.GenerateCandidate(CommandMeta{CaseID: c.CaseID, ActorID: "arch", RequestID: "candidate-stale", ExpectedRevision: c.Revision})
	if domain.ErrorCode(err) != domain.ErrInvalidState {
		t.Fatalf("应拒绝使用过期检查: %v", err)
	}
}
