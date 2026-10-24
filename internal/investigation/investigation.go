package investigation

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
)

type Hypothesis struct {
	Explanation string   `json:"explanation"`
	EvidenceIDs []string `json:"evidence_ids"`
}
type Result struct {
	Summary            string       `json:"summary"`
	Hypotheses         []Hypothesis `json:"hypotheses"`
	RecommendedActions []string     `json:"recommended_actions"`
	EvidenceIDs        []string     `json:"evidence_ids"`
	Limitations        []string     `json:"limitations"`
}

func SnapshotHash(ids []string) string {
	copyIDs := append([]string(nil), ids...)
	sort.Strings(copyIDs)
	hash := sha256.Sum256([]byte(join(copyIDs)))
	return hex.EncodeToString(hash[:])
}
func join(ids []string) string {
	out := ""
	for _, id := range ids {
		out += id + "\n"
	}
	return out
}

// ValidateResult is the independent evidence gate used for any model adapter.
func ValidateResult(result Result, allowedIDs []string) error {
	if result.Summary == "" || len(result.Limitations) == 0 {
		return errors.New("investigation content incomplete")
	}
	allowed := map[string]bool{}
	for _, id := range allowedIDs {
		allowed[id] = true
	}
	for _, id := range result.EvidenceIDs {
		if !allowed[id] {
			return errors.New("fabricated evidence ID")
		}
	}
	for _, hypothesis := range result.Hypotheses {
		if hypothesis.Explanation == "" {
			return errors.New("empty hypothesis")
		}
		for _, id := range hypothesis.EvidenceIDs {
			if !allowed[id] {
				return errors.New("fabricated hypothesis evidence ID")
			}
		}
	}
	return nil
}

// Offline generates a labeled local explanation and has no write capability.
func Offline(kind string, evidenceIDs []string) Result {
	summary := "Synthetic provider observations need human review."
	explanation := "The stored evidence does not establish a real-world bank balance."
	if kind == "SETTLEMENT_NOT_OBSERVED" {
		summary = "An accepted synthetic payment has no matching observed settlement through the completed source window."
		explanation = "The simulated settlement may be delayed; check the provider reference and later observations."
	}
	if kind == "AMOUNT_MISMATCH" {
		summary = "Synthetic payment and settlement amounts differ for the same transfer reference."
		explanation = "The amount discrepancy needs source verification by a reviewer."
	}
	copyIDs := append([]string(nil), evidenceIDs...)
	result := Result{Summary: summary, Hypotheses: []Hypothesis{{Explanation: explanation, EvidenceIDs: copyIDs}}, RecommendedActions: []string{"Review the cited simulated records and request provider confirmation if needed."}, EvidenceIDs: copyIDs, Limitations: []string{"OFFLINE_STUB: deterministic local analysis of synthetic evidence; no Bedrock invocation or real bank confirmation."}}
	return result
}
