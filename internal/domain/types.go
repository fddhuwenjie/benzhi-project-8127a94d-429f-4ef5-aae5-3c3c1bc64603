package domain

import "time"

type CaseStatus string

const (
	StatusDraft       CaseStatus = "draft"
	StatusFrozen      CaseStatus = "baseline_frozen"
	StatusRemediation CaseStatus = "remediation_required"
	StatusCandidate   CaseStatus = "candidate_ready"
	StatusReview      CaseStatus = "under_review"
	StatusReturned    CaseStatus = "returned"
	StatusSealed      CaseStatus = "sealed"
)

type Policy string

const (
	PolicyAllow     Policy = "allow"
	PolicyDelay     Policy = "delay"
	PolicyAnonymous Policy = "anonymous"
	PolicyDeny      Policy = "deny"
)

type Disposition string

const (
	DispositionOriginal Disposition = "original"
	DispositionReplace  Disposition = "replace"
	DispositionMute     Disposition = "mute"
	DispositionExclude  Disposition = "exclude"
)

type OralHistoryCase struct {
	CaseID                string               `json:"case_id"`
	Title                 string               `json:"title"`
	CollectionDate        string               `json:"collection_date"`
	CustodyReference      string               `json:"custody_reference"`
	SourceAudioURI        string               `json:"source_audio_uri"`
	SourceSHA256          string               `json:"source_sha256"`
	ConsentDocumentSHA256 string               `json:"consent_document_sha256"`
	ConsentBaselineSHA256 string               `json:"consent_baseline_sha256,omitempty"`
	ArchivistID           string               `json:"archivist_id"`
	ReviewerID            string               `json:"reviewer_id"`
	Status                CaseStatus           `json:"status"`
	Revision              int64                `json:"revision"`
	CandidateRevision     int64                `json:"candidate_revision,omitempty"`
	ReviewRound           int                  `json:"review_round,omitempty"`
	CreatedAt             time.Time            `json:"created_at"`
	UpdatedAt             time.Time            `json:"updated_at"`
	SealedAt              *time.Time           `json:"sealed_at,omitempty"`
	Segments              []TranscriptSegment  `json:"segments"`
	Constraints           []ConsentConstraint  `json:"constraints"`
	Conflicts             []DisclosureConflict `json:"conflicts"`
	Candidate             *CandidateRelease    `json:"candidate,omitempty"`
	Reviews               []ReviewDecision     `json:"reviews"`
	OpenReturnSegmentIDs  []string             `json:"open_return_segment_ids,omitempty"`
	Manifest              *ReleaseManifest     `json:"manifest,omitempty"`
	ConflictChecks        []ConflictCheck      `json:"conflict_checks,omitempty"`
	ReturnTasks           []ReturnTask         `json:"return_tasks,omitempty"`
	CandidateHistory      []CandidateRelease   `json:"candidate_history,omitempty"`
}

type TranscriptSegment struct {
	SegmentID    string      `json:"segment_id"`
	CaseID       string      `json:"case_id"`
	StartMS      int64       `json:"start_ms"`
	EndMS        int64       `json:"end_ms"`
	SourceText   string      `json:"source_text"`
	SubjectIDs   []string    `json:"subject_ids"`
	TopicCodes   []string    `json:"topic_codes"`
	Disposition  Disposition `json:"disposition"`
	PublicText   string      `json:"public_text,omitempty"`
	MuteRanges   []TimeRange `json:"mute_ranges,omitempty"`
	EvidenceRefs []string    `json:"evidence_refs,omitempty"`
	Reason       string      `json:"reason,omitempty"`
	Revision     int64       `json:"revision"`
}

type TimeRange struct {
	StartMS int64 `json:"start_ms"`
	EndMS   int64 `json:"end_ms"`
}

type ConsentConstraint struct {
	ConstraintID      string `json:"constraint_id"`
	CaseID            string `json:"case_id"`
	ScopeType         string `json:"scope_type"`
	ScopeValue        string `json:"scope_value"`
	Policy            Policy `json:"policy"`
	NotBefore         string `json:"not_before,omitempty"`
	RequiredAlias     string `json:"required_alias,omitempty"`
	EvidenceReference string `json:"evidence_reference"`
	FrozenRevision    int64  `json:"frozen_revision"`
}

type DisclosureConflict struct {
	ConflictID   string `json:"conflict_id"`
	SegmentID    string `json:"segment_id"`
	ConstraintID string `json:"constraint_id,omitempty"`
	Severity     string `json:"severity"`
	Code         string `json:"code"`
	Message      string `json:"message"`
	Resolved     bool   `json:"resolved"`
}

