package main

import "testing"

func TestUltimosDois(t *testing.T) {
	m := ultimosDois([]cotDia{
		{"AAAA3", "2026-11-05", 10}, {"AAAA3", "2026-11-06", 0}, {"AAAA3", "2026-11-09", 11},
		{"BBBB4", "2026-11-09", 5},
	})
	if f := m["AAAA3"]; f != (Fechamento{"2026-11-09", 11, 10}) { // o fechamento zerado é ignorado
		t.Errorf("AAAA3: %+v", f)
	}
	if f := m["BBBB4"]; f != (Fechamento{"2026-11-09", 5, 0}) {
		t.Errorf("BBBB4: %+v", f)
	}
}
