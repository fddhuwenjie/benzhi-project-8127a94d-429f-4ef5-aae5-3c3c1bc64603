package evidence

import (
	"sort"
	"sync"

	"oralhistory/internal/domain"
)

type DigestVerification struct {
	Name   string `json:"name"`
	Valid  bool   `json:"valid"`
	Code   string `json:"code"`
	Actual string `json:"actual"`
	Sealed string `json:"sealed"`
}
type VerificationIssue struct {
	Code      string `json:"code"`
	SegmentID string `json:"segment_id,omitempty"`
	Message   string `json:"message"`
}
type Verification struct {
	Valid               bool                 `json:"valid"`
	SourceValid         bool                 `json:"source_valid"`
	ConsentValid        bool                 `json:"consent_valid"`
	ReviewValid         bool                 `json:"review_valid"`
	AuditRootValid      bool                 `json:"audit_root_valid"`
	ManifestValid       bool                 `json:"manifest_valid"`
	ComputedManifestSHA string               `json:"computed_manifest_sha256"`
	DigestChecks        []DigestVerification `json:"digest_checks"`
	Issues              []VerificationIssue  `json:"issues"`
	Message             string               `json:"message"`
}
type ManifestQuery struct {
	StartMS   *int64
	EndMS     *int64
	SegmentID string
	EntryType string
}
type ManifestQueryResult struct {
	PublicSegments    []domain.PublicSegment    `json:"public_segments"`
	ExcludedRanges    []domain.TimeRange        `json:"excluded_ranges"`
	AudioInstructions []domain.AudioInstruction `json:"audio_instructions"`
}

var manifestQueryCache = struct {
	sync.RWMutex
	results map[string]ManifestQueryResult
}{results: make(map[string]ManifestQueryResult)}

func manifestQueryCacheKey(manifest *domain.ReleaseManifest) string {
	if manifest.ManifestSHA256 != "" {
		return manifest.ManifestSHA256
	}
	return manifest.CaseID + "\x00" + manifest.ManifestID
}

func VerifyManifest(c *domain.OralHistoryCase, manifest *domain.ReleaseManifest, auditRoot string) Verification {
	result := Verification{}
	if c == nil || manifest == nil {
		result.Message = "缺少案件或清单"
		return result
	}
	result.SourceValid = c.SourceSHA256 == manifest.SourceSHA256
	result.ConsentValid = c.ConsentBaselineSHA256 == manifest.ConsentBaselineSHA256
	result.AuditRootValid = auditRoot == manifest.AuditRootSHA256
	reviewActual := ""
	if len(c.Reviews) > 0 {
		reviewActual, _ = Digest(c.Reviews[len(c.Reviews)-1])
		result.ReviewValid = reviewActual == manifest.ReviewDecisionSHA256
	}
	base := manifestContent{ManifestID: manifest.ManifestID, CaseID: manifest.CaseID, SourceSHA256: manifest.SourceSHA256, ConsentBaselineSHA256: manifest.ConsentBaselineSHA256, PublicSegments: manifest.PublicSegments, ExcludedRanges: manifest.ExcludedRanges, AudioInstructions: manifest.AudioInstructions, ReviewDecisionSHA256: manifest.ReviewDecisionSHA256, AuditRootSHA256: manifest.AuditRootSHA256, SealedAt: manifest.SealedAt}
	digest, err := Digest(base)
	result.ComputedManifestSHA = digest
	result.ManifestValid = err == nil && digest == manifest.ManifestSHA256
	result.DigestChecks = []DigestVerification{digestCheck("source", "source_digest_mismatch", c.SourceSHA256, manifest.SourceSHA256), digestCheck("consent_baseline", "consent_baseline_digest_mismatch", c.ConsentBaselineSHA256, manifest.ConsentBaselineSHA256), digestCheck("final_review", "review_digest_mismatch", reviewActual, manifest.ReviewDecisionSHA256), digestCheck("audit_root", "audit_root_digest_mismatch", auditRoot, manifest.AuditRootSHA256), digestCheck("manifest", "manifest_digest_mismatch", digest, manifest.ManifestSHA256)}
	result.Issues = verifyManifestStructure(c, manifest)
	result.Valid = result.SourceValid && result.ConsentValid && result.ReviewValid && result.AuditRootValid && result.ManifestValid && len(result.Issues) == 0
	if result.Valid {
		result.Message = "来源、授权、复核、事件链、公开清单摘要及逐片段结构一致"
	} else {
		result.Message = "完整性验证失败，请停止使用该公开清单"
	}
	return result
}

