package evidence

import "oralhistory/internal/domain"

type CandidateEvidence struct {
	Diffs             []domain.SegmentDiff      `json:"diffs"`
	AudioInstructions []domain.AudioInstruction `json:"audio_instructions"`
	ContentSHA256     string                    `json:"content_sha256"`
}

func CandidateDetails(candidate *domain.CandidateRelease) CandidateEvidence {
	if candidate == nil {
		return CandidateEvidence{}
	}
	return CandidateEvidence{
		Diffs:             candidate.Diffs,
		AudioInstructions: candidate.AudioInstructions,
		ContentSHA256:     candidate.ContentSHA256,
	}
}

func ExcludedRanges(candidate *domain.CandidateRelease) []domain.TimeRange {
	var result []domain.TimeRange
	if candidate == nil {
		return result
	}
	for _, instruction := range candidate.AudioInstructions {
		if instruction.Action == "exclude" {
			result = append(result, domain.TimeRange{StartMS: instruction.StartMS, EndMS: instruction.EndMS})
		}
	}
	return result
}
