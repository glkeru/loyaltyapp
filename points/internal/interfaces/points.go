package points

import (
	"context"
	"time"

	model "github.com/glkeru/loyalty/points/internal/models"
	"github.com/google/uuid"
)

type PointsStorage interface {
	TnxCreate(ctx context.Context, tnx model.PointTransaction) error
	TnxDelete(ctx context.Context, orderId string) error
	TnxCommitOnDate(ctx context.Context, date time.Time) error
	Redeem(ctx context.Context, user string, points float64, redeemId string) (err error)
	Transfer(ctx context.Context, userfrom string, userto string, points float64, transferId string) (err error)
	GetBalance(ctx context.Context, user string) (points float64, err error)
	GetTnx(ctx context.Context, user string, from time.Time, to time.Time) (tnxs []model.PointTransaction, err error)
	GetUserUUID(ctx context.Context, user string) (account uuid.UUID, err error)
}

type CacheStorage interface {
	GetBalance(ctx context.Context, user string) (points float64, err error)
	SetBalance(ctx context.Context, user string, points float64) (err error)
	InvalidateBalance(ctx context.Context, user string) error
}
