package application

import (
	"testing"

	"oralhistory/internal/domain"
	"oralhistory/internal/persistence"
)

func TestReturnedCaseOnlyAllowsNamedSegments(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(store)
	c := mustCreate(t, s)
	c, err = s.FreezeBaseline(CommandMeta{CaseID: c.CaseID, ActorID: "arch", RequestID: "freeze", ExpectedRevision: c.Revision})
	if err != nil {
		t.Fatal(err)
	}
	for i, segment := range []domain.TranscriptSegment{
		{SegmentID: "s1", StartMS: 0, EndMS: 100, SourceText: "甲", SubjectIDs: []string{"p"}, Disposition: domain.DispositionOriginal},
		{SegmentID: "s2", StartMS: 100, EndMS: 200, SourceText: "乙", SubjectIDs: []string{"p"}, Disposition: domain.DispositionOriginal},
	} {
		c, err = s.AddSegment(AddSegmentCommand{CommandMeta: CommandMeta{CaseID: c.CaseID, ActorID: "arch", RequestID: "segment-" + string(rune('1'+i)), ExpectedRevision: c.Revision}, Segment: segment})
		if err != nil {
			t.Fatal(err)
		}
	}
	c, err = s.AddConstraint(AddConstraintCommand{CommandMeta: CommandMeta{CaseID: c.CaseID, ActorID: "arch", RequestID: "constraint", ExpectedRevision: c.Revision}, Constraint: domain.ConsentConstraint{ConstraintID: "allow", ScopeType: "subject", ScopeValue: "p", Policy: domain.PolicyAllow, EvidenceReference: "doc#1"}})
	if err != nil {
		t.Fatal(err)
	}
	c, err = s.CheckConflicts(CommandMeta{CaseID: c.CaseID, ActorID: "arch", RequestID: "check", ExpectedRevision: c.Revision})
	if err != nil {
		t.Fatal(err)
	}
	c, err = s.GenerateCandidate(CommandMeta{CaseID: c.CaseID, ActorID: "arch", RequestID: "candidate", ExpectedRevision: c.Revision})
	if err != nil {
		t.Fatal(err)
	}
	c, err = s.SubmitReview(CommandMeta{CaseID: c.CaseID, ActorID: "arch", RequestID: "submit", ExpectedRevision: c.Revision})
	if err != nil {
		t.Fatal(err)
	}
	items := []domain.ReviewItemResult{{SegmentID: "s1", ConsentValid: true}, {SegmentID: "s2", ConsentValid: true, RedactionValid: true}}
	c, err = s.DecideReview(ReviewCommand{CommandMeta: CommandMeta{CaseID: c.CaseID, ActorID: "review", RequestID: "return", ExpectedRevision: c.Revision}, ItemResults: items, Decision: "returned", ReturnReasons: []domain.ReturnReason{{SegmentID: "s1", Code: "wording", Comment: "需修改"}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Remediate(RemediateCommand{CommandMeta: CommandMeta{CaseID: c.CaseID, ActorID: "arch", RequestID: "wrong", ExpectedRevision: c.Revision}, SegmentID: "s2", Disposition: domain.DispositionReplace, PublicText: "改乙", EvidenceRefs: []string{"review"}, Reason: "尝试越权修改"})
	if domain.ErrorCode(err) != domain.ErrForbidden {
		t.Fatalf("应拒绝修改未被退回的片段: %v", err)
	}
}

func mustCreate(t *testing.T, s *Service) *domain.OralHistoryCase {
	t.Helper()
	value, err := s.CreateCase(CreateCaseCommand{
		CommandMeta: CommandMeta{CaseID: "case-workflow", ActorID: "arch", RequestID: "create", ExpectedRevision: 0},
		Title:       "测试案件", CollectionDate: "2026-08-01", CustodyReference: "C1", SourceAudioURI: "archive://a",
		SourceSHA256:          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ConsentDocumentSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ArchivistID:           "arch", ReviewerID: "review",
	})
	if err != nil {
		t.Fatal(err)
	}
	return value
}
