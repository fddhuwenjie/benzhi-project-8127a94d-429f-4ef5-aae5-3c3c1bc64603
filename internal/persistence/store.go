package persistence

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	root string
	mu   sync.RWMutex
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

// safeName encodes an arbitrary identifier into a filesystem-safe form that is
// reversible: every distinct input maps to a distinct output, so identifiers
// that only differ in characters that would otherwise be dropped (slashes,
// spaces, etc.) no longer collapse onto the same storage key. Non-safe bytes
// are escaped as "~xx" (lowercase hex); the tilde itself is escaped as "~7e",
// guaranteeing that "~" only ever appears as an escape prefix in the output.
func safeName(value string) string {
	const hex = "0123456789abcdef"
	result := make([]byte, 0, len(value))
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			result = append(result, c)
		} else {
			result = append(result, '~', hex[c>>4], hex[c&0x0f])
		}
	}
	if len(result) == 0 {
		return "invalid"
	}
	return string(result)
}

// unescapeName reverses safeName, recovering the original identifier from an
// encoded file name. It is only applied to names produced by safeName, so a
// stray "~" not followed by two hex digits is treated as a literal tilde.
func unescapeName(value string) string {
	result := make([]byte, 0, len(value))
	for i := 0; i < len(value); i++ {
		if value[i] == '~' && i+2 < len(value) {
			hi, ok1 := unhex(value[i+1])
			lo, ok2 := unhex(value[i+2])
			if ok1 && ok2 {
				result = append(result, hi<<4|lo)
				i += 2
				continue
			}
		}
		result = append(result, value[i])
	}
	return string(result)
}

func unhex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
