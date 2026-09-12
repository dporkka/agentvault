// Package contextcompiler assembles deterministic, evidence-backed context for
// agents from AgentVault's notes, structured knowledge, memories, relations,
// and durable session history.
package contextcompiler

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
	"github.com/agentvault/core/internal/search"
)

const (
	defaultTokenBudget = 8000
	minimumTokenBudget = 256
	maximumTokenBudget = 128000
	defaultMaxItems     = 40
	maximumMaxItems     = 200
)

// Compiler builds model-agnostic context bundles without making an LLM call.
type Compiler struct {
	searcher  *search.Searcher
	knowledge *knowledge.Store
}

// New creates a context compiler over the existing search and knowledge stores.
func New(searcher *search.Searcher, knowledgeStore *knowledge.Store) *Compiler {
	return &Compiler{searcher: searcher, knowledge: knowledgeStore}
}

// Compile assembles and ranks context for one task. TokenBudget applies to the
// included ContextItems; request/envelope metadata is intentionally excluded.
func (c *Compiler) Compile(req contract.CompileContextRequest) (contract.ContextBundle, error) {
	req.Task = strings.TrimSpace(req.Task)
	if req.Task == "" {
		return contract.ContextBundle{}, errors.New("task is required")
	}
	if c.searcher == nil || c.knowledge == nil {
		return contract.ContextBundle{}, errors.New("context compiler dependencies are not configured")
	}

	budget, err := normalizeTokenBudget(req.TokenBudget)
	if err != nil {
		return contract.ContextBundle{}, err
	}
	maxItems, err := normalizeMaxItems(req.MaxItems)
	if err != nil {
		return contract.ContextBundle{}, err
	}
	asOf, err := normalizeAsOf(req.AsOf)
	if err != nil {
		return contract.ContextBundle{}, err
	}

	collector := &candidateCollector{
		compiler:   c,
		req:        req,
		asOf:       asOf,
		seen:       make(map[string]bool),
		provenance: make(map[string]*contract.ContextProvenance),
	}

	if err := collector.addCurrentSession(); err != nil {
		return contract.ContextBundle{}, err
	}
	if err := collector.addMemories(); err != nil {
		return contract.ContextBundle{}, err
	}
	if err := collector.addObjects(); err != nil {
		return contract.ContextBundle{}, err
	}
	if err := collector.addRelations(); err != nil {
		return contract.ContextBundle{}, err
	}
	if err := collector.addNotes(); err != nil {
		return contract.ContextBundle{}, err
	}
	if err := collector.addRecentSessions(); err != nil {
		return contract.ContextBundle{}, err
	}

	sort.SliceStable(collector.items, func(i, j int) bool {
		if collector.items[i].Score != collector.items[j].Score {
			return collector.items[i].Score > collector.items[j].Score
		}
		if collector.items[i].Kind != collector.items[j].Kind {
			return collector.items[i].Kind < collector.items[j].Kind
		}
		return collector.items[i].ID < collector.items[j].ID
	})

	items, used, dropped, truncated := fitItems(collector.items, budget, maxItems)
	byKind := make(map[string]int)
	for _, item := range items {
		byKind[item.Kind]++
	}

	return contract.ContextBundle{
		Version:         "1",
		Task:            req.Task,
		Project:         req.Project,
		AgentID:         req.AgentID,
		SessionID:       req.SessionID,
		AsOf:            asOf.Format(time.RFC3339Nano),
		TokenBudget:     budget,
		EstimatedTokens: used,
		Truncated:       truncated,
		Items:           items,
		Stats: contract.ContextBundleStats{
			Candidates: len(collector.items),
			Included:   len(items),
			Dropped:    dropped,
			ByKind:     byKind,
		},
	}, nil
}

type candidateCollector struct {
	compiler   *Compiler
	req        contract.CompileContextRequest
	asOf       time.Time
	items      []contract.ContextItem
	seen       map[string]bool
	objectIDs  []string
	objectSeen map[string]bool
	provenance map[string]*contract.ContextProvenance
}

