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
	// 幂等检查和不可变清单存在性检查必须在事件落盘之前完成，
	// 否则失败请求的事件会残留并导致快照修订与事件数不一致。
	var manifestContent []byte
	if change.Manifest != nil {
		manifestContent, err = json.MarshalIndent(change.Manifest, "", "  ")
		if err != nil {
			return err
		}
		if _, statErr := os.Stat(s.manifestPath(change.Case.CaseID)); statErr == nil {
			return domain.NewError(domain.ErrInvalidState, "不可变公开清单已经存在")
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
	}
	// 保存调用前状态，任何后续写入失败都要回滚，使快照修订、事件数量、
	// 公开清单和幂等响应保持调用前状态。
	eventPath := s.eventPath(change.Case.CaseID)
	casePath := s.casePath(change.Case.CaseID)
	manifestPath := s.manifestPath(change.Case.CaseID)
	idempotencyPath := s.idempotencyPath(change.Case.CaseID, change.Idempotency.RequestID)
	preEventStat, _ := os.Stat(eventPath)
	preCaseStat, _ := os.Stat(casePath)
	preCaseContent := []byte(nil)
	if preCaseStat != nil {
		if existing, readErr := os.ReadFile(casePath); readErr == nil {
			preCaseContent = existing
		}
	}
	rollback := func(writtenManifest, writtenCase bool) {
		// 回滚事件帧至调用前状态。
		if preEventStat == nil {
			_ = os.Remove(eventPath)
		} else {
			_ = os.Truncate(eventPath, preEventStat.Size())
		}
		// 回滚快照至调用前状态。
		if writtenCase {
			if preCaseStat == nil {
				_ = os.Remove(casePath)
			} else {
				_ = atomicWrite(casePath, preCaseContent, 0o640)
			}
		}
		// 回滚公开清单。
		if writtenManifest {
			_ = os.Remove(manifestPath)
		}
		// 幂等响应若已写入则删除，避免残留导致重复请求被误判为幂等冲突。
		if _, statErr := os.Stat(idempotencyPath); statErr == nil {
			_ = os.Remove(idempotencyPath)
		}
	}
	// 事件先落盘，恢复时能明确识别快照落后；快照和响应均使用原子替换。
	eventFile, err := os.OpenFile(eventPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	if _, err = eventFile.Write(eventContent); err == nil {
		err = eventFile.Sync()
	}
	closeErr := eventFile.Close()
	if err != nil {
		rollback(false, false)
		return err
	}
	if closeErr != nil {
		rollback(false, false)
		return closeErr
	}
	if change.Manifest != nil {
		if err := atomicWrite(manifestPath, append(manifestContent, '\n'), 0o440); err != nil {
			rollback(false, false)
			return err
		}
	}
	if err := atomicWrite(casePath, caseContent, 0o640); err != nil {
		rollback(change.Manifest != nil, false)
		return err
	}
	if err := atomicWrite(idempotencyPath, idempotencyContent, 0o640); err != nil {
		// 幂等响应未写入，回滚事件帧、快照和公开清单，使状态保持调用前状态。
		rollback(change.Manifest != nil, true)
		return err
	}
	return nil
}
