package domain

func CanTransition(from, to CaseStatus) bool {
	allowed := map[CaseStatus][]CaseStatus{
		StatusDraft:       {StatusFrozen},
		StatusFrozen:      {StatusRemediation, StatusCandidate},
		StatusRemediation: {StatusRemediation, StatusCandidate},
		StatusCandidate:   {StatusReview},
		StatusReview:      {StatusReturned, StatusSealed},
		StatusReturned:    {StatusReturned, StatusCandidate},
	}
	for _, value := range allowed[from] {
		if value == to {
			return true
		}
	}
	return false
}

func Transition(c *OralHistoryCase, to CaseStatus) error {
	if !CanTransition(c.Status, to) {
		return WrapError(ErrInvalidState, "不能从 %s 转换到 %s", c.Status, to)
	}
	c.Status = to
	return nil
}

func EnsureMutable(c *OralHistoryCase) error {
	if c.Status == StatusSealed {
		return NewError(ErrInvalidState, "案件已封存，禁止修改")
	}
	return nil
}

func EnsureBaselineEditable(c *OralHistoryCase) error {
	if c.Status != StatusDraft {
		return NewError(ErrInvalidState, "基线冻结后不得替换来源或授权文书")
	}
	return nil
}

func EnsureContentEditable(c *OralHistoryCase, segmentID string) error {
	switch c.Status {
	case StatusFrozen, StatusRemediation:
		return nil
	case StatusReturned:
		if ContainsString(c.OpenReturnSegmentIDs, segmentID) {
			return nil
		}
		return NewError(ErrForbidden, "退回后只能修订复核员指出的片段")
	default:
		return NewError(ErrInvalidState, "当前状态不允许修订片段")
	}
}
