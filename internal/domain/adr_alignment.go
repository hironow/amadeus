package domain

import (
	"fmt"
	"sort"
	"strings"
)

// ADRAlignmentScore holds compliance information for a single ADR.
type ADRAlignmentScore struct {
	Number  string `json:"number"`  // "0002"
	Title   string `json:"title"`   // "Four-Axis Divergence Scoring"
	Score   int    `json:"score"`   // 0-100: violation severity (0=compliant, 100=fully violated)
	Verdict string `json:"verdict"` // "compliant" | "partial" | "violated"
	Reason  string `json:"reason"`  // one-sentence rationale
}

// ADRAlignmentMap is keyed by ADR number string ("0001", "0002", ...).
type ADRAlignmentMap map[string]ADRAlignmentScore

// DeriveADRIntegrityScore computes the aggregate adr_integrity axis score
// from per-ADR alignment scores. Returns 0 for nil/empty map.
func DeriveADRIntegrityScore(alignment ADRAlignmentMap) int {
	if len(alignment) == 0 {
		return 0
	}
	total := 0
	for _, a := range alignment {
		total += ClampAxisScore(a.Score)
	}
	return total / len(alignment)
}

// PerADRViolationFrequency returns, for each ADR number, the fraction of
// recent CheckResults in which that ADR was scored at or above threshold.
func PerADRViolationFrequency(results []CheckResult, threshold int) map[string]float64 {
	counts := make(map[string]int)
	appearances := make(map[string]int)
	for _, r := range results {
		for num, a := range r.ADRAlignment {
			appearances[num]++
			if a.Score >= threshold {
				counts[num]++
			}
		}
	}
	freq := make(map[string]float64, len(appearances))
	for num, total := range appearances {
		freq[num] = float64(counts[num]) / float64(total)
	}
	return freq
}

// TopViolatedADRs returns the N ADR numbers with highest violation frequency.
func TopViolatedADRs(results []CheckResult, n int, threshold int) []string {
	freq := PerADRViolationFrequency(results, threshold)
	type entry struct {
		num  string
		freq float64
	}
	var entries []entry
	for num, f := range freq {
		if f > 0 {
			entries = append(entries, entry{num, f})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].freq != entries[j].freq {
			return entries[i].freq > entries[j].freq
		}
		return entries[i].num < entries[j].num
	})
	result := make([]string, 0, n)
	for i, e := range entries {
		if i >= n {
			break
		}
		result = append(result, e.num)
	}
	return result
}

// FormatViolatedADRsSection generates a Markdown table of violated ADRs
// for inclusion in D-Mail body. Returns empty string if no violations.
func FormatViolatedADRsSection(alignment ADRAlignmentMap, results []CheckResult, threshold int) string {
	var violated []ADRAlignmentScore
	for _, a := range alignment {
		if a.Verdict == "violated" || a.Score >= threshold {
			violated = append(violated, a)
		}
	}
	if len(violated) == 0 {
		return ""
	}
	sort.Slice(violated, func(i, j int) bool {
		return violated[i].Number < violated[j].Number
	})

	var b strings.Builder
	b.WriteString("## Violated ADRs\n\n")
	b.WriteString("| ADR | Title | Score | Reason |\n")
	b.WriteString("|-----|-------|-------|--------|\n")
	for _, v := range violated {
		b.WriteString(fmt.Sprintf("| %s | %s | %d | %s |\n", v.Number, v.Title, v.Score, v.Reason))
	}

	// Add top violated ADR trend if results available
	if len(results) > 0 {
		top := TopViolatedADRs(results, 1, threshold)
		if len(top) > 0 {
			freq := PerADRViolationFrequency(results, threshold)
			b.WriteString(fmt.Sprintf("\n_Top violated ADR: %s (violated in %.0f%% of recent checks)_\n",
				top[0], freq[top[0]]*100))
		}
	}

	return b.String()
}
