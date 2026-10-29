package main

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type AnoRow struct {
	Ano         int      `json:"ano"`
	Rotulo      string   `json:"rotulo"`
	Parcial     bool     `json:"parcial"`
	Base        string   `json:"base"`
	Receita     *float64 `json:"receita"`
	Cresc       *float64 `json:"cresc_receita"`
	LucroBruto  *float64 `json:"lucro_bruto"`
	EBIT        *float64 `json:"ebit"`
	Lucro       *float64 `json:"lucro"`
	LucroCtrl   *float64 `json:"lucro_ctrl"`
	CrescLucro  *float64 `json:"cresc_lucro"`
	MBruta      *float64 `json:"margem_bruta"`
	MEBIT       *float64 `json:"margem_ebit"`
	MLiq        *float64 `json:"margem_liq"`
	ROE         *float64 `json:"roe"`
	LPA         *float64 `json:"lpa"`
	Acoes       *float64 `json:"acoes_mi"`
	Ativo       *float64 `json:"ativo"`
	PL          *float64 `json:"pl"`
	PLCtrl      *float64 `json:"pl_ctrl"`
	Caixa       *float64 `json:"caixa"`
	DivBruta    *float64 `json:"div_bruta"`
	DivLiq      *float64 `json:"div_liq"`
	DivEBITDA   *float64 `json:"div_ebitda"`
	FCO         *float64 `json:"fco"`
	Capex       *float64 `json:"capex"`
	Proventos   *float64 `json:"proventos"`
	Recompras   *float64 `json:"recompras"`
	Payout      *float64 `json:"payout"`
	ProvAcao    *float64 `json:"prov_acao"`
	Preco       *float64 `json:"preco"`
	PrecoData   string   `json:"preco_data,omitempty"`
	PLx         *float64 `json:"p_l"`
	PVP         *float64 `json:"p_vp"`
	DY          *float64 `json:"dy"`
	DYDataCom   *float64 `json:"dy_data_com"`
	ProvDataCom *float64 `json:"prov_data_com"`
	VMbi        *float64 `json:"valor_mercado_bi"`
}

type TriOut struct {
	Tri
	VarReceita *float64 `json:"var_receita"`
	VarLucro   *float64 `json:"var_lucro"`
}

type CotOut struct {
	Mes   string  `json:"mes"`
	Data  string  `json:"data"`
	Close float64 `json:"fechamento"`
	Adj   float64 `json:"ajustado"`
	Prov  float64 `json:"provento"`
}

type Split struct {
	Data  string  `json:"data"`
	Fator float64 `json:"fator"`
}

type Ficha struct {
	Ticker       string            `json:"ticker"`
	Tickers      []string          `json:"tickers"`
	Empresa      string            `json:"empresa"`
	NomeCurto    string            `json:"nome_curto"`
	CNPJ         string            `json:"cnpj"`
	CodCVM       string            `json:"cod_cvm"`
	Setor        string            `json:"setor"`
	Tipo         string            `json:"tipo"`
	Financeira   bool              `json:"financeira"`
	Ativa        bool              `json:"ativa"`
	Classe       string            `json:"classe"`
	UltDFP       int               `json:"ult_dfp"`
	UltTri       string            `json:"ult_tri"`
	Periodo12m   string            `json:"periodo_12m"`
	RotEBIT      string            `json:"rotulo_ebit"`
	RotBruto     string            `json:"rotulo_bruto"`
	RotReceita   string            `json:"rotulo_receita"`
	Anual        []AnoRow          `json:"anual"`
	Trimestres   []TriOut          `json:"trimestres"`
	Cotacoes     []CotOut          `json:"cotacoes"`
	Splits       []Split           `json:"splits"`
	Atual        map[string]any    `json:"atual"`
	PLMedia      *float64          `json:"pl_media"`
	Notas        []string          `json:"notas"`
	AvisoCotacao string            `json:"aviso_cotacao,omitempty"`
	FonteCot     string            `json:"fonte_cotacao,omitempty"`
	GeradoEm     string            `json:"gerado_em"`
	Contas       map[string]string `json:"contas"`
	Fonte        string            `json:"fonte"`
}

func fp(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}

func div(a, b *float64) *float64 {
	if a == nil || b == nil || *b == 0 {
		return nil
	}
	return fp(*a / *b)
}

func sub(a, b *float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	return fp(*a - *b)
}

func cresc(a, b *float64) *float64 {
	if a == nil || b == nil || *b == 0 {
		return nil
	}
	return fp(*a/math.Abs(*b) - *b/math.Abs(*b))
}

