package domain

import (
	"testing"
	"time"
)

func TestBatchValidationReportsExactRowsAndNormalizes(t *testing.T) {
	c := &OralHistoryCase{Segments: []TranscriptSegment{{SegmentID: "existing", StartMS: 0, EndMS: 100, SourceText: "已有", SubjectIDs: []string{"p"}}}}
	result := ValidateRegistrationBatch(c, []TranscriptSegment{
		{SegmentID: " next ", StartMS: 100, EndMS: 200, SourceText: " 新增 ", SubjectIDs: []string{" p ", "p"}},
		{SegmentID: "overlap", StartMS: 150, EndMS: 250, SourceText: "重叠", SubjectIDs: []string{"p"}},
	}, []ConsentConstraint{{ConstraintID: " c1 ", ScopeType: "subject", ScopeValue: "p", Policy: PolicyAllow, EvidenceReference: " doc#1 "}})
	if len(result.Issues) != 1 || result.Issues[0].Line != 2 || result.Issues[0].Code != "timeline_overlap" {
		t.Fatalf("行错误不准确: %#v", result.Issues)
	}
	if result.Segments[0].SegmentID != "next" || len(result.Segments[0].SubjectIDs) != 1 || result.Constraints[0].EvidenceReference != "doc#1" {
		t.Fatalf("规范化结果错误: %#v", result)
	}
}

func TestCoverageMatrixReportsMissingAmbiguousAndElapsed(t *testing.T) {
	c := &OralHistoryCase{Revision: 3, Segments: []TranscriptSegment{{SegmentID: "s", StartMS: 0, EndMS: 10, SourceText: "内容", SubjectIDs: []string{"p"}, TopicCodes: []string{"migration", "history"}}}, Constraints: []ConsentConstraint{
		{ConstraintID: "a", ScopeType: "subject", ScopeValue: "p", Policy: PolicyAllow, EvidenceReference: "e1"},
		{ConstraintID: "b", ScopeType: "subject", ScopeValue: "p", Policy: PolicyAnonymous, RequiredAlias: "甲", EvidenceReference: "e2"},
		{ConstraintID: "d", ScopeType: "topic", ScopeValue: "history", Policy: PolicyDelay, NotBefore: "2020-01-01", EvidenceReference: "e3"},
	}}
	matrix := BuildCoverageMatrix(c, time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC))
	if matrix.Summary.Ambiguous != 1 || matrix.Summary.Uncovered != 1 || matrix.Summary.EmbargoElapsed != 1 {
		t.Fatalf("覆盖统计错误: %#v", matrix.Summary)
	}
}

func TestRemediationPreviewMergesAdjacentAndRequiresAlias(t *testing.T) {
	c := &OralHistoryCase{Segments: []TranscriptSegment{{SegmentID: "s", StartMS: 10, EndMS: 30, SourceText: "张某发言", SubjectIDs: []string{"p"}}}, Constraints: []ConsentConstraint{{ConstraintID: "a", ScopeType: "subject", ScopeValue: "p", Policy: PolicyAnonymous, RequiredAlias: "受访者甲", EvidenceReference: "doc"}}}
	preview, err := PreviewRemediation(c, "s", DispositionReplace, "匿名者发言", nil, []string{" doc ", "doc"}, "匿名", time.Now())
	if err != nil || preview.CanConfirm {
		t.Fatalf("缺少指定别名不应允许确认: %#v %v", preview, err)
	}
	preview, err = PreviewRemediation(c, "s", DispositionReplace, "受访者甲发言", nil, []string{"doc"}, "匿名", time.Now())
	if err != nil || !preview.CanConfirm {
		t.Fatalf("有效别名应允许确认: %#v %v", preview, err)
	}
	_, err = PreviewRemediation(c, "s", DispositionMute, "", []TimeRange{{StartMS: 10, EndMS: 20}, {StartMS: 19, EndMS: 25}}, []string{"doc"}, "静音", time.Now())
	if ErrorCode(err) != ErrInvalidInput {
		t.Fatalf("应拒绝重叠静音: %v", err)
	}
}
