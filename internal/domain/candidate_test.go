package domain

import (
	"testing"
	"time"
)

func TestBuildCandidateHonorsDisposition(t *testing.T) {
	c := &OralHistoryCase{
		Revision: 4,
		Segments: []TranscriptSegment{
			{SegmentID: "a", StartMS: 0, EndMS: 100, SourceText: "实名内容", SubjectIDs: []string{"p"}, Disposition: DispositionReplace, PublicText: "受访者甲", EvidenceRefs: []string{"e"}, Reason: "匿名"},
			{SegmentID: "b", StartMS: 100, EndMS: 200, SourceText: "禁止内容", SubjectIDs: []string{"q"}, Disposition: DispositionExclude, EvidenceRefs: []string{"e"}, Reason: "禁止公开"},
		},
		Conflicts: []DisclosureConflict{{ConflictID: "x", SegmentID: "a", Severity: "blocking", Resolved: true}},
	}
	candidate, err := BuildCandidate(c, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Revision != 5 || len(candidate.PublicSegments) != 1 || candidate.PublicSegments[0].Text != "受访者甲" {
		t.Fatalf("公开片段错误: %#v", candidate)
	}
	if len(candidate.AudioInstructions) != 1 || candidate.AudioInstructions[0].Action != "exclude" {
		t.Fatalf("音频指令错误: %#v", candidate.AudioInstructions)
	}
}
