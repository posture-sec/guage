package report

import (
	"encoding/json"
	"io"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/sanitize"
)

// SchemaVersion is the version of the JSON envelope written by JSON. It changes
// only when a field name, type or required field changes, not when a new report
// adds an optional field.
const SchemaVersion = 1

// JSON writes the report as stable, indented JSON with severity names and a
// limits field. The input is copied and sanitized first, so rendering never
// changes the report the caller holds.
func JSON(w io.Writer, r model.Report) error {
	document := struct {
		SchemaVersion int `json:"schema_version"`
		model.Report
		Limits string `json:"limits"`
	}{
		SchemaVersion: SchemaVersion,
		Report:        prepare(r),
		Limits:        Limits,
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(document)
}

// prepare returns a sanitized copy of the report with findings sorted and
// empty collections rendered as empty arrays rather than null.
func prepare(r model.Report) model.Report {
	out := r
	out.ToolVersion = sanitize.Text(r.ToolVersion)
	out.Network = sanitize.Text(r.Network)
	out.Accounts = make([]model.AccountReport, len(r.Accounts))
	for i, ar := range r.Accounts {
		snapshot := ar.Snapshot
		snapshot.ID = sanitize.Text(snapshot.ID)
		snapshot.Network = sanitize.Text(snapshot.Network)
		snapshot.Endpoint = sanitize.Text(snapshot.Endpoint)
		snapshot.Sequence = sanitize.Text(snapshot.Sequence)
		snapshot.HomeDomain, _ = sanitize.Domain(snapshot.HomeDomain)
		snapshot.Sponsor = sanitize.Text(snapshot.Sponsor)
		snapshot.Data = sanitizeData(snapshot.Data)

		signers := make([]model.Signer, len(snapshot.Signers))
		for j, s := range snapshot.Signers {
			s.Key = sanitize.Text(s.Key)
			s.Sponsor = sanitize.Text(s.Sponsor)
			s.Type = model.SignerType(sanitize.Text(string(s.Type)))
			signers[j] = s
		}
		snapshot.Signers = signers

		balances := make([]model.Balance, len(snapshot.Balances))
		for j, b := range snapshot.Balances {
			b.AssetCode = sanitize.Text(b.AssetCode)
			b.AssetIssuer = sanitize.Text(b.AssetIssuer)
			balances[j] = b
		}
		snapshot.Balances = balances

		findings := make([]model.Finding, len(ar.Findings))
		for j, f := range ar.Findings {
			f.RuleID = sanitize.Text(f.RuleID)
			f.Account = sanitize.Text(f.Account)
			f.Message = sanitize.Text(f.Message)
			f.Evidence = sanitize.Text(f.Evidence)
			f.Fingerprint = sanitize.Text(f.Fingerprint)
			findings[j] = f
		}
		model.SortFindings(findings)
		out.Accounts[i] = model.AccountReport{
			Snapshot: snapshot,
			Facts:    prepareFacts(ar.Facts),
			Findings: findings,
			Missing:  ar.Missing,
			Skipped:  sanitize.Text(ar.Skipped),
		}
	}
	return out
}

// prepareFacts sanitizes the fact block the same way every other printed string
// is sanitized. Remote content reaches a report through these fields: a
// stellar.toml value, a home domain the fetcher refused, the name an operator gave
// an account. The counts and the booleans are this program's own arithmetic and
// need nothing.
func prepareFacts(facts *model.AccountFacts) *model.AccountFacts {
	if facts == nil {
		return nil
	}
	out := model.AccountFacts{Reachability: facts.Reachability}
	if declared := facts.Declared; declared != nil {
		copied := *declared
		copied.Name = sanitize.Text(copied.Name)
		copied.Role = model.Role(sanitize.Text(string(copied.Role)))
		copied.HomeDomain = sanitizePointer(copied.HomeDomain)
		copied.Coordinator = sanitizePointer(copied.Coordinator)
		out.Declared = &copied
	}
	if identity := facts.Identity; identity != nil {
		copied := *identity
		copied.HomeDomain = sanitize.Text(copied.HomeDomain)
		copied.Step = sanitize.Text(copied.Step)
		copied.Detail = sanitize.Text(copied.Detail)
		copied.NetworkPassphrase = sanitizePointer(copied.NetworkPassphrase)
		copied.SigningKey = sanitizePointer(copied.SigningKey)
		copied.HorizonURL = sanitizePointer(copied.HorizonURL)
		copied.Version = sanitizePointer(copied.Version)
		accounts := make([]string, len(copied.Accounts))
		for i, account := range copied.Accounts {
			accounts[i] = sanitize.Text(account)
		}
		copied.Accounts = accounts
		out.Identity = &copied
	}
	if coord := facts.Coordinator; coord != nil {
		copied := *coord
		copied.DataKey = sanitize.Text(copied.DataKey)
		copied.Domain = sanitize.Text(copied.Domain)
		out.Coordinator = &copied
	}
	if len(facts.Trustlines) > 0 {
		trustlines := make([]model.TrustlineFact, len(facts.Trustlines))
		for i, tl := range facts.Trustlines {
			tl.AssetCode = sanitize.Text(tl.AssetCode)
			tl.AssetIssuer = sanitize.Text(tl.AssetIssuer)
			tl.IssuerStatus = sanitize.Text(tl.IssuerStatus)
			trustlines[i] = tl
		}
		out.Trustlines = trustlines
	}
	return &out
}

func sanitizePointer(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := sanitize.Text(*value)
	return &cleaned
}

func sanitizeData(data map[string]string) map[string]string {
	if data == nil {
		return nil
	}
	clean := make(map[string]string, len(data))
	for key, value := range data {
		clean[sanitize.Text(key)] = sanitize.Text(value)
	}
	return clean
}
