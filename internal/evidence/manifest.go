package evidence

import (
	"fmt"
	"time"

	"oralhistory/internal/domain"
)

type manifestContent struct {
	ManifestID            string                    `json:"manifest_id"`
	CaseID                string                    `json:"case_id"`
	SourceSHA256          string                    `json:"source_sha256"`
	ConsentBaselineSHA256 string                    `json:"consent_baseline_sha256"`
	PublicSegments        []domain.PublicSegment    `json:"public_segments"`
	ExcludedRanges        []domain.TimeRange        `json:"excluded_ranges"`
	AudioInstructions     []domain.AudioInstruction `json:"audio_instructions"`
	ReviewDecisionSHA256  string                    `json:"review_decision_sha256"`
	AuditRootSHA256       string                    `json:"audit_root_sha256"`
	SealedAt              time.Time                 `json:"sealed_at"`
}

func BuildManifest(c *domain.OralHistoryCase, auditRoot string, now time.Time) (*domain.ReleaseManifest, error) {
	if c.Candidate == nil || len(c.Reviews) == 0 {
		return nil, domain.NewError(domain.ErrInvalidState, "缺少候选版或复核决定")
	}
	review := c.Reviews[len(c.Reviews)-1]
	if review.Decision != "approved" {
		return nil, domain.NewError(domain.ErrInvalidState, "最终复核决定不是批准")
	}
	reviewDigest, err := Digest(review)
	if err != nil {
		return nil, err
	}
	sealedAt := now.UTC()
	manifestID := fmt.Sprintf("manifest-%s-r%d", c.CaseID, c.Candidate.Revision)
	base := manifestContent{
		ManifestID:            manifestID,
		CaseID:                c.CaseID,
		SourceSHA256:          c.SourceSHA256,
		ConsentBaselineSHA256: c.ConsentBaselineSHA256,
		PublicSegments:        c.Candidate.PublicSegments,
		ExcludedRanges:        ExcludedRanges(c.Candidate),
		AudioInstructions:     c.Candidate.AudioInstructions,
		ReviewDecisionSHA256:  reviewDigest,
		AuditRootSHA256:       auditRoot,
		SealedAt:              sealedAt,
	}
	digest, err := Digest(base)
	if err != nil {
		return nil, err
	}
	return &domain.ReleaseManifest{
		ManifestID:            base.ManifestID,
		CaseID:                base.CaseID,
		SourceSHA256:          base.SourceSHA256,
		ConsentBaselineSHA256: base.ConsentBaselineSHA256,
		PublicSegments:        base.PublicSegments,
		ExcludedRanges:        base.ExcludedRanges,
		AudioInstructions:     base.AudioInstructions,
		ReviewDecisionSHA256:  base.ReviewDecisionSHA256,
		AuditRootSHA256:       base.AuditRootSHA256,
		ManifestSHA256:        digest,
		SealedAt:              base.SealedAt,
	}, nil
}
