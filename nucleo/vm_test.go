package nucleo

import (
	"math"
	"testing"
)

// PETR4/PETR3 em 06/11/2026: 7.442.231.382 ON a R$ 51,20 e 5.446.501.379 PN a R$ 45,25
// → R$ 627.496 mi. Com um preço só (PETR4), daria 583.215 mi (o erro que esta correção resolve).
func TestVMClassesPETR(t *testing.T) {
	got := vmClasses(7442231382, 51.20, 5446501379, 45.25)
	if math.Abs(got-627496.43) > 1 {
		t.Fatalf("vmClasses = %.2f mi, esperado ≈ 627496.43", got)
	}
}

func TestVMDasClassesPrecisaDasDuasClasses(t *testing.T) {
	merc := map[string]Mercado{"PETR4": {Preco: 45.25}}
	a := Acoes{Qt: 1, Data: "2026-06-30", ON: 7442231382, PN: 5446501379}
	if _, ok := VmDasClasses([]string{"PETR3", "PETR4"}, merc, a); ok {
		t.Fatal("sem o preço da PETR3 não deveria calcular por classe")
	}
	merc["PETR3"] = Mercado{Preco: 51.20}
	v, ok := VmDasClasses([]string{"PETR3", "PETR4"}, merc, a)
	if !ok || math.Abs(v-627496.43) > 1 {
		t.Fatalf("VmDasClasses = %.2f, ok=%v", v, ok)
	}
	if _, ok := VmDasClasses([]string{"PETR4"}, merc, a); ok {
		t.Fatal("empresa sem ON não deveria usar a conta por classe")
	}
}
