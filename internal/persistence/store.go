package persistence

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	root                   string
	mu                     sync.RWMutex
	manifestSnapshotCaseID string
	manifestSnapshot       []byte
}

func Open(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("数据目录不能为空")
	}
	for _, dir := range []string{root, filepath.Join(root, "cases"), filepath.Join(root, "events"), filepath.Join(root, "idempotency"), filepath.Join(root, "manifests")} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, err
		}
	}
	store := &Store{root: root}
	if err := store.Recover(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Root() string { return s.root }

func (s *Store) casePath(caseID string) string {
	return filepath.Join(s.root, "cases", safeName(caseID)+".json")
}

func (s *Store) eventPath(caseID string) string {
	return filepath.Join(s.root, "events", safeName(caseID)+".jsonl")
}

func (s *Store) idempotencyPath(caseID, requestID string) string {
	return filepath.Join(s.root, "idempotency", safeName(caseID)+"--"+safeName(requestID)+".json")
}

func (s *Store) manifestPath(caseID string) string {
	return filepath.Join(s.root, "manifests", safeName(caseID)+".json")
}

func safeName(value string) string {
	result := make([]byte, 0, len(value))
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			result = append(result, c)
		}
	}
	if len(result) == 0 {
		return "invalid"
	}
	return string(result)
}
