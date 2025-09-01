package exchange

import (
	"context"
	"time"
)

type Order struct {
	ID   string
	Side string
	Px   float64
	Sz   float64
	Inst string
	TS   time.Time
}

type Position struct {
	Inst string
	Side string
	Sz   float64
}

type ExchangeClient interface {
	PlacePostOnly(ctx context.Context, inst string, side string, px, sz float64) (string, error)
	Cancel(ctx context.Context, inst string, orderID string) error
	OpenOrders(ctx context.Context, inst string) ([]Order, error)
	Position(ctx context.Context, inst string) (Position, error)
}