var (
	reAcento   = strings.NewReplacer("á", "a", "à", "a", "ã", "a", "â", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "õ", "o", "ô", "o", "ú", "u", "ç", "c", "Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "Ç", "c", "Ã", "a", "Õ", "o", "Â", "a", "Ê", "e", "Ô", "o")
	reEBIT     = regexp.MustCompile(`antes do resultado financeiro`)
	rePreTax   = regexp.MustCompile(`antes dos tributos`)
	reLucro    = regexp.MustCompile(`(lucro|prejuizo|resultado).*(periodo|exercicio)`)
	reCapex    = regexp.MustCompile(`imobilizado|intangive|ativo fixo|ativos fixos|capex|investimentos? em (bens|ativos)`)
	reNaoCapex = regexp.MustCompile(`aliena|venda|recebimento|recebid|baixa`)
	reProv     = regexp.MustCompile(`dividendo|juros sobre (o )?capital|jcp|jscp|juros s/ ?capital|proventos`)
	reNaoProv  = regexp.MustCompile(`recebid|nao controlador|minoritari|de coligad|de controlad`)
	reRecompra = regexp.MustCompile(`acoes em tesouraria|recompra|aquisicao de acoes proprias`)
)

func norm(s string) string { return strings.ToLower(reAcento.Replace(s)) }

func classeTicker(t string) string {
	n := strings.TrimLeftFunc(t, func(r rune) bool { return r < '0' || r > '9' })
	switch n {
	case "4", "5", "6", "7", "8":
		return "PN"
	case "11":
		return "UNIT"
	}
	return "ON"
}

func MontarFicha(e *Empresa, d *EmpData, ticker string, serie *Serie) *Ficha {
	f := &Ficha{Ticker: ticker, Tickers: e.Tickers, Empresa: e.Nome, NomeCurto: e.NomeCurto, CNPJ: e.CNPJ, CodCVM: e.CodCVM,
		Setor: e.Setor, Tipo: e.Tipo, Financeira: strings.HasPrefix(strings.ToLower(e.Tipo), "financ"), Ativa: e.Ativa,
		UltDFP: e.UltDFP, UltTri: e.UltTri, Classe: classeTicker(ticker), Atual: map[string]any{}, Contas: map[string]string{},
		GeradoEm: time.Now().Format("02/01/2006 15:04")}
	f.RotReceita = "Receita líquida"
	f.RotBruto = "Lucro bruto"
	f.RotEBIT = "EBIT"
	if f.Financeira {
		f.RotReceita = "Receitas de intermediação"
		f.RotBruto = "Resultado bruto"
		f.RotEBIT = "Resultado antes dos tributos"
	}

	anos := []int{}
	for k := range d.DRE {
		if n, err := strconv.Atoi(k); err == nil {
			anos = append(anos, n)
		}
	}
	sort.Ints(anos)

	// splits detectados na série de preços
	var pts []Ponto
	if serie != nil {
		pts = serie.Pontos
		f.Splits = detectarSplits(pts)
		f.FonteCot = "Alpha Vantage (" + serie.SimboloAV + "), baixado em " + serie.BaixadoEm
	}
	fatorApos := func(data string) float64 { // splits depois da data
		x := 1.0
		for _, s := range f.Splits {
			if s.Data > data {
				x *= s.Fator
			}
		}
		return x
	}
	precoFimAno := func(ano int) (float64, string, bool) {
		var best *Ponto
		for i := range pts {
			if strings.HasPrefix(pts[i].Data, strconv.Itoa(ano)+"-") {
				best = &pts[i]
			}
		}
		if best == nil {
			return 0, "", false
		}
		return best.Close / fatorApos(best.Data), best.Data, true
	}
	// rendimento de cada provento deduzido do próprio fechamento ajustado
	// (independe de a Alpha Vantage informar o valor ajustado ou não)
	yld := make([]float64, len(pts))   // fração do preço
	provA := make([]float64, len(pts)) // R$ por ação na base atual
	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1], pts[i]
		if a.Close <= 0 || b.Close <= 0 || a.Adj <= 0 || b.Adj <= 0 {
			continue
		}
		k := 1.0
		for _, sp := range f.Splits {
			if sp.Data == b.Data {
				k = sp.Fator
			}
		}
		y := 1 - (a.Adj/a.Close)/(b.Adj/b.Close)*k
		if b.Div <= 0 && y < 0.001 {
			continue // sem provento no mês (diferença é arredondamento)
		}
		if y < 0 || y > 0.5 {
			if b.Div <= 0 {
				continue
			}
			y = b.Div / a.Close
		}
		yld[i] = y
		provA[i] = y * a.Close / fatorApos(a.Data)
	}
	provDataCom := func(ano int) (float64, float64, bool) {
		s, y, ok := 0.0, 0.0, false
		for i, p := range pts {
			if strings.HasPrefix(p.Data, strconv.Itoa(ano)+"-") {
				ok = true
				s += provA[i]
				y += yld[i]
			}
		}
		return s, y, ok
	}

	var rows []AnoRow
	for _, ano := range anos {
		k := strconv.Itoa(ano)
		dre, bp := d.DRE[k], d.BP[k]
		nDR, nBP := d.NomesDR[k], d.NomesBP[k]
		if nDR == nil {
			nDR = map[string]string{}
		}
		if nBP == nil {
			nBP = map[string]string{}
		}
		r := AnoRow{Ano: ano, Rotulo: k, Base: d.BaseAno[k]}
		get := func(m map[string]float64, c string) *float64 {
			if m == nil {
				return nil
			}
			if v, ok := m[c]; ok {
				return fp(v)
			}
			return nil
		}
		r.Receita = get(dre, "3.01")
		r.LucroBruto = get(dre, "3.03")
		// EBIT / resultado antes dos tributos
		for _, c := range codsProf(dre, 2) {
			n := norm(nDR[c])
			if (!f.Financeira && reEBIT.MatchString(n)) || (f.Financeira && rePreTax.MatchString(n)) {
				r.EBIT = get(dre, c)
				f.Contas["ebit"] = c + " " + nDR[c]
				break
			}
		}
		if r.EBIT == nil && !f.Financeira {
			r.EBIT = get(dre, "3.05")
		}
		// lucro líquido
		lc := ""
		if _, ok := dre["3.11"]; ok && reLucro.MatchString(norm(nDR["3.11"])) {
			lc = "3.11"
		} else {
			for _, c := range codsProf(dre, 2) {
				n := norm(nDR[c])
				if reLucro.MatchString(n) && !strings.Contains(n, "antes") && !strings.Contains(n, "continuadas") {
					lc = c
				}
			}
		}
		if lc != "" {
			r.Lucro = get(dre, lc)
			r.LucroCtrl = r.Lucro
			for c := range dre {
				if strings.HasPrefix(c, lc+".") && profundidade(c) == profundidade(lc)+1 {
					n := norm(nDR[c])
					if strings.Contains(n, "controlador") && !strings.Contains(n, "nao controlador") {
						if v := get(dre, c); v != nil && (*v != 0 || r.Lucro == nil || *r.Lucro == 0) {
							r.LucroCtrl = v
						}
					}
				}
			}
			f.Contas["lucro"] = lc + " " + nDR[lc]
		}
		// LPA da classe
		r.LPA = lpaClasse(dre, nDR, f.Classe)
		// balanço
		r.Ativo = get(bp, "1")
		for _, c := range codsProf(bp, 2) {
			if strings.HasPrefix(c, "2.") && strings.HasPrefix(norm(nBP[c]), "patrimonio liquido") {
				r.PL = get(bp, c)
				r.PLCtrl = r.PL
				var nc *float64
				for _, c2 := range codsProf(bp, 3) {
					if !strings.HasPrefix(c2, c+".") {
						continue
					}
					n := norm(nBP[c2])
					if strings.Contains(n, "nao controlador") || strings.Contains(n, "minoritar") {
						nc = get(bp, c2)
					} else if strings.Contains(n, "controlador") {
						r.PLCtrl = get(bp, c2)
						nc = nil
						break
					}
				}
				if nc != nil {
					r.PLCtrl = sub(r.PL, nc)
				}
				break
			}
		}
		if !f.Financeira {
			cx, db := 0.0, 0.0
			tc, td := false, false
			for _, c := range codsProf(bp, 3) {
				n := norm(nBP[c])
				if strings.HasPrefix(c, "1.01.") && (strings.HasPrefix(n, "caixa e equivalentes") || strings.HasPrefix(n, "aplicacoes financeiras")) {
					cx += bp[c]
					tc = true
				}
				if (strings.HasPrefix(c, "2.01.") || strings.HasPrefix(c, "2.02.")) && strings.HasPrefix(n, "emprestimos e financiamentos") {
					db += bp[c]
					td = true
				}
			}
			if tc {
				r.Caixa = fp(cx)
			}
			if td {
				r.DivBruta = fp(db)
			}
			if tc && td {
				r.DivLiq = fp(db - cx)
			}
		}
		// fluxo de caixa
		if cs, ok := d.DFC[k]; ok {
			capex, prov, rec := 0.0, 0.0, 0.0
			tcap, tprov := false, false
			for _, ct := range cs {
				n := norm(ct.Nome)
				switch {
				case ct.Cod == "6.01":
					r.FCO = fp(ct.V)
				case strings.HasPrefix(ct.Cod, "6.02.") && profundidade(ct.Cod) == 3:
					if reCapex.MatchString(n) && !reNaoCapex.MatchString(n) && ct.V < 0 {
						capex += -ct.V
						tcap = true
					}
				case strings.HasPrefix(ct.Cod, "6.03.") && profundidade(ct.Cod) == 3:
					if reProv.MatchString(n) && !reNaoProv.MatchString(n) {
						if ct.V < 0 {
							prov += -ct.V
						}
						tprov = true
					} else if reRecompra.MatchString(n) && ct.V < 0 {
						rec += -ct.V
					}
				}
			}
			if tcap || !f.Financeira {
				r.Capex = fp(capex)
			}
			if tprov {
				r.Proventos = fp(prov)
			} else {
				r.Proventos = fp(0)
			}
			r.Recompras = fp(rec)
		}
		rows = append(rows, r)
	}

	// LPA informado por lote de mil ações: corrige pela mediana
	var ref []float64
	for i := range rows {
		if a := acoesDe(rows[i]); a != nil {
			ref = append(ref, *a)
		}
	}
	med := mediana(ultimos(ref, 3))
	for i := range rows {
		r := &rows[i]
		if a := acoesDe(*r); a != nil && med > 0 {
			if q := *a / med; q > 300 {
				*r.LPA *= 1000
			} else if q < 1.0/300 {
				*r.LPA /= 1000
			}
		}
	}
	// ações em circulação (milhões, na base atual):
	// 1) composição do capital informada à CVM (total − tesouraria);
	// 2) senão, lucro dos controladores ÷ lucro por ação.
	capDe := func(ano int) (float64, string, bool) {
		best := ""
		for dt := range d.Capital {
			if strings.HasPrefix(dt, strconv.Itoa(ano)+"-") && dt > best {
				best = dt
			}
		}
		if best == "" {
			return 0, "", false
		}
		c := d.Capital[best]
		return c.Total - c.TesTot, best, c.Total > 0
	}
	// a quantidade às vezes vem em milhares, e a unidade muda de um ano para outro:
	// escolhe, ano a ano, a leitura (unidades ou milhares) mais próxima da referência
	escAno := map[int]float64{}
	perto := func(c, alvo float64) float64 {
		best, bd := 1.0, math.Inf(1)
		for _, e := range []float64{1, 1e3, 1e6} {
			if d := math.Abs(math.Log(c * e / 1e6 / alvo)); d < bd {
				best, bd = e, d
			}
		}
		return best
	}
	for _, r := range rows {
		if c, _, ok := capDe(r.Ano); ok {
			if a := acoesDe(r); a != nil {
				escAno[r.Ano] = perto(c, *a)
			}
		}
	}
	for pass := 0; pass < 2; pass++ {
		for i, r := range rows {
			c, _, ok := capDe(r.Ano)
			if !ok {
				continue
			}
			if _, feito := escAno[r.Ano]; feito {
				continue
			}
			// vizinho mais próximo já decidido
			for dlt := 1; dlt < len(rows); dlt++ {
				for _, j := range []int{i - dlt, i + dlt} {
					if j < 0 || j >= len(rows) {
						continue
					}
					if e, ok2 := escAno[rows[j].Ano]; ok2 {
						cj, _, _ := capDe(rows[j].Ano)
						escAno[r.Ano] = perto(c, cj*e/1e6)
						break
					}
				}
				if _, feito := escAno[r.Ano]; feito {
					break
				}
			}
			if _, feito := escAno[r.Ano]; !feito && pass == 1 {
				switch {
				case c < 1e4:
					escAno[r.Ano] = 1e6
				case c < 5e6:
					escAno[r.Ano] = 1000
				default:
					escAno[r.Ano] = 1
				}
			}
		}
	}
	fonteAcoes := ""
	for i := range rows {
		r := &rows[i]
		if c, dt, ok := capDe(r.Ano); ok {
			r.Acoes = fp(c * escAno[r.Ano] / 1e6 * fatorApos(dt))
			fonteAcoes = "capital"
		} else if a := acoesDe(*r); a != nil {
			r.Acoes = fp(*a * fatorApos(fmt.Sprintf("%d-03-31", r.Ano+1)))
			if fonteAcoes == "" {
				fonteAcoes = "lpa"
			}
		}
	}
	f.Contas["acoes"] = fonteAcoes
	// preenche ações faltantes com o ano mais próximo
	for i := range rows {
		if rows[i].Acoes == nil {
			for dlt := 1; dlt < len(rows); dlt++ {
				if i-dlt >= 0 && rows[i-dlt].Acoes != nil && !rows[i-dlt].Parcial {
					rows[i].Acoes = rows[i-dlt].Acoes
					break
				}
				if i+dlt < len(rows) && rows[i+dlt].Acoes != nil {
					rows[i].Acoes = rows[i+dlt].Acoes
					break
				}
			}
		}
	}
	acRef := 0.0
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].Acoes != nil {
			acRef = *rows[i].Acoes
			break
		}
	}
	var anosInconsist []int
	for i := range rows {
		r := &rows[i]
		r.MBruta = div(r.LucroBruto, r.Receita)
		r.MEBIT = div(r.EBIT, r.Receita)
		r.MLiq = div(r.Lucro, r.Receita)
		if i > 0 && rows[i-1].Ano == r.Ano-1 {
			p := rows[i-1]
			r.Cresc = cresc(r.Receita, p.Receita)
			r.CrescLucro = cresc(r.LucroCtrl, p.LucroCtrl)
			if r.LucroCtrl != nil && r.PLCtrl != nil && p.PLCtrl != nil {
				r.ROE = fp(*r.LucroCtrl / ((*r.PLCtrl + *p.PLCtrl) / 2))
			}
		} else if r.LucroCtrl != nil && r.PLCtrl != nil {
			r.ROE = div(r.LucroCtrl, r.PLCtrl)
		}
		if r.Proventos != nil && r.LucroCtrl != nil && *r.LucroCtrl > 0 {
			r.Payout = fp(*r.Proventos / *r.LucroCtrl)
		}
		ac := r.Acoes
		if ac != nil && acRef > 0 && (*ac/acRef > 2.5 || *ac/acRef < 0.4) {
			// reorganização societária ou outra companhia listada: valores por ação não comparáveis
			ac = nil
			anosInconsist = append(anosInconsist, r.Ano)
		}
		if r.LPA == nil && ac != nil && r.LucroCtrl != nil && *ac > 0 {
			r.LPA = fp(*r.LucroCtrl / *ac)
		}
		if r.Proventos != nil && ac != nil && *ac > 0 {
			r.ProvAcao = fp(*r.Proventos / *ac)
		}
		if p, dt, ok := precoFimAno(r.Ano); ok {
			r.Preco = fp(p)
			r.PrecoData = dt
			if ac != nil {
				vm := *ac * p // R$ mi
				r.VMbi = fp(vm / 1000)
				if r.LucroCtrl != nil && *r.LucroCtrl != 0 {
					r.PLx = fp(vm / *r.LucroCtrl)
				}
				if r.PLCtrl != nil && *r.PLCtrl > 0 {
					r.PVP = fp(vm / *r.PLCtrl)
				}
				if r.Proventos != nil && vm > 0 {
					r.DY = fp(*r.Proventos / vm)
				}
			}
			if s, y, ok := provDataCom(r.Ano); ok {
				r.ProvDataCom = fp(s)
				r.DYDataCom = fp(y)
			}
		}
	}
	// dívida líquida / EBITDA anual dos indicadores (4º trimestre)
	triPorAno := map[int]Tri{}
	for _, t := range d.Tri {
		if strings.HasPrefix(t.Tri, "4T") {
			triPorAno[t.Ano] = t
		}
	}
	for i := range rows {
		if t, ok := triPorAno[rows[i].Ano]; ok && !f.Financeira {
			rows[i].DivEBITDA = t.DivEBITDA12
		}
	}

	// trimestres e 12 meses
	tris := d.Tri
	var last *Tri
	if n := len(tris); n > 0 {
		last = &tris[n-1]
	}
	if last != nil {
		f.Periodo12m = "12 meses até o " + last.Tri
		soma := func(get func(t Tri) *float64) *float64 {
			if len(tris) < 4 {
				return nil
			}
			s := 0.0
			for _, t := range tris[len(tris)-4:] {
				v := get(t)
				if v == nil {
					return nil
				}
				s += *v
			}
			return fp(s)
		}
		rec12 := last.Receita12
		if rec12 == nil {
			rec12 = soma(func(t Tri) *float64 { return t.Receita })
		}
		lb12 := soma(func(t Tri) *float64 { return t.LucroBruto })
		eb12 := soma(func(t Tri) *float64 { return t.EBIT })
		lu12 := last.Lucro12
		if lu12 == nil {
			lu12 = soma(func(t Tri) *float64 { return t.Lucro })
		}
		lc12 := soma(func(t Tri) *float64 { return t.LucroCtrl })
		if lc12 == nil {
			lc12 = lu12
		}
		if f.Financeira && len(rows) > 0 {
			// para bancos, o resultado antes dos tributos não vem nos indicadores
			eb12 = nil
			lb12 = nil
		}
		m12 := map[string]any{"periodo": f.Periodo12m, "receita": rec12, "lucro_bruto": lb12, "ebit": eb12, "lucro": lu12, "lucro_ctrl": lc12,
			"margem_bruta": div(lb12, rec12), "margem_ebit": div(eb12, rec12), "margem_liq": div(lu12, rec12), "roe": last.ROE12,
			"ebitda": last.EBITDA12, "div_liq": last.DivLiq, "div_ebitda": last.DivEBITDA12, "pl": last.PL, "fco": last.FCO12}
		f.Atual["doze_meses"] = m12
		// linha parcial no histórico anual
		if triKey(last.Ano, last.Tri) > triKey(e.UltDFP, "4T") && rec12 != nil {
			r := AnoRow{Ano: last.Ano, Rotulo: strconv.Itoa(last.Ano) + "*", Parcial: true, Receita: rec12, LucroBruto: lb12, EBIT: eb12, Lucro: lu12, LucroCtrl: lc12,
				PL: last.PL, PLCtrl: last.PL, Ativo: last.Ativo, Caixa: last.Caixa, DivBruta: last.DivBruta, DivLiq: last.DivLiq, FCO: last.FCO12, ROE: last.ROE12}
			if !f.Financeira {
				r.DivEBITDA = last.DivEBITDA12
			}
			r.MBruta, r.MEBIT, r.MLiq = div(lb12, rec12), div(eb12, rec12), div(lu12, rec12)
			if n := len(rows); n > 0 {
				p := rows[n-1]
				r.Cresc = cresc(rec12, p.Receita)
				r.CrescLucro = cresc(lc12, p.LucroCtrl)
				r.Acoes = p.Acoes
			}
			rows = append(rows, r)
		}
	}
	// cotação atual, P/L 12m, DY 12m
	if len(pts) > 0 {
		lp := pts[len(pts)-1]
		f.Atual["cotacao"] = lp.Close
		f.Atual["cotacao_data"] = lp.Data
		lpDate, _ := time.Parse("2006-01-02", lp.Data)
		s12, y12 := 0.0, 0.0
		for i, p := range pts {
			if t, err := time.Parse("2006-01-02", p.Data); err == nil && t.After(lpDate.AddDate(-1, 0, 0)) {
				s12 += provA[i]
				y12 += yld[i]
			}
		}
		f.Atual["proventos_12m_acao"] = s12
		f.Atual["dy_12m"] = y12
		if len(pts) > 12 {
			f.Atual["var_12m"] = lp.Close/(pts[len(pts)-13].Close/fatorEntre(f.Splits, pts[len(pts)-13].Data, lp.Data)) - 1
		}
		// ações hoje: composição do capital mais recente (ITR) ou a do último ano fechado
		var acoesHoje *float64
		var lucro12 *float64
		if m, ok := f.Atual["doze_meses"].(map[string]any); ok {
			lucro12, _ = m["lucro_ctrl"].(*float64)
		}
		ultCap := ""
		for dt := range d.Capital {
			if dt > ultCap {
				ultCap = dt
			}
		}
		if ultCap != "" {
			c := d.Capital[ultCap]
			v := c.Total - c.TesTot
			e := 1.0
			for i := len(rows) - 1; i >= 0; i-- {
				if !rows[i].Parcial && rows[i].Acoes != nil {
					e = perto(v, *rows[i].Acoes/fatorApos(ultCap))
					break
				}
			}
			acoesHoje = fp(v * e / 1e6 * fatorApos(ultCap))
			f.Atual["acoes_data"] = ultCap
		}
		for i := len(rows) - 1; i >= 0; i-- {
			if !rows[i].Parcial && rows[i].Acoes != nil {
				if acoesHoje == nil {
					acoesHoje = rows[i].Acoes
				}
				if lucro12 == nil {
					lucro12 = rows[i].LucroCtrl
				}
				break
			}
		}
		if acoesHoje != nil {
			vm := *acoesHoje * lp.Close
			f.Atual["acoes_mi"] = *acoesHoje
			f.Atual["valor_mercado_bi"] = vm / 1000
			if lucro12 != nil && *lucro12 != 0 {
				f.Atual["p_l"] = vm / *lucro12
				f.Atual["lpa_12m"] = *lucro12 / *acoesHoje
			}
			if last != nil && last.PL != nil && *last.PL > 0 {
				f.Atual["p_vp"] = vm / *last.PL
			}
			if n := len(rows); n > 0 && rows[n-1].Parcial {
				r := &rows[n-1]
				r.Preco = fp(lp.Close)
				r.PrecoData = lp.Data
				r.VMbi = fp(vm / 1000)
				r.Acoes = acoesHoje
				if lucro12 != nil && *lucro12 != 0 {
					r.PLx = fp(vm / *lucro12)
					r.LPA = fp(*lucro12 / *acoesHoje)
				}
				if r.PL != nil && *r.PL > 0 {
					r.PVP = fp(vm / *r.PL)
				}
				r.DYDataCom = fp(y12)
				r.ProvDataCom = fp(s12)
			}
		}
		for i, p := range pts {
			fa := fatorApos(p.Data)
			f.Cotacoes = append(f.Cotacoes, CotOut{Mes: p.Data[:7], Data: p.Data, Close: p.Close / fa, Adj: p.Adj, Prov: provA[i]})
		}
	}
	// P/L médio dos anos fechados com lucro
	var vals []float64
	for _, r := range rows {
		if !r.Parcial && r.PLx != nil && *r.PLx > 0 && *r.PLx < 100 {
			vals = append(vals, *r.PLx)
		}
	}
	md := mediana(vals)
	s, n := 0.0, 0
	for _, v := range vals {
		if v <= md*3 && v >= md/3 {
			s += v
			n++
		}
	}
	if n > 0 {
		f.PLMedia = fp(s / float64(n))
	}
	f.Anual = rows

	// últimos 12 trimestres
	idx := map[int]Tri{}
	for _, t := range tris {
		idx[triKey(t.Ano, t.Tri)] = t
	}
	ini := len(tris) - 12
	if ini < 0 {
		ini = 0
	}
	for _, t := range tris[ini:] {
		o := TriOut{Tri: t}
		if p, ok := idx[triKey(t.Ano-1, t.Tri)]; ok {
			o.VarReceita = cresc(t.Receita, p.Receita)
			o.VarLucro = cresc(t.LucroCtrl, p.LucroCtrl)
		}
		f.Trimestres = append(f.Trimestres, o)
	}

	// notas
	bases := map[string]int{}
	for _, r := range rows {
		if r.Base != "" {
			bases[r.Base]++
		}
	}
	if bases["individual"] > 0 && bases["consolidado"] > 0 {
		f.Notas = append(f.Notas, "Alguns anos só têm demonstrações individuais (sem consolidado) e foram usados assim.")
	} else if bases["individual"] > 0 {
		f.Notas = append(f.Notas, "A companhia publica só demonstrações individuais; os números são da controladora.")
	}
	for _, sp := range f.Splits {
		tipo := "desdobramento"
		if sp.Fator < 1 {
			tipo = "grupamento"
		} else if sp.Fator < 1.9 {
			tipo = "bonificação"
		}
		f.Notas = append(f.Notas, fmt.Sprintf("%s de %s detectado em %s/%s (fator %s): preços e valores por ação anteriores foram ajustados para a base atual.",
			strings.ToUpper(tipo[:1])+tipo[1:], strings.ReplaceAll(strconv.FormatFloat(sp.Fator, 'f', -1, 64), ".", ","), sp.Data[5:7], sp.Data[:4], strings.ReplaceAll(strconv.FormatFloat(sp.Fator, 'f', -1, 64), ".", ",")))
	}
	if len(anosInconsist) > 0 {
		f.Notas = append(f.Notas, fmt.Sprintf("Em %s o número de ações implícito no lucro por ação é muito diferente do atual (reorganização societária, incorporação ou outra companhia listada); P/L, valor de mercado, P/VP e yield desses anos foram omitidos.", listaAnos(anosInconsist)))
	}
	if f.Financeira {
		f.Notas = append(f.Notas, "Instituição financeira: receita = receitas de intermediação financeira; margem bruta = resultado bruto da intermediação ÷ receita; não há EBITDA nem dívida líquida.")
	}
	return f
}

