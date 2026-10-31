package main

// Fonte "Supabase": lê empresas, demonstrações e trimestres do banco fichas-b3
// pela API REST (PostgREST), com a chave publicável (somente leitura).

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	supaURLPadrao   = "https://crdjqnivmxuzxihgeohm.supabase.co"
	supaChavePadrao = "sb_publishable_ij_YqC0j1gEBTiPNXW5Erg_zF6fjnZi"
	supaPagina      = 1000
)

type Supa struct {
	URL, Chave string
	cli        *http.Client

	mu      sync.Mutex
	lista   []*Empresa
	porTk   map[string]*Empresa
	listaEm time.Time
	meta    map[string]any
	cache   map[int]*EmpData
	cacheEm map[int]time.Time
	res12   map[int]Ult12
	res12Em time.Time
	cot     map[string]cotCache
	merc    map[string]Mercado
	acoes   map[int]Acoes
	mercEm  time.Time
}

func NovoSupa(u, k string) *Supa {
	u = strings.TrimRight(strings.TrimSpace(u), "/")
	return &Supa{URL: u, Chave: strings.TrimSpace(k), cli: &http.Client{Timeout: 40 * time.Second},
		cache: map[int]*EmpData{}, cacheEm: map[int]time.Time{}}
}

func (s *Supa) get(caminho string, q url.Values, offset int, destino any) (int, error) {
	if s.URL == "" || s.Chave == "" {
		return 0, errors.New("informe a URL e a chave do Supabase na configuração")
	}
	if offset >= 0 {
		q.Set("limit", strconv.Itoa(supaPagina))
		q.Set("offset", strconv.Itoa(offset))
	}
	req, _ := http.NewRequest("GET", s.URL+"/rest/v1/"+caminho+"?"+q.Encode(), nil)
	req.Header.Set("apikey", s.Chave)
	if strings.HasPrefix(s.Chave, "eyJ") {
		req.Header.Set("Authorization", "Bearer "+s.Chave)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.cli.Do(req)
	if err != nil {
		return 0, fmt.Errorf("sem conexão com o Supabase: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode >= 300 {
		var e struct {
			Message string `json:"message"`
			Hint    string `json:"hint"`
		}
		_ = json.Unmarshal(body, &e)
		msg := e.Message
		if msg == "" {
			msg = strings.TrimSpace(string(body))
			if len(msg) > 200 {
				msg = msg[:200]
			}
		}
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return 0, fmt.Errorf("o Supabase recusou a chave (%d): %s", resp.StatusCode, msg)
		}
		return 0, fmt.Errorf("Supabase respondeu %d: %s", resp.StatusCode, msg)
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(body, &arr); err != nil {
		return 0, json.Unmarshal(body, destino)
	}
	if err := json.Unmarshal(body, destino); err != nil {
		return 0, err
	}
	return len(arr), nil
}

// busca todas as páginas de uma consulta (a API devolve no máximo 1.000 linhas por vez)
func supaTodos[T any](s *Supa, caminho string, q url.Values) ([]T, error) {
	var out []T
	for off := 0; ; off += supaPagina {
		var pg []T
		qq := url.Values{}
		for k, v := range q {
			qq[k] = v
		}
		n, err := s.get(caminho, qq, off, &pg)
		if err != nil {
			return nil, err
		}
		out = append(out, pg...)
		if n < supaPagina {
			return out, nil
		}
	}
}

type supaEmp struct {
	ID      int      `json:"id"`
	CNPJ    string   `json:"cnpj"`
	CodCVM  *string  `json:"cod_cvm"`
	Nome    *string  `json:"nome"`
	Curto   *string  `json:"nome_curto"`
	Setor   *string  `json:"setor"`
	Tipo    *string  `json:"tipo"`
	Ativa   *bool    `json:"ativa"`
	Tickers []string `json:"tickers"`
	UltDFP  *int     `json:"ult_dfp"`
	UltTri  *string  `json:"ult_tri"`
}

func sv(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// lista de empresas com ticker (guardada por 15 minutos)
func (s *Supa) Lista(forcar bool) ([]*Empresa, error) {
	s.mu.Lock()
	if !forcar && s.lista != nil && time.Since(s.listaEm) < 15*time.Minute {
		l := s.lista
		s.mu.Unlock()
		return l, nil
	}
	s.mu.Unlock()
	q := url.Values{}
	q.Set("select", "id,cnpj,cod_cvm,nome,nome_curto,setor,tipo,ativa,tickers,ult_dfp,ult_tri")
	q.Set("tickers", "neq.{}")
	q.Set("ult_dfp", "not.is.null")
	q.Set("order", "id")
	rows, err := supaTodos[supaEmp](s, "empresas", q)
	if err != nil {
		return nil, err
	}
	lista := make([]*Empresa, 0, len(rows))
	porTk := map[string]*Empresa{}
	for _, r := range rows {
		e := &Empresa{ID: r.ID, CNPJ: r.CNPJ, CodCVM: sv(r.CodCVM), Nome: sv(r.Nome), NomeCurto: sv(r.Curto), Setor: sv(r.Setor),
			Tipo: sv(r.Tipo), Tickers: r.Tickers, UltTri: sv(r.UltTri)}
		if r.Ativa != nil {
			e.Ativa = *r.Ativa
		}
		if r.UltDFP != nil {
			e.UltDFP = *r.UltDFP
		}
		if e.NomeCurto == "" {
			e.NomeCurto = e.Nome
		}
		lista = append(lista, e)
		for _, t := range e.Tickers {
			if _, ok := porTk[t]; !ok {
				porTk[t] = e
			}
		}
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].Tickers[0] < lista[j].Tickers[0] })
	s.mu.Lock()
	s.lista, s.porTk, s.listaEm = lista, porTk, time.Now()
	s.mu.Unlock()
	return lista, nil
}

// último trimestre de cada empresa (por id), numa consulta só; guardado por 15 minutos
func (s *Supa) Ultimos12(forcar bool) (map[int]Ult12, error) {
	s.mu.Lock()
	if !forcar && s.res12 != nil && time.Since(s.res12Em) < 15*time.Minute {
		r := s.res12
		s.mu.Unlock()
		return r, nil
	}
	s.mu.Unlock()
	type linha struct {
		EmpresaID int `json:"empresa_id"`
		Tri
	}
	q := url.Values{}
	q.Set("select", "empresa_id,ano,tri,receita_12m,lucro_12m,lucro_ctrl,roe_12m,div_ebitda_12m,pl")
	// shortcut: só os 2 últimos anos; empresa sem trimestre recente fica sem os números
	q.Set("ano", "gte."+strconv.Itoa(time.Now().Year()-2))
	q.Set("order", "empresa_id,dt_refer")
	rows, err := supaTodos[linha](s, "trimestres", q)
	if err != nil {
		return nil, err
	}
	porEmp := map[int][]Tri{}
	for _, l := range rows {
		porEmp[l.EmpresaID] = append(porEmp[l.EmpresaID], l.Tri) // já em ordem de data
	}
	r := map[int]Ult12{}
	for id, tris := range porEmp {
		r[id] = ult12De(tris)
	}
	s.mu.Lock()
	s.res12, s.res12Em = r, time.Now()
	s.mu.Unlock()
	return r, nil
}

func (s *Supa) PorTicker(t string) (*Empresa, error) {
	if _, err := s.Lista(false); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.porTk[t]; ok {
		return e, nil
	}
	raiz := strings.TrimRightFunc(t, func(r rune) bool { return r >= '0' && r <= '9' })
	for _, e := range s.lista {
		for _, x := range e.Tickers {
			if raiz != "" && strings.TrimRightFunc(x, func(r rune) bool { return r >= '0' && r <= '9' }) == raiz {
				return e, nil
			}
		}
	}
	return nil, nil
}

func (s *Supa) Meta() (map[string]any, error) {
	var m []map[string]any
	if _, err := s.get("meta", url.Values{"select": {"*"}}, -1, &m); err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, errors.New("o banco do Supabase ainda não tem carga concluída")
	}
	s.mu.Lock()
	s.meta = m[0]
	s.mu.Unlock()
	return m[0], nil
}

