package reconcile

import "time"

type Order struct {
	ID, Reference, Currency string
	Amount                  int64
}
type Observation struct {
	ID, Kind, TransferReference, PaymentReference, OrderReference, Currency string
	Amount                                                                  int64
	OccurredAt                                                              time.Time
}
type Pair struct{ Left, Right string }
type Result struct {
	OrderPayment      []Pair
	PaymentSettlement []Pair
	Missing           []string
	Mismatched        []string
	Ambiguous         []string
}

// Evaluate is pure and deterministic. It never matches on amount alone and
// treats accepted payment and posted settlement as separate evidence stages.
func Evaluate(orders []Order, events []Observation, now, completeThrough time.Time, sourceHealthy bool, grace time.Duration) Result {
	result := Result{OrderPayment: []Pair{}, PaymentSettlement: []Pair{}, Missing: []string{}, Mismatched: []string{}, Ambiguous: []string{}}
	payments := []Observation{}
	settlements := []Observation{}
	for _, event := range events {
		switch event.Kind {
		case "PAYMENT_ACCEPTED":
			payments = append(payments, event)
		case "SETTLEMENT_POSTED":
			settlements = append(settlements, event)
		}
	}
	orderPaymentCount := map[string]int{}
	paymentRefCount := map[string]int{}
	for _, payment := range payments {
		paymentRefCount[payment.TransferReference]++
		for _, order := range orders {
			if order.Reference == payment.OrderReference && order.Amount == payment.Amount && order.Currency == payment.Currency {
				orderPaymentCount[order.ID]++
			}
		}
	}
	usedOrders := map[string]bool{}
	usedSettlements := map[string]bool{}
	for _, payment := range payments {
		candidates := []Order{}
		for _, order := range orders {
			if order.Reference == payment.OrderReference && order.Amount == payment.Amount && order.Currency == payment.Currency {
				candidates = append(candidates, order)
			}
		}
		if len(candidates) == 1 && orderPaymentCount[candidates[0].ID] == 1 && !usedOrders[candidates[0].ID] {
			result.OrderPayment = append(result.OrderPayment, Pair{candidates[0].ID, payment.ID})
			usedOrders[candidates[0].ID] = true
		} else if len(candidates) > 1 || (len(candidates) == 1 && orderPaymentCount[candidates[0].ID] > 1) {
			result.Ambiguous = append(result.Ambiguous, payment.ID)
		}
		match := []Observation{}
		refCount := 0
		for _, settlement := range settlements {
			if settlement.TransferReference == payment.TransferReference && settlement.PaymentReference == payment.TransferReference && settlement.Currency == payment.Currency {
				refCount++
				if settlement.Amount == payment.Amount {
					match = append(match, settlement)
				}
			}
		}
		if len(match) == 1 && refCount == 1 && paymentRefCount[payment.TransferReference] == 1 && !usedSettlements[match[0].ID] {
			result.PaymentSettlement = append(result.PaymentSettlement, Pair{payment.ID, match[0].ID})
			usedSettlements[match[0].ID] = true
			continue
		}
		if paymentRefCount[payment.TransferReference] > 1 {
			result.Ambiguous = append(result.Ambiguous, payment.ID)
			continue
		}
		if refCount > 1 {
			result.Ambiguous = append(result.Ambiguous, payment.ID)
			continue
		}
		if refCount == 1 {
			result.Mismatched = append(result.Mismatched, payment.ID)
			continue
		}
		due := payment.OccurredAt.Add(grace)
		if sourceHealthy && !now.Before(due) && !completeThrough.Before(due) {
			result.Missing = append(result.Missing, payment.ID)
		}
	}
	return result
}
