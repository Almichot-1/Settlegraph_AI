package investigation

import "testing"

func TestFabricatedEvidenceIsRejected(t *testing.T) {
	allowed := []string{"55555555-5555-4555-8555-555555555555"}
	result := Offline("SETTLEMENT_NOT_OBSERVED", allowed)
	if err := ValidateResult(result, allowed); err != nil {
		t.Fatal(err)
	}
	result.Hypotheses[0].EvidenceIDs = []string{"99999999-9999-4999-8999-999999999999"}
	if err := ValidateResult(result, allowed); err == nil {
		t.Fatal("fabricated hypothesis ID accepted")
	}
}
