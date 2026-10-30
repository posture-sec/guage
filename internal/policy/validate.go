package policy

import (
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/strkey"
)

func (c *Config) validate() error {
	if c.Version != 1 {
		return ErrVersion
	}
	if !validNetwork(c.Network) {
		return ErrNetwork
	}
	for i, endpoint := range c.Endpoints {
		if !validEndpoint(endpoint) {
			return withDetail(ErrEndpoint, "endpoints["+strconv.Itoa(i)+"]")
		}
	}

	seen := make(map[string]bool, len(c.Accounts))
	for i := range c.Accounts {
		if err := c.Accounts[i].validate(i); err != nil {
			return err
		}
		if seen[c.Accounts[i].ID] {
			return withDetail(ErrDuplicateAccount, "account["+strconv.Itoa(i)+"].id")
		}
		seen[c.Accounts[i].ID] = true
	}

	accepted := make(map[string]bool, len(c.Accepts))
	for i := range c.Accepts {
		if err := c.Accepts[i].validate(i); err != nil {
			return err
		}
		key := c.Accepts[i].Rule + "\x00" + c.Accepts[i].Account
		if accepted[key] {
			return withDetail(ErrDuplicateAccept, "accept["+strconv.Itoa(i)+"]")
		}
		accepted[key] = true
	}
	return nil
}

func validNetwork(network string) bool {
	switch network {
	case "mainnet", "testnet":
		return true
	}
	if network == "" || len(network) > 200 || strings.TrimSpace(network) != network {
		return false
	}
	for _, r := range network {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return strings.Contains(network, " ; ")
}

// validEndpoint mirrors the rule in internal/source: https anywhere, plain
// HTTP only for a loopback host, because a config must be able to name every
// endpoint the client itself accepts.
func validEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	if u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return true
	case "http":
		return loopbackHost(u.Hostname())
	}
	return false
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (a *Account) validate(index int) error {
	field := "account[" + strconv.Itoa(index) + "]"
	kind, err := strkey.Classify(a.ID)
	if err != nil || kind != strkey.AccountID {
		return withDetail(ErrNotAccount, field+".id")
	}

	if a.Role == "" {
		a.Role = model.RoleOther
	} else if !validRole(a.Role) {
		return withDetail(ErrRole, field+".role")
	}

	if a.ExpectedSigners != nil {
		seen := make(map[string]bool, len(*a.ExpectedSigners))
		for i, signer := range *a.ExpectedSigners {
			if !validSignerKey(signer.Key) {
				return withDetail(ErrSignerKey, field+".expected_signers["+strconv.Itoa(i)+"].key")
			}
			if signer.Weight < 0 || signer.Weight > 255 {
				return withDetail(ErrWeight, field+".expected_signers["+strconv.Itoa(i)+"].weight")
			}
			if seen[signer.Key] {
				return withDetail(ErrDuplicateSigner, field+".expected_signers["+strconv.Itoa(i)+"].key")
			}
			seen[signer.Key] = true
		}
	}

	if t := a.ExpectedThresholds; t != nil {
		for _, v := range []struct {
			name  string
			value *int
		}{
			{"low", t.Low},
			{"medium", t.Medium},
			{"high", t.High},
		} {
			if v.value != nil && (*v.value < 0 || *v.value > 255) {
				return withDetail(ErrThreshold, field+".expected_thresholds."+v.name)
			}
		}
	}

	if a.HomeDomain != nil && !validHomeDomain(*a.HomeDomain) {
		return withDetail(ErrHomeDomain, field+".home_domain")
	}
	return nil
}

func validRole(role model.Role) bool {
	switch role {
	case model.RoleIssuer, model.RoleTreasury, model.RoleDistribution, model.RoleMerchant, model.RoleOther:
		return true
	}
	return false
}

func validSignerKey(key string) bool {
	kind, err := strkey.Classify(key)
	if err != nil {
		return false
	}
	switch kind {
	case strkey.AccountID, strkey.HashX, strkey.PreAuthTx, strkey.SignedPayload:
		return true
	}
	return false
}

func validHomeDomain(domain string) bool {
	if domain == "" || len(domain) > 253 || strings.HasSuffix(domain, ".") {
		return false
	}
	if strings.ContainsAny(domain, " \t\r\n/@:?#[]") {
		return false
	}
	return net.ParseIP(domain) == nil
}

func (a *Acceptance) validate(index int) error {
	field := "accept[" + strconv.Itoa(index) + "]"
	if !validRuleID(a.Rule) || nonAcceptable(a.Rule) {
		return withDetail(ErrAcceptRule, field+".rule")
	}
	if kind, err := strkey.Classify(a.Account); err != nil || kind != strkey.AccountID {
		return withDetail(ErrAccept, field+".account")
	}
	if strings.TrimSpace(a.Reason) == "" {
		return withDetail(ErrAccept, field+".reason")
	}
	if a.Expires != nil {
		if _, err := time.Parse("2006-01-02", *a.Expires); err != nil {
			return withDetail(ErrAccept, field+".expires")
		}
	}
	return nil
}

func validRuleID(rule string) bool {
	if rule == "" || strings.ContainsAny(rule, "*?[] \t") {
		return false
	}
	if strings.HasPrefix(rule, "-") || strings.HasSuffix(rule, "-") || !strings.Contains(rule, "-") {
		return false
	}
	for _, r := range rule {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

// Acceptable reports whether an [[accept]] entry may cover a rule. The
// orchestrator uses it so a hand-built accept cannot silence a structural
// failure such as a missing account or an unreachable threshold tier.
func Acceptable(rule string) bool {
	return !nonAcceptable(rule)
}

// nonAcceptable lists the rules an accepted-risk entry may never silence, from
// ROADMAP task 2.4 and ARCHITECTURE.md 6.6.
func nonAcceptable(rule string) bool {
	switch rule {
	case "acct-missing", "acct-network-mismatch", "sig-threshold-unreachable":
		return true
	}
	return false
}
