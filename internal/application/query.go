package application

import (
	"oralhistory/internal/domain"
	"oralhistory/internal/evidence"
)

func (s *Service) ListCases() ([]CaseSummary, error) {
	s.caseListMu.RLock()
	if s.caseListCacheWarm {
		result := append([]CaseSummary(nil), s.caseListCache...)
		s.caseListMu.RUnlock()
		return result, nil
	}
	s.caseListMu.RUnlock()

	cases, err := s.store.ListCases()
	if err != nil {
		return nil, err
	}
	result := make([]CaseSummary, 0, len(cases))
	for _, c := range cases {
		result = append(result, CaseSummary{CaseID: c.CaseID, Title: c.Title, Status: c.Status, Revision: c.Revision, UpdatedAt: c.UpdatedAt})
	}
	s.caseListMu.Lock()
	s.caseListCache = append([]CaseSummary(nil), result...)
	s.caseListCacheWarm = true
	s.caseListMu.Unlock()
	return result, nil
}

func (s *Service) GetWorkbench(caseID string) (*Workbench, error) {
	c, err := s.store.LoadCase(caseID)
	if err != nil {
		return nil, err
	}
	events, err := s.store.LoadEvents(caseID)
	if err != nil {
		return nil, err
	}
	result := &Workbench{Case: c, AllowedActions: domain.AllowedActions(c), Timeline: evidence.BuildTimeline(events)}
	result.Coverage = domain.BuildCoverageMatrix(c, s.now())
	if len(c.ConflictChecks) > 0 {
		latest := c.ConflictChecks[len(c.ConflictChecks)-1]
		result.LatestCheck = &latest
		currentDigest, digestErr := evidence.ConflictInputDigest(c, latest.CheckDate)
		result.CheckStale = digestErr != nil || currentDigest != latest.InputSHA256
	}
	var previous *domain.CandidateRelease
	if len(c.CandidateHistory) > 1 {
		previous = &c.CandidateHistory[len(c.CandidateHistory)-2]
	}
	result.ReviewDifference = evidence.CompareCandidates(previous, c.Candidate, taskSegmentsForPreviousRound(c))
	if c.Status == domain.StatusSealed {
		manifest, err := s.store.LoadManifest(caseID)
		if err != nil {
			return nil, err
		}
		auditRoot := ""
		if len(events) > 0 {
			// 清单在封存事件提交前构造，其根必须等于封存事件声明的前序摘要。
			auditRoot = events[len(events)-1].PreviousSHA256
		}
		verification := evidence.VerifyManifest(c, manifest, auditRoot)
		result.Verification = &verification
		query := evidence.QueryManifest(manifest, evidence.ManifestQuery{})
		result.ManifestQuery = &query
	}
	return result, nil
}

func taskSegmentsForPreviousRound(c *domain.OralHistoryCase) []string {
	var result []string
	for _, task := range c.ReturnTasks {
		if task.Round == c.ReviewRound-1 || task.Status == "pending_review" {
			result = append(result, task.SegmentID)
		}
	}
	return result
}

func (s *Service) QueryManifest(caseID string, query ManifestQuery) (evidence.ManifestQueryResult, error) {
	c, err := s.store.LoadCase(caseID)
	if err != nil {
		return evidence.ManifestQueryResult{}, err
	}
	if c.Status != domain.StatusSealed {
		return evidence.ManifestQueryResult{}, domain.NewError(domain.ErrInvalidState, "案件尚未封存")
	}
	if query.EntryType != "" && query.EntryType != "public" && query.EntryType != "excluded" {
		return evidence.ManifestQueryResult{}, domain.NewError(domain.ErrInvalidInput, "entry_type 仅支持 public 或 excluded")
	}
	if query.StartMS != nil && query.EndMS != nil && *query.EndMS <= *query.StartMS {
		return evidence.ManifestQueryResult{}, domain.NewError(domain.ErrInvalidInput, "查询结束时间必须晚于开始时间")
	}
	manifest, err := s.store.LoadManifest(caseID)
	if err != nil {
		return evidence.ManifestQueryResult{}, err
	}
	return evidence.QueryManifest(manifest, evidence.ManifestQuery{StartMS: query.StartMS, EndMS: query.EndMS, SegmentID: query.SegmentID, EntryType: query.EntryType}), nil
}

func (s *Service) Verify(caseID string) (evidence.Verification, error) {
	workbench, err := s.GetWorkbench(caseID)
	if err != nil {
		return evidence.Verification{}, err
	}
	if workbench.Verification == nil {
		return evidence.Verification{}, domain.NewError(domain.ErrInvalidState, "案件尚未封存")
	}
	return *workbench.Verification, nil
}

func (s *Service) ActiveCaseLocks() int { return s.locks.size() }
