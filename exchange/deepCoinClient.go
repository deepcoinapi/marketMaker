package exchange

import (
	"context"
	"fmt"
	"marketMaker/exchange/httpClient"
	"strconv"
	"sync"
	"time"
)

type DcClient struct {
	mu     sync.Mutex
	orders map[string]Order
	ob     *OrderBook
	Sign   *httpClient.Sign
}

func NewDcClient(ob *OrderBook) *DcClient {
	sign := httpClient.NewSign()
	return &DcClient{orders: map[string]Order{}, ob: ob, Sign: sign}
}

func (m *DcClient) PlacePostOnly(ctx context.Context, inst, side string, px, sz float64) (string, error) {
	id := fmt.Sprintf("Dc%d", time.Now().UnixNano())
	pside := httpClient.POSITION_SIDE_LONG
	if side != httpClient.SIDE_BUY {
		pside = httpClient.POSITION_SIDE_SHORT
	}
	req := httpClient.OrderRequest{
		InstId:      inst,
		TdMode:      httpClient.CROSS,
		ClOrdId:     id,
		OrdType:     httpClient.ORDER_TYPE_POST_ONLY,
		Side:        side,
		PosSide:     pside,
		Px:          strconv.FormatFloat(px, 'f', 1, 64),
		Sz:          strconv.FormatFloat(sz, 'f', 0, 64),
		MrgPosition: httpClient.MERGE,
	}
	fmt.Printf("post order:%+v \n", req)
	if err := httpClient.Order(req, m.Sign); err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orders[id] = Order{ID: id, Side: side, Px: px, Sz: sz, Inst: inst, TS: time.Now()}
	return id, nil
}

func (m *DcClient) Cancel(ctx context.Context, inst, oid string) error {
	cancel := httpClient.CancelOrderRequest{
		InstId:  inst,
		ClOrdId: oid,
	}
	fmt.Printf("Cancel order:%+v \n", cancel)
	if err := httpClient.CancelOrder(cancel, m.Sign); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.orders, oid)
	return nil
}

func (m *DcClient) OpenOrders(ctx context.Context, inst string) ([]Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Order
	for _, o := range m.orders {
		if o.Inst == inst {
			out = append(out, o)
		}
	}
	return out, nil
}

func (m *DcClient) Position(ctx context.Context, inst string) (Position, error) {
	pos := Position{
		Inst: inst,
	}
	//fmt.Printf("Position :%+v \n", pos)
	ps, err := httpClient.Positions(inst, m.Sign)
	if err != nil {
		return pos, err
	}
	for _, p := range ps {
		pp, _ := strconv.ParseFloat(p.Pos, 64)
		pos.Sz += pp
	}
	if pos.Sz > 0 {
		pos.Side = httpClient.POSITION_SIDE_LONG
	} else {
		pos.Side = httpClient.POSITION_SIDE_SHORT
	}

	return pos, nil
}
