package manifestquerycachescope

import (
	"testing"

	"oralhistory/internal/domain"
	"oralhistory/internal/evidence"
)

func TestManifestQueryCacheSeparatesFilters(t *testing.T) {
	manifest := &domain.ReleaseManifest{
		ManifestID:     "manifest-cache-scope",
		CaseID:         "case-cache-scope",
		ManifestSHA256: "manifest-cache-scope-digest",
		PublicSegments: []domain.PublicSegment{
			{SegmentID: "segment-a", StartMS: 0, EndMS: 100, Text: "甲"},
			{SegmentID: "segment-b", StartMS: 100, EndMS: 200, Text: "乙"},
		},
	}

	first := evidence.QueryManifest(manifest, evidence.ManifestQuery{SegmentID: "segment-a", EntryType: "public"})
	if len(first.PublicSegments) != 1 || first.PublicSegments[0].SegmentID != "segment-a" {
		t.Fatalf("第一次过滤结果错误: %#v", first.PublicSegments)
	}

	second := evidence.QueryManifest(manifest, evidence.ManifestQuery{SegmentID: "segment-b", EntryType: "public"})
	if len(second.PublicSegments) != 1 || second.PublicSegments[0].SegmentID != "segment-b" {
		t.Fatalf("第二次过滤复用了第一次查询的缓存: %#v", second.PublicSegments)
	}
}
