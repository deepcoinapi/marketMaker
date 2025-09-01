package exchange

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type WSMsg struct {
	Arg struct {
		Channel, InstID string `json:"channel" json2:"instId"`
	} `json:"arg"`
	Data []struct {
		Asks     [][]string `json:"asks"`
		Bids     [][]string `json:"bids"`
		Checksum int        `json:"checksum"`
		Ts       string     `json:"ts"`
	} `json:"data"`
	Event string `json:"event"`
}

// ===================== OrderBook (top-of-book) =====================

//	type OrderBook struct {
//		mu      sync.RWMutex
//		bestBid float64
//		bestAsk float64
//		//seq     int64
//	}
type OrderBook struct {
	Symbol string
	mu     sync.RWMutex
	Bids   map[float64]Order // 买单，价格到数量的映射
	Asks   map[float64]Order // 卖单，价格到数量的映射
	Ask1   float64
	Bid1   float64
	Depth  int // 维护的深度
}

func NewOrderBook(symbol string, depth int) *OrderBook {
	ss := strings.Split(symbol, "-")
	symbol = "DeepCoin_" + ss[0] + ss[1]
	return &OrderBook{
		Symbol: symbol,
		Bids:   make(map[float64]Order, depth),
		Asks:   make(map[float64]Order, depth),
		Depth:  depth,
	}
}

// 中间价获取
func (ob *OrderBook) Mid() (float64, bool) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	if ob.Ask1 == 0 || ob.Bid1 == 0 {
		return 0, false
	}
	fmt.Println("ask1:", ob.Ask1, "bid1:", ob.Bid1)
	return (ob.Ask1 + ob.Bid1) / 2, true
}

// 更新订单簿
func (ob *OrderBook) UpdateSnapshot(msg *DcResponseWSMsg) error {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	//fmt.Println("msg:", *msg)
	// 更新买单
	for _, order := range msg.Result {
		if order.Data.Direction == "0" {
			//买单
			if _, ok := ob.Bids[order.Data.Price]; ok {
				if order.Data.Volume == 0 {
					delete(ob.Bids, order.Data.Price)
				} else {
					ob.Bids[order.Data.Price] = Order{Px: order.Data.Price, Sz: order.Data.Volume}
				}
			} else {
				if order.Data.Volume > 0 {
					ob.Bids[order.Data.Price] = Order{Px: order.Data.Price, Sz: order.Data.Volume}
				}
			}
		} else {
			//卖单
			if _, ok := ob.Asks[order.Data.Price]; ok {
				if order.Data.Volume > 0 {
					ob.Asks[order.Data.Price] = Order{Px: order.Data.Price, Sz: order.Data.Volume}
				} else {
					delete(ob.Asks, order.Data.Price)
				}
			} else {
				if order.Data.Volume > 0 {
					ob.Asks[order.Data.Price] = Order{Px: order.Data.Price, Sz: order.Data.Volume}
				}
			}
		}
	}
	ob.maintainDepth()
	return nil
}

// UpdateFromDelta 从增量更新订单簿
func (ob *OrderBook) UpdateFromDelta(delta *DcResponseWSMsg) error {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	return nil
}

// maintainDepth 维护订单簿深度
func (ob *OrderBook) maintainDepth() {
	//fmt.Println("asks:", ob.Asks)
	//fmt.Println("bids:", ob.Bids)
	bidl := len(ob.Bids)
	askl := len(ob.Asks)
	bidPrices1 := make([]Order, 0, bidl)
	askPrices1 := make([]Order, 0, askl)
	for _, v := range ob.Bids {
		bidPrices1 = append(bidPrices1, v)
	}
	for _, v := range ob.Asks {
		askPrices1 = append(askPrices1, v)
	}

	//降序
	sort.Slice(bidPrices1, func(i, j int) bool {
		return bidPrices1[i].Px > bidPrices1[j].Px
	})
	//升序
	sort.Slice(askPrices1, func(i, j int) bool {
		return askPrices1[i].Px < askPrices1[j].Px
	})

	// 只保留指定深度的价格档位
	if bidl > 0 {
		ob.Bid1 = bidPrices1[0].Px
	}
	if askl > 0 {
		ob.Ask1 = askPrices1[0].Px
	}
}

func (ob *OrderBook) UpdateBest(bids, asks [][]string, seq int64) {

}

func (ob *OrderBook) Clear() {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	ob.Bids = make(map[float64]Order)
	ob.Asks = make(map[float64]Order)
}
