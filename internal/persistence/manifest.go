package persistence

import (
	"encoding/json"
	"errors"
	"os"

	"oralhistory/internal/domain"
)

func (s *Store) LoadManifest(caseID string) (*domain.ReleaseManifest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	content, err := os.ReadFile(s.manifestPath(caseID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, domain.NewError(domain.ErrNotFound, "公开清单不存在")
	}
	if err != nil {
		return nil, err
	}
	var manifest domain.ReleaseManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, domain.WrapError(domain.ErrIntegrity, "公开清单损坏: %v", err)
	}
	return &manifest, nil
}
