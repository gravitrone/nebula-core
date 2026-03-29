package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

type relationshipSummaryEntry struct {
	Rel  string
	Dir  string
	Node string
}

// renderRelationshipSummaryTable renders render relationship summary table.
func renderRelationshipSummaryTable(nodeType, nodeID string, rels []api.Relationship, maxRows, width int) string {
	rows, extra := relationshipSummaryEntries(nodeType, nodeID, rels, maxRows)
	if len(rows) == 0 && extra == 0 {
		return components.TitledBox("Relationships", "No relationships yet.", width)
	}

	contentWidth := components.BoxContentWidth(width) - 2
	if contentWidth < 32 {
		contentWidth = 32
	}
	relWidth, dirWidth, nodeWidth := relationshipSummaryColumnWidths(contentWidth)
	columns := []components.TableColumn{
		{Header: "Rel", Width: relWidth, Align: lipgloss.Left},
		{Header: "Direction", Width: dirWidth, Align: lipgloss.Left},
		{Header: "Node", Width: nodeWidth, Align: lipgloss.Left},
	}

	gridRows := make([][]string, 0, len(rows)+1)
	for _, row := range rows {
		gridRows = append(gridRows, []string{
			row.Rel,
			row.Dir,
			row.Node,
		})
	}
	if extra > 0 {
		gridRows = append(gridRows, []string{
			"More",
			"+",
			fmt.Sprintf("%d more relationships", extra),
		})
	}

	content := components.RenderGridTable(columns, gridRows, contentWidth)
	return components.TitledBox("Relationships", content, width)
}

// relationshipSummaryRows handles relationship summary rows.
func relationshipSummaryRows(nodeType, nodeID string, rels []api.Relationship, maxRows int) []components.TableRow {
	entries, extra := relationshipSummaryEntries(nodeType, nodeID, rels, maxRows)
	rows := make([]components.TableRow, 0, len(entries)+1)
	for i, entry := range entries {
		rows = append(rows, components.TableRow{
			Label: fmt.Sprintf("%s %d", entry.Rel, i+1),
			Value: fmt.Sprintf("%s %s", entry.Dir, entry.Node),
		})
	}
	if extra > 0 {
		rows = append(rows, components.TableRow{
			Label: "More",
			Value: fmt.Sprintf("+%d relationships", extra),
		})
	}
	return rows
}

// relationshipSummaryEntries handles relationship summary entries.
func relationshipSummaryEntries(nodeType, nodeID string, rels []api.Relationship, maxRows int) ([]relationshipSummaryEntry, int) {
	if maxRows <= 0 {
		maxRows = 5
	}
	entries := make([]relationshipSummaryEntry, 0, maxRows)
	for i, rel := range rels {
		if i >= maxRows {
			break
		}
		relType := strings.TrimSpace(components.SanitizeOneLine(rel.Type))
		if relType == "" {
			relType = "-"
		}
		direction, endpoint := relationshipDirectionAndEndpoint(nodeType, nodeID, rel)
		entries = append(entries, relationshipSummaryEntry{
			Rel:  relType,
			Dir:  direction,
			Node: endpoint,
		})
	}
	extra := len(rels) - maxRows
	if extra < 0 {
		extra = 0
	}
	return entries, extra
}

// relationshipSummaryColumnWidths handles relationship summary column widths.
func relationshipSummaryColumnWidths(contentWidth int) (int, int, int) {
	if contentWidth < 32 {
		contentWidth = 32
	}
	usable := contentWidth - 2 // separators

	rel := usable * 28 / 100
	dir := usable * 18 / 100
	node := usable - rel - dir

	if dir < 9 {
		dir = 9
		node = usable - rel - dir
	}
	return rel, dir, node
}

// relationshipDirectionAndEndpoint handles relationship direction and endpoint.
func relationshipDirectionAndEndpoint(nodeType, nodeID string, rel api.Relationship) (string, string) {
	sourceID := strings.TrimSpace(rel.SourceID)
	targetID := strings.TrimSpace(rel.TargetID)
	sourceType := strings.TrimSpace(strings.ToLower(rel.SourceType))
	targetType := strings.TrimSpace(strings.ToLower(rel.TargetType))

	sourceLabel := relationshipNodeLabel(rel.SourceName, sourceID, sourceType)
	targetLabel := relationshipNodeLabel(rel.TargetName, targetID, targetType)

	switch {
	case sourceType == nodeType && sourceID == nodeID:
		return "->", targetLabel
	case targetType == nodeType && targetID == nodeID:
		return "<-", sourceLabel
	default:
		return "<>", fmt.Sprintf("%s <-> %s", sourceLabel, targetLabel)
	}
}

// relationshipNodeLabel handles relationship node label.
func relationshipNodeLabel(name, nodeID, nodeType string) string {
	clean := strings.TrimSpace(components.SanitizeOneLine(name))
	if clean != "" {
		return clean
	}
	switch strings.TrimSpace(nodeType) {
	case "entity":
		return "entity:" + shortID(nodeID)
	case "context":
		return "context:" + shortID(nodeID)
	case "job":
		return "job:" + shortID(nodeID)
	case "log":
		return "log:" + shortID(nodeID)
	case "file":
		return "file:" + shortID(nodeID)
	case "protocol":
		return "protocol:" + shortID(nodeID)
	case "agent":
		return "agent:" + shortID(nodeID)
	default:
		if strings.TrimSpace(nodeID) != "" {
			return shortID(nodeID)
		}
		return "unknown"
	}
}
