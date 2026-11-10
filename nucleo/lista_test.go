package nucleo

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

func TestMensalYahoo(t *testing.T) {
	// 01/2024, 02/2024 sem valor, 03/2024 duas vezes (barra do mês e cotação do dia): fica a última
	corpo := `{"chart":{"result":[{"timestamp":[1704078000,1706756400,1709262000,1710500000],"indicators":{"adjclose":[{"adjclose":[10,null,12,13]}]}}]}}`
	pts, err := mensalYahoo([]byte(corpo))
	if err != nil || len(pts) != 2 || pts[0] != (PontoMes{"2024-01", 10}) || pts[1] != (PontoMes{"2024-03", 13}) {
		t.Errorf("pts=%+v err=%v", pts, err)
	}
	if _, err := mensalYahoo([]byte(`{"chart":{"result":null}}`)); err == nil {
		t.Error("ticker desconhecido devia dar erro")
	}
	if simboloYahoo("VALE3") != "VALE3.SA" || simboloYahoo("^BVSP") != "%5EBVSP" {
		t.Error("simboloYahoo")
	}
}
