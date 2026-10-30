package source

import (
	"context"
	"fmt"

	"github.com/posture-sec/guage/internal/model"
)

type Source interface {
	Network(ctx context.Context) (model.NetworkInfo, error)
	Account(ctx context.Context, id string) (model.AccountSnapshot, error)
}

// CheckPassphrase compares the endpoint's reported passphrase with the one the
// policy declares. It never repeats either value in the error.
func CheckPassphrase(info model.NetworkInfo, want string) error {
	if want == "" {
		return fmt.Errorf("%w: no expected network passphrase", ErrOptions)
	}
	if info.Passphrase == want {
		return nil
	}
	return fmt.Errorf("%w: the endpoint is on a different network", ErrPassphraseMismatch)
}
