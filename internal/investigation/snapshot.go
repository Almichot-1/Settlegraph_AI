package investigation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"
)

const PromptVersion = "evidence-v1"

var ErrEvidence = errors.New("EVIDENCE_INVALID")

type Record struct {
	ID              string     `json:"id"`
	Kind            string     `json:"kind"`
	Reference       string     `json:"reference,omitempty"`
	ProviderID      string     `json:"provider_id,omitempty"`
	AmountMinor     string     `json:"amount_minor,omitempty"`
	Currency        string     `json:"currency,omitempty"`
	OccurredAt      time.Time  `json:"occurred_at"`
	SourceStatus    string     `json:"source_status,omitempty"`
	CompleteThrough *time.Time `json:"complete_through,omitempty"`
	PayloadHash     string     `json:"payload_sha256,omitempty"`
}
type Packet struct {
	CaseID      string   `json:"case_id"`
	CaseKind    string   `json:"case_kind"`
	CaseVersion int      `json:"case_version"`
	Evidence    []Record `json:"evidence"`
}

func (p Packet) IDs() []string {
	ids := make([]string, 0, len(p.Evidence))
	for _, r := range p.Evidence {
		ids = append(ids, r.ID)
	}
	return ids
}
func (p Packet) Canonical() ([]byte, error) {
	if p.CaseID == "" || p.CaseVersion < 1 || len(p.Evidence) == 0 || len(p.Evidence) > 100 {
		return nil, ErrEvidence
	}
	seen := map[string]bool{}
	for _, r := range p.Evidence {
		if r.ID == "" || seen[r.ID] {
			return nil, ErrEvidence
		}
		seen[r.ID] = true
	}
	p.Evidence = append([]Record(nil), p.Evidence...)
	sort.Slice(p.Evidence, func(i, j int) bool {
		return p.Evidence[i].Kind+":"+p.Evidence[i].ID < p.Evidence[j].Kind+":"+p.Evidence[j].ID
	})
	raw, err := json.Marshal(p)
	if err != nil || len(raw) > 65536 {
		return nil, ErrEvidence
	}
	return raw, nil
}
func (p Packet) Hash() (string, error) {
	raw, err := p.Canonical()
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:]), nil
}

// LoadPacket reads only product evidence explicitly attached to the chosen
// tenant's case. Provider descriptions and private simulator data are excluded.
func LoadPacket(ctx context.Context, tx *sql.Tx, org, caseID, kind string, version int) (Packet, error) {
	p := Packet{CaseID: caseID, CaseKind: kind, CaseVersion: version, Evidence: []Record{}}
	rows, err := tx.QueryContext(ctx, `SELECT evidence_kind,evidence_id FROM case_evidence WHERE organization_id=$1 AND case_id=$2 ORDER BY evidence_kind,evidence_id LIMIT 101`, org, caseID)
	if err != nil {
		return p, err
	}
	var records []Record
	for rows.Next() {
		var r Record
		if err = rows.Scan(&r.Kind, &r.ID); err != nil {
			rows.Close()
			return p, err
		}
		records = append(records, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return p, err
	}
	if len(records) == 0 || len(records) > 100 {
		return p, ErrEvidence
	}
	for _, r := range records {
		loaded, err := loadRecord(ctx, tx, org, r)
		if err != nil {
			return p, err
		}
		p.Evidence = append(p.Evidence, loaded)
	}
	if _, err = p.Canonical(); err != nil {
		return p, err
	}
	return p, nil
}

// VerifyPacket permits later appended evidence, but every cited historical
// record must still belong to this case and retain the same contents.
func VerifyPacket(ctx context.Context, tx *sql.Tx, org string, p Packet) error {
	var actualKind string
	err := tx.QueryRowContext(ctx, "SELECT kind FROM investigation_cases WHERE organization_id=$1 AND id=$2", org, p.CaseID).Scan(&actualKind)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrEvidence
	}
	if err != nil {
		return err
	}
	if actualKind != p.CaseKind {
		return ErrEvidence
	}
	for _, record := range p.Evidence {
		kind := record.Kind
		if kind != "ORDER" && kind != "SOURCE_STATUS" {
			kind = "OBSERVATION"
		}
		var member bool
		err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM case_evidence WHERE organization_id=$1 AND case_id=$2 AND evidence_kind=$3 AND evidence_id=$4)`, org, p.CaseID, kind, record.ID).Scan(&member)
		if err != nil {
			return err
		}
		if !member {
			return ErrEvidence
		}
		current, err := loadRecord(ctx, tx, org, Record{ID: record.ID, Kind: kind})
		if err != nil {
			return err
		}
		before, _ := json.Marshal(record)
		after, _ := json.Marshal(current)
		if string(before) != string(after) {
			return ErrEvidence
		}
	}
	return nil
}

func loadRecord(ctx context.Context, tx *sql.Tx, org string, r Record) (Record, error) {
	var err error
	switch r.Kind {
	case "ORDER":
		err = tx.QueryRowContext(ctx, `SELECT external_reference,amount_minor::text,currency,created_at FROM merchant_orders WHERE organization_id=$1 AND id=$2`, org, r.ID).Scan(&r.Reference, &r.AmountMinor, &r.Currency, &r.OccurredAt)
	case "OBSERVATION":
		err = tx.QueryRowContext(ctx, `SELECT o.kind,o.transfer_reference,o.provider_id,o.amount_minor::text,o.currency,o.occurred_at,i.payload_sha256 FROM observations o JOIN inbox_events i ON i.organization_id=o.organization_id AND i.id=o.inbox_event_id WHERE o.organization_id=$1 AND o.id=$2`, org, r.ID).Scan(&r.Kind, &r.Reference, &r.ProviderID, &r.AmountMinor, &r.Currency, &r.OccurredAt, &r.PayloadHash)
	case "SOURCE_STATUS":
		var complete sql.NullTime
		err = tx.QueryRowContext(ctx, `SELECT provider_id,source_status,complete_through,observed_at FROM source_status_snapshots WHERE organization_id=$1 AND id=$2`, org, r.ID).Scan(&r.ProviderID, &r.SourceStatus, &complete, &r.OccurredAt)
		if complete.Valid {
			v := complete.Time.UTC()
			r.CompleteThrough = &v
		}
	default:
		return r, ErrEvidence
	}
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrEvidence
	}
	if err != nil {
		return r, err
	}
	if len(r.Reference) > 256 {
		return r, ErrEvidence
	}
	r.OccurredAt = r.OccurredAt.UTC()
	return r, nil
}
