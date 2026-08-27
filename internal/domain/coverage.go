package domain

import (
	"sort"
	"time"
)

type CoverageConstraint struct {
	ConstraintID      string `json:"constraint_id"`
	Policy            Policy `json:"policy"`
	EvidenceReference string `json:"evidence_reference"`
	RequiredAlias     string `json:"required_alias,omitempty"`
	NotBefore         string `json:"not_before,omitempty"`
}

type CoverageItem struct {
	SegmentID   string               `json:"segment_id"`
	StartMS     int64                `json:"start_ms"`
	ScopeType   string               `json:"scope_type"`
	ScopeValue  string               `json:"scope_value"`
	Status      string               `json:"status"`
	Diagnostic  string               `json:"diagnostic_code"`
	Constraints []CoverageConstraint `json:"constraints"`
}

type CoverageSummary struct {
	Clear          int      `json:"clear"`
	Uncovered      int      `json:"uncovered"`
	Ambiguous      int      `json:"ambiguous"`
	EmbargoElapsed int      `json:"embargo_elapsed"`
	SegmentIDs     []string `json:"affected_segment_ids"`
}

type CoverageMatrix struct {
	Revision int64           `json:"revision"`
	Items    []CoverageItem  `json:"items"`
	Summary  CoverageSummary `json:"summary"`
}

func BuildCoverageMatrix(c *OralHistoryCase, today time.Time) CoverageMatrix {
	result := CoverageMatrix{Revision: c.Revision}
	segments := append([]TranscriptSegment(nil), c.Segments...)
	sortSegments(segments)
	for _, segment := range segments {
		for _, value := range append([]string(nil), segment.SubjectIDs...) {
			result.Items = append(result.Items, coverageItem(c, segment, "subject", value, today))
		}
		for _, value := range append([]string(nil), segment.TopicCodes...) {
			result.Items = append(result.Items, coverageItem(c, segment, "topic", value, today))
		}
	}
	sort.Slice(result.Items, func(i, j int) bool {
		if result.Items[i].StartMS != result.Items[j].StartMS {
			return result.Items[i].StartMS < result.Items[j].StartMS
		}
		if result.Items[i].ScopeType != result.Items[j].ScopeType {
			return result.Items[i].ScopeType < result.Items[j].ScopeType
		}
		return result.Items[i].ScopeValue < result.Items[j].ScopeValue
	})
	affected := map[string]bool{}
	for _, item := range result.Items {
		switch item.Status {
		case "clear":
			result.Summary.Clear++
		case "uncovered":
			result.Summary.Uncovered++
		case "ambiguous":
			result.Summary.Ambiguous++
		case "embargo_elapsed":
			result.Summary.EmbargoElapsed++
		}
		if item.Status != "clear" {
			affected[item.SegmentID] = true
		}
	}
	for id := range affected {
		result.Summary.SegmentIDs = append(result.Summary.SegmentIDs, id)
	}
	sort.Strings(result.Summary.SegmentIDs)
	return result
}

func coverageItem(c *OralHistoryCase, segment TranscriptSegment, scopeType, scopeValue string, today time.Time) CoverageItem {
	item := CoverageItem{SegmentID: segment.SegmentID, StartMS: segment.StartMS, ScopeType: scopeType, ScopeValue: scopeValue, Status: "clear", Diagnostic: "coverage_clear"}
	var policies []ConsentConstraint
	for _, constraint := range c.Constraints {
		if constraint.ScopeType == scopeType && constraint.ScopeValue == scopeValue {
			policies = append(policies, constraint)
		}
	}
	sort.Slice(policies, func(i, j int) bool { return policies[i].ConstraintID < policies[j].ConstraintID })
	for _, value := range policies {
		item.Constraints = append(item.Constraints, CoverageConstraint{ConstraintID: value.ConstraintID, Policy: value.Policy, EvidenceReference: value.EvidenceReference, RequiredAlias: value.RequiredAlias, NotBefore: value.NotBefore})
	}
	if len(policies) == 0 {
		item.Status, item.Diagnostic = "uncovered", "coverage_missing"
		return item
	}
	if ambiguousCoverage(policies) {
		item.Status, item.Diagnostic = "ambiguous", "coverage_ambiguous"
		return item
	}
	if policies[0].Policy == PolicyDelay {
		date, err := time.Parse("2006-01-02", policies[0].NotBefore)
		if err == nil && !today.Before(date) {
			item.Status, item.Diagnostic = "embargo_elapsed", "embargo_elapsed"
		}
	}
	return item
}

func ambiguousCoverage(values []ConsentConstraint) bool {
	if len(values) < 2 {
		return false
	}
	first := values[0]
	for _, value := range values[1:] {
		if value.Policy != first.Policy || value.RequiredAlias != first.RequiredAlias || value.NotBefore != first.NotBefore {
			return true
		}
	}
	return false
}
