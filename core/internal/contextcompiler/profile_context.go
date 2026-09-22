package contextcompiler

import (
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/contract"
)

var standingProfileTypes = map[string]bool{
	"person":       true,
	"user":         true,
	"agent":        true,
	"project":      true,
	"organization": true,
	"repository":   true,
	"team":         true,
}

func (c *candidateCollector) addProfiles() error {
	for _, objectID := range c.objectIDs {
		object, err := c.compiler.knowledge.GetObject(objectID)
		if err != nil {
			return fmt.Errorf("load profile object %s: %w", objectID, err)
		}
		if !standingProfileTypes[strings.ToLower(strings.TrimSpace(object.Type))] {
			continue
		}
		profile, err := c.compiler.knowledge.BuildEntityProfile(objectID, c.asOf, 16)
		if err != nil {
			return fmt.Errorf("build profile %s: %w", objectID, err)
		}
		if len(profile.Facts) == 0 {
			continue
		}

		lines := make([]string, 0, len(profile.Facts))
		objectIDs := []string{profile.Subject.ID}
		includedFactIDs := make([]string, 0, len(profile.Facts))
		for _, fact := range profile.Facts {
			value := strings.TrimSpace(fact.Value)
			if fact.ObjectID != "" {
				target, err := c.compiler.knowledge.GetObject(fact.ObjectID)
				if err != nil {
					return fmt.Errorf("load profile fact target %s: %w", fact.ObjectID, err)
				}
				// A project-scoped compilation does not fold a target from a
				// different project into standing profile text. The ordinary
				// fact path can still surface it for an unscoped/root caller.
				if c.req.Project != "" && target.Project != c.req.Project {
					continue
				}
				objectIDs = append(objectIDs, target.ID)
				if value == "" {
					value = target.Title
				} else {
					value = target.Title + " (" + value + ")"
				}
			}
			if value == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s: %s [fact:%s]", fact.Predicate, value, fact.ID))
			includedFactIDs = append(includedFactIDs, fact.ID)
		}
		if len(lines) == 0 {
			continue
		}

		score := 0.92
		if requestedObject(c.req.ObjectIDs, objectID) {
			score = 0.985
		}
		c.add(contract.ContextItem{
			Kind:      "profile",
			ID:        profile.Subject.ID,
			Title:     "Standing profile: " + profile.Subject.Title,
			Content:   strings.Join(lines, "\n"),
			Score:     score,
			ObjectIDs: compactStrings(objectIDs...),
			Metadata: map[string]interface{}{
				"type":      profile.Subject.Type,
				"project":   profile.Subject.Project,
				"asOf":      profile.AsOf,
				"factIds":   includedFactIDs,
				"factCount": len(includedFactIDs),
			},
		})
	}
	return nil
}

func requestedObject(ids []string, objectID string) bool {
	for _, id := range ids {
		if strings.TrimSpace(id) == objectID {
			return true
		}
	}
	return false
}
