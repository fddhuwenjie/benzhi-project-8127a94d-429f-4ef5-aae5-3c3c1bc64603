package application

import (
	"sort"

	"oralhistory/internal/domain"
)

func (s *Service) AddSegment(cmd AddSegmentCommand) (*domain.OralHistoryCase, error) {
	return s.withCase(cmd.CommandMeta, cmd, "segment_saved", func(c *domain.OralHistoryCase) (any, *domain.ReleaseManifest, error) {
		if cmd.ActorID != c.ArchivistID {
			return nil, nil, domain.NewError(domain.ErrForbidden, "只有档案员可以登记片段")
		}
		if err := domain.EnsureContentEditable(c, cmd.Segment.SegmentID); err != nil {
			return nil, nil, err
		}
		segment := cmd.Segment
		segment.CaseID = c.CaseID
		segment.Revision = c.Revision + 1
		if segment.Disposition == "" {
			segment.Disposition = domain.DispositionOriginal
		}
		if err := domain.ValidateSegment(segment); err != nil {
			return nil, nil, err
		}
		replaced := false
		for i := range c.Segments {
			if c.Segments[i].SegmentID == segment.SegmentID {
				c.Segments[i] = segment
				replaced = true
				break
			}
		}
		if !replaced {
			c.Segments = append(c.Segments, segment)
		}
		sort.Slice(c.Segments, func(i, j int) bool { return c.Segments[i].StartMS < c.Segments[j].StartMS })
		if err := domain.ValidateTimeline(c.Segments); err != nil {
			return nil, nil, err
		}
		c.Candidate = nil
		return map[string]any{"segment_id": segment.SegmentID, "replaced": replaced}, nil, nil
	})
}

func (s *Service) AddConstraint(cmd AddConstraintCommand) (*domain.OralHistoryCase, error) {
	return s.withCase(cmd.CommandMeta, cmd, "constraint_saved", func(c *domain.OralHistoryCase) (any, *domain.ReleaseManifest, error) {
		if cmd.ActorID != c.ArchivistID {
			return nil, nil, domain.NewError(domain.ErrForbidden, "只有档案员可以登记授权约束")
		}
		if c.Status != domain.StatusFrozen && c.Status != domain.StatusRemediation {
			return nil, nil, domain.NewError(domain.ErrInvalidState, "当前状态不允许登记授权约束")
		}
		constraint := cmd.Constraint
		constraint.CaseID = c.CaseID
		constraint.FrozenRevision = c.Revision
		if err := domain.ValidateConstraint(constraint); err != nil {
			return nil, nil, err
		}
		for _, existing := range c.Constraints {
			if existing.ConstraintID == constraint.ConstraintID {
				return nil, nil, domain.NewError(domain.ErrInvalidInput, "constraint_id 已存在")
			}
		}
		c.Constraints = append(c.Constraints, constraint)
		return map[string]any{"constraint_id": constraint.ConstraintID}, nil, nil
	})
}
