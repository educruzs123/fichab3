package handler

import (
	"net/http"
	"regexp"
	"strings"

	"fichab3/nucleo"
)

var reData = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// /api/itr?t=VALE3 lista os ITRs; com &dt=2026-06-30 traz as contas daquele trimestre.
func Handler(w http.ResponseWriter, r *http.Request) {
	t := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("t")))
	dt := r.URL.Query().Get("dt")
	if dt != "" && !reData.MatchString(dt) {
		nucleo.Erro(w, 400, "data inválida: use AAAA-MM-DD")
		return
	}
	s := nucleo.SupaPadrao()
	e, err := s.PorTicker(t)
	if err != nil {
		nucleo.Erro(w, 502, err.Error())
		return
	}
	if e == nil {
		nucleo.Erro(w, 404, "ticker não encontrado no banco do Supabase: "+t)
		return
	}
	itrs, err := s.ITRs(e)
	if err != nil {
		nucleo.Erro(w, 502, err.Error())
		return
	}
	out := map[string]any{"ticker": t, "empresa": e.NomeCurto, "tickers": e.Tickers, "itrs": itrs}
	if dt != "" {
		dre, bp, dfc, base, err := s.ITR(e, dt)
		if err != nil {
			nucleo.Erro(w, 502, err.Error())
			return
		}
		if len(dre)+len(bp)+len(dfc) == 0 {
			nucleo.Erro(w, 404, "o banco não tem as demonstrações de "+t+" em "+dt)
			return
		}
		out["dt"], out["base"], out["dre"], out["bp"], out["dfc"] = dt, base, dre, bp, dfc
	}
	nucleo.Responde(w, out)
}
