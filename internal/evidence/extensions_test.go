package evidence

import (
	"testing"
	"time"

	"oralhistory/internal/domain"
)

func TestVerifyManifestReportsAllStructureIssues(t *testing.T) {
	candidate := &domain.CandidateRelease{PublicSegments: []domain.PublicSegment{{SegmentID: "s1", StartMS: 0, EndMS: 100, Text: "批准文本"}}, AudioInstructions: []domain.AudioInstruction{{SegmentID: "s1", Action: "mute", StartMS: 10, EndMS: 20, Reason: "依据"}}}
	c := &domain.OralHistoryCase{SourceSHA256: "source", ConsentBaselineSHA256: "consent", Candidate: candidate, Segments: []domain.TranscriptSegment{{SegmentID: "s1", StartMS: 0, EndMS: 100}}, Reviews: []domain.ReviewDecision{{Decision: "approved"}}}
	manifest := &domain.ReleaseManifest{ManifestID: "m", CaseID: "c", SourceSHA256: "source", ConsentBaselineSHA256: "consent", PublicSegments: []domain.PublicSegment{{SegmentID: "s1", StartMS: 0, EndMS: 100, Text: "错误文本"}, {SegmentID: "s1", StartMS: 0, EndMS: 100, Text: "重复文本"}}, ExcludedRanges: []domain.TimeRange{{StartMS: 20, EndMS: 50}, {StartMS: 40, EndMS: 60}}, AudioInstructions: []domain.AudioInstruction{{SegmentID: "missing", Action: "mute", StartMS: 1, EndMS: 2}}, SealedAt: time.Unix(1, 0)}
	verification := VerifyManifest(c, manifest, "root")
	codes := map[string]bool{}
	for _, issue := range verification.Issues {
		codes[issue.Code] = true
	}
	for _, code := range []string{"manifest_segment_duplicate", "manifest_segment_content_mismatch", "excluded_range_overlap", "audio_instruction_uncovered", "audio_instruction_missing"} {
		if !codes[code] {
			t.Fatalf("未报告 %s: %#v", code, verification.Issues)
		}
	}
}

func TestQueryManifestFiltersAssociatedEntries(t *testing.T) {
	manifest := &domain.ReleaseManifest{PublicSegments: []domain.PublicSegment{{SegmentID: "public", StartMS: 0, EndMS: 100, Text: "公开"}}, ExcludedRanges: []domain.TimeRange{{StartMS: 100, EndMS: 200}}, AudioInstructions: []domain.AudioInstruction{{SegmentID: "public", Action: "mute", StartMS: 10, EndMS: 20}, {SegmentID: "secret", Action: "exclude", StartMS: 100, EndMS: 200}}}
	result := QueryManifest(manifest, ManifestQuery{SegmentID: "secret", EntryType: "excluded"})
	if len(result.PublicSegments) != 0 || len(result.ExcludedRanges) != 1 || len(result.AudioInstructions) != 1 || result.AudioInstructions[0].SegmentID != "secret" {
		t.Fatalf("范围查询错误: %#v", result)
	}
}
