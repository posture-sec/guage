package source

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/strkey"
)

// Network reads the endpoint's own network passphrase and latest ledger.
func (h *Horizon) Network(ctx context.Context) (model.NetworkInfo, error) {
	var wire networkWire
	if err := h.get(ctx, "/", &wire); err != nil {
		return model.NetworkInfo{}, err
	}
	if wire.Passphrase == nil || *wire.Passphrase == "" {
		return model.NetworkInfo{}, fmt.Errorf("source: GET /: %w: no network passphrase", ErrDecode)
	}
	if wire.Ledger == nil || *wire.Ledger <= 0 {
		return model.NetworkInfo{}, fmt.Errorf("source: GET /: %w: no latest ledger", ErrDecode)
	}
	return model.NetworkInfo{Passphrase: *wire.Passphrase, Ledger: *wire.Ledger}, nil
}

// Account reads one account's observed state. It returns ErrNotFound when the
// endpoint does not serve the account, and ErrDecode when any field of the
// response is missing, unknown where Gauge is strict, or out of range.
func (h *Horizon) Account(ctx context.Context, id string) (model.AccountSnapshot, error) {
	if !strkey.IsAccountID(id) {
		return model.AccountSnapshot{}, fmt.Errorf("%w: an account must be a G address", ErrRequest)
	}
	var wire accountWire
	if err := h.get(ctx, "/accounts/"+id, &wire); err != nil {
		return model.AccountSnapshot{}, err
	}
	return h.snapshot(id, wire)
}

func (h *Horizon) snapshot(id string, wire accountWire) (model.AccountSnapshot, error) {
	fail := func(field string) (model.AccountSnapshot, error) {
		return model.AccountSnapshot{}, fmt.Errorf("source: account response: %s: %w", field, ErrDecode)
	}

	if wire.AccountID == "" {
		return fail("no account id")
	}
	if wire.AccountID != id {
		return fail("the endpoint returned a different account")
	}
	if wire.Sequence == nil || !validSequence(*wire.Sequence) {
		return fail("sequence is not a decimal string")
	}
	if wire.Ledger == nil || *wire.Ledger <= 0 {
		return fail("no last modified ledger")
	}

	snapshot := model.AccountSnapshot{
		ID:         wire.AccountID,
		Endpoint:   h.endpoint,
		Ledger:     *wire.Ledger,
		Sequence:   *wire.Sequence,
		HomeDomain: wire.HomeDomain,
		Sponsor:    wire.Sponsor,
	}

	thresholds, err := decodeThresholds(wire.Thresholds)
	if err != nil {
		return fail("thresholds")
	}
	snapshot.Thresholds = thresholds

	flags, err := decodeFlags(wire.Flags)
	if err != nil {
		return fail("flags")
	}
	snapshot.Flags = flags

	signers, err := decodeSigners(wire.Signers)
	if err != nil {
		return fail(err.Error())
	}
	snapshot.Signers = signers

	balances, err := decodeBalances(wire.Balances)
	if err != nil {
		return fail(err.Error())
	}
	snapshot.Balances = balances

	return snapshot, nil
}

func decodeThresholds(raw json.RawMessage) (model.Thresholds, error) {
	var wire struct {
		Low    *int `json:"low_threshold"`
		Medium *int `json:"med_threshold"`
		High   *int `json:"high_threshold"`
	}
	if err := decodeStrict(raw, &wire); err != nil {
		return model.Thresholds{}, err
	}
	if wire.Low == nil || wire.Medium == nil || wire.High == nil {
		return model.Thresholds{}, errors.New("a threshold tier is missing")
	}
	for _, value := range []int{*wire.Low, *wire.Medium, *wire.High} {
		if value < 0 || value > 255 {
			return model.Thresholds{}, errors.New("a threshold is out of range")
		}
	}
	return model.Thresholds{Low: *wire.Low, Medium: *wire.Medium, High: *wire.High}, nil
}

func decodeFlags(raw json.RawMessage) (model.Flags, error) {
	var wire struct {
		AuthRequired    *bool `json:"auth_required"`
		AuthRevocable   *bool `json:"auth_revocable"`
		AuthImmutable   *bool `json:"auth_immutable"`
		ClawbackEnabled *bool `json:"auth_clawback_enabled"`
	}
	if err := decodeStrict(raw, &wire); err != nil {
		return model.Flags{}, err
	}
	if wire.AuthRequired == nil || wire.AuthRevocable == nil || wire.AuthImmutable == nil || wire.ClawbackEnabled == nil {
		return model.Flags{}, errors.New("a flag is missing")
	}
	return model.Flags{
		AuthRequired:    *wire.AuthRequired,
		AuthRevocable:   *wire.AuthRevocable,
		AuthImmutable:   *wire.AuthImmutable,
		ClawbackEnabled: *wire.ClawbackEnabled,
	}, nil
}