func fatorEntre(sp []Split, de, ate string) float64 {
	x := 1.0
	for _, s := range sp {
		if s.Data > de && s.Data <= ate {
			x *= s.Fator
		}
	}
	return x
}

func acoesDe(r AnoRow) *float64 {
	if r.LPA == nil || r.LucroCtrl == nil || math.Abs(*r.LPA) < 0.005 || *r.LucroCtrl == 0 {
		return nil
	}
	a := *r.LucroCtrl / *r.LPA
	if a <= 0 {
		return nil
	}
	return fp(a)
}

func lpaClasse(dre map[string]float64, nomes map[string]string, classe string) *float64 {
	if dre == nil {
		return nil
	}
	var on, pn *float64
	// lucro básico por ação: 3.99.01.xx (ON / PN)
	for c, v := range dre {
		if !strings.HasPrefix(c, "3.99.01.") {
			continue
		}
		n := strings.ToUpper(strings.TrimSpace(nomes[c]))
		vv := v
		if strings.HasPrefix(n, "ON") || strings.Contains(n, "ORDIN") {
			if on == nil || (*on == 0 && vv != 0) {
				on = &vv
			}
		} else if strings.HasPrefix(n, "PN") || strings.Contains(n, "PREFER") {
			if pn == nil || (*pn == 0 && vv != 0) {
				pn = &vv
			}
		}
	}
	if on == nil {
		if v, ok := dre["3.99.01.01"]; ok {
			on = fp(v)
		}
	}
	if classe == "PN" && pn != nil && *pn != 0 {
		return pn
	}
	if on != nil && *on != 0 {
		return on
	}
	if pn != nil && *pn != 0 {
		return pn
	}
	// alguns anos trazem só o total (3.99.01 ou 3.99), sem abrir por classe
	for _, c := range []string{"3.99.01", "3.99"} {
		if v, ok := dre[c]; ok && v != 0 {
			return fp(v)
		}
	}
	return nil
}