type ConflictDelta struct {
	New       []DisclosureConflict `json:"new"`
	Resolved  []DisclosureConflict `json:"resolved"`
	Reopened  []DisclosureConflict `json:"reopened"`
	Unchanged []DisclosureConflict `json:"unchanged"`
}

type ConflictCheck struct {
	BasedOnRevision  int64                `json:"based_on_revision"`
	ComparedRevision int64                `json:"compared_revision,omitempty"`
	CheckDate        string               `json:"check_date"`
	InputSHA256      string               `json:"input_sha256"`
	ResultSHA256     string               `json:"result_sha256"`
	BlockingCount    int                  `json:"blocking_count"`
	NoticeCount      int                  `json:"notice_count"`
	Conflicts        []DisclosureConflict `json:"conflicts"`
	Delta            ConflictDelta        `json:"delta"`
}

type ReturnTask struct {
	TaskID    string `json:"task_id"`
	Round     int    `json:"round"`
	SegmentID string `json:"segment_id"`
	Code      string `json:"code"`
	Comment   string `json:"comment"`
	Status    string `json:"status"`
}

type CandidateRelease struct {
	Revision          int64              `json:"revision"`
	GeneratedAt       time.Time          `json:"generated_at"`
	PublicSegments    []PublicSegment    `json:"public_segments"`
	AudioInstructions []AudioInstruction `json:"audio_instructions"`
	Diffs             []SegmentDiff      `json:"diffs"`
	ContentSHA256     string             `json:"content_sha256"`
}

type PublicSegment struct {
	SegmentID string `json:"segment_id"`
	StartMS   int64  `json:"start_ms"`
	EndMS     int64  `json:"end_ms"`
	Text      string `json:"text"`
}

type AudioInstruction struct {
	SegmentID string `json:"segment_id"`
	Action    string `json:"action"`
	StartMS   int64  `json:"start_ms"`
	EndMS     int64  `json:"end_ms"`
	Reason    string `json:"reason"`
}

type SegmentDiff struct {
	SegmentID  string `json:"segment_id"`
	SourceText string `json:"source_text"`
	PublicText string `json:"public_text"`
	Changed    bool   `json:"changed"`
	Action     string `json:"action"`
}

type ReviewItemResult struct {
	SegmentID      string `json:"segment_id"`
	ConsentValid   bool   `json:"consent_valid"`
	RedactionValid bool   `json:"redaction_valid"`
	Comment        string `json:"comment,omitempty"`
}

type ReturnReason struct {
	SegmentID string `json:"segment_id"`
	Code      string `json:"code"`
	Comment   string `json:"comment"`
}

type ReviewDecision struct {
	ReviewID          string             `json:"review_id"`
	CaseID            string             `json:"case_id"`
	CandidateRevision int64              `json:"candidate_revision"`
	ReviewerID        string             `json:"reviewer_id"`
	ItemResults       []ReviewItemResult `json:"item_results"`
	Decision          string             `json:"decision"`
	ReturnReasons     []ReturnReason     `json:"return_reasons,omitempty"`
	SubmittedAt       time.Time          `json:"submitted_at"`
	DecidedAt         time.Time          `json:"decided_at"`
}

type ReleaseManifest struct {
	ManifestID            string             `json:"manifest_id"`
	CaseID                string             `json:"case_id"`
	SourceSHA256          string             `json:"source_sha256"`
	ConsentBaselineSHA256 string             `json:"consent_baseline_sha256"`
	PublicSegments        []PublicSegment    `json:"public_segments"`
	ExcludedRanges        []TimeRange        `json:"excluded_ranges"`
	AudioInstructions     []AudioInstruction `json:"audio_instructions"`
	ReviewDecisionSHA256  string             `json:"review_decision_sha256"`
	AuditRootSHA256       string             `json:"audit_root_sha256"`
	ManifestSHA256        string             `json:"manifest_sha256"`
	SealedAt              time.Time          `json:"sealed_at"`
}

type AuditEvent struct {
	Sequence       int64     `json:"sequence"`
	CaseID         string    `json:"case_id"`
	Type           string    `json:"type"`
	ActorID        string    `json:"actor_id"`
	Revision       int64     `json:"revision"`
	RequestID      string    `json:"request_id"`
	RequestSHA256  string    `json:"request_sha256"`
	PreviousSHA256 string    `json:"previous_sha256"`
	Payload        any       `json:"payload,omitempty"`
	OccurredAt     time.Time `json:"occurred_at"`
	EventSHA256    string    `json:"event_sha256"`
}
