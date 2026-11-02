package main

// Tickers da carteira (planilha de outubro de 2026); o mesmo conjunto da tabela public.carteira
// (sql/carteira.sql). A fonte Supabase já vem filtrada pelo banco; aqui filtra a fonte local.
var carteira = map[string]bool{}

func init() {
	for _, t := range []string{"BBDC4", "BBDC3", "BRBI11", "STOC34", "LREN3", "BMEB3", "BBAS3", "TIMS3", "BPAC11", "ALOS3",
		"VULC3", "BMEB4", "BRSR6", "WIZC3", "LEVE3", "MULT3", "SAPR4", "RANI3", "SAPR11", "LOGG3", "BAZA3", "ITSA4", "PINE4",
		"PSSA3", "PASS3", "ABCB4", "BMGB4", "PAGS34", "DEXP3", "MDIA3", "B3SA3", "SAPR3", "JHSF3", "SBSP3", "AGRO3", "ITUB4",
		"POMO3", "CPFE3", "SAUD3", "FESA4", "CMIG4", "BRAP4", "KLBN4", "CSUD3", "EQPA3", "IRBR3", "GOAU3", "CGAS5", "VIVT3",
		"KLBN11", "CGRA4", "POMO4", "AXIA3", "CSMG3", "BRAP3", "GGBR3", "FIQE3", "CLSC4", "ISAE4", "KLBN3", "VALE3", "GRND3",
		"RECV3", "UNIP6", "SHUL4", "SLCE3", "ITUB3", "CPLE3", "SANB4", "SANB11", "ALUP11", "SANB3", "PETR4", "CEBR5", "SUZB3",
		"CEBR3", "VBBR3", "BBSE3", "ABEV3", "CXSE3", "EGIE3", "TAEE3", "CEBR6", "TGMA3", "TAEE11", "TAEE4", "FLRY3", "BEES3",
		"CMIN3", "KEPL3"} {
		carteira[t] = true
	}
}

// só os tickers da carteira; empresa sem nenhum fica de fora
func naCarteira(l []*Empresa) []*Empresa {
	out := l[:0]
	for _, e := range l {
		tk := e.Tickers[:0]
		for _, t := range e.Tickers {
			if carteira[t] {
				tk = append(tk, t)
			}
		}
		if e.Tickers = tk; len(tk) > 0 {
			out = append(out, e)
		}
	}
	return out
}
