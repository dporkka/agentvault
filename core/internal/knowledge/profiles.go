package knowledge

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/agentvault/core/internal/contract"
)

const (
	defaultProfileFacts = 16
	maximumProfileFacts = 100
)

// BuildEntityProfile returns a standing, deterministic view over an object and
// the facts about that object that were both valid and known at asOf.
func (s *Store) BuildEntityProfile(objectID string, asOf time.Time, maxFacts int) (contract.EntityProfile, error) {
	objectID = strings.TrimSpace(objectID)
	if objectID == "" {
		return contract.EntityProfile{}, fmt.Errorf("object id is required")
	}
	subject, err := s.GetObject(objectID)
	if err != nil {
		return contract.EntityProfile{}, err
	}
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	} else {
		asOf = asOf.UTC()
	}
	if maxFacts <= 0 {
		maxFacts = defaultProfileFacts
	}
	if maxFacts > maximumProfileFacts {
		maxFacts = maximumProfileFacts
	}

	rows, err := s.FactsForSubject(objectID, 1000)
	if err != nil {
		return contract.EntityProfile{}, err
	}
	facts := make([]contract.TemporalFact, 0, min(maxFacts, len(rows)))
	for _, fact := range rows {
		if !FactVisibleAt(fact, asOf) {
			continue
		}
		facts = append(facts, fact)
	}

	sort.SliceStable(facts, func(i, j int) bool {
		if facts[i].Predicate != facts[j].Predicate {
			return facts[i].Predicate < facts[j].Predicate
		}
		if facts[i].Confidence != facts[j].Confidence {
			return facts[i].Confidence > facts[j].Confidence
		}
		if facts[i].UpdatedAt != facts[j].UpdatedAt {
			return facts[i].UpdatedAt > facts[j].UpdatedAt
		}
		return facts[i].ID < facts[j].ID
	})
	if len(facts) > maxFacts {
		facts = facts[:maxFacts]
	}

	return contract.EntityProfile{
		Subject: subject,
		AsOf:    asOf.Format(time.RFC3339Nano),
		Facts:   facts,
	}, nil
}

// FactVisibleAt applies AgentVault's bi-temporal fact semantics. The validity
// interval describes domain truth; CreatedAt/SupersededAt describe when the
// claim existed in AgentVault's knowledge state.
func FactVisibleAt(fact contract.TemporalFact, asOf time.Time) bool {
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	} else {
		asOf = asOf.UTC()
	}
	if !knowledgeIntervalContains(fact.ValidFrom, fact.ValidTo, asOf) {
		return false
	}
	if fact.CreatedAt != "" {
		createdAt, err := parseKnowledgeTime(fact.CreatedAt)
		if err != nil || createdAt.After(asOf) {
			return false
		}
	}
	if fact.SupersededAt != "" {
		supersededAt, err := parseKnowledgeTime(fact.SupersededAt)
		if err != nil || !supersededAt.After(asOf) {
			return false
		}
	}
	return true
}

func knowledgeIntervalContains(validFrom, validTo string, asOf time.Time) bool {
	if validFrom != "" {
		from, err := parseKnowledgeTime(validFrom)
		if err != nil || from.After(asOf) {
			return false
		}
	}
	if validTo != "" {
		to, err := parseKnowledgeTime(validTo)
		if err != nil || !to.After(asOf) {
			return false
		}
	}
	return true
}