type supaConta struct {
	Ano   int      `json:"ano"`
	Cod   string   `json:"cod_conta"`
	Valor *float64 `json:"valor"`
	Nome  *struct {
		Nome string `json:"nome"`
	} `json:"contas_nome"`
}

// dados completos de uma empresa (guardados por 30 minutos)
func (s *Supa) Dados(e *Empresa, forcar bool) (*EmpData, error) {
	s.mu.Lock()
	if d, ok := s.cache[e.ID]; ok && !forcar && time.Since(s.cacheEm[e.ID]) < 30*time.Minute {
		s.mu.Unlock()
		return d, nil
	}
	s.mu.Unlock()
	id := strconv.Itoa(e.ID)
	contas := func(tab string) ([]supaConta, error) {
		q := url.Values{}
		q.Set("select", "ano,cod_conta,valor,contas_nome(nome)")
		q.Set("empresa_id", "eq."+id)
		q.Set("order", "ano,cod_conta")
		return supaTodos[supaConta](s, tab, q)
	}
	var (
		wg           sync.WaitGroup
		dre, bp, dfc []supaConta
		bases        []struct {
			Ano  int    `json:"ano"`
			Base string `json:"base"`
		}
		tris []Tri
		caps []struct {
			Dt     string   `json:"dt_refer"`
			ON     *float64 `json:"qt_on"`
			PN     *float64 `json:"qt_pn"`
			Total  *float64 `json:"qt_total"`
			TesON  *float64 `json:"tes_on"`
			TesPN  *float64 `json:"tes_pn"`
			TesTot *float64 `json:"tes_total"`
		}
		errs [6]error
	)
	wg.Add(6)
	go func() { defer wg.Done(); dre, errs[0] = contas("dre") }()
	go func() { defer wg.Done(); bp, errs[1] = contas("bp") }()
	go func() { defer wg.Done(); dfc, errs[2] = contas("dfc") }()
	go func() {
		defer wg.Done()
		_, errs[3] = s.get("base_ano", url.Values{"select": {"ano,base"}, "empresa_id": {"eq." + id}}, -1, &bases)
	}()
	go func() {
		defer wg.Done()
		q := url.Values{"select": {"*"}, "empresa_id": {"eq." + id}, "order": {"dt_refer"}}
		tris, errs[4] = supaTodos[Tri](s, "trimestres", q)
	}()
	go func() {
		defer wg.Done()
		q := url.Values{"select": {"dt_refer,qt_on,qt_pn,qt_total,tes_on,tes_pn,tes_total"}, "empresa_id": {"eq." + id}, "order": {"dt_refer"}}
		_, errs[5] = s.get("capital", q, -1, &caps)
	}()
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	d := &EmpData{CNPJ: e.CNPJ, DRE: map[string]map[string]float64{}, BP: map[string]map[string]float64{}, DFC: map[string][]Conta{},
		NomesDR: map[string]map[string]string{}, NomesBP: map[string]map[string]string{}, BaseAno: map[string]string{}, Capital: map[string]Capital{}}
	encher := func(rows []supaConta, vals map[string]map[string]float64, nomes map[string]map[string]string) {
		for _, r := range rows {
			if r.Valor == nil {
				continue
			}
			a := strconv.Itoa(r.Ano)
			if vals[a] == nil {
				vals[a] = map[string]float64{}
				nomes[a] = map[string]string{}
			}
			vals[a][r.Cod] = *r.Valor
			if r.Nome != nil {
				nomes[a][r.Cod] = r.Nome.Nome
			}
		}
	}
	encher(dre, d.DRE, d.NomesDR)
	encher(bp, d.BP, d.NomesBP)
	for _, r := range dfc {
		if r.Valor == nil {
			continue
		}
		a := strconv.Itoa(r.Ano)
		n := ""
		if r.Nome != nil {
			n = r.Nome.Nome
		}
		d.DFC[a] = append(d.DFC[a], Conta{Cod: r.Cod, Nome: n, V: *r.Valor})
	}
	for a := range d.DFC {
		cs := d.DFC[a]
		sort.Slice(cs, func(i, j int) bool { return codLess(cs[i].Cod, cs[j].Cod) })
	}
	for _, b := range bases {
		d.BaseAno[strconv.Itoa(b.Ano)] = b.Base
	}
	d.Tri = tris
	sort.Slice(d.Tri, func(i, j int) bool { return triKey(d.Tri[i].Ano, d.Tri[i].Tri) < triKey(d.Tri[j].Ano, d.Tri[j].Tri) })
	f := func(p *float64) float64 {
		if p == nil {
			return 0
		}
		return *p
	}
	for _, c := range caps {
		if f(c.Total) > 0 {
			d.Capital[c.Dt] = Capital{ON: f(c.ON), PN: f(c.PN), Total: f(c.Total), TesON: f(c.TesON), TesPN: f(c.TesPN), TesTot: f(c.TesTot)}
		}
	}
	s.mu.Lock()
	s.cache[e.ID] = d
	s.cacheEm[e.ID] = time.Now()
	s.mu.Unlock()
	return d, nil
}