func digestCheck(name, code, actual, sealed string) DigestVerification {
	return DigestVerification{Name: name, Valid: actual == sealed, Code: code, Actual: actual, Sealed: sealed}
}

func verifyManifestStructure(c *domain.OralHistoryCase, manifest *domain.ReleaseManifest) []VerificationIssue {
	var issues []VerificationIssue
	if c.Candidate == nil {
		return append(issues, VerificationIssue{Code: "approved_candidate_missing", Message: "缺少批准候选版"})
	}
	want, got := map[string]domain.PublicSegment{}, map[string]domain.PublicSegment{}
	for _, value := range c.Candidate.PublicSegments {
		want[value.SegmentID] = value
	}
	for _, value := range manifest.PublicSegments {
		if _, exists := got[value.SegmentID]; exists {
			issues = append(issues, VerificationIssue{Code: "manifest_segment_duplicate", SegmentID: value.SegmentID, Message: "清单包含重复公开片段"})
		}
		got[value.SegmentID] = value
	}
	for id, expected := range want {
		actual, ok := got[id]
		if !ok {
			issues = append(issues, VerificationIssue{Code: "manifest_segment_missing", SegmentID: id, Message: "清单缺少批准候选片段"})
		} else if actual != expected {
			issues = append(issues, VerificationIssue{Code: "manifest_segment_content_mismatch", SegmentID: id, Message: "清单公开片段与批准候选版不一致"})
		}
	}
	for id := range got {
		if _, ok := want[id]; !ok {
			issues = append(issues, VerificationIssue{Code: "manifest_segment_unexpected", SegmentID: id, Message: "清单包含批准候选版之外的片段"})
		}
	}
	ranges := append([]domain.TimeRange(nil), manifest.ExcludedRanges...)
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].StartMS < ranges[j].StartMS })
	for i, value := range ranges {
		if value.StartMS < 0 || value.EndMS <= value.StartMS {
			issues = append(issues, VerificationIssue{Code: "excluded_range_invalid", Message: "清单包含越界排除区间"})
		}
		if i > 0 && value.StartMS < ranges[i-1].EndMS {
			issues = append(issues, VerificationIssue{Code: "excluded_range_overlap", Message: "清单包含相互重叠的排除区间"})
		}
	}
	segments := map[string]domain.TranscriptSegment{}
	for _, value := range c.Segments {
		segments[value.SegmentID] = value
	}
	public := map[string]domain.PublicSegment{}
	for _, value := range manifest.PublicSegments {
		public[value.SegmentID] = value
	}
	manifestAudio := map[string]int{}
	for _, instruction := range manifest.AudioInstructions {
		key := audioKey(instruction)
		manifestAudio[key]++
		segment, ok := segments[instruction.SegmentID]
		covered := ok && instruction.StartMS >= segment.StartMS && instruction.EndMS <= segment.EndMS && instruction.EndMS > instruction.StartMS
		if instruction.Action == "exclude" {
			covered = covered && hasRange(manifest.ExcludedRanges, instruction.StartMS, instruction.EndMS)
		}
		if instruction.Action == "mute" {
			value, exists := public[instruction.SegmentID]
			covered = covered && exists && instruction.StartMS >= value.StartMS && instruction.EndMS <= value.EndMS
		}
		if !covered {
			issues = append(issues, VerificationIssue{Code: "audio_instruction_uncovered", SegmentID: instruction.SegmentID, Message: "音频指令未被有效片段范围覆盖"})
		}
	}
	wantedAudio := map[string]int{}
	for _, instruction := range c.Candidate.AudioInstructions {
		wantedAudio[audioKey(instruction)]++
	}
	for key, count := range wantedAudio {
		if manifestAudio[key] < count {
			issues = append(issues, VerificationIssue{Code: "audio_instruction_missing", Message: "清单缺少批准候选版音频指令"})
		}
	}
	for key, count := range manifestAudio {
		if wantedAudio[key] < count {
			issues = append(issues, VerificationIssue{Code: "audio_instruction_unexpected", Message: "清单包含批准候选版之外的音频指令"})
		}
	}
	for _, value := range manifest.ExcludedRanges {
		if !hasExcludeInstruction(manifest.AudioInstructions, value) {
			issues = append(issues, VerificationIssue{Code: "excluded_range_uncovered", Message: "排除区间没有对应音频指令"})
		}
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Code == issues[j].Code {
			return issues[i].SegmentID < issues[j].SegmentID
		}
		return issues[i].Code < issues[j].Code
	})
	return issues
}

