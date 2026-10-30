package report

import (
	"encoding/json"
	"io"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/sanitize"
)

// JSON writes the report as stable, indented JSON with severity names and a
// limits field. The input is copied and sanitized first, so rendering never
// changes the report the caller holds.
func JSON(w io.Writer, r model.Report) error {
	document := struct {
		model.Report
		Limits string `json:"limits"`
	}{prepare(r), Limits}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(document)
}

// prepare returns a sanitized copy of the report with findings sorted and
// empty collections rendered as empty arrays rather than null.
func prepare(r model.Report) model.Report {
	out := r
	out.Accounts = make([]model.AccountReport, len(r.Accounts))
	for i, ar := range r.Accounts {
		snapshot := ar.Snapshot
		snapshot.ID = sanitize.Text(snapshot.ID)
		snapshot.Sequence = sanitize.Text(snapshot.Sequence)
		snapshot.HomeDomain, _ = sanitize.Domain(snapshot.HomeDomain)
		snapshot.Sponsor = sanitize.Text(snapshot.Sponsor)

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
			f.Account = sanitize.Text(f.Account)
			f.Message = sanitize.Text(f.Message)
			f.Evidence = sanitize.Text(f.Evidence)
			findings[j] = f
		}
		model.SortFindings(findings)
		out.Accounts[i] = model.AccountReport{
			Snapshot: snapshot,
			Findings: findings,
			Missing:  ar.Missing,
			Skipped:  sanitize.Text(ar.Skipped),
		}
	}
	return out
}
