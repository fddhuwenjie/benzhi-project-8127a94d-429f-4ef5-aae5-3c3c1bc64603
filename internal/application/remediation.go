package application

import (
	"oralhistory/internal/domain"
	"oralhistory/internal/evidence"
)

func (s *Service) CheckConflicts(meta CommandMeta) (*domain.OralHistoryCase, error) {
	return s.withCase(meta, meta, "conflicts_checked", func(c *domain.OralHistoryCase) (any, *domain.ReleaseManifest, error) {
		if meta.ActorID != c.ArchivistID {
			return nil, nil, domain.NewError(domain.ErrForbidden, "只有档案员可以运行冲突检查")
		}
		if c.Status != domain.StatusFrozen && c.Status != domain.StatusRemediation && c.Status != domain.StatusReturned {
			return nil, nil, domain.NewError(domain.ErrInvalidState, "当前状态不能运行冲突检查")
		}
		if len(c.Segments) == 0 || len(c.Constraints) == 0 {
			return nil, nil, domain.NewError(domain.ErrInvalidInput, "至少登记一个片段和一条授权约束")
		}
		checkDate := s.now().UTC().Format("2006-01-02")
		inputDigest, err := evidence.ConflictInputDigest(c, checkDate)
		if err != nil {
			return nil, nil, err
		}
		c.Conflicts = domain.DetectConflicts(c, s.now())
		resultDigest, err := evidence.ConflictResultDigest(c.Conflicts)
		if err != nil {
			return nil, nil, err
		}
		sameInput := len(c.ConflictChecks) > 0 && c.ConflictChecks[len(c.ConflictChecks)-1].InputSHA256 == inputDigest
		delta := evidence.CompareConflicts(c.Conflicts, c.ConflictChecks, sameInput)
		check := domain.ConflictCheck{BasedOnRevision: c.Revision, CheckDate: checkDate, InputSHA256: inputDigest, ResultSHA256: resultDigest, Conflicts: append([]domain.DisclosureConflict(nil), c.Conflicts...), Delta: delta}
		if len(c.ConflictChecks) > 0 {
			check.ComparedRevision = c.ConflictChecks[len(c.ConflictChecks)-1].BasedOnRevision
		}
		for _, conflict := range c.Conflicts {
			if conflict.Severity == "blocking" && !conflict.Resolved {
				check.BlockingCount++
			} else if conflict.Severity == "notice" {
				check.NoticeCount++
			}
		}
		c.ConflictChecks = append(c.ConflictChecks, check)
		if domain.HasOpenBlocking(c.Conflicts) {
			if c.Status != domain.StatusReturned {
				c.Status = domain.StatusRemediation
			}
		} else if c.Status == domain.StatusRemediation {
			c.Status = domain.StatusFrozen
		}
		return map[string]any{"check": check}, nil, nil
	})
}

func (s *Service) Remediate(cmd RemediateCommand) (*domain.OralHistoryCase, error) {
	return s.withCase(cmd.CommandMeta, cmd, "segment_remediated", func(c *domain.OralHistoryCase) (any, *domain.ReleaseManifest, error) {
		if cmd.ActorID != c.ArchivistID {
			return nil, nil, domain.NewError(domain.ErrForbidden, "只有档案员可以提交整改")
		}
		if err := domain.EnsureContentEditable(c, cmd.SegmentID); err != nil {
			return nil, nil, err
		}
		preview, err := s.remediationPreview(c, cmd)
		if err != nil {
			return nil, nil, err
		}
		if !preview.CanConfirm {
			return nil, nil, domain.NewError(domain.ErrInvalidInput, "当前处置仍有未满足的阻断授权要求")
		}
		found := false
		for i := range c.Segments {
			if c.Segments[i].SegmentID != cmd.SegmentID {
				continue
			}
			found = true
			c.Segments[i] = preview.Segment
			c.Segments[i].Revision = c.Revision + 1
		}
		if !found {
			return nil, nil, domain.NewError(domain.ErrNotFound, "待整改片段不存在")
		}
		c.Conflicts = domain.DetectConflicts(c, s.now())
		c.Candidate = nil
		for i := range c.ReturnTasks {
			if c.ReturnTasks[i].SegmentID == cmd.SegmentID && c.ReturnTasks[i].Status == "to_remediate" {
				c.ReturnTasks[i].Status = "pending_review"
			}
		}
		c.OpenReturnSegmentIDs = domain.OpenTaskSegmentIDs(c.ReturnTasks)
		return map[string]any{"segment_id": cmd.SegmentID, "disposition": cmd.Disposition}, nil, nil
	})
}

func (s *Service) PreviewRemediation(cmd RemediateCommand) (RemediationPreviewResult, error) {
	if err := s.validateMeta(cmd.CommandMeta); err != nil {
		return RemediationPreviewResult{}, err
	}
	unlock := s.locks.lock(cmd.CaseID)
	defer unlock()
	c, err := s.store.LoadCase(cmd.CaseID)
	if err != nil {
		return RemediationPreviewResult{}, err
	}
	if c.Revision != cmd.ExpectedRevision {
		return RemediationPreviewResult{}, domain.WrapError(domain.ErrRevisionConflict, "修订冲突: 当前为 %d，提交期望 %d", c.Revision, cmd.ExpectedRevision)
	}
	if cmd.ActorID != c.ArchivistID {
		return RemediationPreviewResult{}, domain.NewError(domain.ErrForbidden, "只有档案员可以预演整改")
	}
	if err := domain.EnsureContentEditable(c, cmd.SegmentID); err != nil {
		return RemediationPreviewResult{}, err
	}
	preview, err := domain.PreviewRemediation(c, cmd.SegmentID, cmd.Disposition, cmd.PublicText, cmd.MuteRanges, cmd.EvidenceRefs, cmd.Reason, s.now())
	if err != nil {
		return RemediationPreviewResult{}, err
	}
	key, err := remediationPreviewKey(cmd)
	if err != nil {
		return RemediationPreviewResult{}, err
	}
	s.previewResults[key] = preview
	return RemediationPreviewResult{Revision: c.Revision, Preview: preview}, nil
}

func (s *Service) remediationPreview(c *domain.OralHistoryCase, cmd RemediateCommand) (domain.RemediationPreview, error) {
	key, err := remediationPreviewKey(cmd)
	if err != nil {
		return domain.RemediationPreview{}, err
	}
	preview, ok := s.previewResults[key]
	if ok {
		delete(s.previewResults, key)
	}
	if ok {
		return preview, nil
	}
	return domain.PreviewRemediation(c, cmd.SegmentID, cmd.Disposition, cmd.PublicText, cmd.MuteRanges, cmd.EvidenceRefs, cmd.Reason, s.now())
}

func remediationPreviewKey(cmd RemediateCommand) (string, error) {
	normalized := cmd
	normalized.Preview = false
	normalized.RequestID = ""
	return fingerprint(normalized)
}
