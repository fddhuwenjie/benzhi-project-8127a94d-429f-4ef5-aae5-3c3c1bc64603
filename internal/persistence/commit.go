package persistence

import (
	"encoding/json"
	"errors"
	"os"
	"time"

	"oralhistory/internal/domain"
)

type Commit struct {
	Case             *domain.OralHistoryCase
	ExpectedRevision int64
	Event            domain.AuditEvent
	Idempotency      *IdempotencyRecord
	Manifest         *domain.ReleaseManifest
}

func (s *Store) Commit(change Commit) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if change.Case == nil || change.Idempotency == nil {
		return errors.New("提交缺少案件或幂等响应")
	}
	current, err := s.loadCaseUnlocked(change.Case.CaseID)
	if err != nil && domain.ErrorCode(err) != domain.ErrNotFound {
		return err
	}
	if current == nil {
		if change.ExpectedRevision != 0 {
			return domain.NewError(domain.ErrRevisionConflict, "新案件 expected_revision 必须为 0")
		}
	} else if current.Revision != change.ExpectedRevision {
		return domain.WrapError(domain.ErrRevisionConflict, "修订冲突: 当前为 %d，提交期望 %d", current.Revision, change.ExpectedRevision)
	}
	if existing, err := s.lookupIdempotencyUnlocked(change.Case.CaseID, change.Idempotency.RequestID, change.Idempotency.Fingerprint); err != nil {
		return err
	} else if existing != nil {
		return nil
	}
	events, err := s.loadEventsUnlocked(change.Case.CaseID)
	if err != nil {
		return err
	}
	change.Event.Sequence = int64(len(events) + 1)
	if len(events) > 0 {
		change.Event.PreviousSHA256 = events[len(events)-1].EventSHA256
	}
	digest, err := eventDigest(change.Event)
	if err != nil {
		return err
	}
	change.Event.EventSHA256 = digest
	change.Case.Revision = change.ExpectedRevision + 1
	change.Case.UpdatedAt = time.Now().UTC()
	caseContent, err := marshalCase(change.Case)
	if err != nil {
		return err
	}
	eventContent, err := marshalEvent(change.Event)
	if err != nil {
		return err
	}
	idempotencyContent, err := marshalIdempotency(change.Idempotency)
	if err != nil {
		return err
	}
	// 事件先落盘，恢复时能明确识别快照落后；快照和响应均使用原子替换。
	eventFile, err := os.OpenFile(s.eventPath(change.Case.CaseID), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	if _, err = eventFile.Write(eventContent); err == nil {
		err = eventFile.Sync()
	}
	closeErr := eventFile.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if change.Manifest != nil {
		manifestContent, err := json.MarshalIndent(change.Manifest, "", "  ")
		if err != nil {
			return err
		}
		if _, statErr := os.Stat(s.manifestPath(change.Case.CaseID)); statErr == nil {
			return domain.NewError(domain.ErrInvalidState, "不可变公开清单已经存在")
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
		if err := atomicWrite(s.manifestPath(change.Case.CaseID), append(manifestContent, '\n'), 0o440); err != nil {
			return err
		}
	}
	if err := atomicWrite(s.casePath(change.Case.CaseID), caseContent, 0o640); err != nil {
		return err
	}
	if err := atomicWrite(s.idempotencyPath(change.Case.CaseID, change.Idempotency.RequestID), idempotencyContent, 0o640); err != nil {
		return err
	}
	return nil
}
