package strategy

import (
	"context"
	"fmt"
	"marketMaker/exchange"
	"marketMaker/exchange/httpClient"
	"math"
	"sync"
	"time"
)

type Config struct {
	InstID         string        // e.g. "BTC-USDT-SWAP"
	WSPublicURL    string        // DeepCoin public ws
	QuoteSize      float64       // per order size
	BaseSpreadBps  float64       // base spread from mid in bps (1/10000)
	RepriceBps     float64       // when mid moves this much -> reprice
	MaxPosition    float64       // absolute position cap
	TargetPosition float64       // desired inventory (usually 0)
	HedgeThreshold float64       // when |pos-target| > threshold, skew quotes
	Refresh        time.Duration // how often to refresh quotes
}

func DefaultConfig() Config {
	return Config{
		InstID:         "BTC-USDT-SWAP",
		WSPublicURL:    "wss://stream.deepcoin.com/public/ws",
		QuoteSize:      1,
		BaseSpreadBps:  5,  // 5 bps on each side (0.05%)
		RepriceBps:     2,  // reprice if mid changes > 2 bps
		MaxPosition:    10, // 1 = 0.001 BTC
		TargetPosition: 0,  // pos model Net , auto merge long and short ,only one side
		HedgeThreshold: 3,  // skew when off-target by > 0.002
		Refresh:        time.Second,
	}
}

type Strategy struct {
	cfg      Config
	ob       *exchange.OrderBook
	ex       exchange.ExchangeClient
	mu       sync.Mutex
	lastMid  float64
	active   map[string]string // current working order ids by side
	position float64           // simulated inventory
}

func NewStrategy(cfg Config, ob *exchange.OrderBook, ex exchange.ExchangeClient) *Strategy {
	return &Strategy{cfg: cfg, ob: ob, ex: ex, active: map[string]string{}}
}

func (s *Strategy) Run(ctx context.Context) {
	t := time.NewTicker(s.cfg.Refresh)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.tick(ctx)
		}
	}
}

func (s *Strategy) tick(ctx context.Context) {
	mid, ok := s.ob.Mid()
	if !ok {
		return
	}

	// get pos
	if p, err := s.ex.Position(ctx, s.cfg.InstID); err != nil {
		return
	} else {
		s.position = p.Sz
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// reprice condition
	needReprice := s.lastMid == 0 || math.Abs(mid-s.lastMid)/s.lastMid*1e4 >= s.cfg.RepriceBps

	// compute skew based on inventory distance to target
	delta := s.position - s.cfg.TargetPosition
	skewBps := 0.0
	if math.Abs(delta) > s.cfg.HedgeThreshold {
		// if long > target, make buys worse (wider) and sells better (tighter)
		// linear skew up to 2x base spread
		k := math.Min(math.Abs(delta)/s.cfg.MaxPosition, 1.0)
		if delta > 0 { // too long
			skewBps = s.cfg.BaseSpreadBps * k
		} else { // too short
			skewBps = -s.cfg.BaseSpreadBps * k
		}
	}

	bidPx := mid * (1 - (s.cfg.BaseSpreadBps+skewBps)/1e4)
	askPx := mid * (1 + (s.cfg.BaseSpreadBps-skewBps)/1e4)

	// enforce position cap: if too long, suppress bid; if too short, suppress ask
	suppressBid := s.position >= s.cfg.MaxPosition
	suppressAsk := s.position <= -s.cfg.MaxPosition

	fmt.Println("pos:", s.position, "tagetPos:", s.cfg.TargetPosition, "maxpos:", s.cfg.MaxPosition, "supperBid,ask:", suppressBid, suppressAsk, "needReprice:", needReprice)
	// cancel/replace if needed
	if needReprice || suppressBid {
		s.cancelSide(ctx, httpClient.SIDE_BUY)
	}
	if needReprice || suppressAsk {
		s.cancelSide(ctx, httpClient.SIDE_SELL)
	}

	if !suppressBid {
		s.ensureOrder(ctx, httpClient.SIDE_BUY, bidPx, s.cfg.QuoteSize)
	}
	if !suppressAsk {
		s.ensureOrder(ctx, httpClient.SIDE_SELL, askPx, s.cfg.QuoteSize)
	}

	s.lastMid = mid
}

func (s *Strategy) ensureOrder(ctx context.Context, side string, px, sz float64) {
	if oid, ok := s.active[side]; ok && oid != "" {
		// already working; in a real impl we would check drift and amend; mock keeps simple
		return
	}
	id, err := s.ex.PlacePostOnly(ctx, s.cfg.InstID, side, px, sz)
	if err != nil {
		fmt.Printf("place %s error: %v \n", side, err)
		return
	}
	s.active[side] = id
}

func (s *Strategy) cancelSide(ctx context.Context, side string) {
	if oid, ok := s.active[side]; ok && oid != "" {
		if err := s.ex.Cancel(ctx, s.cfg.InstID, oid); err != nil {
			fmt.Printf("place %s error: %v \n", side, err)
		}
		s.active[side] = ""
	}
}
