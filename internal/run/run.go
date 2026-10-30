package run

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/report"
	"github.com/posture-sec/guage/internal/rules"
	"github.com/posture-sec/guage/internal/source"
)

const defaultConcurrency = 8

// ErrUsage reports a request the orchestrator will not run.
var ErrUsage = errors.New("run: invalid request")

// Evaluator returns the findings for one observation. It must be pure: no I/O,
// no globals, no time and no randomness, so the same observation and policy
// always produce the same findings.
type Evaluator func(rules.Context) []model.Finding

// SourceFactory builds one source for an endpoint. Tests inject fakes here.
type SourceFactory func(endpoint string) (source.Source, error)

// Job is everything one run needs. Check and Audit build it from their own
// inputs, and tests build it directly.
type Job struct {
	ToolVersion string
	Network     string
	Endpoints   []string
	Accounts    []policy.Account
	Accepts     []policy.Acceptance
	FailOn      model.Severity

	Concurrency int
	NewSource   SourceFactory
	Evaluate    Evaluator
	Now         func() time.Time
}

// Run verifies the endpoint's network, fetches every account with a bounded
// worker pool, evaluates rules, applies accepted risks and maps the result to
// an exit code. It returns an error only when no trustworthy report exists.
func Run(ctx context.Context, job Job) (model.Report, error) {
	if err := job.validate(); err != nil {
		return model.Report{}, err
	}
	now := job.Now
	if now == nil {
		now = time.Now
	}
	factory := job.NewSource
	if factory == nil {
		factory = horizonFactory(job.ToolVersion)
	}

	src, err := factory(job.Endpoints[0])
	if err != nil {
		return model.Report{}, fmt.Errorf("run: building the source: %w", err)
	}
	info, err := src.Network(ctx)
	if err != nil {
		return model.Report{}, fmt.Errorf("run: reading the network: %w", err)
	}
	// A mismatch is a finding about the endpoint, not a transport failure, so
	// the run reports it per account instead of stopping. Nothing is read from
	// an endpoint on the wrong network.
	mismatch := false
	if err := source.CheckPassphrase(info, job.Network); err != nil {
		if !errors.Is(err, source.ErrPassphraseMismatch) {
			return model.Report{}, fmt.Errorf("run: %w", err)
		}
		mismatch = true
	}

	observations, err := observe(ctx, src, job.Endpoints[0], job.Accounts, job.Concurrency, mismatch)
	if err != nil {
		return model.Report{}, fmt.Errorf("run: %w", err)
	}

	result := model.Report{
		ToolVersion: job.ToolVersion,
		Network:     info.Passphrase,
		Ledger:      info.Ledger,
		Accounts:    make([]model.AccountReport, 0, len(job.Accounts)),
	}
	panicked := false
	for i, obs := range observations {
		snapshot := obs.snapshot
		snapshot.Network = info.Passphrase
		findings, recovered := evaluate(job.Evaluate, rules.Context{
			Account:  job.Accounts[i],
			Snapshot: snapshot,
			Missing:  obs.missing,
			Mismatch: mismatch,
			Network:  info.Passphrase,
			Expected: job.Network,
		})
		panicked = panicked || recovered
		applyAccepts(findings, job.Accepts, now())
		model.SortFindings(findings)
		report := model.AccountReport{Snapshot: snapshot, Findings: findings, Missing: obs.missing}
		if mismatch {
			report.Skipped = "the endpoint is on a different network"
		}
		result.Accounts = append(result.Accounts, report)
	}

	result.ExitCode = report.Exit(result, job.FailOn)
	if panicked {
		result.ExitCode = model.ExitSource
	}
	return result, nil
}