func (c *candidateCollector) add(item contract.ContextItem) {
	key := item.Kind + ":" + item.ID
	if item.ID == "" || c.seen[key] {
		return
	}
	c.seen[key] = true
	item.Score = clampScore(item.Score)
	if item.ObjectIDs == nil {
		item.ObjectIDs = []string{}
	}
	if item.Metadata == nil {
		item.Metadata = map[string]interface{}{}
	}
	c.items = append(c.items, item)
}

func (c *candidateCollector) addCurrentSession() error {
	if c.req.SessionID == "" {
		return nil
	}
	session, err := c.compiler.knowledge.GetSession(c.req.SessionID)
	if err != nil {
		return fmt.Errorf("load current session: %w", err)
	}
	if c.req.AgentID != "" && session.AgentID != c.req.AgentID {
		return fmt.Errorf("session %s belongs to agent %s, not %s", session.ID, session.AgentID, c.req.AgentID)
	}
	if c.req.Project != "" && session.Project != "" && session.Project != c.req.Project {
		return fmt.Errorf("session %s belongs to project %s, not %s", session.ID, session.Project, c.req.Project)
	}

	contextJSON, _ := json.Marshal(session.Context)
	content := fmt.Sprintf("Objective: %s\nStatus: %s", session.Objective, session.Status)
	if session.Branch != "" {
		content += "\nBranch: " + session.Branch
	}
	if session.Worktree != "" {
		content += "\nWorktree: " + session.Worktree
	}
	if len(contextJSON) > 2 {
		content += "\nInitial context: " + string(contextJSON)
	}
	c.add(contract.ContextItem{
		Kind:    "session",
		ID:      session.ID,
		Title:   "Current agent session",
		Content: content,
		Score:   1,
		Metadata: map[string]interface{}{
			"agentId":   session.AgentID,
			"project":   session.Project,
			"startedAt": session.StartedAt,
			"updatedAt": session.UpdatedAt,
		},
	})

	start := len(session.Events) - 12
	if start < 0 {
		start = 0
	}
	for i := len(session.Events) - 1; i >= start; i-- {
		event := session.Events[i]
		payload, _ := json.Marshal(event.Payload)
		c.add(contract.ContextItem{
			Kind:       "session_event",
			ID:         event.ID,
			Title:      event.EventType,
			Content:    string(payload),
			Score:      0.98 - float64(len(session.Events)-1-i)*0.005,
			Provenance: c.provenanceFor(event.ProvenanceID),
			Metadata: map[string]interface{}{
				"sessionId": event.SessionID,
				"createdAt": event.CreatedAt,
			},
		})
	}
	return nil
}

func (c *candidateCollector) addMemories() error {
	type scope struct {
		typeName string
		id       string
		bonus    float64
	}
	scopes := []scope{
		{typeName: "session", id: c.req.SessionID, bonus: 0.08},
		{typeName: "project", id: c.req.Project, bonus: 0.06},
		{typeName: "agent", id: c.req.AgentID, bonus: 0.04},
	}

	memories := make(map[string]contract.MemoryRecord)
	scopeBonus := make(map[string]float64)
	for _, currentScope := range scopes {
		if currentScope.id == "" {
			continue
		}
		rows, err := c.compiler.knowledge.ListMemories(currentScope.typeName, currentScope.id, "", 100)
		if err != nil {
			return fmt.Errorf("list %s memories: %w", currentScope.typeName, err)
		}
		for _, memory := range rows {
			memories[memory.ID] = memory
			if currentScope.bonus > scopeBonus[memory.ID] {
				scopeBonus[memory.ID] = currentScope.bonus
			}
		}
	}

	superseded := make(map[string]bool)
	for _, memory := range memories {
		if memory.SupersedesID != "" && memoryValidAt(memory, c.asOf) {
			superseded[memory.SupersedesID] = true
		}
	}
	for _, memory := range memories {
		if superseded[memory.ID] || !memoryValidAt(memory, c.asOf) {
			continue
		}
		base := map[string]float64{
			"procedural": 0.90,
			"semantic":   0.88,
			"working":    0.82,
			"episodic":   0.75,
		}[memory.MemoryClass]
		title := fmt.Sprintf("%s memory (%s:%s)", memory.MemoryClass, memory.ScopeType, memory.ScopeID)
		if memory.MemoryKind != "" {
			title = fmt.Sprintf("%s %s memory (%s:%s)", memory.MemoryClass, memory.MemoryKind, memory.ScopeType, memory.ScopeID)
		}
		c.add(contract.ContextItem{
			Kind:       "memory",
			ID:         memory.ID,
			Title:      title,
			Content:    memory.Content,
			Score:      base + scopeBonus[memory.ID] + memory.Confidence*0.02,
			ObjectIDs:  compactStrings(memory.ObjectID),
			Provenance: c.provenanceFor(memory.ProvenanceID),
			Metadata: map[string]interface{}{
				"memoryClass": memory.MemoryClass,
				"memoryKind":  memory.MemoryKind,
				"scopeType":   memory.ScopeType,
				"scopeId":     memory.ScopeID,
				"confidence":  memory.Confidence,
				"validFrom":   memory.ValidFrom,
				"validTo":     memory.ValidTo,
			},
		})
	}
	return nil
}

