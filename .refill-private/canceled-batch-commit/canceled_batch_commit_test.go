package canceledbatchcommit_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"oralhistory/internal/application"
	"oralhistory/internal/domain"
	"oralhistory/internal/persistence"
)

type checkpointContext struct {
	context.Context
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *checkpointContext) Err() error {
	err := c.Context.Err()
	c.once.Do(func() {
		close(c.entered)
		<-c.release
	})
	return err
}

func TestCanceledBatchDoesNotCommit(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store)
	created, err := service.CreateCase(application.CreateCaseCommand{
		CommandMeta: application.CommandMeta{CaseID: "case-canceled-batch", ActorID: "archivist", RequestID: "create", ExpectedRevision: 0},
		Title:       "取消边界测试", CollectionDate: "2026-08-27", CustodyReference: "CANCEL-1", SourceAudioURI: "archive://cancel",
		SourceSHA256:          "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ConsentDocumentSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ArchivistID:           "archivist", ReviewerID: "reviewer",
	})
	if err != nil {
		t.Fatal(err)
	}
	created, err = service.FreezeBaseline(application.CommandMeta{CaseID: created.CaseID, ActorID: "archivist", RequestID: "freeze", ExpectedRevision: created.Revision})
	if err != nil {
		t.Fatal(err)
	}

	parent, cancel := context.WithCancel(context.Background())
	ctx := &checkpointContext{Context: parent, entered: make(chan struct{}), release: make(chan struct{})}
	command := application.BatchRegistrationCommand{
		CommandMeta: application.CommandMeta{CaseID: created.CaseID, ActorID: "archivist", RequestID: "batch-after-cancel", ExpectedRevision: created.Revision},
		Segments:    []domain.TranscriptSegment{{SegmentID: "segment-1", StartMS: 0, EndMS: 1000, SourceText: "不应提交的片段", SubjectIDs: []string{"person-1"}}},
		Constraints: []domain.ConsentConstraint{{ConstraintID: "allow-1", ScopeType: "subject", ScopeValue: "person-1", Policy: domain.PolicyAllow, EvidenceReference: "consent#1"}},
	}
	type outcome struct {
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		_, callErr := service.RegisterBatchContext(ctx, command)
		done <- outcome{err: callErr}
	}()

	<-ctx.entered
	cancel()
	close(ctx.release)
	result := <-done
	loaded, err := store.LoadCase(created.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(result.err, context.Canceled) || loaded.Revision != created.Revision || len(loaded.Segments) != 0 || len(loaded.Constraints) != 0 {
		t.Fatalf("TestCanceledBatchDoesNotCommit: 已取消请求返回 err=%v，并污染持久状态 revision=%d segments=%d constraints=%d", result.err, loaded.Revision, len(loaded.Segments), len(loaded.Constraints))
	}
}

var _ context.Context = (*checkpointContext)(nil)
