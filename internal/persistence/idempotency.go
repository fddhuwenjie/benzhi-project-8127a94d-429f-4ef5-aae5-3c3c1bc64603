package persistence

import (
	"encoding/json"
	"errors"
	"os"
	"time"

	"oralhistory/internal/domain"
)

type IdempotencyRecord struct {
	CaseID      string          `json:"case_id"`
	RequestID   string          `json:"request_id"`
	Fingerprint string          `json:"fingerprint"`
	StatusCode  int             `json:"status_code"`
	Response    json.RawMessage `json:"response"`
	CreatedAt   time.Time       `json:"created_at"`
}

func (s *Store) LookupIdempotency(caseID, requestID, fingerprint string) (*IdempotencyRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lookupIdempotencyUnlocked(caseID, requestID, fingerprint)
}

func (s *Store) lookupIdempotencyUnlocked(caseID, requestID, fingerprint string) (*IdempotencyRecord, error) {
	content, err := os.ReadFile(s.idempotencyPath(caseID, requestID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var record IdempotencyRecord
	if err := json.Unmarshal(content, &record); err != nil {
		return nil, domain.WrapError(domain.ErrIntegrity, "幂等记录损坏: %v", err)
	}
	if record.Fingerprint != fingerprint {
		return nil, domain.NewError(domain.ErrIdempotencyConflict, "同一 request_id 对应不同请求载荷")
	}
	return &record, nil
}

func marshalIdempotency(record *IdempotencyRecord) ([]byte, error) {
	content, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}
