package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"oralhistory/internal/domain"
)

func (s *Store) Recover() error {
	entries, err := os.ReadDir(filepath.Join(s.root, "events"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		caseID := unescapeName(strings.TrimSuffix(entry.Name(), ".jsonl"))
		events, err := s.LoadEvents(caseID)
		if err != nil {
			return describeEventError(caseID, err)
		}
		caseContent, err := os.ReadFile(s.casePath(caseID))
		if err != nil {
			return domain.WrapError(domain.ErrIntegrity, "事件存在但案件快照 %s 不可读", caseID)
		}
		var c domain.OralHistoryCase
		if err := json.Unmarshal(caseContent, &c); err != nil {
			return domain.WrapError(domain.ErrIntegrity, "案件快照 %s 无法解析", caseID)
		}
		if int64(len(events)) != c.Revision {
			return domain.WrapError(domain.ErrIntegrity, "案件 %s 快照修订 %d 与事件数 %d 不一致", caseID, c.Revision, len(events))
		}
	}
	return nil
}
