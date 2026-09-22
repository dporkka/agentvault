package contextcompiler

import (
	"fmt"
	"strings"
	"time"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

func (c *candidateCollector) addEpisodes() error {
	type scope struct {
		typeName string
		id       string
		bonus    float64
	}
	scopes := []scope{
		{typeName: "session", id: c.req.SessionID, bonus: 0.10},
		{typeName: "project", id: c.req.Project, bonus: 0.07},
		{typeName: "agent", id: c.req.AgentID, bonus: 0.04},
	}

	terms := taskTerms(c.req.Task)
	for _, currentScope := range scopes {
		if strings.TrimSpace(currentScope.id) == "" {
			continue
		}
		episodes, err := c.compiler.knowledge.ListEpisodes(currentScope.typeName, currentScope.id, 40)
		if err != nil {
			return fmt.Errorf("list %s episodes: %w", currentScope.typeName, err)
		}
		for _, episode := range episodes {
			occurredAt, err := parseFlexibleTime(episode.OccurredAt)
			if err != nil || occurredAt.After(c.asOf) {
				continue
			}
			if episode.CreatedAt != "" {
				createdAt, err := parseFlexibleTime(episode.CreatedAt)
				if err != nil || createdAt.After(c.asOf) {
					continue
				}
			}
			relevance := lexicalRelevance(terms, episode.EventType+" "+episode.Summary)
			if relevance == 0 && currentScope.typeName != "session" {
				continue
			}
			score := 0.64 + currentScope.bonus + relevance*0.20
			c.add(contract.ContextItem{
				Kind:       "episode",
				ID:         episode.ID,
				Title:      episode.EventType,
				Content:    episode.Summary,
				Score:      score,
				ObjectIDs:  episode.ObjectIDs,
				Provenance: c.provenanceFor(episode.ProvenanceID),
				Metadata: map[string]interface{}{
					"scopeType":  episode.ScopeType,
					"scopeId":    episode.ScopeID,
					"occurredAt": episode.OccurredAt,
					"endedAt":    episode.EndedAt,
				},
			})
		}
	}
	return nil
}

func (c *candidateCollector) addFacts() error {
	facts := make(map[string]contract.TemporalFact)
	if c.req.Project != "" {
		rows, err := c.compiler.knowledge.FactsForProject(c.req.Project, 200)
		if err != nil {
			return fmt.Errorf("list project facts: %w", err)
		}
		for _, fact := range rows {
			facts[fact.ID] = fact
		}
	}
	for _, objectID := range c.objectIDs {
		rows, err := c.compiler.knowledge.FactsForObject(objectID, 100)
		if err != nil {
			return fmt.Errorf("list facts for %s: %w", objectID, err)
		}
		for _, fact := range rows {
			facts[fact.ID] = fact
		}
	}

	terms := taskTerms(c.req.Task)
	for _, fact := range facts {
		if !factValidAt(fact, c.asOf) {
			continue
		}
		value := fact.Value
		if fact.ObjectID != "" {
			value = fact.ObjectID
			if fact.Value != "" {
				value += " (" + fact.Value + ")"
			}
		}
		content := fmt.Sprintf("%s --%s--> %s", fact.SubjectID, fact.Predicate, value)
		relevance := lexicalRelevance(terms, fact.Predicate+" "+fact.Value)
		objectRelevant := c.objectSeen[fact.SubjectID] || (fact.ObjectID != "" && c.objectSeen[fact.ObjectID])
		if relevance == 0 && !objectRelevant {
			continue
		}
		c.add(contract.ContextItem{
			Kind:       "fact",
			ID:         fact.ID,
			Title:      fact.Predicate,
			Content:    content,
			Score:      0.88 + fact.Confidence*0.05 + relevance*0.06,
			ObjectIDs:  compactStrings(fact.SubjectID, fact.ObjectID),
			Provenance: c.provenanceFor(fact.ProvenanceID),
			Metadata: map[string]interface{}{
				"subjectId":    fact.SubjectID,
				"predicate":    fact.Predicate,
				"objectId":     fact.ObjectID,
				"value":        fact.Value,
				"confidence":   fact.Confidence,
				"validFrom":    fact.ValidFrom,
				"validTo":      fact.ValidTo,
				"supersedesId": fact.SupersedesID,
				"supersededAt": fact.SupersededAt,
				"supersededBy": fact.SupersededBy,
			},
		})
	}
	return nil
}

func factValidAt(fact contract.TemporalFact, asOf time.Time) bool {
	return knowledge.FactVisibleAt(fact, asOf)
}
