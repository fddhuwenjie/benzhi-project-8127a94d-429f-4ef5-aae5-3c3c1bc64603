package evidence

import (
	"sort"

	"oralhistory/internal/domain"
)

func ConflictInputDigest(c *domain.OralHistoryCase, checkDate string) (string, error) {
	segments := append([]domain.TranscriptSegment(nil), c.Segments...)
	constraints := append([]domain.ConsentConstraint(nil), c.Constraints...)
	sort.Slice(segments, func(i, j int) bool {
		if segments[i].StartMS == segments[j].StartMS {
			return segments[i].SegmentID < segments[j].SegmentID
		}
		return segments[i].StartMS < segments[j].StartMS
	})
	sort.Slice(constraints, func(i, j int) bool { return constraints[i].ConstraintID < constraints[j].ConstraintID })
	return Digest(struct {
		Segments    []domain.TranscriptSegment `json:"segments"`
		Constraints []domain.ConsentConstraint `json:"constraints"`
		CheckDate   string                     `json:"check_date"`
	}{segments, constraints, checkDate})
}

func ConflictContentDigest(c *domain.OralHistoryCase) (string, error) {
	return ConflictInputDigest(c, "")
}

func ConflictResultDigest(values []domain.DisclosureConflict) (string, error) {
	ids := make([]string, len(values))
	for i := range values {
		ids[i] = values[i].ConflictID
	}
	sort.Strings(ids)
	return Digest(ids)
}

func CompareConflicts(current []domain.DisclosureConflict, checks []domain.ConflictCheck, sameInput bool) domain.ConflictDelta {
	if sameInput {
		return domain.ConflictDelta{}
	}
	delta := domain.ConflictDelta{}
	if len(checks) == 0 {
		delta.New = append(delta.New, current...)
		return delta
	}
	previous := checks[len(checks)-1].Conflicts
	prev, ever := map[string]domain.DisclosureConflict{}, map[string]bool{}
	for _, check := range checks {
		for _, conflict := range check.Conflicts {
			ever[conflict.ConflictID] = true
		}
	}
	for _, conflict := range previous {
		prev[conflict.ConflictID] = conflict
	}
	currentIDs := map[string]bool{}
	for _, conflict := range current {
		currentIDs[conflict.ConflictID] = true
		if old, ok := prev[conflict.ConflictID]; ok {
			if !old.Resolved && conflict.Resolved {
				delta.Resolved = append(delta.Resolved, conflict)
			} else if old.Resolved && !conflict.Resolved {
				delta.Reopened = append(delta.Reopened, conflict)
			} else {
				delta.Unchanged = append(delta.Unchanged, conflict)
			}
		} else if ever[conflict.ConflictID] {
			delta.Reopened = append(delta.Reopened, conflict)
		} else {
			delta.New = append(delta.New, conflict)
		}
	}
	for _, conflict := range previous {
		if !currentIDs[conflict.ConflictID] && !conflict.Resolved {
			delta.Resolved = append(delta.Resolved, conflict)
		}
	}
	return delta
}
