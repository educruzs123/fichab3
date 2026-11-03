package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"fichab3/nucleo"
)

// /api/cotacoes-diarias?t=ITUB3: fechamentos diários dos últimos 5 anos, com a última cotação do Yahoo.
func Handler(w http.ResponseWriter, r *http.Request) {
	t := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("t")))
	l, err := nucleo.SupaPadrao().CotacoesDiarias(t)
	if err != nil {
		nucleo.Erro(w, 502, err.Error())
		return
	}
	if u := nucleo.UltimaCotacao(t, false); u != nil && len(l) > 0 {
		var ult struct {
			Data string `json:"data"`
		}
		if json.Unmarshal(l[len(l)-1], &ult) == nil && u.Data > ult.Data {
			if b, err := json.Marshal(map[string]any{"data": u.Data, "fechamento": u.Preco}); err == nil {
				l = append(l, b)
			}
		}
	}
	nucleo.Responde(w, l)
}