func (c *candidateCollector) addObjects() error {
	c.objectSeen = make(map[string]bool)
	for _, id := range c.req.ObjectIDs {
		id = strings.TrimSpace(id)
		if id == "" || c.objectSeen[id] {
			continue
		}
		object, err := c.compiler.knowledge.GetObject(id)
		if err != nil {
			return fmt.Errorf("load requested object %s: %w", id, err)
		}
		c.addObject(object, 0.995, true)
	}

	if c.req.Project == "" {
		return nil
	}
	objects, err := c.compiler.knowledge.ListObjects(contract.KnowledgeObjectFilter{Project: c.req.Project, Limit: 100})
	if err != nil {
		return fmt.Errorf("list project objects: %w", err)
	}
	terms := taskTerms(c.req.Task)
	for _, object := range objects {
		if c.objectSeen[object.ID] {
			continue
		}
		data, _ := json.Marshal(object.Data)
		relevance := lexicalRelevance(terms, object.Title+" "+string(data))
		typeBonus := map[string]float64{
			"decision":    0.08,
			"requirement": 0.07,
			"task":        0.06,
			"repository":  0.05,
			"artifact":    0.03,
		}[object.Type]
		score := 0.60 + relevance*0.27 + typeBonus
		if relevance == 0 && typeBonus < 0.05 {
			continue
		}
		c.addObject(object, score, false)
	}
	return nil
}

func (c *candidateCollector) addObject(object contract.KnowledgeObject, score float64, explicit bool) {
	if c.objectSeen[object.ID] {
		return
	}
	c.objectSeen[object.ID] = true
	c.objectIDs = append(c.objectIDs, object.ID)
	data, _ := json.Marshal(object.Data)
	content := fmt.Sprintf("Type: %s", object.Type)
	if object.Status != "" {
		content += "\nStatus: " + object.Status
	}
	if object.Organization != "" {
		content += "\nOrganization: " + object.Organization
	}
	if object.Project != "" {
		content += "\nProject: " + object.Project
	}
	if len(data) > 2 {
		content += "\nData: " + string(data)
	}
	c.add(contract.ContextItem{
		Kind:       "object",
		ID:         object.ID,
		Title:      object.Title,
		Content:    content,
		Path:       object.CanonicalPath,
		Score:      score,
		ObjectIDs:  []string{object.ID},
		Provenance: c.provenanceFor(object.ProvenanceID),
		Metadata: map[string]interface{}{
			"type":      object.Type,
			"explicit":  explicit,
			"updatedAt": object.UpdatedAt,
		},
	})
}

func (c *candidateCollector) addRelations() error {
	seenRelations := make(map[string]bool)
	for _, objectID := range c.objectIDs {
		relations, err := c.compiler.knowledge.RelationsForObject(objectID)
		if err != nil {
			return fmt.Errorf("list relations for %s: %w", objectID, err)
		}
		for _, relation := range relations {
			if seenRelations[relation.ID] || !relationValidAt(relation, c.asOf) {
				continue
			}
			seenRelations[relation.ID] = true
			c.add(contract.ContextItem{
				Kind:       "relation",
				ID:         relation.ID,
				Title:      relation.RelationType,
				Content:    fmt.Sprintf("%s --%s--> %s", relation.FromObjectID, relation.RelationType, relation.ToObjectID),
				Score:      0.72 + relation.Confidence*0.05,
				ObjectIDs:  []string{relation.FromObjectID, relation.ToObjectID},
				Provenance: c.provenanceFor(relation.ProvenanceID),
				Metadata: map[string]interface{}{
					"confidence": relation.Confidence,
					"validFrom":  relation.ValidFrom,
					"validTo":    relation.ValidTo,
				},
			})
		}
	}
	return nil
}

