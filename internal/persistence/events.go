package persistence

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"oralhistory/internal/domain"
)

func (s *Store) LoadEvents(caseID string) ([]domain.AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loadEventsUnlocked(caseID)
}

func (s *Store) loadEventsUnlocked(caseID string) ([]domain.AuditEvent, error) {
	file, err := os.Open(s.eventPath(caseID))
	if errors.Is(err, os.ErrNotExist) {
		return []domain.AuditEvent{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	var events []domain.AuditEvent
	var previous string
	for line := 1; ; line++ {
		raw, readErr := reader.ReadBytes('\n')
		if len(raw) > 0 {
			if raw[len(raw)-1] != '\n' {
				return nil, domain.WrapError(domain.ErrIntegrity, "事件日志尾帧在第 %d 行被截断", line)
			}
			var event domain.AuditEvent
			if err := json.Unmarshal(bytes.TrimSpace(raw), &event); err != nil {
				return nil, domain.WrapError(domain.ErrIntegrity, "事件日志第 %d 帧无法解析", line)
			}
			if event.Sequence != int64(line) {
				return nil, domain.WrapError(domain.ErrIntegrity, "事件序号不连续: 期望 %d，实际 %d", line, event.Sequence)
			}
			if event.PreviousSHA256 != previous {
				return nil, domain.WrapError(domain.ErrIntegrity, "事件第 %d 帧前序摘要不匹配", line)
			}
			actual, err := eventDigest(event)
			if err != nil || actual != event.EventSHA256 {
				return nil, domain.WrapError(domain.ErrIntegrity, "事件第 %d 帧摘要不匹配: 封存 %s，实际 %s", line, event.EventSHA256, actual)
			}
			previous = event.EventSHA256
			events = append(events, event)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	return events, nil
}

func eventDigest(event domain.AuditEvent) (string, error) {
	event.EventSHA256 = ""
	// Payload 可以由结构体或读回后的通用 JSON 值承载，只规范化该字段，
	// 同时保持既有事件顶层字段的摘要顺序兼容性。
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return "", err
	}
	var canonical any
	if err := json.Unmarshal(payload, &canonical); err != nil {
		return "", err
	}
	event.Payload = canonical
	content, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}

func marshalEvent(event domain.AuditEvent) ([]byte, error) {
	content, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func (s *Store) VerifyEventLog(caseID string) (string, error) {
	events, err := s.LoadEvents(caseID)
	if err != nil {
		return "", err
	}
	if len(events) == 0 {
		return "", nil
	}
	return events[len(events)-1].EventSHA256, nil
}

func describeEventError(caseID string, err error) error {
	return fmt.Errorf("案件 %s 的事件链校验失败: %w", caseID, err)
}