// Cotações mensais do ticker (tabela cotacoes_mensais), guardadas por 30 minutos.
// O banco traz fechamento e proventos já ajustados por desdobramentos e bonificações;
// aqui eles voltam ao valor da época (como a Alpha Vantage entrega), e os eventos
// seguem à parte em Splits, para a ficha ajustar também o lucro por ação da CVM.
func (s *Supa) Cotacoes(ticker string, forcar bool) (*Serie, error) {
	s.mu.Lock()
	if c, ok := s.cot[ticker]; ok && !forcar && time.Since(c.em) < 30*time.Minute {
		s.mu.Unlock()
		return c.serie, nil
	}
	s.mu.Unlock()
	type linha struct {
		Data       string  `json:"data"`
		Fechamento float64 `json:"fechamento"`
		Ajustado   float64 `json:"ajustado"`
		Dividendos float64 `json:"dividendos"`
		Split      float64 `json:"split"`
	}
	q := url.Values{"select": {"data,fechamento,ajustado,dividendos,split"}, "ticker": {"eq." + ticker}, "order": {"mes"}}
	rows, err := supaTodos[linha](s, "cotacoes_mensais", q)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("sem cotações no Supabase para " + ticker)
	}
	splits := []Split{}
	for _, r := range rows {
		if r.Split > 0 && r.Split != 1 {
			splits = append(splits, Split{Data: r.Data, Fator: r.Split})
		}
	}
	pts := make([]Ponto, 0, len(rows))
	for _, r := range rows {
		if r.Fechamento <= 0 {
			continue
		}
		f := fatorEntre(splits, r.Data, "9999") // eventos depois deste mês
		pts = append(pts, Ponto{Data: r.Data, Close: r.Fechamento * f, Adj: r.Ajustado, Div: r.Dividendos * f})
	}
	ult := rows[len(rows)-1].Data
	sr := &Serie{Pontos: pts, Splits: splits, SimboloAV: ticker, BaixadoEm: ult[8:10] + "/" + ult[5:7] + "/" + ult[:4],
		Fonte: "banco fichas-b3 no Supabase (cotações mensais até " + ult[8:10] + "/" + ult[5:7] + "/" + ult[:4] + ")"}
	s.mu.Lock()
	if s.cot == nil {
		s.cot = map[string]cotCache{}
	}
	s.cot[ticker] = cotCache{sr, time.Now()}
	s.mu.Unlock()
	return sr, nil
}

