package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"oralhistory/internal/application"
	"oralhistory/internal/domain"
	"oralhistory/internal/evidence"
)

type selfCheckClient struct {
	baseURL  string
	client   *http.Client
	caseID   string
	revision int64
}

func runSelfCheck(cfg config) error {
	tempDir, err := os.MkdirTemp("", "oralhistory-self-check-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	cfg.DataDir = filepath.Join(tempDir, "data")
	rt, err := assemble(cfg)
	if err != nil {
		return err
	}
	serveResult := rt.serve()
	client := &selfCheckClient{
		baseURL: "http://" + rt.listener.Addr().String(),
		client:  &http.Client{Timeout: 4 * time.Second},
		caseID:  "self-check-case",
	}
	flowErr := client.runFlow()
	shutdownErr := rt.shutdown()
	serveErr := <-serveResult
	if flowErr != nil {
		return flowErr
	}
	if shutdownErr != nil {
		return shutdownErr
	}
	if serveErr != nil {
		return serveErr
	}
	if rt.app.ActiveCaseLocks() != 0 {
		return fmt.Errorf("自检结束后案件锁未回收")
	}
	result, _ := json.Marshal(map[string]any{"status": "ok", "message": "完整业务流程及封存摘要验证通过", "addr": cfg.Addr})
	fmt.Println(string(result))
	return nil
}

func (c *selfCheckClient) runFlow() error {
	const source = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const consent = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	create := application.CreateCaseCommand{
		CommandMeta: application.CommandMeta{CaseID: c.caseID, ActorID: "archivist-a", RequestID: "req-create", ExpectedRevision: 0},
		Title:       "自检口述史", CollectionDate: "2026-08-01", CustodyReference: "CUST-SC-001",
		SourceAudioURI: "archive://self-check/source.wav", SourceSHA256: source,
		ConsentDocumentSHA256: consent, ArchivistID: "archivist-a", ReviewerID: "reviewer-b",
	}
	if err := c.command("/api/cases", create); err != nil {
		return err
	}
	if err := c.metaCommand("freeze", "archivist-a", "req-freeze"); err != nil {
		return err
	}
	segment := domain.TranscriptSegment{SegmentID: "seg-001", StartMS: 0, EndMS: 12000, SourceText: "张老师在旧址讲述了家庭迁徙经过。", SubjectIDs: []string{"person-zhang"}, TopicCodes: []string{"migration"}, Disposition: domain.DispositionOriginal}
	if err := c.command(c.path("segments"), application.AddSegmentCommand{CommandMeta: c.meta("archivist-a", "req-segment"), Segment: segment}); err != nil {
		return err
	}
	constraint := domain.ConsentConstraint{ConstraintID: "cons-001", ScopeType: "subject", ScopeValue: "person-zhang", Policy: domain.PolicyAnonymous, RequiredAlias: "受访者甲", EvidenceReference: "consent.pdf#clause-2"}
	if err := c.command(c.path("constraints"), application.AddConstraintCommand{CommandMeta: c.meta("archivist-a", "req-constraint"), Constraint: constraint}); err != nil {
		return err
	}
	if err := c.metaCommand("check-conflicts", "archivist-a", "req-check-1"); err != nil {
		return err
	}
	first := application.RemediateCommand{CommandMeta: c.meta("archivist-a", "req-remediate-1"), SegmentID: "seg-001", Disposition: domain.DispositionReplace, PublicText: "受访者甲在旧址讲述了家庭迁徙经过。", EvidenceRefs: []string{"consent.pdf#clause-2", "edit-note#1"}, Reason: "依授权要求使用固定别名"}
	if err := c.command(c.path("remediate"), first); err != nil {
		return err
	}
	if err := c.metaCommand("check-conflicts", "archivist-a", "req-check-after-remediation-1"); err != nil {
		return err
	}
	if err := c.metaCommand("candidate", "archivist-a", "req-candidate-1"); err != nil {
		return err
	}
	if err := c.metaCommand("submit-review", "archivist-a", "req-submit-1"); err != nil {
		return err
	}
	returned := application.ReviewCommand{CommandMeta: c.meta("reviewer-b", "req-return"), ItemResults: []domain.ReviewItemResult{{SegmentID: "seg-001", ConsentValid: true, RedactionValid: false, Comment: "别名上下文仍需收紧"}}, Decision: "returned", ReturnReasons: []domain.ReturnReason{{SegmentID: "seg-001", Code: "context_exposure", Comment: "删除可推断住址的旧址描述"}}}
	if err := c.command(c.path("review"), returned); err != nil {
		return err
	}
	second := application.RemediateCommand{CommandMeta: c.meta("archivist-a", "req-remediate-2"), SegmentID: "seg-001", Disposition: domain.DispositionReplace, PublicText: "受访者甲讲述了家庭迁徙经过。", EvidenceRefs: []string{"consent.pdf#clause-2", "review-1", "edit-note#2"}, Reason: "按退回意见消除地点推断线索"}
	if err := c.command(c.path("remediate"), second); err != nil {
		return err
	}
	if err := c.metaCommand("check-conflicts", "archivist-a", "req-check-2"); err != nil {
		return err
	}
	if err := c.metaCommand("candidate", "archivist-a", "req-candidate-2"); err != nil {
		return err
	}
	if err := c.metaCommand("submit-review", "archivist-a", "req-submit-2"); err != nil {
		return err
	}
	approved := application.ReviewCommand{CommandMeta: c.meta("reviewer-b", "req-approve"), ItemResults: []domain.ReviewItemResult{{SegmentID: "seg-001", ConsentValid: true, RedactionValid: true}}, Decision: "approved", ReturnReasons: []domain.ReturnReason{}}
	if err := c.command(c.path("review"), approved); err != nil {
		return err
	}
	var verification evidence.Verification
	if err := c.get(c.path("verify"), &verification); err == nil {
		return fmt.Errorf("verify 路由应只接受 POST")
	}
	if err := c.postRaw(c.path("verify"), map[string]any{}, &verification); err != nil {
		return err
	}
	if !verification.Valid {
		return fmt.Errorf("封存摘要验证失败: %s", verification.Message)
	}
	return nil
}

func (c *selfCheckClient) path(action string) string { return "/api/cases/" + c.caseID + "/" + action }
func (c *selfCheckClient) meta(actor, request string) application.CommandMeta {
	return application.CommandMeta{CaseID: c.caseID, ActorID: actor, RequestID: request, ExpectedRevision: c.revision}
}
func (c *selfCheckClient) metaCommand(action, actor, request string) error {
	return c.command(c.path(action), c.meta(actor, request))
}

func (c *selfCheckClient) command(path string, input any) error {
	var result application.CommandResult
	if err := c.postRaw(path, input, &result); err != nil {
		return err
	}
	if result.Case == nil {
		return fmt.Errorf("%s 未返回案件", path)
	}
	c.revision = result.Case.Revision
	return nil
}

func (c *selfCheckClient) postRaw(path string, input, output any) error {
	content, err := json.Marshal(input)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewReader(content))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	return c.do(request, output)
}

func (c *selfCheckClient) get(path string, output any) error {
	request, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(request, output)
}

func (c *selfCheckClient) do(request *http.Request, output any) error {
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%s %s 返回 %d: %s", request.Method, request.URL.Path, response.StatusCode, string(content))
	}
	if err := json.Unmarshal(content, output); err != nil {
		return fmt.Errorf("解析 %s 响应失败: %w", request.URL.Path, err)
	}
	return nil
}