func (c *candidateCollector) addNotes() error {
	queryText := strings.Join(taskTerms(c.req.Task), " ")
	if queryText == "" {
		queryText = c.req.Task
	}
	results, err := c.compiler.searcher.Search(search.Query{
		Q:       queryText,
		Project: c.req.Project,
		Limit:   24,
	})
	if err != nil {
		return fmt.Errorf("search notes for context: %w", err)
	}
	for i, result := range results {
		detail, err := c.compiler.searcher.GetByID(result.ID)
		if err != nil {
			return fmt.Errorf("load note %s: %w", result.ID, err)
		}
		c.add(contract.ContextItem{
			Kind:    "note",
			ID:      result.ID,
			Title:   result.Title,
			Content: detail.Snippet,
			Path:    result.Path,
			Score:   0.84 - float64(i)*0.012,
			Metadata: map[string]interface{}{
				"type":      result.Type,
				"project":   result.Project,
				"status":    result.Status,
				"tags":      result.Tags,
				"updatedAt": result.UpdatedAt,
			},
		})
	}
	return nil
}

func (c *candidateCollector) addRecentSessions() error {
	sessions, err := c.compiler.knowledge.ListSessions(c.req.AgentID, c.req.Project, c.req.SessionID, 5)
	if err != nil {
		return fmt.Errorf("list recent sessions: %w", err)
	}
	terms := taskTerms(c.req.Task)
	for _, session := range sessions {
		payload := strings.Builder{}
		payload.WriteString("Objective: ")
		payload.WriteString(session.Objective)
		payload.WriteString("\nStatus: ")
		payload.WriteString(session.Status)
		start := len(session.Events) - 3
		if start < 0 {
			start = 0
		}
		for _, event := range session.Events[start:] {
			encoded, _ := json.Marshal(event.Payload)
			payload.WriteString("\n")
			payload.WriteString(event.EventType)
			payload.WriteString(": ")
			payload.Write(encoded)
		}
		relevance := lexicalRelevance(terms, payload.String())
		if relevance == 0 && c.req.Project == "" && c.req.AgentID == "" {
			continue
		}
		c.add(contract.ContextItem{
			Kind:    "session_history",
			ID:      session.ID,
			Title:   "Prior agent session",
			Content: payload.String(),
			Score:   0.55 + relevance*0.20,
			Metadata: map[string]interface{}{
				"agentId":   session.AgentID,
				"project":   session.Project,
				"updatedAt": session.UpdatedAt,
				"endedAt":   session.EndedAt,
			},
		})
	}
	return nil
}

func (c *candidateCollector) provenanceFor(id string) *contract.ContextProvenance {
	if id == "" {
		return nil
	}
	if cached, ok := c.provenance[id]; ok {
		return cached
	}
	record, err := c.compiler.knowledge.GetProvenance(id)
	if err != nil {
		return &contract.ContextProvenance{ID: id, SourceType: "unresolved"}
	}
	value := &contract.ContextProvenance{
		ID:         record.ID,
		SourceType: record.SourceType,
		SourceID:   record.SourceID,
		AgentID:    record.AgentID,
		SessionID:  record.SessionID,
		Model:      record.Model,
		Confidence: record.Confidence,
		ObservedAt: record.ObservedAt,
		Evidence:   record.Evidence,
	}
	c.provenance[id] = value
	return value
}

func normalizeTokenBudget(value int) (int, error) {
	if value == 0 {
		return defaultTokenBudget, nil
	}
	if value < minimumTokenBudget || value > maximumTokenBudget {
		return 0, fmt.Errorf("tokenBudget must be between %d and %d", minimumTokenBudget, maximumTokenBudget)
	}
	return value, nil
}

func normalizeMaxItems(value int) (int, error) {
	if value == 0 {
		return defaultMaxItems, nil
	}
	if value < 1 || value > maximumMaxItems {
		return 0, fmt.Errorf("maxItems must be between 1 and %d", maximumMaxItems)
	}
	return value, nil
}

