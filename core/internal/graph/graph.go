// Package graph provides subgraph traversal over the AgentVault links table.
package graph

import (
	"fmt"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/db"
)

// BuildSubgraph returns the subgraph centered on centerID, expanding out to
// depth hops through the links table via BFS.
func BuildSubgraph(database *db.DB, centerID string, depth int) (*contract.Graph, error) {
	if depth < 0 {
		depth = 0
	}

	// Verify the center note exists.
	var title, noteType string
	var nullProject *string
	err := database.QueryRow(
		`SELECT title, type, project FROM notes WHERE id = ?`,
		centerID,
	).Scan(&title, &noteType, &nullProject)
	if err != nil {
		return nil, fmt.Errorf("center note %q not found: %w", centerID, err)
	}
	proj := ""
	if nullProject != nil {
		proj = *nullProject
	}

	nodes := map[string]contract.GraphNode{
		centerID: {ID: centerID, Title: title, Type: noteType, Project: proj},
	}
	edges := map[string]contract.GraphEdge{} // keyed by "fromID|toID|linkType"
	seen := map[string]bool{centerID: true}
	current := []string{centerID}

	for range depth {
		if len(current) == 0 {
			break
		}
		next := []string{}

		for _, noteID := range current {
			// Fetch outgoing links.
			outRows, err := database.Query(
				`SELECT from_note_id, to_note_id, link_type FROM links WHERE from_note_id = ?`,
				noteID,
			)
			if err != nil {
				return nil, fmt.Errorf("querying outgoing links for %q: %w", noteID, err)
			}
			outLinks := scanLinkRows(outRows)
			outRows.Close()

			// Fetch incoming links (backlinks).
			inRows, err := database.Query(
				`SELECT from_note_id, to_note_id, link_type FROM links WHERE to_note_id = ?`,
				noteID,
			)
			if err != nil {
				return nil, fmt.Errorf("querying incoming links for %q: %w", noteID, err)
			}
			inLinks := scanLinkRows(inRows)
			inRows.Close()

			allLinks := append(outLinks, inLinks...)

			for _, l := range allLinks {
				fromID := l.fromNoteID
				toID := l.toNoteID
				lt := l.linkType
				if lt == "" {
					lt = "wiki"
				}

				edgeKey := fmt.Sprintf("%s|%s|%s", fromID, toID, lt)
				edges[edgeKey] = contract.GraphEdge{
					FromID:   fromID,
					ToID:     toID,
					LinkType: lt,
				}

				// Discover neighbor.
				neighbor := toID
				if neighbor == "" {
					// Unresolved link — skip node lookup.
					continue
				}
				// For backlinks, the neighbor is the from_note_id.
				if neighbor == noteID {
					neighbor = fromID
				}

				if seen[neighbor] {
					continue
				}

				var nTitle, nType string
				var nProject *string
				if err := database.QueryRow(
					`SELECT title, type, project FROM notes WHERE id = ?`,
					neighbor,
				).Scan(&nTitle, &nType, &nProject); err != nil {
					// Note may have been deleted; skip.
					seen[neighbor] = true
					continue
				}
				nProj := ""
				if nProject != nil {
					nProj = *nProject
				}
				nodes[neighbor] = contract.GraphNode{ID: neighbor, Title: nTitle, Type: nType, Project: nProj}
				seen[neighbor] = true
				next = append(next, neighbor)
			}
		}

		current = next
	}

	// Build ordered slices from the maps.
	nodeList := make([]contract.GraphNode, 0, len(nodes))
	for _, n := range nodes {
		nodeList = append(nodeList, n)
	}
	edgeList := make([]contract.GraphEdge, 0, len(edges))
	for _, e := range edges {
		edgeList = append(edgeList, e)
	}

	return &contract.Graph{Nodes: nodeList, Edges: edgeList}, nil
}

type rawLink struct {
	fromNoteID string
	toNoteID   string
	linkType   string
}

func scanLinkRows(rows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}) []rawLink {
	var links []rawLink
	for rows.Next() {
		var l rawLink
		var toID, lt *string
		if err := rows.Scan(&l.fromNoteID, &toID, &lt); err != nil {
			continue
		}
		if toID != nil {
			l.toNoteID = *toID
		}
		if lt != nil {
			l.linkType = *lt
		}
		links = append(links, l)
	}
	return links
}
