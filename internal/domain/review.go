package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

type CandidateChange struct {
	SegmentID string `json:"segment_id"`
	Kind      string `json:"kind"`
	Anomaly   bool   `json:"anomaly"`
}

type ReviewRoundDifference struct {
	FromRevision int64             `json:"from_revision,omitempty"`
	ToRevision   int64             `json:"to_revision,omitempty"`
	Changes      []CandidateChange `json:"changes"`
	HasAnomaly   bool              `json:"has_anomaly"`
}

func NewReturnTask(round int, reason ReturnReason) ReturnTask {
	sum := sha256.Sum256([]byte(reason.SegmentID + "\x00" + reason.Code + "\x00" + fmt.Sprintf("%d", round)))
	return ReturnTask{TaskID: "task-" + hex.EncodeToString(sum[:6]), Round: round, SegmentID: reason.SegmentID, Code: reason.Code, Comment: reason.Comment, Status: "to_remediate"}
}

func OpenTaskSegmentIDs(tasks []ReturnTask) []string {
	seen := map[string]bool{}
	for _, task := range tasks {
		if task.Status == "to_remediate" || task.Status == "pending_review" {
			seen[task.SegmentID] = true
		}
	}
	result := make([]string, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}
