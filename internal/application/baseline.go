package application

import (
	"oralhistory/internal/domain"
	"oralhistory/internal/evidence"
)

func (s *Service) FreezeBaseline(meta CommandMeta) (*domain.OralHistoryCase, error) {
	return s.withCase(meta, meta, "baseline_frozen", func(c *domain.OralHistoryCase) (any, *domain.ReleaseManifest, error) {
		if c.Status != domain.StatusDraft {
			return nil, nil, domain.NewError(domain.ErrInvalidState, "仅草拟案件可冻结基线")
		}
		if meta.ActorID != c.ArchivistID {
			return nil, nil, domain.NewError(domain.ErrForbidden, "只有档案员可以冻结基线")
		}
		baseline, err := evidence.Digest(struct {
			Source  string `json:"source_sha256"`
			Consent string `json:"consent_document_sha256"`
		}{c.SourceSHA256, c.ConsentDocumentSHA256})
		if err != nil {
			return nil, nil, err
		}
		c.ConsentBaselineSHA256 = baseline
		if err := domain.Transition(c, domain.StatusFrozen); err != nil {
			return nil, nil, err
		}
		return map[string]any{"consent_baseline_sha256": baseline}, nil, nil
	})
}