func decodeSigners(wire *[]signerWire) ([]model.Signer, error) {
	if wire == nil {
		return nil, errors.New("no signers array")
	}
	signers := make([]model.Signer, 0, len(*wire))
	seen := make(map[string]struct{}, len(*wire))
	for _, entry := range *wire {
		if entry.Key == "" {
			return nil, errors.New("a signer has no key")
		}
		if entry.Weight == nil || *entry.Weight < 0 || *entry.Weight > 255 {
			return nil, errors.New("a signer weight is missing or out of range")
		}
		var kind strkey.Kind
		var signerType model.SignerType
		switch entry.Type {
		case "ed25519_public_key":
			kind, signerType = strkey.AccountID, model.SignerEd25519
		case "sha256_hash":
			kind, signerType = strkey.HashX, model.SignerHashX
		case "preauth_tx":
			kind, signerType = strkey.PreAuthTx, model.SignerPreAuthTx
		default:
			return nil, errors.New("a signer type Gauge does not model")
		}
		classified, err := strkey.Classify(entry.Key)
		if err != nil || classified != kind {
			return nil, errors.New("a signer key does not match its type")
		}
		if _, duplicate := seen[entry.Key]; duplicate {
			return nil, errors.New("a signer key appears twice")
		}
		seen[entry.Key] = struct{}{}
		signers = append(signers, model.Signer{
			Key:     entry.Key,
			Type:    signerType,
			Weight:  *entry.Weight,
			Sponsor: entry.Sponsor,
		})
	}
	return signers, nil
}

func decodeBalances(wire *[]balanceWire) ([]model.Balance, error) {
	if wire == nil {
		return nil, errors.New("no balances array")
	}
	balances := make([]model.Balance, 0, len(*wire))
	for _, entry := range *wire {
		switch entry.AssetType {
		case "native":
			balances = append(balances, model.Balance{IsAuthorized: true})
		case "credit_alphanum4", "credit_alphanum12":
			if !validAssetCode(entry.AssetCode) || !strkey.IsAccountID(entry.AssetIssuer) {
				return nil, errors.New("a credit balance names no valid asset")
			}
			if entry.IsAuthorized == nil {
				return nil, errors.New("a credit balance has no authorization flag")
			}
			balance := model.Balance{
				AssetCode:    entry.AssetCode,
				AssetIssuer:  entry.AssetIssuer,
				IsAuthorized: *entry.IsAuthorized,
			}
			if entry.IsClawbackEnabled != nil {
				balance.ClawbackEnabled = *entry.IsClawbackEnabled
			}
			balances = append(balances, balance)
		default:
			return nil, errors.New("an asset type Gauge does not model")
		}
	}
	return balances, nil
}

func validSequence(s string) bool {
	if s == "" || len(s) > 19 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

func validAssetCode(code string) bool {
	if len(code) == 0 || len(code) > 12 {
		return false
	}
	for i := 0; i < len(code); i++ {
		c := code[i]
		alnum := (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
		if !alnum {
			return false
		}
	}
	return true
}

// decodeStrict decodes one object, rejecting unknown fields, null and trailing
// data. It is applied to the sub-objects whose shape Gauge must not guess at.
func decodeStrict(raw json.RawMessage, out any) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return errors.New("missing")
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("trailing data")
	}
	return nil
}

type networkWire struct {
	Passphrase *string `json:"network_passphrase"`
	Ledger     *int64  `json:"history_latest_ledger"`
}

type accountWire struct {
	AccountID  string          `json:"account_id"`
	Sequence   *string         `json:"sequence"`
	Ledger     *int64          `json:"last_modified_ledger"`
	Thresholds json.RawMessage `json:"thresholds"`
	Flags      json.RawMessage `json:"flags"`
	Signers    *[]signerWire   `json:"signers"`
	Balances   *[]balanceWire  `json:"balances"`
	HomeDomain string          `json:"home_domain"`
	Sponsor    string          `json:"sponsor"`
}

type signerWire struct {
	Key     string `json:"key"`
	Type    string `json:"type"`
	Weight  *int   `json:"weight"`
	Sponsor string `json:"sponsor"`
}

type balanceWire struct {
	AssetType         string `json:"asset_type"`
	AssetCode         string `json:"asset_code"`
	AssetIssuer       string `json:"asset_issuer"`
	IsAuthorized      *bool  `json:"is_authorized"`
	IsClawbackEnabled *bool  `json:"is_clawback_enabled"`
}
