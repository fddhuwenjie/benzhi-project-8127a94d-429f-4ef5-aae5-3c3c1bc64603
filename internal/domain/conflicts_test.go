package domain

import (
	"reflect"
	"testing"
	"time"
)

func TestDetectConflictsIsDeterministic(t *testing.T) {
	c := &OralHistoryCase{
		Segments: []TranscriptSegment{
			{SegmentID: "b", StartMS: 100, EndMS: 200, SourceText: "乙", SubjectIDs: []string{"p2"}, Disposition: DispositionOriginal},
			{SegmentID: "a", StartMS: 0, EndMS: 100, SourceText: "甲", SubjectIDs: []string{"p1"}, Disposition: DispositionOriginal},
		},
		Constraints: []ConsentConstraint{
			{ConstraintID: "c2", ScopeType: "subject", ScopeValue: "p2", Policy: PolicyDeny, EvidenceReference: "e2"},
			{ConstraintID: "c1", ScopeType: "subject", ScopeValue: "p1", Policy: PolicyAnonymous, RequiredAlias: "受访者甲", EvidenceReference: "e1"},
		},
	}
	now := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	first := DetectConflicts(c, now)
	second := DetectConflicts(c, now)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("相同输入产生不同冲突: %#v / %#v", first, second)
	}
	if len(first) != 2 || first[0].SegmentID != "a" || first[1].SegmentID != "b" {
		t.Fatalf("冲突排序不稳定: %#v", first)
	}
	if first[0].ConflictID == "" || first[0].ConflictID == first[1].ConflictID {
		t.Fatalf("冲突编号无效: %#v", first)
	}
}

func TestValidateTimelineRejectsGap(t *testing.T) {
	err := ValidateTimeline([]TranscriptSegment{
		{SegmentID: "a", StartMS: 0, EndMS: 100, SourceText: "甲", SubjectIDs: []string{"p1"}},
		{SegmentID: "b", StartMS: 101, EndMS: 200, SourceText: "乙", SubjectIDs: []string{"p2"}},
	})
	if ErrorCode(err) != ErrInvalidInput {
		t.Fatalf("应拒绝不连续时间线，实际错误: %v", err)
	}
}