type cotCache struct {
	serie *Serie
	em    time.Time
}

// Mercado: cotação, variação e proventos de 12 meses de um ticker, para a lista
type Mercado struct {
	Preco  float64
	Var12  *float64
	DY12   float64
	Splits []Split // do último ano, para trazer as ações da CVM à base atual
}

// Acoes: ações em circulação informadas à CVM e a data da informação
type Acoes struct {
	Qt   float64
	Data string
}

// Último ano de cotações de todos os tickers e ações em circulação de cada empresa
// (por id), em poucas consultas; guardado por 15 minutos.
func (s *Supa) Mercados(forcar bool) (map[string]Mercado, map[int]Acoes, error) {
	s.mu.Lock()
	if !forcar && s.merc != nil && time.Since(s.mercEm) < 15*time.Minute {
		m, a := s.merc, s.acoes
		s.mu.Unlock()
		return m, a, nil
	}
	s.mu.Unlock()
	type cot struct {
		Ticker     string  `json:"ticker"`
		Data       string  `json:"data"`
		Fechamento float64 `json:"fechamento"`
		Ajustado   float64 `json:"ajustado"`
		Dividendos float64 `json:"dividendos"`
		Split      float64 `json:"split"`
	}
	ini := time.Now().AddDate(-1, -2, 0).Format("2006-01-02")
	cs, err := supaTodos[cot](s, "cotacoes_mensais", url.Values{"select": {"ticker,data,fechamento,ajustado,dividendos,split"}, "data": {"gte." + ini}, "order": {"ticker,mes"}})
	if err != nil {
		return nil, nil, err
	}
	porTk := map[string][]cot{}
	for _, c := range cs {
		if c.Fechamento > 0 {
			porTk[c.Ticker] = append(porTk[c.Ticker], c)
		}
	}
	merc := map[string]Mercado{}
	for tk, l := range porTk {
		u := l[len(l)-1]
		ud, _ := time.Parse("2006-01-02", u.Data)
		corte := ud.AddDate(-1, 0, 0).Format("2006-01-02")
		m := Mercado{Preco: u.Fechamento}
		for i, c := range l {
			if c.Split > 0 && c.Split != 1 {
				m.Splits = append(m.Splits, Split{Data: c.Data, Fator: c.Split})
			}
			if c.Data <= corte {
				m.Var12 = fp(u.Fechamento/c.Fechamento - 1) // fica o último mês até 12 meses antes
				continue
			}
			if i == 0 || c.Ajustado <= 0 || l[i-1].Ajustado <= 0 {
				continue
			}
			// a coluna de proventos é incompleta: o rendimento sai do salto no fechamento
			// ajustado, como na ficha (os dois preços já estão na mesma base de ações)
			a := l[i-1]
			y := 1 - (a.Ajustado/a.Fechamento)/(c.Ajustado/c.Fechamento)
			if y < 0 || y > 0.5 {
				y = c.Dividendos / a.Fechamento
			}
			if y >= 0.001 || c.Dividendos > 0 {
				m.DY12 += y
			}
		}
		merc[tk] = m
	}
	type capt struct {
		EmpresaID int     `json:"empresa_id"`
		Data      string  `json:"dt_refer"`
		Total     float64 `json:"qt_total"`
		Tes       float64 `json:"tes_total"`
	}
	ini = time.Now().AddDate(-2, 0, 0).Format("2006-01-02")
	caps, err := supaTodos[capt](s, "capital", url.Values{"select": {"empresa_id,dt_refer,qt_total,tes_total"}, "dt_refer": {"gte." + ini}, "order": {"empresa_id,dt_refer"}})
	if err != nil {
		return nil, nil, err
	}
	acoes := map[int]Acoes{}
	for _, c := range caps {
		if c.Total > 0 {
			acoes[c.EmpresaID] = Acoes{c.Total - c.Tes, c.Data} // ordenado por data: fica a última
		}
	}
	s.mu.Lock()
	s.merc, s.acoes, s.mercEm = merc, acoes, time.Now()
	s.mu.Unlock()
	return merc, acoes, nil
}

