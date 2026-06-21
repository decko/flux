package ticket

import (
	"regexp"

	"github.com/decko/flux/internal/model"
)

// Compiled regexes used by ParseRelationships.
var (
	// Keyword-based body patterns.
	reCloses    = regexp.MustCompile(`(?i)\b(?:closes|fixes|resolves)\s+#(\d+)`)
	reBlocks    = regexp.MustCompile(`(?i)\bblocks\s+#(\d+)`)
	reBlockedBy = regexp.MustCompile(`(?i)\b(?:blocked\s+by|blocked-by|depends\s+on)\s+#(\d+)`)
	reParentOf  = regexp.MustCompile(`(?i)\bparent\s+of\s+#(\d+)`)
	reChildOf   = regexp.MustCompile(`(?i)\bchild\s+of\s+#(\d+)`)
	reBare      = regexp.MustCompile(`#(\d+)`)

	// Label-based patterns.
	reLabelBlockedBy = regexp.MustCompile(`^blocked-by:(\d+)$`)
	reLabelBlocks    = regexp.MustCompile(`^blocks:(\d+)$`)
)

// ParseRelationships extracts ticket relationships from issue body text
// and labels. It recognizes #N bare references, keyword-based references
// (closes, fixes, resolves, blocks, blocked by, blocked-by, depends on,
// parent of, child of), and label-encoded relationships (blocked-by:N,
// blocks:N).  Self-references (where the referenced issue matches
// selfExternalID) are filtered out. Duplicate relationships (same type +
// target) are deduplicated to a single entry.
//
// Keyword matching is case-insensitive.
func ParseRelationships(body string, labels []string, selfExternalID string) []model.Relationship {
	type item struct {
		typ      model.RelationType
		targetID string
	}

	var items []item

	// span represents the byte range of a #N occurrence claimed by a keyword
	// pattern, used to prevent the bare-reference regex from double-matching.
	type span struct{ start, end int }
	var claimed []span

	// addKeywords finds all matches of re in body, records each as an item
	// with the given relation type, and marks the #N range as claimed.
	addKeywords := func(re *regexp.Regexp, typ model.RelationType) {
		for _, m := range re.FindAllStringSubmatchIndex(body, -1) {
			if len(m) < 4 || m[2] < 0 {
				continue
			}
			targetID := body[m[2]:m[3]]
			items = append(items, item{typ: typ, targetID: targetID})
			claimed = append(claimed, span{start: m[2] - 1, end: m[3]})
		}
	}

	addKeywords(reCloses, model.RelationRelatesTo)
	addKeywords(reBlocks, model.RelationBlocks)
	addKeywords(reBlockedBy, model.RelationBlockedBy)
	addKeywords(reParentOf, model.RelationChild)
	addKeywords(reChildOf, model.RelationParent)

	// Bare #N references — skip any #N whose byte range overlaps with a
	// keyword-claimed range.
	for _, m := range reBare.FindAllStringSubmatchIndex(body, -1) {
		if len(m) < 4 || m[2] < 0 {
			continue
		}
		hashStart, hashEnd := m[0], m[1]
		isClaimed := false
		for _, c := range claimed {
			if hashStart >= c.start && hashEnd <= c.end {
				isClaimed = true
				break
			}
		}
		if !isClaimed {
			items = append(items, item{typ: model.RelationRelatesTo, targetID: body[m[2]:m[3]]})
		}
	}

	// Label-encoded relationships.
	for _, label := range labels {
		if m := reLabelBlockedBy.FindStringSubmatch(label); len(m) >= 2 {
			items = append(items, item{typ: model.RelationBlockedBy, targetID: m[1]})
		} else if m := reLabelBlocks.FindStringSubmatch(label); len(m) >= 2 {
			items = append(items, item{typ: model.RelationBlocks, targetID: m[1]})
		}
	}

	// Deduplicate by (Type, TargetID) and filter self-references.
	seen := make(map[string]bool)
	var result []model.Relationship
	for _, it := range items {
		if selfExternalID != "" && it.targetID == selfExternalID {
			continue
		}
		key := string(it.typ) + ":" + it.targetID
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, model.Relationship{Type: it.typ, TargetID: it.targetID})
	}

	return result
}
