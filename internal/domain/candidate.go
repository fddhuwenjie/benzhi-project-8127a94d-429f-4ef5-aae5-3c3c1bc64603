package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"
)

func BuildCandidate(c *OralHistoryCase, now time.Time) (*CandidateRelease, error) {
	if HasOpenBlocking(c.Conflicts) {
		return nil, NewError(ErrInvalidState, "仍有未处置的阻断冲突")
	}
	segments := append([]TranscriptSegment(nil), c.Segments...)
	sort.Slice(segments, func(i, j int) bool { return segments[i].StartMS < segments[j].StartMS })
	candidate := &CandidateRelease{Revision: c.Revision + 1, GeneratedAt: now.UTC()}
	for _, segment := range segments {
		publicText := segment.SourceText
		action := string(segment.Disposition)
		switch segment.Disposition {
		case DispositionExclude:
			candidate.AudioInstructions = append(candidate.AudioInstructions, AudioInstruction{
				SegmentID: segment.SegmentID, Action: "exclude", StartMS: segment.StartMS, EndMS: segment.EndMS, Reason: segment.Reason,
			})
			publicText = ""
		case DispositionMute:
			for _, mute := range segment.MuteRanges {
				candidate.AudioInstructions = append(candidate.AudioInstructions, AudioInstruction{
					SegmentID: segment.SegmentID, Action: "mute", StartMS: mute.StartMS, EndMS: mute.EndMS, Reason: segment.Reason,
				})
			}
			if segment.PublicText != "" {
				publicText = segment.PublicText
			}
		case DispositionReplace:
			publicText = segment.PublicText
		case DispositionOriginal, "":
			action = string(DispositionOriginal)
		}
		if publicText != "" {
			candidate.PublicSegments = append(candidate.PublicSegments, PublicSegment{
				SegmentID: segment.SegmentID, StartMS: segment.StartMS, EndMS: segment.EndMS, Text: publicText,
			})
		}
		candidate.Diffs = append(candidate.Diffs, SegmentDiff{
			SegmentID: segment.SegmentID, SourceText: segment.SourceText, PublicText: publicText,
			Changed: publicText != segment.SourceText, Action: action,
		})
	}
	content, _ := json.Marshal(struct {
		Public []PublicSegment    `json:"public_segments"`
		Audio  []AudioInstruction `json:"audio_instructions"`
	}{candidate.PublicSegments, candidate.AudioInstructions})
	sum := sha256.Sum256(content)
	candidate.ContentSHA256 = hex.EncodeToString(sum[:])
	return candidate, nil
}