// Log das entregas de documentos das empresas à CVM (RAD) inseridas no banco,
// mais recentes primeiro; vem da função public.log_cvm_empresas (sql/log_cvm_empresas.sql).
func (s *Supa) LogCVM() ([]json.RawMessage, error) {
	return s.rpcLog("log_cvm_empresas")
}

// Situação da atualização de cotações por ticker; função public.log_cotacoes (sql/log_cotacoes.sql).
func (s *Supa) LogCotacoes() ([]json.RawMessage, error) {
	return s.rpcLog("log_cotacoes")
}

func (s *Supa) rpcLog(funcao string) ([]json.RawMessage, error) {
	l, err := supaTodos[json.RawMessage](s, "rpc/"+funcao, url.Values{})
	if err != nil && strings.Contains(err.Error(), funcao) {
		return nil, fmt.Errorf("o log ainda não está ativado no banco: falta criar a função %s (sql/%s.sql)", funcao, funcao)
	}
	return l, err
}

// ContaITR: uma conta das demonstrações de um ITR (R$ milhões; contas 3.99 em R$ por ação)
type ContaITR struct {
	Cod   string   `json:"cod"`
	Nome  string   `json:"nome"`
	Valor *float64 `json:"valor"`
	Ini   string   `json:"dt_ini,omitempty"` // início do período (DRE: trimestre ou acumulado; DFC: acumulado)
}

