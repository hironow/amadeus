package domain

import (
	"strings"
	"testing"
)

func TestDeriveADRIntegrityScore_Empty(t *testing.T) {
	if got := DeriveADRIntegrityScore(nil); got != 0 {
		t.Errorf("nil map: got %d, want 0", got)
	}
	if got := DeriveADRIntegrityScore(ADRAlignmentMap{}); got != 0 {
		t.Errorf("empty map: got %d, want 0", got)
	}
}

func TestDeriveADRIntegrityScore_SingleADR(t *testing.T) {
	m := ADRAlignmentMap{"0001": {Score: 75}}
	if got := DeriveADRIntegrityScore(m); got != 75 {
		t.Errorf("got %d, want 75", got)
	}
}

func TestDeriveADRIntegrityScore_MultipleADRs(t *testing.T) {
	m := ADRAlignmentMap{
		"0001": {Score: 80},
		"0002": {Score: 40},
		"0003": {Score: 60},
	}
	// (80+40+60)/3 = 60
	if got := DeriveADRIntegrityScore(m); got != 60 {
		t.Errorf("got %d, want 60", got)
	}
}

func TestDeriveADRIntegrityScore_ClampsScores(t *testing.T) {
	m := ADRAlignmentMap{
		"0001": {Score: 150}, // over 100
		"0002": {Score: -10}, // under 0
	}
	// (100+0)/2 = 50
	if got := DeriveADRIntegrityScore(m); got != 50 {
		t.Errorf("got %d, want 50", got)
	}
}

func TestPerADRViolationFrequency(t *testing.T) {
	results := []CheckResult{
		{ADRAlignment: ADRAlignmentMap{"0001": {Score: 80}, "0002": {Score: 30}}},
		{ADRAlignment: ADRAlignmentMap{"0001": {Score: 70}, "0002": {Score: 60}}},
		{ADRAlignment: ADRAlignmentMap{"0001": {Score: 90}, "0002": {Score: 20}}},
	}
	freq := PerADRViolationFrequency(results, 50)
	// 0001: 3/3 = 1.0, 0002: 1/3 ≈ 0.333
	if freq["0001"] != 1.0 {
		t.Errorf("0001 freq = %f, want 1.0", freq["0001"])
	}
	if freq["0002"] < 0.33 || freq["0002"] > 0.34 {
		t.Errorf("0002 freq = %f, want ~0.333", freq["0002"])
	}
}

func TestPerADRViolationFrequency_NilAlignment(t *testing.T) {
	results := []CheckResult{
		{}, // no ADRAlignment
	}
	freq := PerADRViolationFrequency(results, 50)
	if len(freq) != 0 {
		t.Errorf("expected empty freq for nil alignment, got %v", freq)
	}
}

func TestTopViolatedADRs(t *testing.T) {
	results := []CheckResult{
		{ADRAlignment: ADRAlignmentMap{"0001": {Score: 80}, "0002": {Score: 30}, "0003": {Score: 90}}},
		{ADRAlignment: ADRAlignmentMap{"0001": {Score: 70}, "0002": {Score: 60}, "0003": {Score: 80}}},
		{ADRAlignment: ADRAlignmentMap{"0001": {Score: 90}, "0002": {Score: 20}, "0003": {Score: 70}}},
	}
	top := TopViolatedADRs(results, 2, 50)
	// 0001: 3/3, 0003: 3/3, 0002: 1/3 → top 2 = [0001, 0003] (sorted by num on tie)
	if len(top) != 2 {
		t.Fatalf("expected 2, got %d", len(top))
	}
	if top[0] != "0001" || top[1] != "0003" {
		t.Errorf("got %v, want [0001, 0003]", top)
	}
}

func TestFormatViolatedADRsSection_NoViolations(t *testing.T) {
	m := ADRAlignmentMap{
		"0001": {Score: 10, Verdict: "compliant"},
	}
	if got := FormatViolatedADRsSection(m, nil, 50); got != "" {
		t.Errorf("expected empty for no violations, got %q", got)
	}
}

func TestFormatViolatedADRsSection_WithViolations(t *testing.T) {
	m := ADRAlignmentMap{
		"0002": {Number: "0002", Title: "Scoring", Score: 72, Verdict: "violated", Reason: "weights changed"},
		"0009": {Number: "0009", Title: "Flat Pkg", Score: 61, Verdict: "violated", Reason: "sub-pkg added"},
		"0001": {Number: "0001", Title: "Pipeline", Score: 10, Verdict: "compliant", Reason: "ok"},
	}
	got := FormatViolatedADRsSection(m, nil, 50)
	if !strings.Contains(got, "## Violated ADRs") {
		t.Error("missing header")
	}
	if !strings.Contains(got, "0002") || !strings.Contains(got, "0009") {
		t.Error("missing violated ADR numbers")
	}
	if strings.Contains(got, "0001") {
		t.Error("compliant ADR 0001 should not appear")
	}
}
