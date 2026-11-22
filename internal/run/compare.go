package run

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/source"
)

// maxEndpoints bounds how many endpoints one run compares. Each endpoint after
// the first repeats every account read, so the cap keeps the request count a
// run can make proportional to its account list rather than open-ended.
const maxEndpoints = 5

// endpoint is one configured endpoint after its network read.
type endpoint struct {
	label string
	url   string
	src   source.Source
	info  model.NetworkInfo
}

// connect builds a source for every endpoint and reads each one's network. Any
// failure ends the run: an endpoint that cannot be read is not a second opinion
// that went quiet, and comparing against fewer endpoints than were asked for
// would print a comparison that did not happen.
func connect(ctx context.Context, factory SourceFactory, urls []string) ([]endpoint, error) {
	out := make([]endpoint, 0, len(urls))
	for i, url := range urls {
		label := "endpoints[" + strconv.Itoa(i) + "]"
		src, err := factory(url)
		if err != nil {
			return nil, fmt.Errorf("run: building the source for %s: %w", label, err)
		}
		info, err := src.Network(ctx)
		if err != nil {
			return nil, fmt.Errorf("run: reading the network of %s: %w", label, err)
		}
		out = append(out, endpoint{label: label, url: url, src: src, info: info})
	}
	return out, nil
}

// crossRead reads every account from every endpoint after the first and returns,
// per account, one src-disagreement finding for each endpoint whose reading
// differs from the first endpoint's. The first endpoint's snapshot stays the one
// the rules evaluate; the others are compared and never merged into it, because
// a disagreement is reported rather than resolved. Endpoints are read one after
// another through the same bounded pool, so a run never holds more requests open
// than its concurrency allows. Nothing is read when any endpoint is on another
// network.
func crossRead(ctx context.Context, endpoints []endpoint, accounts []policy.Account, limit int, primary []observation, mismatch bool) ([][]model.Finding, error) {
	if mismatch || len(endpoints) < 2 {
		return nil, nil
	}
	out := make([][]model.Finding, len(accounts))
	for _, other := range endpoints[1:] {
		observed, err := fetchAll(ctx, other.src, other.url, accounts, limit)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", other.label, err)
		}
		for i := range accounts {
			fields := source.Compare(reading(primary[i]), reading(observed[i]))
			if len(fields) == 0 {
				continue
			}
			out[i] = append(out[i], disagreement(accounts[i].ID, endpoints[0].label, other.label, fields))
		}
	}
	return out, nil
}

func reading(obs observation) source.Reading {
	return source.Reading{Snapshot: obs.snapshot, Missing: obs.missing}
}

// disagreement names the two endpoints by their position in the configuration
// rather than by URL, and the fields by name rather than by value, so the
// fingerprint stays fixed while a stale replica keeps lagging and no
// chain-derived string reaches evidence.
func disagreement(account, primary, other string, fields []string) model.Finding {
	return model.NewFinding(
		"src-disagreement",
		model.SevHigh,
		account,
		"two endpoints returned different state for this account, so neither reading can be trusted on its own; one of them is stale or filtering",
		other+" disagrees with "+primary+" on: "+strings.Join(fields, ", "),
	)
}
