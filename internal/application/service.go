package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"oralhistory/internal/domain"
	"oralhistory/internal/evidence"
	"oralhistory/internal/persistence"
)

type Service struct {
	store *persistence.Store
	locks *caseLocks
	now   func() time.Time
}

func NewService(store *persistence.Store) *Service {
	return &Service{store: store, locks: newCaseLocks(), now: time.Now}
}

func (s *Service) validateMeta(meta CommandMeta) error {
	if strings.TrimSpace(meta.CaseID) == "" || strings.TrimSpace(meta.ActorID) == "" || strings.TrimSpace(meta.RequestID) == "" {
		return domain.NewError(domain.ErrInvalidInput, "case_id、actor_id 和 request_id 不能为空")
	}
	if meta.ExpectedRevision < 0 {
		return domain.NewError(domain.ErrInvalidInput, "expected_revision 不能为负数")
	}
	return nil
}

func fingerprint(value any) (string, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}

func cloneCase(c *domain.OralHistoryCase) (*domain.OralHistoryCase, error) {
	content, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var result domain.OralHistoryCase
	if err := json.Unmarshal(content, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type mutation struct {
	Meta     CommandMeta
	Type     string
	Input    any
	Payload  any
	Manifest *domain.ReleaseManifest
	Case     *domain.OralHistoryCase
}

func (s *Service) commitMutation(m mutation) (*domain.OralHistoryCase, error) {
	if err := s.validateMeta(m.Meta); err != nil {
		return nil, err
	}
	digest, err := fingerprint(m.Input)
	if err != nil {
		return nil, err
	}
	if record, err := s.store.LookupIdempotency(m.Meta.CaseID, m.Meta.RequestID, digest); err != nil {
		return nil, err
	} else if record != nil {
		return decodeCommandResult(record.Response)
	}
	m.Case.Revision = m.Meta.ExpectedRevision + 1
	resultContent, err := json.Marshal(CommandResult{Case: m.Case})
	if err != nil {
		return nil, err
	}
	event := evidence.NewAuditEvent(m.Meta.CaseID, m.Type, m.Meta.ActorID, m.Meta.RequestID, digest, m.Case.Revision, m.Payload, s.now())
	err = s.store.Commit(persistence.Commit{
		Case:             m.Case,
		ExpectedRevision: m.Meta.ExpectedRevision,
		Event:            event,
		Manifest:         m.Manifest,
		Idempotency: &persistence.IdempotencyRecord{
			CaseID: m.Meta.CaseID, RequestID: m.Meta.RequestID, Fingerprint: digest,
			StatusCode: 200, Response: resultContent, CreatedAt: s.now().UTC(),
		},
	})
	if err != nil {
		return nil, err
	}
	return m.Case, nil
}

func (s *Service) withCase(meta CommandMeta, input any, eventType string, mutate func(*domain.OralHistoryCase) (any, *domain.ReleaseManifest, error)) (*domain.OralHistoryCase, error) {
	if err := s.validateMeta(meta); err != nil {
		return nil, err
	}
	unlock := s.locks.lock(meta.CaseID)
	defer unlock()
	digest, err := fingerprint(input)
	if err != nil {
		return nil, err
	}
	if record, err := s.store.LookupIdempotency(meta.CaseID, meta.RequestID, digest); err != nil {
		return nil, err
	} else if record != nil {
		return decodeCommandResult(record.Response)
	}
	current, err := s.store.LoadCase(meta.CaseID)
	if err != nil {
		return nil, err
	}
	if current.Revision != meta.ExpectedRevision {
		return nil, domain.WrapError(domain.ErrRevisionConflict, "修订冲突: 当前为 %d，提交期望 %d", current.Revision, meta.ExpectedRevision)
	}
	working, err := cloneCase(current)
	if err != nil {
		return nil, err
	}
	payload, manifest, err := mutate(working)
	if err != nil {
		return nil, err
	}
	return s.commitMutation(mutation{Meta: meta, Type: eventType, Input: input, Payload: payload, Manifest: manifest, Case: working})
}
