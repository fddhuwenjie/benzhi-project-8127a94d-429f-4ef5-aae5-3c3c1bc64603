package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

func DetectConflicts(c *OralHistoryCase, today time.Time) []DisclosureConflict {
	constraints := append([]ConsentConstraint(nil), c.Constraints...)
	segments := append([]TranscriptSegment(nil), c.Segments...)
	sort.Slice(constraints, func(i, j int) bool { return constraints[i].ConstraintID < constraints[j].ConstraintID })
	sort.Slice(segments, func(i, j int) bool {
		if segments[i].StartMS == segments[j].StartMS {
			return segments[i].SegmentID < segments[j].SegmentID
		}
		return segments[i].StartMS < segments[j].StartMS
	})
	var result []DisclosureConflict
	for _, segment := range segments {
		matched := false
		for _, constraint := range constraints {
			if !constraintApplies(segment, constraint) {
				continue
			}
			matched = true
			severity, code, message := conflictForPolicy(constraint, today)
			if code == "" {
				continue
			}
			resolved := remediationSatisfies(segment, constraint)
			result = append(result, DisclosureConflict{
				ConflictID: stableConflictID(segment.SegmentID, constraint.ConstraintID, code),
				SegmentID:  segment.SegmentID, ConstraintID: constraint.ConstraintID,
				Severity: severity, Code: code, Message: message, Resolved: resolved,
			})
		}
		if !matched {
			result = append(result, DisclosureConflict{
				ConflictID: stableConflictID(segment.SegmentID, "none", "missing_consent"),
				SegmentID:  segment.SegmentID, Severity: "blocking", Code: "missing_consent",
				Message: "片段涉及的主体或主题没有可执行授权条款", Resolved: segment.Disposition == DispositionExclude,
			})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].SegmentID == result[j].SegmentID {
			return result[i].ConflictID < result[j].ConflictID
		}
		return segmentStart(segments, result[i].SegmentID) < segmentStart(segments, result[j].SegmentID)
	})
	return result
}

func constraintApplies(segment TranscriptSegment, c ConsentConstraint) bool {
	if c.ScopeType == "subject" {
		return ContainsString(segment.SubjectIDs, c.ScopeValue)
	}
	return ContainsString(segment.TopicCodes, c.ScopeValue)
}

func conflictForPolicy(c ConsentConstraint, today time.Time) (string, string, string) {
	switch c.Policy {
	case PolicyDeny:
		return "blocking", "disclosure_denied", "授权条款禁止公开"
	case PolicyAnonymous:
		return "blocking", "anonymization_required", fmt.Sprintf("必须使用别名 %s", c.RequiredAlias)
	case PolicyDelay:
		date, _ := time.Parse("2006-01-02", c.NotBefore)
		if today.Before(date) {
			return "blocking", "embargo_active", "授权约定的延后公开日期尚未到达"
		}
		return "notice", "embargo_elapsed", "延后公开期限已届满，请复核日期依据"
	default:
		return "", "", ""
	}
}

func remediationSatisfies(s TranscriptSegment, constraint ConsentConstraint) bool {
	if len(s.EvidenceRefs) == 0 || s.Reason == "" {
		return false
	}
	switch constraint.Policy {
	case PolicyDeny, PolicyDelay:
		return s.Disposition == DispositionExclude || (s.Disposition == DispositionMute && len(s.MuteRanges) > 0)
	case PolicyAnonymous:
		return s.Disposition == DispositionReplace && s.PublicText != "" && s.PublicText != s.SourceText && strings.Contains(s.PublicText, constraint.RequiredAlias)
	default:
		return true
	}
}

func stableConflictID(segmentID, constraintID, code string) string {
	sum := sha256.Sum256([]byte(segmentID + "\x00" + constraintID + "\x00" + code))
	return "conf-" + hex.EncodeToString(sum[:6])
}

func segmentStart(segments []TranscriptSegment, id string) int64 {
	for _, segment := range segments {
		if segment.SegmentID == id {
			return segment.StartMS
		}
	}
	return 0
}

func HasOpenBlocking(conflicts []DisclosureConflict) bool {
	for _, conflict := range conflicts {
		if conflict.Severity == "blocking" && !conflict.Resolved {
			return true
		}
	}
	return false
}
