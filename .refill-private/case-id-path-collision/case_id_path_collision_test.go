package caseidpathcollision_test

import (
	"testing"

	"oralhistory/internal/application"
	"oralhistory/internal/persistence"
)

func TestDistinctCaseIDsRemainIsolated(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store)

	create := func(caseID, requestID, title string) error {
		_, err := service.CreateCase(application.CreateCaseCommand{
			CommandMeta: application.CommandMeta{
				CaseID: caseID, ActorID: "archivist", RequestID: requestID, ExpectedRevision: 0,
			},
			Title: title, CollectionDate: "2026-08-01", CustodyReference: "custody-1",
			SourceAudioURI: "archive://audio", SourceSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ConsentDocumentSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			ArchivistID: "archivist", ReviewerID: "reviewer",
		})
		return err
	}

	if err := create("case/a", "create-one", "案件一"); err != nil {
		t.Fatalf("创建第一个案件失败: %v", err)
	}
	if err := create("casea", "create-two", "案件二"); err != nil {
		t.Fatalf("不同 case_id 不应共享持久化路径: %v", err)
	}

	first, err := store.LoadCase("case/a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.LoadCase("casea")
	if err != nil {
		t.Fatal(err)
	}
	if first.CaseID != "case/a" || second.CaseID != "casea" {
		t.Fatalf("案件读取发生串用: first=%q second=%q", first.CaseID, second.CaseID)
	}
}
