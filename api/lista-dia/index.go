package handler

import (
	"net/http"

	"fichab3/nucleo"
)

// /api/lista-dia: último fechamento e o anterior de cada companhia ativa, com o Ibovespa do Yahoo no topo.
func Handler(w http.ResponseWriter, r *http.Request) {
	s := nucleo.SupaPadrao()
	l, err := s.Lista(false)
	if err != nil {
		nucleo.Erro(w, 502, err.Error())
		return
	}
	fs, err := s.Fechamentos()
	if err != nil {
		nucleo.Erro(w, 502, err.Error())
		return
	}
	out := []map[string]any{}
	// sem resposta do Yahoo, a lista segue só com as ações
	if u := nucleo.UltimaCotacao("^BVSP", false); u != nil {
		out = append(out, map[string]any{"ticker": "IBOV", "nome": "Índice Ibovespa", "data": u.Data, "preco": u.Preco, "ant": u.Anterior, "indice": true})
	}
	for _, e := range l { // já em ordem de ticker
		if f, ok := fs[e.Tickers[0]]; ok && e.Ativa {
			out = append(out, map[string]any{"ticker": e.Tickers[0], "nome": e.Nome, "data": f.Data, "preco": f.Preco, "ant": f.Ant})
		}
	}
	nucleo.Responde(w, out)
}
