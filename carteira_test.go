package main

import "testing"

func TestNaCarteira(t *testing.T) {
	l := naCarteira([]*Empresa{{Tickers: []string{"PETR3", "PETR4"}}, {Tickers: []string{"WEGE3"}}})
	if len(carteira) != 90 || len(l) != 1 || len(l[0].Tickers) != 1 || l[0].Tickers[0] != "PETR4" {
		t.Fatalf("carteira=%d lista=%+v", len(carteira), l)
	}
}
