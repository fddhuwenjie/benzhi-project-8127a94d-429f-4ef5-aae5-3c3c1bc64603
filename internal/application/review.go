package application

import (
	"fmt"
	"sort"
	"time"

	"oralhistory/internal/domain"
	"oralhistory/internal/evidence"
)

func (s *Service) DecideReview(cmd ReviewCommand) (*domain.OralHistoryCase, error) {
	return s.withCase(cmd.CommandMeta, cmd, "review_"+cmd.Decision, func(c *domain.OralHistoryCase) (any, *domain.ReleaseManifest, error) {
		if c.Status != domain.StatusReview || c.Candidate == nil {
			return nil, nil, domain.NewError(domain.ErrInvalidState, "案件当前不在复核中")
		}
		if cmd.ActorID != c.ReviewerID || cmd.ActorID == c.ArchivistID {
			return nil, nil, domain.NewError(domain.ErrForbidden, "只能由独立复核员作出决定")
		}
		if cmd.Decision != "returned" && cmd.Decision != "approved" {
			return nil, nil, domain.NewError(domain.ErrInvalidInput, "复核决定仅支持 returned 或 approved")
		}
		if err := validateReviewItems(c.Candidate, cmd.ItemResults); err != nil {
			return nil, nil, err
		}
		now := s.now().UTC()
		decision := domain.ReviewDecision{
			ReviewID: fmt.Sprintf("review-%s-%d", c.CaseID, c.ReviewRound),
			CaseID:   c.CaseID, CandidateRevision: c.Candidate.Revision,
			ReviewerID: cmd.ActorID, ItemResults: cmd.ItemResults,
			Decision: cmd.Decision, ReturnReasons: cmd.ReturnReasons,
			SubmittedAt: c.Candidate.GeneratedAt, DecidedAt: now,
		}
		if cmd.Decision == "returned" {
			if len(cmd.ReturnReasons) == 0 {
				return nil, nil, domain.NewError(domain.ErrInvalidInput, "退回必须提供结构化意见")
			}
			ids := make([]string, 0, len(cmd.ReturnReasons))
			seenReason := map[string]bool{}
			for _, reason := range cmd.ReturnReasons {
				if reason.SegmentID == "" || reason.Code == "" || reason.Comment == "" {
					return nil, nil, domain.NewError(domain.ErrInvalidInput, "退回意见必须包含片段、代码和说明")
				}
				if !candidateHasSegment(c.Candidate, reason.SegmentID) {
					return nil, nil, domain.NewError(domain.ErrInvalidInput, "退回意见引用未知片段")
				}
				key := reason.SegmentID + "\x00" + reason.Code
				if seenReason[key] {
					return nil, nil, domain.NewError(domain.ErrInvalidInput, "同一轮不得重复提交相同片段和问题代码")
				}
				seenReason[key] = true
				ids = append(ids, reason.SegmentID)
			}
			returnedSegments := map[string]bool{}
			for _, reason := range cmd.ReturnReasons {
				returnedSegments[reason.SegmentID] = true
			}
			for i := range c.ReturnTasks {
				if c.ReturnTasks[i].Status == "pending_review" {
					if returnedSegments[c.ReturnTasks[i].SegmentID] {
						c.ReturnTasks[i].Status = "returned"
					} else {
						c.ReturnTasks[i].Status = "passed"
					}
				}
			}
			for _, reason := range cmd.ReturnReasons {
				c.ReturnTasks = append(c.ReturnTasks, domain.NewReturnTask(c.ReviewRound, reason))
			}
			sort.Strings(ids)
			c.Reviews = append(c.Reviews, decision)
			c.OpenReturnSegmentIDs = domain.OpenTaskSegmentIDs(c.ReturnTasks)
			c.Status = domain.StatusReturned
			return map[string]any{"review_id": decision.ReviewID, "open_segment_ids": c.OpenReturnSegmentIDs}, nil, nil
		}
		if len(cmd.ReturnReasons) > 0 {
			return nil, nil, domain.NewError(domain.ErrInvalidInput, "批准决定不得包含退回意见")
		}
		for _, task := range c.ReturnTasks {
			if task.Status == "to_remediate" {
				return nil, nil, domain.NewError(domain.ErrInvalidState, "仍有退回任务待整改")
			}
		}
		var previous *domain.CandidateRelease
		if len(c.CandidateHistory) > 1 {
			previous = &c.CandidateHistory[len(c.CandidateHistory)-2]
		}
		roundDiff := evidence.CompareCandidates(previous, c.Candidate, taskSegmentsForPreviousRound(c))
		if roundDiff.HasAnomaly {
			return nil, nil, domain.NewError(domain.ErrInvalidInput, "候选版包含开放退回任务之外的异常变化")
		}
		for _, item := range cmd.ItemResults {
			if !item.ConsentValid || !item.RedactionValid {
				return nil, nil, domain.NewError(domain.ErrInvalidInput, "批准前必须逐项确认授权和遮蔽效果")
			}
		}
		c.Reviews = append(c.Reviews, decision)
		for i := range c.ReturnTasks {
			if c.ReturnTasks[i].Status == "pending_review" {
				c.ReturnTasks[i].Status = "passed"
			}
		}
		c.OpenReturnSegmentIDs = nil
		auditRoot, err := s.store.VerifyEventLog(c.CaseID)
		if err != nil {
			return nil, nil, err
		}
		manifest, err := evidence.BuildManifest(c, auditRoot, now)
		if err != nil {
			return nil, nil, err
		}
		c.Manifest = manifest
		c.Status = domain.StatusSealed
		c.SealedAt = &now
		return map[string]any{"review_id": decision.ReviewID, "manifest_sha256": manifest.ManifestSHA256}, manifest, nil
	})
}

func validateReviewItems(candidate *domain.CandidateRelease, items []domain.ReviewItemResult) error {
	if len(items) != len(candidate.Diffs) {
		return domain.NewError(domain.ErrInvalidInput, "必须对候选版全部片段逐项复核")
	}
	seen := make(map[string]bool)
	for _, item := range items {
		if seen[item.SegmentID] || !candidateHasSegment(candidate, item.SegmentID) {
			return domain.NewError(domain.ErrInvalidInput, "复核项重复或引用未知片段")
		}
		seen[item.SegmentID] = true
	}
	return nil
}

func candidateHasSegment(candidate *domain.CandidateRelease, segmentID string) bool {
	for _, diff := range candidate.Diffs {
		if diff.SegmentID == segmentID {
			return true
		}
	}
	return false
}

func unique(values []string) []string {
	result := values[:0]
	for i, value := range values {
		if i == 0 || value != values[i-1] {
			result = append(result, value)
		}
	}
	return result
}

var _ = time.Time{}
