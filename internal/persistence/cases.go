package persistence

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"oralhistory/internal/domain"
)

func (s *Store) LoadCase(caseID string) (*domain.OralHistoryCase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loadCaseUnlocked(caseID)
}

func (s *Store) loadCaseUnlocked(caseID string) (*domain.OralHistoryCase, error) {
	content, err := os.ReadFile(s.casePath(caseID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, domain.NewError(domain.ErrNotFound, "案件不存在")
	}
	if err != nil {
		return nil, err
	}
	var result domain.OralHistoryCase
	if err := json.Unmarshal(content, &result); err != nil {
		return nil, domain.WrapError(domain.ErrIntegrity, "案件快照损坏: %v", err)
	}
	return &result, nil
}

func (s *Store) ListCases() ([]domain.OralHistoryCase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(filepath.Join(s.root, "cases"))
	if err != nil {
		return nil, err
	}
	result := make([]domain.OralHistoryCase, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(s.root, "cases", entry.Name()))
		if err != nil {
			return nil, err
		}
		var c domain.OralHistoryCase
		if err := json.Unmarshal(content, &c); err != nil {
			return nil, domain.WrapError(domain.ErrIntegrity, "案件快照 %s 损坏", entry.Name())
		}
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result, nil
}

func marshalCase(c *domain.OralHistoryCase) ([]byte, error) {
	content, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}
