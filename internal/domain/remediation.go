package domain

import (
	"sort"
	"strings"
	"time"
)

type RequirementResult struct {
	ConflictID   string `json:"conflict_id"`
	ConstraintID string `json:"constraint_id,omitempty"`
	Satisfied    bool   `json:"satisfied"`
	Reason       string `json:"reason,omitempty"`
}

type RemediationPreview struct {
	Segment           TranscriptSegment   `json:"normalized_segment"`
	Diff              SegmentDiff         `json:"text_diff"`
	AudioInstructions []AudioInstruction  `json:"audio_instructions"`
	Requirements      []RequirementResult `json:"requirements"`
	CanConfirm        bool                `json:"can_confirm"`
}

func PreviewRemediation(c *OralHistoryCase, segmentID string, disposition Disposition, publicText string, muteRanges []TimeRange, evidenceRefs []string, reason string, today time.Time) (RemediationPreview, error) {
	var segment TranscriptSegment
	found := false
	for _, value := range c.Segments {
		if value.SegmentID == segmentID {
			segment, found = value, true
			break
		}
	}
	if !found {
		return RemediationPreview{}, NewError(ErrNotFound, "待整改片段不存在")
	}
	segment.Disposition = disposition
	segment.PublicText = strings.TrimSpace(publicText)
	segment.EvidenceRefs = cleanStrings(evidenceRefs)
	segment.Reason = strings.TrimSpace(reason)
	if segment.Reason == "" || len(segment.EvidenceRefs) == 0 {
		return RemediationPreview{}, NewError(ErrInvalidInput, "整改理由和证据引用不能为空")
	}
	switch disposition {
	case DispositionReplace:
		if segment.PublicText == "" || segment.PublicText == segment.SourceText {
			return RemediationPreview{}, NewError(ErrInvalidInput, "替换文本必须非空且与原文不同")
		}
		if len(muteRanges) > 0 {
			return RemediationPreview{}, NewError(ErrInvalidInput, "替换处置不得同时包含静音区间")
		}
		segment.MuteRanges = nil
	case DispositionMute:
		if len(muteRanges) == 0 {
			return RemediationPreview{}, NewError(ErrInvalidInput, "静音处置必须提供静音区间")
		}
		segment.MuteRanges = append([]TimeRange(nil), muteRanges...)
		sort.Slice(segment.MuteRanges, func(i, j int) bool { return segment.MuteRanges[i].StartMS < segment.MuteRanges[j].StartMS })
		for i, value := range segment.MuteRanges {
			if value.StartMS < segment.StartMS || value.EndMS > segment.EndMS || value.EndMS <= value.StartMS {
				return RemediationPreview{}, NewError(ErrInvalidInput, "静音区间必须位于片段范围内")
			}
			if i > 0 && value.StartMS < segment.MuteRanges[i-1].EndMS {
				return RemediationPreview{}, NewError(ErrInvalidInput, "静音区间不得重叠")
			}
		}
		segment.MuteRanges = mergeAdjacent(segment.MuteRanges)
	case DispositionExclude:
		if segment.PublicText != "" || len(muteRanges) > 0 {
			return RemediationPreview{}, NewError(ErrInvalidInput, "整段排除必须清空公开文本和静音区间")
		}
		segment.PublicText, segment.MuteRanges = "", nil
	default:
		return RemediationPreview{}, NewError(ErrInvalidInput, "整改仅支持 replace、mute 或 exclude")
	}
	preview := RemediationPreview{Segment: segment, Diff: SegmentDiff{SegmentID: segment.SegmentID, SourceText: segment.SourceText, Action: string(disposition)}}
	preview.Diff.PublicText = segment.SourceText
	if disposition == DispositionReplace || disposition == DispositionMute && segment.PublicText != "" {
		preview.Diff.PublicText = segment.PublicText
	}
	if disposition == DispositionExclude {
		preview.Diff.PublicText = ""
	}
	preview.Diff.Changed = preview.Diff.PublicText != preview.Diff.SourceText
	if disposition == DispositionExclude {
		preview.AudioInstructions = append(preview.AudioInstructions, AudioInstruction{SegmentID: segment.SegmentID, Action: "exclude", StartMS: segment.StartMS, EndMS: segment.EndMS, Reason: segment.Reason})
	}
	if disposition == DispositionMute {
		for _, value := range segment.MuteRanges {
			preview.AudioInstructions = append(preview.AudioInstructions, AudioInstruction{SegmentID: segment.SegmentID, Action: "mute", StartMS: value.StartMS, EndMS: value.EndMS, Reason: segment.Reason})
		}
	}
	testCase := *c
	testCase.Segments = append([]TranscriptSegment(nil), c.Segments...)
	for i := range testCase.Segments {
		if testCase.Segments[i].SegmentID == segmentID {
			testCase.Segments[i] = segment
		}
	}
	for _, conflict := range DetectConflicts(&testCase, today) {
		if conflict.SegmentID != segmentID || conflict.Severity != "blocking" {
			continue
		}
		result := RequirementResult{ConflictID: conflict.ConflictID, ConstraintID: conflict.ConstraintID, Satisfied: conflict.Resolved}
		if !result.Satisfied {
			result.Reason = conflict.Message
		}
		preview.Requirements = append(preview.Requirements, result)
	}
	preview.CanConfirm = true
	for _, result := range preview.Requirements {
		if !result.Satisfied {
			preview.CanConfirm = false
		}
	}
	return preview, nil
}

func mergeAdjacent(values []TimeRange) []TimeRange {
	result := make([]TimeRange, 0, len(values))
	for _, value := range values {
		if len(result) > 0 && value.StartMS == result[len(result)-1].EndMS {
			result[len(result)-1].EndMS = value.EndMS
		} else {
			result = append(result, value)
		}
	}
	return result
}
