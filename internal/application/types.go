package application

import (
	"encoding/json"
	"time"

	"oralhistory/internal/domain"
	"oralhistory/internal/evidence"
)

type CommandMeta struct {
	CaseID           string `json:"case_id"`
	ActorID          string `json:"actor_id"`
	RequestID        string `json:"request_id"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type CreateCaseCommand struct {
	CommandMeta
	Title                 string `json:"title"`
	CollectionDate        string `json:"collection_date"`
	CustodyReference      string `json:"custody_reference"`
	SourceAudioURI        string `json:"source_audio_uri"`
	SourceSHA256          string `json:"source_sha256"`
	ConsentDocumentSHA256 string `json:"consent_document_sha256"`
	ArchivistID           string `json:"archivist_id"`
	ReviewerID            string `json:"reviewer_id"`
}

type AddSegmentCommand struct {
	CommandMeta
	Segment domain.TranscriptSegment `json:"segment"`
}

type AddConstraintCommand struct {
	CommandMeta
	Constraint domain.ConsentConstraint `json:"constraint"`
}

type BatchRegistrationCommand struct {
	CommandMeta
	Preview     bool                       `json:"preview"`
	Segments    []domain.TranscriptSegment `json:"segments"`
	Constraints []domain.ConsentConstraint `json:"constraints"`
}

type BatchRegistrationResult struct {
	Case        *domain.OralHistoryCase    `json:"case,omitempty"`
	Preview     bool                       `json:"preview"`
	Valid       bool                       `json:"valid"`
	Issues      []domain.BatchIssue        `json:"issues"`
	Segments    []domain.TranscriptSegment `json:"normalized_segments"`
	Constraints []domain.ConsentConstraint `json:"normalized_constraints"`
	Summary     domain.BatchSummary        `json:"summary"`
}

type RemediateCommand struct {
	CommandMeta
	Preview      bool               `json:"preview,omitempty"`
	SegmentID    string             `json:"segment_id"`
	Disposition  domain.Disposition `json:"disposition"`
	PublicText   string             `json:"public_text,omitempty"`
	MuteRanges   []domain.TimeRange `json:"mute_ranges,omitempty"`
	EvidenceRefs []string           `json:"evidence_refs"`
	Reason       string             `json:"reason"`
}

type RemediationPreviewResult struct {
	Revision int64                     `json:"revision"`
	Preview  domain.RemediationPreview `json:"preview"`
}

type ReviewCommand struct {
	CommandMeta
	ItemResults   []domain.ReviewItemResult `json:"item_results"`
	Decision      string                    `json:"decision"`
	ReturnReasons []domain.ReturnReason     `json:"return_reasons,omitempty"`
}

type CaseSummary struct {
	CaseID    string            `json:"case_id"`
	Title     string            `json:"title"`
	Status    domain.CaseStatus `json:"status"`
	Revision  int64             `json:"revision"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type Workbench struct {
	Case             *domain.OralHistoryCase       `json:"case"`
	AllowedActions   []string                      `json:"allowed_actions"`
	Timeline         []evidence.TimelineItem       `json:"timeline"`
	Verification     *evidence.Verification        `json:"verification,omitempty"`
	Coverage         domain.CoverageMatrix         `json:"coverage"`
	LatestCheck      *domain.ConflictCheck         `json:"latest_check,omitempty"`
	CheckStale       bool                          `json:"check_stale"`
	ReviewDifference domain.ReviewRoundDifference  `json:"review_difference"`
	ManifestQuery    *evidence.ManifestQueryResult `json:"manifest_query,omitempty"`
}

type ManifestQuery struct {
	StartMS   *int64 `json:"start_ms,omitempty"`
	EndMS     *int64 `json:"end_ms,omitempty"`
	SegmentID string `json:"segment_id,omitempty"`
	EntryType string `json:"entry_type,omitempty"`
}

type CommandResult struct {
	Case *domain.OralHistoryCase `json:"case"`
}

func decodeCommandResult(raw json.RawMessage) (*domain.OralHistoryCase, error) {
	var result CommandResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result.Case, nil
}
