package evidence

import (
	"reflect"
	"sort"

	"oralhistory/internal/domain"
)

func CompareCandidates(previous, current *domain.CandidateRelease, allowedSegmentIDs []string) domain.ReviewRoundDifference {
	result := domain.ReviewRoundDifference{}
	if current == nil {
		return result
	}
	result.ToRevision = current.Revision
	if previous == nil {
		return result
	}
	result.FromRevision = previous.Revision
	allowed := map[string]bool{}
	for _, id := range allowedSegmentIDs {
		allowed[id] = true
	}
	old, now := candidateEntries(previous), candidateEntries(current)
	ids := map[string]bool{}
	for id := range old {
		ids[id] = true
	}
	for id := range now {
		ids[id] = true
	}
	var ordered []string
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	for _, id := range ordered {
		kind := "unchanged"
		_, hadOld := old[id]
		_, hasNow := now[id]
		if !hadOld {
			kind = "added"
		} else if !hasNow {
			kind = "excluded"
		} else if !reflect.DeepEqual(old[id], now[id]) {
			kind = "modified"
		}
		change := domain.CandidateChange{SegmentID: id, Kind: kind, Anomaly: kind != "unchanged" && !allowed[id]}
		if change.Anomaly {
			result.HasAnomaly = true
		}
		result.Changes = append(result.Changes, change)
	}
	return result
}

type candidateEntry struct {
	Public *domain.PublicSegment
	Audio  []domain.AudioInstruction
}

func candidateEntries(candidate *domain.CandidateRelease) map[string]candidateEntry {
	result := map[string]candidateEntry{}
	for i := range candidate.PublicSegments {
		value := candidate.PublicSegments[i]
		entry := result[value.SegmentID]
		entry.Public = &value
		result[value.SegmentID] = entry
	}
	for _, value := range candidate.AudioInstructions {
		entry := result[value.SegmentID]
		entry.Audio = append(entry.Audio, value)
		result[value.SegmentID] = entry
	}
	for _, value := range candidate.Diffs {
		if _, ok := result[value.SegmentID]; !ok {
			result[value.SegmentID] = candidateEntry{}
		}
	}
	return result
}