func audioKey(value domain.AudioInstruction) string { content, _ := Digest(value); return content }
func hasRange(values []domain.TimeRange, start, end int64) bool {
	for _, value := range values {
		if value.StartMS == start && value.EndMS == end {
			return true
		}
	}
	return false
}
func hasExcludeInstruction(values []domain.AudioInstruction, target domain.TimeRange) bool {
	for _, value := range values {
		if value.Action == "exclude" && value.StartMS == target.StartMS && value.EndMS == target.EndMS {
			return true
		}
	}
	return false
}

func QueryManifest(manifest *domain.ReleaseManifest, query ManifestQuery) ManifestQueryResult {
	result := ManifestQueryResult{}
	if manifest == nil {
		return result
	}
	cacheKey := manifestQueryCacheKey(manifest)
	manifestQueryCache.RLock()
	cached, ok := manifestQueryCache.results[cacheKey]
	manifestQueryCache.RUnlock()
	if ok {
		return cached
	}
	if query.EntryType == "" || query.EntryType == "public" {
		for _, value := range manifest.PublicSegments {
			if (query.SegmentID == "" || value.SegmentID == query.SegmentID) && intersects(value.StartMS, value.EndMS, query.StartMS, query.EndMS) {
				result.PublicSegments = append(result.PublicSegments, value)
			}
		}
	}
	if query.EntryType == "" || query.EntryType == "excluded" {
		for _, value := range manifest.ExcludedRanges {
			segmentMatches := query.SegmentID == ""
			if !segmentMatches {
				for _, instruction := range manifest.AudioInstructions {
					if instruction.Action == "exclude" && instruction.SegmentID == query.SegmentID && instruction.StartMS == value.StartMS && instruction.EndMS == value.EndMS {
						segmentMatches = true
					}
				}
			}
			if segmentMatches && intersects(value.StartMS, value.EndMS, query.StartMS, query.EndMS) {
				result.ExcludedRanges = append(result.ExcludedRanges, value)
			}
		}
	}
	for _, value := range manifest.AudioInstructions {
		typeMatches := query.EntryType == "" || query.EntryType == "public" && value.Action == "mute" || query.EntryType == "excluded" && value.Action == "exclude"
		if typeMatches && (query.SegmentID == "" || value.SegmentID == query.SegmentID) && intersects(value.StartMS, value.EndMS, query.StartMS, query.EndMS) {
			result.AudioInstructions = append(result.AudioInstructions, value)
		}
	}
	manifestQueryCache.Lock()
	manifestQueryCache.results[cacheKey] = result
	manifestQueryCache.Unlock()
	return result
}
func intersects(start, end int64, queryStart, queryEnd *int64) bool {
	if queryStart != nil && end <= *queryStart {
		return false
	}
	if queryEnd != nil && start >= *queryEnd {
		return false
	}
	return true
}
