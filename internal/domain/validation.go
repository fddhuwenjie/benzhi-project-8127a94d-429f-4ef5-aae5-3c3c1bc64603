package domain

import (
	"encoding/hex"
	"strings"
	"time"
)

func ValidateNewCase(c *OralHistoryCase) error {
	if strings.TrimSpace(c.CaseID) == "" || strings.TrimSpace(c.Title) == "" {
		return NewError(ErrInvalidInput, "case_id 和 title 不能为空")
	}
	if c.ArchivistID == "" || c.ReviewerID == "" {
		return NewError(ErrInvalidInput, "必须指定档案员和复核员")
	}
	if c.ArchivistID == c.ReviewerID {
		return NewError(ErrInvalidInput, "档案员与复核员必须为不同人员")
	}
	if _, err := time.Parse("2006-01-02", c.CollectionDate); err != nil {
		return NewError(ErrInvalidInput, "collection_date 必须为 YYYY-MM-DD")
	}
	if c.CustodyReference == "" || c.SourceAudioURI == "" {
		return NewError(ErrInvalidInput, "保管标识和原始录音引用不能为空")
	}
	if !validSHA256(c.SourceSHA256) || !validSHA256(c.ConsentDocumentSHA256) {
		return NewError(ErrInvalidInput, "来源与授权文书摘要必须是 64 位 SHA-256")
	}
	return nil
}

func validSHA256(v string) bool {
	if len(v) != 64 {
		return false
	}
	_, err := hex.DecodeString(v)
	return err == nil
}

func ValidateSegment(s TranscriptSegment) error {
	if s.SegmentID == "" || strings.TrimSpace(s.SourceText) == "" {
		return NewError(ErrInvalidInput, "片段标识和原文不能为空")
	}
	if s.StartMS < 0 || s.EndMS <= s.StartMS {
		return NewError(ErrInvalidInput, "片段时间范围无效")
	}
	if len(s.SubjectIDs) == 0 && len(s.TopicCodes) == 0 {
		return NewError(ErrInvalidInput, "片段至少关联一个人物或主题")
	}
	return nil
}

func ValidateConstraint(c ConsentConstraint) error {
	if c.ConstraintID == "" || c.ScopeValue == "" || c.EvidenceReference == "" {
		return NewError(ErrInvalidInput, "授权约束标识、范围和值及依据不能为空")
	}
	if c.ScopeType != "subject" && c.ScopeType != "topic" {
		return NewError(ErrInvalidInput, "scope_type 仅支持 subject 或 topic")
	}
	switch c.Policy {
	case PolicyAllow, PolicyDeny:
	case PolicyDelay:
		if _, err := time.Parse("2006-01-02", c.NotBefore); err != nil {
			return NewError(ErrInvalidInput, "延后公开必须指定有效 not_before")
		}
	case PolicyAnonymous:
		if strings.TrimSpace(c.RequiredAlias) == "" {
			return NewError(ErrInvalidInput, "匿名授权必须指定 required_alias")
		}
	default:
		return NewError(ErrInvalidInput, "未知授权策略")
	}
	return nil
}

func ValidateTimeline(segments []TranscriptSegment) error {
	for i := range segments {
		if err := ValidateSegment(segments[i]); err != nil {
			return err
		}
		if i > 0 {
			if segments[i].StartMS < segments[i-1].EndMS {
				return NewError(ErrInvalidInput, "片段时间范围发生重叠")
			}
			if segments[i].StartMS != segments[i-1].EndMS {
				return NewError(ErrInvalidInput, "片段时间范围必须连续")
			}
		}
	}
	return nil
}

func ContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
