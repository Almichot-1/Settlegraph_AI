package reconcile

import (
	"reflect"
	"testing"
	"time"
)

func TestTwoStageMatchingAndCoverage(t *testing.T) {
	start := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	orders := []Order{{ID: "o1", Reference: "ORD-1", Amount: 250000, Currency: "ETB"}, {ID: "o2", Reference: "ORD-2", Amount: 250000, Currency: "ETB"}}
	payment := Observation{ID: "p1", Kind: "PAYMENT_ACCEPTED", TransferReference: "SIM-1", OrderReference: "ORD-1", Amount: 250000, Currency: "ETB", OccurredAt: start}
	settlement := Observation{ID: "s1", Kind: "SETTLEMENT_POSTED", TransferReference: "SIM-1", PaymentReference: "SIM-1", Amount: 250000, Currency: "ETB", OccurredAt: start.Add(2 * time.Hour)}
	grace := time.Hour
	got := Evaluate(orders, []Observation{payment}, start.Add(2*time.Hour), start.Add(30*time.Minute), true, grace)
	if len(got.Missing) != 0 || !reflect.DeepEqual(got.OrderPayment, []Pair{{"o1", "p1"}}) {
		t.Fatalf("incomplete coverage: %+v", got)
	}
	got = Evaluate(orders, []Observation{payment}, start.Add(2*time.Hour), start.Add(2*time.Hour), true, grace)
	if !reflect.DeepEqual(got.Missing, []string{"p1"}) {
		t.Fatalf("complete coverage: %+v", got)
	}
	got = Evaluate(orders, []Observation{settlement, payment}, start.Add(2*time.Hour), start.Add(2*time.Hour), true, grace)
	if len(got.Missing) != 0 || !reflect.DeepEqual(got.PaymentSettlement, []Pair{{"p1", "s1"}}) {
		t.Fatalf("settlement-first convergence: %+v", got)
	}
	settlement.Amount = 249999
	got = Evaluate(orders, []Observation{payment, settlement}, start.Add(2*time.Hour), start.Add(2*time.Hour), true, grace)
	if !reflect.DeepEqual(got.Mismatched, []string{"p1"}) || len(got.PaymentSettlement) != 0 {
		t.Fatalf("one-unit mismatch: %+v", got)
	}
}

func TestNeverMatchByAmountAlone(t *testing.T) {
	start := time.Now().UTC()
	orders := []Order{{ID: "o1", Reference: "ORD-1", Amount: 250000, Currency: "ETB"}}
	payment := Observation{ID: "p1", Kind: "PAYMENT_ACCEPTED", OrderReference: "UNRELATED", TransferReference: "SIM-1", Amount: 250000, Currency: "ETB", OccurredAt: start}
	got := Evaluate(orders, []Observation{payment}, start.Add(2*time.Hour), start.Add(2*time.Hour), true, time.Hour)
	if len(got.OrderPayment) != 0 {
		t.Fatalf("amount-only match: %+v", got)
	}
	orders = append(orders, Order{ID: "o2", Reference: "UNRELATED", Amount: 250000, Currency: "ETB"}, Order{ID: "o3", Reference: "UNRELATED", Amount: 250000, Currency: "ETB"})
	got = Evaluate(orders, []Observation{payment}, start.Add(2*time.Hour), start.Add(2*time.Hour), true, time.Hour)
	if len(got.OrderPayment) != 0 || len(got.Ambiguous) != 1 {
		t.Fatalf("ambiguous allocation: %+v", got)
	}
}
