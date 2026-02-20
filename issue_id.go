package amadeus

import (
	"regexp"
	"sort"
)

var issueIDPattern = regexp.MustCompile(`MY-\d+`)

// ExtractIssueIDs scans texts for Linear Issue IDs (e.g. "MY-302")
// and returns a unique, sorted list.
func ExtractIssueIDs(texts ...string) []string {
	seen := make(map[string]bool)
	for _, text := range texts {
		for _, id := range issueIDPattern.FindAllString(text, -1) {
			seen[id] = true
		}
	}
	if len(seen) == 0 {
		return nil
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
