package hypercore

import (
	"encoding/json"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
)

func TestHyperliquidClientLoadsPerpsSpotAndOpenOrders(t *testing.T) {
	var calls atomic.Int64
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request struct {
			Type string `json:"type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch request.Type {
		case "spotMetaAndAssetCtxs":
			_, _ = w.Write([]byte(`[
				{"tokens":[
					{"name":"USDC","szDecimals":8,"weiDecimals":8,"index":0,"tokenId":"usdc","isCanonical":true,"evmContract":{"address":"0x0000000000000000000000000000000000000001","evm_extra_wei_decimals":2}},
					{"name":"PURR","szDecimals":0,"weiDecimals":5,"index":1,"tokenId":"purr","isCanonical":true}
				],"universe":[
					{"name":"PURR/USDC","tokens":[1,0],"index":0,"isCanonical":true}
				]},
				[{"dayNtlVlm":"1000","markPx":"2","midPx":"2.1","prevDayPx":"1.9"}]
			]`))
		case "clearinghouseState":
			_, _ = w.Write([]byte(`{
				"marginSummary":{"accountValue":"100","totalMarginUsed":"10"},
				"crossMarginSummary":{"accountValue":"100","totalMarginUsed":"10"},
				"withdrawable":"90",
				"assetPositions":[{"type":"oneWay","position":{"coin":"BTC","szi":"1","entryPx":"50000","positionValue":"60000","unrealizedPnl":"10000","liquidationPx":"40000","marginUsed":"12000","leverage":{"type":"cross","value":5}}}]
			}`))
		case "spotClearinghouseState":
			_, _ = w.Write([]byte(`{"balances":[
				{"coin":"USDC","token":0,"hold":"1","total":"20","entryNtl":"0"},
				{"coin":"PURR","token":1,"hold":"2","total":"10","entryNtl":"15"}
			]}`))
		case "frontendOpenOrders":
			_, _ = w.Write([]byte(`[
				{"coin":"BTC","isPositionTpsl":false,"isTrigger":false,"limitPx":"50000","oid":1,"orderType":"Limit","origSz":"1","reduceOnly":false,"side":"B","sz":"1","timestamp":100,"triggerCondition":"N/A","triggerPx":"0"},
				{"coin":"PURR/USDC","isPositionTpsl":false,"isTrigger":false,"limitPx":"2","oid":2,"orderType":"Limit","origSz":"5","reduceOnly":false,"side":"A","sz":"5","timestamp":101,"triggerCondition":"N/A","triggerPx":"0"}
			]`))
		default:
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	})
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local TCP listener unavailable: %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Close()

	client := NewHyperliquidClient("http://"+listener.Addr().String(), &http.Client{})
	state, err := client.GetClearinghouseState(t.Context(), "0x0000000000000000000000000000000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Positions) != 1 || state.Positions[0].Symbol != "BTC" {
		t.Fatalf("unexpected perpetual positions: %+v", state.Positions)
	}
	if len(state.SpotTokens) != 2 || state.SpotTokens[0].EVMContract != "0x0000000000000000000000000000000000000001" {
		t.Fatalf("unexpected spot token metadata: %+v", state.SpotTokens)
	}
	if len(state.SpotBalances) != 2 || state.SpotBalances[0].UnrealizedPnL != 0 || state.SpotBalances[1].ValueUSD != 20 || state.SpotBalances[1].UnrealizedPnL != 5 {
		t.Fatalf("unexpected spot balances: %+v", state.SpotBalances)
	}
	if len(state.OpenOrders) != 2 || state.OpenOrders[0].MarketType != "perpetual" || state.OpenOrders[1].MarketType != "spot" {
		t.Fatalf("unexpected open orders: %+v", state.OpenOrders)
	}

	if _, err := client.GetClearinghouseState(t.Context(), "0x0000000000000000000000000000000000000001"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 7 {
		t.Fatalf("expected cached spot metadata after first refresh, got %d REST calls", calls.Load())
	}
}
