package handler

import (
	"net/http"
	"regexp"
	"strings"

	"fichab3/nucleo"
)

var reTicker = regexp.MustCompile(`^[A-Z0-9]{4,6}$`)

// /api/comparar?t=VALE3: fechamento ajustado mensal da ação e o Ibovespa, nos meses em que os dois existem.
func Handler(w http.ResponseWriter, r *http.Request) {
	t := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("t")))
	if !reTicker.MatchString(t) {
		nucleo.Erro(w, 400, "informe o código da ação, como VALE3")
		return
	}
	acao, err := nucleo.HistoricoYahoo(t)
	if err != nil {
		nucleo.Erro(w, 404, t+": "+err.Error())
		return
	}
	ibov, err := nucleo.HistoricoYahoo("^BVSP")
	if err != nil {
		nucleo.Erro(w, 502, "Ibovespa: "+err.Error())
		return
	}
	ind := map[string]float64{}
	for _, p := range ibov {
		ind[p.Mes] = p.V
	}
	pts := []map[string]any{}
	for _, p := range acao {
		if v, ok := ind[p.Mes]; ok {
			pts = append(pts, map[string]any{"mes": p.Mes, "acao": p.V, "ibov": v})
		}
	}
	nucleo.Responde(w, map[string]any{"ticker": t, "pontos": pts})
}
