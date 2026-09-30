package contextcompiler

import (
	"strings"
	"time"

	"github.com/agentvault/core/internal/contract"
)

const rankingAlgorithm = "deterministic-multisignal-v1"

type scoreSignal struct {
	name   string
	value  float64
	weight float64
}

func (c *candidateCollector) rank(item contract.ContextItem) contract.ContextItem {
	signals := []scoreSignal{
		{name: "sourcePrior", value: clampScore(item.Score), weight: 0.64},
		{name: "lexicalRelevance", value: lexicalRelevance(taskTerms(c.req.Task), item.Title+" "+item.Content), weight: 0.16},
		{name: "provenanceConfidence", value: provenanceConfidence(item.Provenance), weight: 0.07},
		{name: "recency", value: contextRecency(item, c.asOf), weight: 0.05},
		{name: "explicitObject", value: explicitObjectSignal(item, c.req.ObjectIDs), weight: 0.08},
	}

	score := 0.0
	components := make([]contract.ContextRankingComponent, 0, len(signals))
	for _, signal := range signals {
		value := clampScore(signal.value)
		contribution := value * signal.weight
		score += contribution
		if c.req.Explain {
			components = append(components, contract.ContextRankingComponent{
				Signal:       signal.name,
				Value:        value,
				Weight:       signal.weight,
				Contribution: contribution,
			})
		}
	}
	item.Score = clampScore(score)
	if c.req.Explain {
		item.Ranking = &contract.ContextRankingExplanation{
			Algorithm:  rankingAlgorithm,
			Components: components,
		}
	}
	return item
}

func provenanceConfidence(provenance *contract.ContextProvenance) float64 {
	if provenance == nil || provenance.SourceType == "unresolved" {
		return 0
	}
	return clampScore(provenance.Confidence)
}

func explicitObjectSignal(item contract.ContextItem, requested []string) float64 {
	if len(requested) == 0 {
		return 0
	}
	requestedSet := make(map[string]bool, len(requested))
	for _, id := range requested {
		id = strings.TrimSpace(id)
		if id != "" {
			requestedSet[id] = true
		}
	}
	if requestedSet[item.ID] {
		return 1
	}
	for _, id := range item.ObjectIDs {
		if requestedSet[id] {
			return 1
		}
	}
	return 0
}

func contextRecency(item contract.ContextItem, asOf time.Time) float64 {
	timestamp := contextTimestamp(item)
	if timestamp.IsZero() || timestamp.After(asOf) {
		return 0
	}
	age := asOf.Sub(timestamp)
	days := age.Hours() / 24
	if days < 0 {
		return 0
	}
	// Smooth deterministic decay: 1.0 now, 0.5 at 30 days, 0.25 at 90 days.
	return 1 / (1 + days/30)
}

func contextTimestamp(item contract.ContextItem) time.Time {
	for _, key := range []string{"updatedAt", "occurredAt", "createdAt", "observedAt", "endedAt"} {
		if raw, ok := item.Metadata[key]; ok {
			if value, ok := raw.(string); ok && strings.TrimSpace(value) != "" {
				if parsed, err := parseFlexibleTime(value); err == nil {
					return parsed
				}
			}
		}
	}
	if item.Provenance != nil && strings.TrimSpace(item.Provenance.ObservedAt) != "" {
		if parsed, err := parseFlexibleTime(item.Provenance.ObservedAt); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
