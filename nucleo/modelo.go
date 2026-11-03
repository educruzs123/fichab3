package nucleo

import (
	"strconv"
	"strings"
)

type Empresa struct {
	ID        int      `json:"id,omitempty"` // id no Supabase
	CNPJ      string   `json:"cnpj"`
	CodCVM    string   `json:"cod_cvm"`
	Nome      string   `json:"nome"`
	NomeCurto string   `json:"nome_curto"`
	Setor     string   `json:"setor"`
	Tipo      string   `json:"tipo"`
	Tickers   []string `json:"tickers"`
	UltDFP    int      `json:"ult_dfp"`
	UltTri    string   `json:"ult_tri"`
	Ativa     bool     `json:"ativa"`
	Arquivo   string   `json:"arquivo"`
}

type Conta struct {
	Cod  string  `json:"c"`
	Nome string  `json:"n"`
	V    float64 `json:"v"`
}

type Tri struct {
	Ano         int      `json:"ano"`
	Tri         string   `json:"tri"`
	Base        string   `json:"base"`
	Receita     *float64 `json:"receita"`
	LucroBruto  *float64 `json:"lucro_bruto"`
	EBIT        *float64 `json:"ebit"`
	DA          *float64 `json:"da"`
	EBITDA      *float64 `json:"ebitda"`
	ResFin      *float64 `json:"res_fin"`
	Lucro       *float64 `json:"lucro"`
	LucroCtrl   *float64 `json:"lucro_ctrl"`
	MargemLiq   *float64 `json:"margem_liq"`
	FCO         *float64 `json:"fco"`
	Ativo       *float64 `json:"ativo"`
	Caixa       *float64 `json:"caixa"`
	DivBruta    *float64 `json:"div_bruta"`
	DivLiq      *float64 `json:"div_liq"`
	PL          *float64 `json:"pl"`
	Receita12   *float64 `json:"receita_12m"`
	EBITDA12    *float64 `json:"ebitda_12m"`
	Lucro12     *float64 `json:"lucro_12m"`
	ROE12       *float64 `json:"roe_12m"`
	DivEBITDA12 *float64 `json:"div_ebitda_12m"`
	FCO12       *float64 `json:"fco_12m"`
}

type EmpData struct {
	CNPJ    string                        `json:"cnpj"`
	DRE     map[string]map[string]float64 `json:"dre"`
	BP      map[string]map[string]float64 `json:"bp"`
	DFC     map[string][]Conta            `json:"dfc"`
	NomesDR map[string]map[string]string  `json:"nomes_dre"` // ano -> código -> nome
	NomesBP map[string]map[string]string  `json:"nomes_bp"`
	BaseAno map[string]string             `json:"base_ano"`
	Tri     []Tri                         `json:"tri"`
	Capital map[string]Capital            `json:"capital"` // data de referência -> ações
}

type Capital struct {
	ON     float64 `json:"on"`
	PN     float64 `json:"pn"`
	Total  float64 `json:"total"`
	TesON  float64 `json:"tes_on"`
	TesPN  float64 `json:"tes_pn"`
	TesTot float64 `json:"tes_total"`
}

type Ult12 struct {
	Tri       string   `json:"tri"`
	Receita   *float64 `json:"receita_12m"`
	Lucro     *float64 `json:"lucro_12m"`
	MLiq      *float64 `json:"margem_liq_12m"`
	ROE       *float64 `json:"roe_12m"`
	DivEBITDA *float64 `json:"div_ebitda_12m"`
	PL        *float64 `json:"-"`
	LucroCtrl *float64 `json:"-"` // dos controladores, 4 trimestres (como o P/L da ficha)
}

func ult12De(tris []Tri) Ult12 {
	t := tris[len(tris)-1]
	u := Ult12{Tri: t.Tri, Receita: t.Receita12, Lucro: t.Lucro12, MLiq: div(t.Lucro12, t.Receita12), ROE: t.ROE12, DivEBITDA: t.DivEBITDA12, PL: t.PL}
	u.LucroCtrl = u.Lucro
	if len(tris) >= 4 {
		soma := 0.0
		for _, x := range tris[len(tris)-4:] {
			if x.LucroCtrl == nil {
				return u
			}
			soma += *x.LucroCtrl
		}
		u.LucroCtrl = fp(soma)
	}
	// com patrimônio negativo (no fim ou na média), prejuízo vira ROE positivo
	if (t.PL != nil && *t.PL <= 0) || (u.ROE != nil && *u.ROE > 0 && t.Lucro12 != nil && *t.Lucro12 < 0) {
		u.ROE = nil
	}
	return u
}

func profundidade(cod string) int { return strings.Count(cod, ".") + 1 }

func triKey(ano int, tri string) int {
	q := 0
	if len(tri) > 0 {
		q = int(tri[0] - '0')
	}
	return ano*10 + q
}

func codLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x < y
		}
	}
	return len(pa) < len(pb)
}

func num(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, ".", "")
	s = strings.Replace(s, ",", ".", 1)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func temValores(m map[string]float64, cods ...string) bool {
	for _, c := range cods {
		if v, ok := m[c]; ok && v != 0 {
			return true
		}
	}
	return false
}