func normalizeAsOf(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Now().UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("asOf must be RFC3339: %w", err)
	}
	return parsed.UTC(), nil
}

func memoryValidAt(memory contract.MemoryRecord, asOf time.Time) bool {
	return intervalValidAt(memory.ValidFrom, memory.ValidTo, asOf)
}

func relationValidAt(relation contract.ObjectRelation, asOf time.Time) bool {
	return intervalValidAt(relation.ValidFrom, relation.ValidTo, asOf)
}

func intervalValidAt(validFrom, validTo string, asOf time.Time) bool {
	if validFrom != "" {
		from, err := parseFlexibleTime(validFrom)
		if err != nil || from.After(asOf) {
			return false
		}
	}
	if validTo != "" {
		to, err := parseFlexibleTime(validTo)
		if err != nil || !to.After(asOf) {
			return false
		}
	}
	return true
}

func parseFlexibleTime(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	return time.Parse("2006-01-02", value)
}

func taskTerms(task string) []string {
	var builder strings.Builder
	for _, r := range strings.ToLower(task) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		} else {
			builder.WriteByte(' ')
		}
	}
	seen := make(map[string]bool)
	terms := make([]string, 0)
	for _, term := range strings.Fields(builder.String()) {
		if len([]rune(term)) < 3 || seen[term] || stopWord(term) {
			continue
		}
		seen[term] = true
		terms = append(terms, term)
	}
	return terms
}

func stopWord(term string) bool {
	switch term {
	case "the", "and", "for", "with", "from", "this", "that", "into", "using", "use", "add", "make", "implement", "work":
		return true
	default:
		return false
	}
}

func lexicalRelevance(terms []string, text string) float64 {
	if len(terms) == 0 {
		return 0
	}
	text = strings.ToLower(text)
	matched := 0
	for _, term := range terms {
		if strings.Contains(text, term) {
			matched++
		}
	}
	return float64(matched) / float64(len(terms))
}

func fitItems(candidates []contract.ContextItem, budget, maxItems int) ([]contract.ContextItem, int, int, bool) {
	items := make([]contract.ContextItem, 0, min(maxItems, len(candidates)))
	used := 0
	truncated := false
	for _, candidate := range candidates {
		if len(items) >= maxItems {
			truncated = true
			break
		}
		remaining := budget - used
		if remaining <= 0 {
			truncated = true
			break
		}
		fitted, ok, wasTruncated := fitItem(candidate, remaining)
		if !ok {
			truncated = true
			continue
		}
		items = append(items, fitted)
		used += fitted.EstimatedTokens
		truncated = truncated || wasTruncated
	}
	dropped := len(candidates) - len(items)
	if dropped > 0 {
		truncated = true
	}
	return items, used, dropped, truncated
}

func fitItem(item contract.ContextItem, remaining int) (contract.ContextItem, bool, bool) {
	item.EstimatedTokens = estimateItem(item)
	if item.EstimatedTokens <= remaining {
		return item, true, false
	}
	if remaining < 24 || item.Content == "" {
		return contract.ContextItem{}, false, false
	}

	if item.Metadata == nil {
		item.Metadata = map[string]interface{}{}
	}
	item.Metadata["truncated"] = true
	runes := []rune(item.Content)
	low, high := 0, len(runes)
	best := -1
	bestEstimate := 0
	for low <= high {
		mid := low + (high-low)/2
		item.Content = string(runes[:mid])
		estimate := estimateItem(item)
		if estimate <= remaining {
			best = mid
			bestEstimate = estimate
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	if best <= 0 {
		return contract.ContextItem{}, false, false
	}
	item.Content = string(runes[:best])
	if best < len(runes) {
		item.Content += "…"
	}
	item.EstimatedTokens = estimateItem(item)
	if item.EstimatedTokens > remaining {
		item.Content = string(runes[:best])
		item.EstimatedTokens = bestEstimate
	}
	return item, true, true
}

func estimateItem(item contract.ContextItem) int {
	item.EstimatedTokens = 0
	data, err := json.Marshal(item)
	if err != nil {
		return 1
	}
	return max(1, (len(data)+3)/4)
}

func clampScore(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func compactStrings(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
