package domain

func AllowedActions(c *OralHistoryCase) []string {
	switch c.Status {
	case StatusDraft:
		return []string{"freeze"}
	case StatusFrozen, StatusRemediation:
		return []string{"add_segment", "add_constraint", "register_batch", "check_conflicts", "remediate", "generate_candidate"}
	case StatusCandidate:
		return []string{"submit_review"}
	case StatusReview:
		return []string{"return_review", "approve"}
	case StatusReturned:
		return []string{"remediate_returned", "check_conflicts", "generate_candidate"}
	case StatusSealed:
		return []string{"verify_manifest"}
	default:
		return nil
	}
}
