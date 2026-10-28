package source

import (
	"context"

	"github.com/posture-sec/guage/internal/model"
)

type Source interface {
	Network(ctx context.Context) (model.NetworkInfo, error)
	Account(ctx context.Context, id string) (model.AccountSnapshot, error)
}
