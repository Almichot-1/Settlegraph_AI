package investigation

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

type Output struct {
	Result       Result
	InvocationID string
	ModelID      string
	InputTokens  int
	OutputTokens int
}
type Analyzer interface {
	Provider() string
	Analyze(context.Context, Packet) (Output, error)
}
type OfflineAnalyzer struct{}

func (OfflineAnalyzer) Provider() string { return "OFFLINE_STUB" }
func (OfflineAnalyzer) Analyze(_ context.Context, p Packet) (Output, error) {
	return Output{Result: Offline(p.CaseKind, p.IDs()), ModelID: "offline-v1"}, nil
}

var ErrOutput = errors.New("OUTPUT_REJECTED")
var disallowedSettlement = regexp.MustCompile(`(?i)\bsettled\b|\bsettlement (is|was|has been) (confirmed|completed|posted)\b|\bconfirmed (bank )?balance\b`)
var ErrProvider = errors.New("PROVIDER_UNAVAILABLE")

func ValidatePacketResult(result Result, p Packet) error {
	if err := ValidateResult(result, p.IDs()); err != nil {
		return ErrOutput
	}
	if len(result.EvidenceIDs) == 0 || len(result.Hypotheses) == 0 || len(result.Hypotheses) > 5 || len(result.RecommendedActions) == 0 || len(result.RecommendedActions) > 8 || len(result.Limitations) == 0 || len(result.Limitations) > 8 {
		return ErrOutput
	}
	texts := []string{result.Summary}
	for _, h := range result.Hypotheses {
		if len(h.EvidenceIDs) == 0 {
			return ErrOutput
		}
		texts = append(texts, h.Explanation)
	}
	texts = append(texts, result.RecommendedActions...)
	texts = append(texts, result.Limitations...)
	for _, text := range texts {
		if strings.TrimSpace(text) == "" || len(text) > 2000 {
			return ErrOutput
		}
		if disallowedSettlement.MatchString(text) {
			return ErrOutput
		}
		normalized := strings.ToLower(text)
		for _, claim := range []string{"funds stolen", "fraud", "payment is settled", "funds are settled", "confirmed settlement", "case closed", "case resolved"} {
			if strings.Contains(normalized, claim) {
				return ErrOutput
			}
		}
	}
	if len(result.EvidenceIDs) > 100 {
		return ErrOutput
	}
	return nil
}
