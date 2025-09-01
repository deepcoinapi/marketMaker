// go-demo-market-maker/main.go
// A minimal, production-minded demo market-making bot in Go.
// - Connects to DeepCoin public WS  for one instrument
// - Maintains top-of-book and computes mid price
// - Places/cancels maker orders around mid with inventory/risk controls
// DISCLAIMER: Demo only. Add robust error handling, persistence, retries, and signing for production.

package main

import (
	"context"
	"fmt"
	"marketMaker/exchange"
	"marketMaker/strategy"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := strategy.DefaultConfig()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ob := exchange.NewOrderBook(cfg.InstID, 25)
	go func() {
		for {
			if err := exchange.RunPublicWS(ctx, cfg.WSPublicURL, cfg.InstID, ob); err != nil {
				fmt.Printf("ws error: %v; reconnecting in 2s", err)
				time.Sleep(2 * time.Second)
			} else {
				return
			}
		}
	}()

	client := exchange.NewDcClient(ob)
	strat := strategy.NewStrategy(cfg, ob, client)
	strat.Run(ctx)
}
