package application

import (
	"oralhistory/internal/domain"
	"oralhistory/internal/evidence"
)

func (s *Service) GenerateCandidate(meta CommandMeta) (*domain.OralHistoryCase, error) {
	return s.withCase(meta, meta, "candidate_generated", func(c *domain.OralHistoryCase) (any, *domain.ReleaseManifest, error) {
		if meta.ActorID != c.ArchivistID {
			return nil, nil, domain.NewError(domain.ErrForbidden, "只有档案员可以生成候选版")
		}
		if c.Status != domain.StatusFrozen && c.Status != domain.StatusRemediation && c.Status != domain.StatusReturned {
			return nil, nil, domain.NewError(domain.ErrInvalidState, "当前状态不能生成候选版")
		}
		if len(c.ConflictChecks) == 0 {
			return nil, nil, domain.NewError(domain.ErrInvalidState, "生成候选版前必须运行当前输入的冲突检查")
		}
		latest := c.ConflictChecks[len(c.ConflictChecks)-1]
		currentDigest, err := evidence.ConflictInputDigest(c, latest.CheckDate)
		if err != nil {
			return nil, nil, err
		}
		if currentDigest != latest.InputSHA256 {
			return nil, nil, domain.NewError(domain.ErrInvalidState, "冲突检查已过期，请重新运行检查")
		}
		for _, task := range c.ReturnTasks {
			if task.Status == "to_remediate" {
				return nil, nil, domain.NewError(domain.ErrInvalidState, "仍有退回任务待整改")
			}
		}
		c.Conflicts = domain.DetectConflicts(c, s.now())
		candidate, err := domain.BuildCandidate(c, s.now())
		if err != nil {
			return nil, nil, err
		}
		c.Candidate = candidate
		c.CandidateHistory = append(c.CandidateHistory, *candidate)
		c.CandidateRevision = candidate.Revision
		c.Status = domain.StatusCandidate
		return map[string]any{"candidate_revision": candidate.Revision, "content_sha256": candidate.ContentSHA256}, nil, nil
	})
}

func (s *Service) SubmitReview(meta CommandMeta) (*domain.OralHistoryCase, error) {
	return s.withCase(meta, meta, "review_submitted", func(c *domain.OralHistoryCase) (any, *domain.ReleaseManifest, error) {
		if meta.ActorID != c.ArchivistID {
			return nil, nil, domain.NewError(domain.ErrForbidden, "只有档案员可以送审")
		}
		if c.Status != domain.StatusCandidate || c.Candidate == nil {
			return nil, nil, domain.NewError(domain.ErrInvalidState, "没有可送审的候选公开版")
		}
		c.Status = domain.StatusReview
		c.ReviewRound++
		return map[string]any{"review_round": c.ReviewRound, "candidate_revision": c.Candidate.Revision}, nil, nil
	})
}