// Datas dos ITRs com as demonstrações completas no banco (o balanço tem a conta 1 em todos).
func (s *Supa) ITRs(e *Empresa) ([]string, error) {
	var l []struct {
		Dt string `json:"dt_refer"`
	}
	q := url.Values{"select": {"dt_refer"}, "empresa_id": {"eq." + strconv.Itoa(e.ID)}, "cod_conta": {"eq.1"}, "order": {"dt_refer"}}
	if _, err := s.get("bp_tri", q, -1, &l); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(l))
	for _, x := range l {
		if len(out) == 0 || out[len(out)-1] != x.Dt {
			out = append(out, x.Dt)
		}
	}
	return out, nil
}

// DRE, balanço e DFC de um ITR, conta por conta, e a base (consolidado ou individual).
func (s *Supa) ITR(e *Empresa, dt string) (dre, bp, dfc []ContaITR, base string, err error) {
	type linha struct {
		supaConta
		Ini  string `json:"dt_ini"`
		Base string `json:"base"`
	}
	ler := func(tab, sel string) ([]ContaITR, string, error) {
		q := url.Values{"select": {sel + ",contas_nome(nome)"}, "empresa_id": {"eq." + strconv.Itoa(e.ID)}, "dt_refer": {"eq." + dt}, "order": {"cod_conta"}}
		rows, err := supaTodos[linha](s, tab, q)
		out := make([]ContaITR, 0, len(rows))
		b := ""
		for _, r := range rows {
			c := ContaITR{Cod: r.Cod, Valor: r.Valor, Ini: r.Ini}
			if r.Nome != nil {
				c.Nome = r.Nome.Nome
			}
			out = append(out, c)
			b = r.Base
		}
		return out, b, err
	}
	var wg sync.WaitGroup
	var errs [3]error
	wg.Add(3)
	go func() { defer wg.Done(); dre, base, errs[0] = ler("dre_tri", "cod_conta,valor,dt_ini,base") }()
	go func() { defer wg.Done(); bp, _, errs[1] = ler("bp_tri", "cod_conta,valor,base") }()
	go func() { defer wg.Done(); dfc, _, errs[2] = ler("dfc_tri", "cod_conta,valor,dt_ini,base") }()
	wg.Wait()
	for _, x := range errs {
		if x != nil {
			return nil, nil, nil, "", x
		}
	}
	return dre, bp, dfc, base, nil
}

// Fechamentos diários dos últimos 5 anos (já ajustados por desdobramentos); o histórico longo fica na série mensal.
func (s *Supa) CotacoesDiarias(ticker string) ([]json.RawMessage, error) {
	q := url.Values{"select": {"data,fechamento"}, "ticker": {"eq." + ticker}, "data": {"gte." + time.Now().AddDate(-5, 0, 0).Format("2006-01-02")}, "order": {"data"}}
	return supaTodos[json.RawMessage](s, "cotacoes", q)
}

// As 20 maiores empresas por valor de mercado (tabela public.top20_valor_mercado, leitura pública).
type Top20Linha struct {
	Posicao    int         `json:"posicao"`
	Ticker     string      `json:"ticker"`
	Empresa    string      `json:"empresa"`
	Valor      json.Number `json:"valor_mercado_bi"`
	Estimado   bool        `json:"estimado"`
	Referencia string      `json:"referencia"`
}

func (s *Supa) Top20() ([]Top20Linha, error) {
	return supaTodos[Top20Linha](s, "top20_valor_mercado", url.Values{"select": {"posicao,ticker,empresa,valor_mercado_bi,estimado,referencia"}, "order": {"posicao"}})
}

// PDFs do bucket público demonstrativos listados para o ticker (tabela public.demonstrativos).
func (s *Supa) Demonstrativos(ticker string) ([]map[string]any, error) {
	var l []struct {
		Arquivo string `json:"arquivo"`
		Tamanho int64  `json:"tamanho"`
	}
	q := url.Values{"select": {"arquivo,tamanho"}, "ticker": {"eq." + ticker}}
	if _, err := s.get("demonstrativos", q, -1, &l); err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(l))
	for _, a := range l {
		out = append(out, map[string]any{"nome": a.Arquivo, "tamanho": a.Tamanho,
			"url": s.URL + "/storage/v1/object/public/demonstrativos/" + url.PathEscape(ticker) + "/" + url.PathEscape(a.Arquivo)})
	}
	return out, nil
}
