package application

import (
	"errors"
	"os"

	"oralhistory/internal/domain"
)

func (s *Service) CreateCase(cmd CreateCaseCommand) (*domain.OralHistoryCase, error) {
	if err := s.validateMeta(cmd.CommandMeta); err != nil {
		return nil, err
	}
	if cmd.ActorID != cmd.ArchivistID {
		return nil, domain.NewError(domain.ErrForbidden, "建档操作人必须是指定档案员")
	}
	unlock := s.locks.lock(cmd.CaseID)
	defer unlock()
	digest, err := fingerprint(cmd)
	if err != nil {
		return nil, err
	}
	if record, err := s.store.LookupIdempotency(cmd.CaseID, cmd.RequestID, digest); err != nil {
		return nil, err
	} else if record != nil {
		return decodeCommandResult(record.Response)
	}
	if _, err := s.store.LoadCase(cmd.CaseID); err == nil {
		return nil, domain.NewError(domain.ErrInvalidInput, "case_id 已存在")
	} else if domain.ErrorCode(err) != domain.ErrNotFound && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	now := s.now().UTC()
	c := &domain.OralHistoryCase{
		CaseID: cmd.CaseID, Title: cmd.Title, CollectionDate: cmd.CollectionDate,
		CustodyReference: cmd.CustodyReference, SourceAudioURI: cmd.SourceAudioURI,
		SourceSHA256: cmd.SourceSHA256, ConsentDocumentSHA256: cmd.ConsentDocumentSHA256,
		ArchivistID: cmd.ArchivistID, ReviewerID: cmd.ReviewerID,
		Status: domain.StatusDraft, CreatedAt: now, UpdatedAt: now,
		Segments: []domain.TranscriptSegment{}, Constraints: []domain.ConsentConstraint{},
		Conflicts: []domain.DisclosureConflict{}, Reviews: []domain.ReviewDecision{},
	}
	if err := domain.ValidateNewCase(c); err != nil {
		return nil, err
	}
	return s.commitMutation(mutation{Meta: cmd.CommandMeta, Type: "case_created", Input: cmd, Case: c, Payload: map[string]any{"title": c.Title}})
}
