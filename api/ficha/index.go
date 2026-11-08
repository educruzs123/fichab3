package handler

import (
	"net/http"
	"strings"

	"fichab3/nucleo"
)

// /api/ficha?t=ITUB3: a mesma ficha do app local, lida do Supabase.
func Handler(w http.ResponseWriter, r *http.Request) {
	t := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("t")))
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
	d, err := s.Dados(e, false)
	if err != nil {
		nucleo.Erro(w, 502, "erro ao ler a empresa no Supabase: "+err.Error())
		return
	}
	serie, avErr := s.Cotacoes(t, false)
	ult := nucleo.UltimaCotacao(t, false)
	serie = nucleo.ComUltimaCotacao(serie, ult)
	// a outra classe (ON↔PN), com seu próprio preço, para o valor de mercado
	var outra *nucleo.Serie
	if o := nucleo.TickerOposto(e.Tickers, t); o != "" {
		if s2, err := s.Cotacoes(o, false); err == nil && s2 != nil {
			outra = nucleo.ComUltimaCotacao(s2, nucleo.UltimaCotacao(o, false))
		}
	}
	f := nucleo.MontarFicha(e, d, t, serie, outra)
	if ult != nil && f.Atual["cotacao_data"] == ult.Data {
		f.Atual["cotacao_hora"] = ult.Hora
		if ult.Anterior > 0 {
			f.Atual["var_dia"] = ult.Preco/ult.Anterior - 1
			f.Atual["cotacao_anterior"] = ult.Anterior
		}
		f.FonteCot += "; última cotação: Yahoo Finance"
	}
	f.Fonte = "supabase"
	if avErr != nil {
		f.AvisoCotacao = avErr.Error()
	}
	nucleo.Responde(w, f)
}
