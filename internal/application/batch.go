package application

import (
	"context"
	"encoding/json"

	"oralhistory/internal/domain"
	"oralhistory/internal/evidence"
	"oralhistory/internal/persistence"
)

func (s *Service) RegisterBatch(cmd BatchRegistrationCommand) (BatchRegistrationResult, error) {
	return s.RegisterBatchContext(context.Background(), cmd)
}

func (s *Service) RegisterBatchContext(ctx context.Context, cmd BatchRegistrationCommand) (BatchRegistrationResult, error) {
	if err := ctx.Err(); err != nil {
		return BatchRegistrationResult{}, err
	}
	if err := s.validateMeta(cmd.CommandMeta); err != nil {
		return BatchRegistrationResult{}, err
	}
	unlock := s.locks.lock(cmd.CaseID)
	defer unlock()
	digest, err := fingerprint(cmd)
	if err != nil {
		return BatchRegistrationResult{}, err
	}
	if record, err := s.store.LookupIdempotency(cmd.CaseID, cmd.RequestID, digest); err != nil {
		return BatchRegistrationResult{}, err
	} else if record != nil {
		var result BatchRegistrationResult
		if err := json.Unmarshal(record.Response, &result); err != nil {
			return BatchRegistrationResult{}, err
		}
		return result, nil
	}
	c, err := s.store.LoadCase(cmd.CaseID)
	if err != nil {
		return BatchRegistrationResult{}, err
	}
	if c.Revision != cmd.ExpectedRevision {
		return BatchRegistrationResult{}, domain.WrapError(domain.ErrRevisionConflict, "修订冲突: 当前为 %d，提交期望 %d", c.Revision, cmd.ExpectedRevision)
	}
	if cmd.ActorID != c.ArchivistID {
		return BatchRegistrationResult{}, domain.NewError(domain.ErrForbidden, "只有档案员可以批量登记")
	}
	if c.Status != domain.StatusFrozen && c.Status != domain.StatusRemediation {
		return BatchRegistrationResult{}, domain.NewError(domain.ErrInvalidState, "批量登记仅允许在基线已冻结或待整改状态执行")
	}
	validation := domain.ValidateRegistrationBatch(c, cmd.Segments, cmd.Constraints)
	result := BatchRegistrationResult{Preview: cmd.Preview, Valid: len(validation.Issues) == 0, Issues: validation.Issues, Segments: validation.Segments, Constraints: validation.Constraints, Summary: validation.Summary}
	if cmd.Preview || !result.Valid {
		return result, nil
	}
	working, err := cloneCase(c)
	if err != nil {
		return BatchRegistrationResult{}, err
	}
	domain.ApplyRegistrationBatch(working, validation, c.Revision+1)
	working.Candidate = nil
	working.Revision = c.Revision + 1
	result.Case = working
	response, err := json.Marshal(result)
	if err != nil {
		return BatchRegistrationResult{}, err
	}
	event := evidence.NewAuditEvent(c.CaseID, "registration_batch_saved", cmd.ActorID, cmd.RequestID, digest, c.Revision+1, result.Summary, s.now())
	err = s.store.Commit(persistence.Commit{Case: working, ExpectedRevision: c.Revision, Event: event, Idempotency: &persistence.IdempotencyRecord{CaseID: c.CaseID, RequestID: cmd.RequestID, Fingerprint: digest, StatusCode: 200, Response: response, CreatedAt: s.now().UTC()}})
	if err != nil {
		return BatchRegistrationResult{}, err
	}
	var committed BatchRegistrationResult
	if err := json.Unmarshal(response, &committed); err != nil {
		return BatchRegistrationResult{}, err
	}
	return committed, nil
}