func (job Job) validate() error {
	if job.Network == "" {
		return fmt.Errorf("%w: no network", ErrUsage)
	}
	if len(job.Endpoints) == 0 {
		return fmt.Errorf("%w: no endpoint", ErrUsage)
	}
	if len(job.Endpoints) > 1 {
		return fmt.Errorf("%w: multi-endpoint comparison is not implemented", ErrUsage)
	}
	if len(job.Accounts) == 0 {
		return fmt.Errorf("%w: no accounts", ErrUsage)
	}
	if job.Concurrency < 0 {
		return fmt.Errorf("%w: concurrency cannot be negative", ErrUsage)
	}
	if job.FailOn < model.SevInfo || job.FailOn > model.SevHigh {
		return fmt.Errorf("%w: unknown failure threshold", ErrUsage)
	}
	return nil
}

func horizonFactory(version string) SourceFactory {
	agent := "stellar-gauge"
	if version != "" {
		agent += "/" + version
	}
	return func(endpoint string) (source.Source, error) {
		return source.NewHorizon(source.Options{Endpoint: endpoint, UserAgent: agent})
	}
}

// observation is one account's result: a snapshot, or the fact that the
// endpoint reported the account as missing.
type observation struct {
	snapshot model.AccountSnapshot
	missing  bool
}

// observe keeps the policy order, runs at most limit fetches at a time and
// stops every worker on the first failure that is not a missing account. A
// missing account is observation, not an orchestration failure, because
// acct-missing is a finding. When the endpoint is on the wrong network nothing
// is fetched at all.
func observe(ctx context.Context, src source.Source, endpoint string, accounts []policy.Account, limit int, mismatch bool) ([]observation, error) {
	if mismatch {
		out := make([]observation, len(accounts))
		for i, account := range accounts {
			out[i].snapshot = model.AccountSnapshot{ID: account.ID, Endpoint: endpoint}
		}
		return out, nil
	}
	return fetchAll(ctx, src, endpoint, accounts, limit)
}

func fetchAll(ctx context.Context, src source.Source, endpoint string, accounts []policy.Account, limit int) ([]observation, error) {
	if limit <= 0 {
		limit = defaultConcurrency
	}
	observed := make([]observation, len(accounts))
	fetchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	var once sync.Once
	var firstErr error
	fail := func(err error) {
		once.Do(func() {
			firstErr = err
			cancel()
		})
	}

	for i := range accounts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-fetchCtx.Done():
				fail(fetchCtx.Err())
				return
			}
			defer func() { <-sem }()
			snapshot, err := src.Account(fetchCtx, accounts[i].ID)
			if errors.Is(err, source.ErrNotFound) {
				observed[i] = observation{
					snapshot: model.AccountSnapshot{ID: accounts[i].ID, Endpoint: endpoint},
					missing:  true,
				}
				return
			}
			if err != nil {
				fail(fmt.Errorf("account %s: %w", accounts[i].ID, err))
				return
			}
			observed[i] = observation{snapshot: snapshot}
		}(i)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return observed, nil
}

func evaluate(evaluator Evaluator, c rules.Context) (findings []model.Finding, recovered bool) {
	if evaluator == nil {
		return nil, false
	}
	defer func() {
		if recover() != nil {
			findings = []model.Finding{model.NewFinding(
				"src-rule-panic",
				model.SevHigh,
				c.Snapshot.ID,
				"a rule panicked, so this account's result is incomplete",
				"rule evaluation recovered from a panic",
			)}
			recovered = true
		}
	}()
	return evaluator(c), false
}

func applyAccepts(findings []model.Finding, accepts []policy.Acceptance, now time.Time) {
	for i := range findings {
		if !policy.Acceptable(findings[i].RuleID) {
			continue
		}
		for _, accept := range accepts {
			if accept.Rule != findings[i].RuleID || accept.Account != findings[i].Account {
				continue
			}
			if !acceptLive(accept, now) {
				continue
			}
			findings[i].Accepted = true
			break
		}
	}
}

// acceptLive treats the expiry date as the last day the acceptance holds.
func acceptLive(accept policy.Acceptance, now time.Time) bool {
	if accept.Expires == nil {
		return true
	}
	if _, err := time.Parse("2006-01-02", *accept.Expires); err != nil {
		return false
	}
	return now.UTC().Format("2006-01-02") <= *accept.Expires
}
