package domain

import (
	"fmt"
	"sort"
	"strings"
)

type BatchIssue struct {
	RecordType string `json:"record_type"`
	Line       int    `json:"line"`
	Field      string `json:"field"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

type BatchSummary struct {
	SegmentsAdded      int   `json:"segments_added"`
	SegmentsUpdated    int   `json:"segments_updated"`
	ConstraintsAdded   int   `json:"constraints_added"`
	ConstraintsUpdated int   `json:"constraints_updated"`
	CoverageStartMS    int64 `json:"coverage_start_ms"`
	CoverageEndMS      int64 `json:"coverage_end_ms"`
}

type BatchValidation struct {
	Segments    []TranscriptSegment `json:"segments"`
	Constraints []ConsentConstraint `json:"constraints"`
	Issues      []BatchIssue        `json:"issues"`
	Summary     BatchSummary        `json:"summary"`
}

func ValidateRegistrationBatch(c *OralHistoryCase, incomingSegments []TranscriptSegment, incomingConstraints []ConsentConstraint) BatchValidation {
	result := BatchValidation{Segments: normalizeSegments(incomingSegments), Constraints: normalizeConstraints(incomingConstraints)}
	if len(incomingSegments) == 0 && len(incomingConstraints) == 0 {
		result.Issues = append(result.Issues, issue("batch", 0, "records", "batch_empty", "批次至少包含一个片段或授权约束"))
	}
	segmentLines, constraintLines := map[string]int{}, map[string]int{}
	for i, segment := range result.Segments {
		line := i + 1
		if previous, ok := segmentLines[segment.SegmentID]; ok && segment.SegmentID != "" {
			result.Issues = append(result.Issues, issue("segment", line, "segment_id", "duplicate_segment_id", fmt.Sprintf("片段标识与第 %d 行重复", previous)))
		}
		segmentLines[segment.SegmentID] = line
		if err := ValidateSegment(segment); err != nil {
			result.Issues = append(result.Issues, issue("segment", line, segmentField(segment), "invalid_segment", err.Error()))
		}
	}
	for i, constraint := range result.Constraints {
		line := i + 1
		if previous, ok := constraintLines[constraint.ConstraintID]; ok && constraint.ConstraintID != "" {
			result.Issues = append(result.Issues, issue("constraint", line, "constraint_id", "duplicate_constraint_id", fmt.Sprintf("约束标识与第 %d 行重复", previous)))
		}
		constraintLines[constraint.ConstraintID] = line
		if err := ValidateConstraint(constraint); err != nil {
			result.Issues = append(result.Issues, issue("constraint", line, constraintField(constraint), "invalid_constraint", err.Error()))
		}
	}

	mergedSegments := append([]TranscriptSegment(nil), c.Segments...)
	for _, value := range result.Segments {
		mergedSegments = upsertSegment(mergedSegments, value)
	}
	mergedConstraints := append([]ConsentConstraint(nil), c.Constraints...)
	for _, value := range result.Constraints {
		mergedConstraints = upsertConstraint(mergedConstraints, value)
	}
	sortSegments(mergedSegments)
	if err := ValidateTimeline(mergedSegments); err != nil {
		for i, value := range mergedSegments {
			if i == 0 {
				continue
			}
			field, code, message := "", "", ""
			if value.StartMS < mergedSegments[i-1].EndMS {
				field, code, message = "start_ms", "timeline_overlap", "片段与前一片段重叠"
			} else if value.StartMS != mergedSegments[i-1].EndMS {
				field, code, message = "start_ms", "timeline_gap", "片段与前一片段之间存在时间码断裂"
			}
			if line, ok := segmentLines[value.SegmentID]; ok && code != "" {
				result.Issues = append(result.Issues, issue("segment", line, field, code, message))
			} else if line, ok := segmentLines[mergedSegments[i-1].SegmentID]; ok && code != "" {
				result.Issues = append(result.Issues, issue("segment", line, "end_ms", code, message))
			}
		}
	}
	subjects, topics := map[string]bool{}, map[string]bool{}
	for _, segment := range mergedSegments {
		for _, value := range segment.SubjectIDs {
			subjects[value] = true
		}
		for _, value := range segment.TopicCodes {
			topics[value] = true
		}
	}
	for i, constraint := range result.Constraints {
		known := constraint.ScopeType == "subject" && subjects[constraint.ScopeValue] || constraint.ScopeType == "topic" && topics[constraint.ScopeValue]
		if (constraint.ScopeType == "subject" || constraint.ScopeType == "topic") && !known {
			result.Issues = append(result.Issues, issue("constraint", i+1, "scope_value", "unknown_scope_reference", "授权范围未被任何片段引用"))
		}
	}
	for _, segment := range result.Segments {
		if hasSegment(c.Segments, segment.SegmentID) {
			result.Summary.SegmentsUpdated++
		} else {
			result.Summary.SegmentsAdded++
		}
	}
	for _, constraint := range result.Constraints {
		if hasConstraint(c.Constraints, constraint.ConstraintID) {
			result.Summary.ConstraintsUpdated++
		} else {
			result.Summary.ConstraintsAdded++
		}
	}
	if len(mergedSegments) > 0 {
		result.Summary.CoverageStartMS = mergedSegments[0].StartMS
		result.Summary.CoverageEndMS = mergedSegments[len(mergedSegments)-1].EndMS
	}
	return result
}

func ApplyRegistrationBatch(c *OralHistoryCase, validation BatchValidation, revision int64) {
	for _, segment := range validation.Segments {
		segment.CaseID, segment.Revision = c.CaseID, revision
		if segment.Disposition == "" {
			segment.Disposition = DispositionOriginal
		}
		c.Segments = upsertSegment(c.Segments, segment)
	}
	for _, constraint := range validation.Constraints {
		constraint.CaseID, constraint.FrozenRevision = c.CaseID, revision
		c.Constraints = upsertConstraint(c.Constraints, constraint)
	}
	sortSegments(c.Segments)
	sort.Slice(c.Constraints, func(i, j int) bool { return c.Constraints[i].ConstraintID < c.Constraints[j].ConstraintID })
}

func normalizeSegments(values []TranscriptSegment) []TranscriptSegment {
	result := append([]TranscriptSegment(nil), values...)
	for i := range result {
		result[i].SegmentID = strings.TrimSpace(result[i].SegmentID)
		result[i].SourceText = strings.TrimSpace(result[i].SourceText)
		result[i].SubjectIDs = cleanStrings(result[i].SubjectIDs)
		result[i].TopicCodes = cleanStrings(result[i].TopicCodes)
	}
	return result
}

func normalizeConstraints(values []ConsentConstraint) []ConsentConstraint {
	result := append([]ConsentConstraint(nil), values...)
	for i := range result {
		result[i].ConstraintID = strings.TrimSpace(result[i].ConstraintID)
		result[i].ScopeType = strings.TrimSpace(result[i].ScopeType)
		result[i].ScopeValue = strings.TrimSpace(result[i].ScopeValue)
		result[i].EvidenceReference = strings.TrimSpace(result[i].EvidenceReference)
		result[i].RequiredAlias = strings.TrimSpace(result[i].RequiredAlias)
		result[i].NotBefore = strings.TrimSpace(result[i].NotBefore)
	}
	return result
}

func cleanStrings(values []string) []string {
	seen, result := map[string]bool{}, []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func sortSegments(values []TranscriptSegment) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].StartMS == values[j].StartMS {
			return values[i].SegmentID < values[j].SegmentID
		}
		return values[i].StartMS < values[j].StartMS
	})
}
func upsertSegment(values []TranscriptSegment, value TranscriptSegment) []TranscriptSegment {
	for i := range values {
		if values[i].SegmentID == value.SegmentID {
			values[i] = value
			return values
		}
	}
	return append(values, value)
}
func upsertConstraint(values []ConsentConstraint, value ConsentConstraint) []ConsentConstraint {
	for i := range values {
		if values[i].ConstraintID == value.ConstraintID {
			values[i] = value
			return values
		}
	}
	return append(values, value)
}
func hasSegment(values []TranscriptSegment, id string) bool {
	for _, v := range values {
		if v.SegmentID == id {
			return true
		}
	}
	return false
}
func hasConstraint(values []ConsentConstraint, id string) bool {
	for _, v := range values {
		if v.ConstraintID == id {
			return true
		}
	}
	return false
}
func issue(kind string, line int, field, code, message string) BatchIssue {
	return BatchIssue{RecordType: kind, Line: line, Field: field, Code: code, Message: message}
}
func segmentField(s TranscriptSegment) string {
	if s.SegmentID == "" {
		return "segment_id"
	}
	if strings.TrimSpace(s.SourceText) == "" {
		return "source_text"
	}
	if s.EndMS <= s.StartMS {
		return "end_ms"
	}
	return "subject_ids"
}
func constraintField(c ConsentConstraint) string {
	if c.ConstraintID == "" {
		return "constraint_id"
	}
	if c.ScopeType != "subject" && c.ScopeType != "topic" {
		return "scope_type"
	}
	if c.ScopeValue == "" {
		return "scope_value"
	}
	if c.EvidenceReference == "" {
		return "evidence_reference"
	}
	if c.Policy == PolicyAnonymous && c.RequiredAlias == "" {
		return "required_alias"
	}
	if c.Policy == PolicyDelay {
		return "not_before"
	}
	return "policy"
}