func codsProf(m map[string]float64, prof int) []string {
	out := []string{}
	for c := range m {
		if profundidade(c) == prof {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return codLess(out[i], out[j]) })
	return out
}

func ultimos(x []float64, n int) []float64 {
	if len(x) <= n {
		return x
	}
	return x[len(x)-n:]
}

func mediana(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	y := append([]float64(nil), x...)
	sort.Float64s(y)
	return y[len(y)/2]
}

var fatoresComuns = []float64{0.01, 0.02, 0.05, 0.1, 0.125, 0.2, 0.25, 1.0 / 3, 0.5, 1.1, 1.15, 1.2, 1.25, 1.3, 1.5, 2, 2.5, 3, 4, 5, 6, 8, 10, 20, 50, 100}

// Detecta desdobramentos/grupamentos/bonificações pela quebra entre o fechamento e o ajustado.
func detectarSplits(p []Ponto) []Split {
	out := []Split{}
	for i := 1; i < len(p); i++ {
		a, b := p[i-1], p[i]
		if a.Close <= 0 || b.Close <= 0 || a.Adj <= 0 || b.Adj <= 0 {
			continue
		}
		// adj = close × fator acumulado dos eventos posteriores; num desdobramento de k
		// para 1 no mês b, os preços anteriores são divididos por k: fa/fb = (1 − D/P)/k
		fa, fb := a.Adj/a.Close, b.Adj/b.Close
		dv := 1 - b.Div/a.Close
		if dv <= 0.3 {
			continue
		}
		q := dv / (fa / fb) // ≈ k
		if q > 0.93 && q < 1.08 {
			continue
		}
		best, bd := 0.0, 1.0
		for _, c := range fatoresComuns {
			if d := math.Abs(q/c - 1); d < bd {
				best, bd = c, d
			}
		}
		if bd < 0.05 {
			if best == 1.0/3 {
				best = math.Round(best*1000) / 1000
			}
			out = append(out, Split{Data: b.Data, Fator: best})
		}
	}
	return out
}

func listaAnos(a []int) string {
	if len(a) == 0 {
		return ""
	}
	out := []string{}
	ini, ant := a[0], a[0]
	flush := func() {
		if ini == ant {
			out = append(out, strconv.Itoa(ini))
		} else {
			out = append(out, fmt.Sprintf("%d–%d", ini, ant))
		}
	}
	for _, x := range a[1:] {
		if x == ant+1 {
			ant = x
			continue
		}
		flush()
		ini, ant = x, x
	}
	flush()
	return strings.Join(out, ", ")
}
