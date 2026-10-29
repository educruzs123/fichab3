package main

import (
	"archive/zip"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------- arquivos de origem ----------

type arqBase struct {
	chave    string
	padrao   string
	desc     string
	opcional bool
}

var arquivosBase = []arqBase{
	{"dre", "DRE_*.csv", "Demonstração do resultado anual (DFP)", false},
	{"bpa", "BP_Ativo_*.csv", "Balanço patrimonial — ativo (DFP)", false},
	{"bpp", "BP_Passivo_*.csv", "Balanço patrimonial — passivo e PL (DFP)", false},
	{"dfc", "DFC_*.csv", "Fluxo de caixa anual (DFP)", false},
	{"tri", "Trimestral_indicadores_*.csv", "Indicadores trimestrais com tickers (ITR)", false},
}

// devolve o arquivo mais recente (por nome) que casa com o padrão
func encontrarArquivo(pasta, padrao string) string {
	m, _ := filepath.Glob(filepath.Join(pasta, padrao))
	if len(m) == 0 {
		return ""
	}
	sort.Strings(m)
	return m[len(m)-1]
}

func verificarArquivos(pasta string) []arqInfo {
	out := []arqInfo{}
	for _, a := range arquivosBase {
		i := arqInfo{Nome: a.padrao, Descricao: a.desc, Opcional: a.opcional}
		if f := encontrarArquivo(pasta, a.padrao); f != "" {
			if st, err := os.Stat(f); err == nil {
				i.Nome = filepath.Base(f)
				i.Existe = true
				i.Tamanho = st.Size()
				i.Alterado = st.ModTime().Format("02/01/2006 15:04")
			}
		}
		out = append(out, i)
	}
	return out
}

// ---------- modelo do banco ----------

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

// valores em R$ milhões
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

// composição do capital (quantidade de ações, como informada à CVM)
type Capital struct {
	ON     float64 `json:"on"`
	PN     float64 `json:"pn"`
	Total  float64 `json:"total"`
	TesON  float64 `json:"tes_on"`
	TesPN  float64 `json:"tes_pn"`
	TesTot float64 `json:"tes_total"`
}

type MetaBanco struct {
	ImportadoEm string    `json:"importado_em"`
	Fonte       string    `json:"fonte"`
	Baixados    []string  `json:"baixados,omitempty"`
	PastaZips   string    `json:"pasta_zips"`
	ZipsLidos   int       `json:"zips_lidos"`
	PastaDados  string    `json:"pasta_dados"`
	Arquivos    []arqInfo `json:"arquivos"`
	NEmpresas   int       `json:"n_empresas"`
	AnoMin      int       `json:"ano_min"`
	AnoMax      int       `json:"ano_max"`
	UltTri      string    `json:"ult_tri"`
	Segundos    float64   `json:"segundos"`
	TamanhoMB   float64   `json:"tamanho_mb"`
}

type Banco struct {
	Pasta    string
	Meta     MetaBanco
	Empresas []*Empresa
	porTk    map[string]*Empresa
	cacheMu  sync.Mutex
	cache    map[string]*EmpData
	ordem    []string
	res12    map[string]Ult12
}

// Ult12: números dos 12 meses até o último trimestre, para a lista de empresas
type Ult12 struct {
	Tri       string   `json:"tri"`
	Receita   *float64 `json:"receita_12m"`
	Lucro     *float64 `json:"lucro_12m"`
	MLiq      *float64 `json:"margem_liq_12m"`
	ROE       *float64 `json:"roe_12m"`
	DivEBITDA *float64 `json:"div_ebitda_12m"`
}

func ult12De(t Tri) Ult12 {
	u := Ult12{Tri: t.Tri, Receita: t.Receita12, Lucro: t.Lucro12, MLiq: div(t.Lucro12, t.Receita12), ROE: t.ROE12, DivEBITDA: t.DivEBITDA12}
	// com patrimônio negativo (no fim ou na média), prejuízo vira ROE positivo
	if (t.PL != nil && *t.PL <= 0) || (u.ROE != nil && *u.ROE > 0 && t.Lucro12 != nil && *t.Lucro12 < 0) {
		u.ROE = nil
	}
	return u
}

// Ultimos12 lê o último trimestre de cada empresa uma vez por banco importado.
func (b *Banco) Ultimos12() map[string]Ult12 {
	b.cacheMu.Lock()
	r := b.res12
	b.cacheMu.Unlock()
	if r != nil {
		return r
	}
	r = map[string]Ult12{}
	for _, e := range b.Empresas {
		if d, err := b.Dados(e.CNPJ); err == nil && len(d.Tri) > 0 {
			r[e.CNPJ] = ult12De(d.Tri[len(d.Tri)-1])
		}
	}
	b.cacheMu.Lock()
	b.res12 = r
	b.cacheMu.Unlock()
	return r
}

func CarregarBanco(pasta string) (*Banco, error) {
	b := &Banco{Pasta: pasta, porTk: map[string]*Empresa{}, cache: map[string]*EmpData{}}
	mb, err := os.ReadFile(filepath.Join(pasta, "meta.json"))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(mb, &b.Meta); err != nil {
		return nil, err
	}
	eb, err := os.ReadFile(filepath.Join(pasta, "empresas.json"))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(eb, &b.Empresas); err != nil {
		return nil, err
	}
	for _, e := range b.Empresas {
		for _, t := range e.Tickers {
			if _, ok := b.porTk[t]; !ok {
				b.porTk[t] = e
			}
		}
	}
	return b, nil
}

func (b *Banco) PorTicker(t string) *Empresa {
	if e, ok := b.porTk[t]; ok {
		return e
	}
	// aceita sem o dígito (ex.: BBDC) ou outra classe (BBDC11 -> BBDC3/4)
	raiz := strings.TrimRightFunc(t, func(r rune) bool { return r >= '0' && r <= '9' })
	for _, e := range b.Empresas {
		for _, x := range e.Tickers {
			if strings.TrimRightFunc(x, func(r rune) bool { return r >= '0' && r <= '9' }) == raiz && raiz != "" {
				return e
			}
		}
	}
	return nil
}

func (b *Banco) Dados(cnpj string) (*EmpData, error) {
	b.cacheMu.Lock()
	if d, ok := b.cache[cnpj]; ok {
		b.cacheMu.Unlock()
		return d, nil
	}
	b.cacheMu.Unlock()
	f, err := os.Open(filepath.Join(b.Pasta, "empresas", digitos(cnpj)+".json.gz"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	var d EmpData
	if err := json.NewDecoder(gz).Decode(&d); err != nil {
		return nil, err
	}
	b.cacheMu.Lock()
	b.cache[cnpj] = &d
	b.ordem = append(b.ordem, cnpj)
	if len(b.ordem) > 40 {
		delete(b.cache, b.ordem[0])
		b.ordem = b.ordem[1:]
	}
	b.cacheMu.Unlock()
	return &d, nil
}

func digitos(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// ---------- importação ----------

type Progresso struct {
	Rodando  bool    `json:"rodando"`
	Etapa    string  `json:"etapa"`
	Pct      float64 `json:"pct"`
	Erro     string  `json:"erro,omitempty"`
	Concluiu string  `json:"concluiu,omitempty"`
	Inicio   string  `json:"inicio,omitempty"`
	Resumo   string  `json:"resumo,omitempty"`
}

var (
	progMu sync.Mutex
	prog   Progresso
)

func progressoAtual() Progresso {
	progMu.Lock()
	defer progMu.Unlock()
	return prog
}

func setProg(f func(p *Progresso)) {
	progMu.Lock()
	f(&prog)
	progMu.Unlock()
}

func IniciarImportacao(pastaDados, pastaBanco, pastaZips string) error {
	progMu.Lock()
	if prog.Rodando {
		progMu.Unlock()
		return errors.New("a importação já está em andamento")
	}
	prog = Progresso{Rodando: true, Etapa: "Verificando arquivos", Inicio: time.Now().Format("15:04:05")}
	progMu.Unlock()
	go func() {
		t0 := time.Now()
		meta, err := importar(pastaDados, pastaBanco, pastaZips)
		if err != nil {
			log.Printf("importação falhou: %v", err)
			setProg(func(p *Progresso) { p.Rodando = false; p.Erro = err.Error() })
			return
		}
		meta.Segundos = time.Since(t0).Seconds()
		mb, _ := json.MarshalIndent(meta, "", "  ")
		_ = os.WriteFile(filepath.Join(pastaBanco, "meta.json"), mb, 0644)
		b, err := CarregarBanco(pastaBanco)
		if err != nil {
			setProg(func(p *Progresso) { p.Rodando = false; p.Erro = "banco gravado mas não abriu: " + err.Error() })
			return
		}
		bancoMu.Lock()
		banco = b
		bancoMu.Unlock()
		log.Printf("importação concluída: %d empresas em %.0fs", meta.NEmpresas, meta.Segundos)
		setProg(func(p *Progresso) {
			p.Rodando = false
			p.Pct = 100
			p.Etapa = "Concluído"
			p.Concluiu = time.Now().Format("02/01/2006 15:04")
			p.Resumo = fmt.Sprintf("%d empresas, DFP %d–%d, trimestres até %s, em %.0f s", meta.NEmpresas, meta.AnoMin, meta.AnoMax, meta.UltTri, meta.Segundos)
		})
	}()
	return nil
}

type contador struct {
	r     io.Reader
	n     int64
	total int64
	base  int64
	all   int64
	etapa string
	ult   time.Time
}

func (c *contador) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if time.Since(c.ult) > 250*time.Millisecond {
		c.ult = time.Now()
		pct := float64(c.base+c.n) / float64(c.all) * 95
		setProg(func(pr *Progresso) { pr.Pct = pct; pr.Etapa = c.etapa })
	}
	return n, err
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

// números no formato "1234,56" (sem separador de milhar)
func numSimples(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
	if err != nil {
		return nil
	}
	return &v
}

func profundidade(cod string) int { return strings.Count(cod, ".") + 1 }

type lerFn func(get func(string) string) error

func lerCSV(caminho string, c *contador, etapa string, fn lerFn) error {
	f, err := os.Open(caminho)
	if err != nil {
		return err
	}
	defer f.Close()
	st, _ := f.Stat()
	c.r = f
	c.n = 0
	c.total = st.Size()
	c.etapa = etapa
	r := csv.NewReader(c)
	r.Comma = ';'
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	r.ReuseRecord = true
	hdr, err := r.Read()
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(caminho), err)
	}
	idx := map[string]int{}
	for i, h := range hdr {
		idx[strings.TrimSpace(strings.TrimPrefix(h, "\ufeff"))] = i
	}
	var rec []string
	get := func(k string) string {
		i, ok := idx[k]
		if !ok || i >= len(rec) {
			return ""
		}
		return rec[i]
	}
	for {
		rec, err = r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if e := fn(get); e != nil {
			return e
		}
	}
	c.base += c.total
	return nil
}

type acum struct {
	emp     *Empresa
	dre     map[string]map[string]float64 // base|ano -> cod -> v
	bp      map[string]map[string]float64
	dfc     map[string]map[string]Conta
	nomesDR map[string]map[string]string // base|ano -> cod -> nome
	nomesBP map[string]map[string]string
	tri     []Tri
	capital map[string]Capital
	anoNome int
}

func importar(pastaDados, pastaBanco, pastaZips string) (MetaBanco, error) {
	meta := MetaBanco{PastaDados: pastaDados}
	caminhos := map[string]string{}
	var total int64
	for _, a := range arquivosBase {
		p := encontrarArquivo(pastaDados, a.padrao)
		if p == "" {
			if a.opcional {
				continue
			}
			return meta, fmt.Errorf("arquivo %s não encontrado em %s", a.padrao, pastaDados)
		}
		st, err := os.Stat(p)
		if err != nil {
			return meta, err
		}
		caminhos[a.chave] = p
		total += st.Size()
	}
	meta.Arquivos = verificarArquivos(pastaDados)
	c := &contador{all: total}

	emps := map[string]*acum{}
	ultTri := ""
	ultTriKey := 0

	// 1) indicadores trimestrais: define as empresas com ticker
	err := lerCSV(caminhos["tri"], c, "Lendo indicadores trimestrais", func(g func(string) string) error {
		cnpj := strings.TrimSpace(g("CNPJ"))
		if cnpj == "" {
			return nil
		}
		a := emps[cnpj]
		if a == nil {
			a = &acum{emp: &Empresa{CNPJ: cnpj}, dre: map[string]map[string]float64{}, bp: map[string]map[string]float64{}, dfc: map[string]map[string]Conta{}, nomesDR: map[string]map[string]string{}, nomesBP: map[string]map[string]string{}}
			emps[cnpj] = a
		}
		e := a.emp
		if tk := g("Ticker(s)"); tk != "" {
			for _, t := range strings.Split(tk, ",") {
				t = strings.ToUpper(strings.TrimSpace(t))
				if t != "" && !contem(e.Tickers, t) {
					e.Tickers = append(e.Tickers, t)
				}
			}
		}
		if n := g("Empresa"); n != "" {
			e.NomeCurto = n
		}
		if t := g("Tipo"); t != "" {
			e.Tipo = t
		}
		ano, _ := strconv.Atoi(g("Ano"))
		tri := g("Trimestre")
		t := Tri{Ano: ano, Tri: tri, Base: g("Base"),
			Receita: numSimples(g("Receita")), LucroBruto: numSimples(g("Lucro bruto")), EBIT: numSimples(g("EBIT")),
			DA: numSimples(g("D&A")), EBITDA: numSimples(g("EBITDA (aprox.)")), ResFin: numSimples(g("Resultado financeiro")),
			Lucro: numSimples(g("Lucro líquido")), LucroCtrl: numSimples(g("Lucro controladores")), MargemLiq: numSimples(g("Margem líquida")),
			FCO: numSimples(g("FCO")), Ativo: numSimples(g("Ativo total")), Caixa: numSimples(g("Caixa + aplicações")),
			DivBruta: numSimples(g("Dívida bruta")), DivLiq: numSimples(g("Dívida líquida")), PL: numSimples(g("Patrimônio líquido")),
			Receita12: numSimples(g("Receita 12m")), EBITDA12: numSimples(g("EBITDA 12m")), Lucro12: numSimples(g("Lucro 12m")),
			ROE12: numSimples(g("ROE 12m")), DivEBITDA12: numSimples(g("Dív. líq./EBITDA 12m")), FCO12: numSimples(g("FCO 12m"))}
		a.tri = append(a.tri, t)
		if k := triKey(ano, tri); k > ultTriKey {
			ultTriKey, ultTri = k, tri
		}
		return nil
	})
	if err != nil {
		return meta, err
	}
	if len(emps) == 0 {
		return meta, errors.New("nenhuma empresa encontrada no arquivo de indicadores trimestrais")
	}
	meta.UltTri = ultTri

	anoMin, anoMax := 9999, 0
	dfp := func(chave, etapa string, destino func(a *acum) map[string]map[string]float64, nomes func(a *acum) map[string]map[string]string, maxProf int) error {
		return lerCSV(caminhos[chave], c, etapa, func(g func(string) string) error {
			a := emps[strings.TrimSpace(g("cnpj"))]
			if a == nil {
				return nil
			}
			cod := g("cod_conta")
			if maxProf > 0 && profundidade(cod) > maxProf {
				return nil
			}
			v := num(g("valor_r$_mil"))
			if v == nil {
				return nil
			}
			ano, _ := strconv.Atoi(g("ano"))
			if ano == 0 {
				return nil
			}
			if ano < anoMin {
				anoMin = ano
			}
			if ano > anoMax {
				anoMax = ano
			}
			k := g("base") + "|" + strconv.Itoa(ano)
			m := destino(a)[k]
			if m == nil {
				m = map[string]float64{}
				destino(a)[k] = m
			}
			val := *v / 1000
			if strings.HasPrefix(cod, "3.99") {
				val = *v // lucro por ação já está em R$/ação
			}
			m[cod] = val
			nm := nomes(a)[k]
			if nm == nil {
				nm = map[string]string{}
				nomes(a)[k] = nm
			}
			nm[cod] = g("conta")
			if ano >= a.anoNome {
				a.anoNome = ano
				e := a.emp
				if n := g("empresa"); n != "" {
					e.Nome = n
				}
				if s := g("setor"); s != "" {
					e.Setor = s
				}
				if cv := g("cod_cvm"); cv != "" {
					e.CodCVM = cv
				}
				e.Ativa = strings.EqualFold(g("situacao_cvm"), "ATIVO")
			}
			return nil
		})
	}
	if err := dfp("dre", "Lendo DRE", func(a *acum) map[string]map[string]float64 { return a.dre }, func(a *acum) map[string]map[string]string { return a.nomesDR }, 0); err != nil {
		return meta, err
	}
	if err := dfp("bpa", "Lendo balanço (ativo)", func(a *acum) map[string]map[string]float64 { return a.bp }, func(a *acum) map[string]map[string]string { return a.nomesBP }, 3); err != nil {
		return meta, err
	}
	if err := dfp("bpp", "Lendo balanço (passivo)", func(a *acum) map[string]map[string]float64 { return a.bp }, func(a *acum) map[string]map[string]string { return a.nomesBP }, 3); err != nil {
		return meta, err
	}
	err = lerCSV(caminhos["dfc"], c, "Lendo fluxo de caixa", func(g func(string) string) error {
		a := emps[strings.TrimSpace(g("cnpj"))]
		if a == nil {
			return nil
		}
		cod := g("cod_conta")
		if profundidade(cod) > 3 {
			return nil
		}
		v := num(g("valor_r$_mil"))
		if v == nil {
			return nil
		}
		k := g("base") + "|" + g("ano")
		m := a.dfc[k]
		if m == nil {
			m = map[string]Conta{}
			a.dfc[k] = m
		}
		m[cod] = Conta{Cod: cod, Nome: g("conta"), V: *v / 1000}
		return nil
	})
	if err != nil {
		return meta, err
	}
	meta.AnoMin, meta.AnoMax = anoMin, anoMax

	// composição do capital nos zips DFP/ITR (opcional)
	meta.PastaZips = pastaZips
	if zs := zipsCapital(pastaZips); len(zs) > 0 {
		for i, z := range zs {
			setProg(func(p *Progresso) { p.Etapa = fmt.Sprintf("Lendo quantidade de ações (%d de %d zips)", i+1, len(zs)) })
			if err := lerCapitalZip(z, emps); err != nil {
				log.Printf("composição do capital em %s: %v", filepath.Base(z), err)
				continue
			}
			meta.ZipsLidos++
		}
	}

	// 2) grava
	n, bytes, err := gravarBanco(emps, pastaBanco)
	if err != nil {
		return meta, err
	}
	meta.NEmpresas = n
	meta.ImportadoEm = time.Now().Format("02/01/2006 15:04")
	meta.TamanhoMB = float64(bytes) / 1e6
	meta.Fonte = "csv"
	return meta, nil
}

func gravarMeta(pastaBanco string, meta MetaBanco) error {
	mb, _ := json.MarshalIndent(meta, "", "  ")
	return os.WriteFile(filepath.Join(pastaBanco, "meta.json"), mb, 0644)
}

// grava as empresas numa pasta nova e troca pela antiga só no fim
func gravarBanco(emps map[string]*acum, pastaBanco string) (int, int64, error) {
	setProg(func(p *Progresso) { p.Etapa = "Gravando o banco"; p.Pct = 96 })
	tmp := pastaBanco + ".novo"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(filepath.Join(tmp, "empresas"), 0755); err != nil {
		return 0, 0, fmt.Errorf("não consegui criar a pasta do banco: %w", err)
	}
	lista := []*Empresa{}
	var bytes int64
	for cnpj, a := range emps {
		if len(a.emp.Tickers) == 0 {
			continue
		}
		d := EmpData{CNPJ: cnpj, DRE: map[string]map[string]float64{}, BP: map[string]map[string]float64{}, DFC: map[string][]Conta{}, NomesDR: map[string]map[string]string{}, NomesBP: map[string]map[string]string{}, BaseAno: map[string]string{}, Tri: a.tri, Capital: a.capital}
		anos := map[string]bool{}
		for k := range a.dre {
			anos[strings.SplitN(k, "|", 2)[1]] = true
		}
		for k := range a.bp {
			anos[strings.SplitN(k, "|", 2)[1]] = true
		}
		ult := 0
		for ano := range anos {
			base := "consolidado"
			if !temValores(a.dre[base+"|"+ano], "3.01", "3.11") && !temValores(a.bp[base+"|"+ano], "1") {
				// sem consolidado (ou consolidado zerado): usa a individual, se existir
				if temValores(a.dre["individual|"+ano], "3.01", "3.11") || temValores(a.bp["individual|"+ano], "1") {
					base = "individual"
				}
			}
			d.BaseAno[ano] = base
			if m, ok := a.dre[base+"|"+ano]; ok {
				nm := a.nomesDR[base+"|"+ano]
				// o lucro por ação às vezes só vem preenchido na demonstração individual
				if base == "consolidado" {
					tem := false
					for c, v := range m {
						if strings.HasPrefix(c, "3.99") && v != 0 {
							tem = true
							break
						}
					}
					if mi, ok := a.dre["individual|"+ano]; ok && !tem {
						ni := a.nomesDR["individual|"+ano]
						for c, v := range mi {
							if strings.HasPrefix(c, "3.99") {
								m[c] = v
								if nm != nil {
									nm[c] = ni[c]
								}
							}
						}
					}
				}
				d.DRE[ano] = m
				d.NomesDR[ano] = nm
			}
			if m, ok := a.bp[base+"|"+ano]; ok {
				d.BP[ano] = m
				d.NomesBP[ano] = a.nomesBP[base+"|"+ano]
			}
			if m, ok := a.dfc[base+"|"+ano]; ok {
				cs := make([]Conta, 0, len(m))
				for _, ct := range m {
					cs = append(cs, ct)
				}
				sort.Slice(cs, func(i, j int) bool { return codLess(cs[i].Cod, cs[j].Cod) })
				d.DFC[ano] = cs
			}
			if n, _ := strconv.Atoi(ano); n > ult {
				ult = n
			}
		}
		sort.Slice(d.Tri, func(i, j int) bool { return triKey(d.Tri[i].Ano, d.Tri[i].Tri) < triKey(d.Tri[j].Ano, d.Tri[j].Tri) })
		a.emp.UltDFP = ult
		if n := len(d.Tri); n > 0 {
			a.emp.UltTri = d.Tri[n-1].Tri
		}
		if a.emp.Nome == "" {
			a.emp.Nome = a.emp.NomeCurto
		}
		a.emp.Arquivo = digitos(cnpj) + ".json.gz"
		n, err := gravarGz(filepath.Join(tmp, "empresas", a.emp.Arquivo), d)
		if err != nil {
			return 0, 0, err
		}
		bytes += n
		lista = append(lista, a.emp)
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].Tickers[0] < lista[j].Tickers[0] })
	eb, _ := json.MarshalIndent(lista, "", " ")
	if err := os.WriteFile(filepath.Join(tmp, "empresas.json"), eb, 0644); err != nil {
		return 0, 0, err
	}
	bytes += int64(len(eb))
	// meta provisória (a definitiva é gravada em seguida por quem chamou)
	_ = os.WriteFile(filepath.Join(tmp, "meta.json"), []byte("{}"), 0644)
	old := pastaBanco + ".antigo"
	_ = os.RemoveAll(old)
	if _, err := os.Stat(pastaBanco); err == nil {
		if err := os.Rename(pastaBanco, old); err != nil {
			return 0, 0, fmt.Errorf("não consegui substituir o banco antigo: %w", err)
		}
	}
	if err := os.Rename(tmp, pastaBanco); err != nil {
		return 0, 0, err
	}
	_ = os.RemoveAll(old)
	return len(lista), bytes, nil
}

func gravarGz(p string, v any) (int64, error) {
	f, err := os.Create(p)
	if err != nil {
		return 0, err
	}
	gz := gzip.NewWriter(f)
	if err := json.NewEncoder(gz).Encode(v); err != nil {
		f.Close()
		return 0, err
	}
	gz.Close()
	st, _ := f.Stat()
	f.Close()
	if st == nil {
		return 0, nil
	}
	return st.Size(), nil
}

func contem(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

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

type zipsInfo struct {
	Pasta  string `json:"pasta"`
	Existe bool   `json:"existe"`
	NDFP   int    `json:"n_dfp"`
	NITR   int    `json:"n_itr"`
	Anos   string `json:"anos"`
}

func verificarZips(pasta string) zipsInfo {
	z := zipsInfo{Pasta: pasta}
	if st, err := os.Stat(pasta); err == nil && st.IsDir() {
		z.Existe = true
	}
	d, _ := filepath.Glob(filepath.Join(pasta, "dfp_cia_aberta_*.zip"))
	i, _ := filepath.Glob(filepath.Join(pasta, "itr_cia_aberta_*.zip"))
	z.NDFP, z.NITR = len(d), len(i)
	if len(d) > 0 {
		sort.Strings(d)
		a := func(p string) string { b := strings.TrimSuffix(filepath.Base(p), ".zip"); return b[len(b)-4:] }
		z.Anos = a(d[0]) + "–" + a(d[len(d)-1])
	}
	return z
}

// todos os DFP + os 2 ITR mais recentes
func zipsCapital(pasta string) []string {
	if pasta == "" {
		return nil
	}
	d, _ := filepath.Glob(filepath.Join(pasta, "dfp_cia_aberta_*.zip"))
	i, _ := filepath.Glob(filepath.Join(pasta, "itr_cia_aberta_*.zip"))
	sort.Strings(d)
	sort.Strings(i)
	if len(i) > 2 {
		i = i[len(i)-2:]
	}
	return append(d, i...)
}

func lerCapitalZip(caminho string, emps map[string]*acum) error {
	zr, err := zip.OpenReader(caminho)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if !strings.Contains(f.Name, "composicao_capital") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		r := csv.NewReader(rc)
		r.Comma = ';'
		r.LazyQuotes = true
		r.FieldsPerRecord = -1
		hdr, err := r.Read()
		if err != nil {
			rc.Close()
			return err
		}
		idx := map[string]int{}
		for i, h := range hdr {
			idx[strings.TrimSpace(strings.TrimPrefix(h, "\ufeff"))] = i
		}
		get := func(rec []string, k string) string {
			if i, ok := idx[k]; ok && i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
			return ""
		}
		fl := func(rec []string, k string) float64 {
			v, _ := strconv.ParseFloat(strings.Replace(get(rec, k), ",", ".", 1), 64)
			return v
		}
		for {
			rec, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				continue
			}
			a := emps[get(rec, "CNPJ_CIA")]
			if a == nil {
				continue
			}
			c := Capital{ON: fl(rec, "QT_ACAO_ORDIN_CAP_INTEGR"), PN: fl(rec, "QT_ACAO_PREF_CAP_INTEGR"), Total: fl(rec, "QT_ACAO_TOTAL_CAP_INTEGR"),
				TesON: fl(rec, "QT_ACAO_ORDIN_TESOURO"), TesPN: fl(rec, "QT_ACAO_PREF_TESOURO"), TesTot: fl(rec, "QT_ACAO_TOTAL_TESOURO")}
			if c.Total <= 0 {
				continue
			}
			if a.capital == nil {
				a.capital = map[string]Capital{}
			}
			a.capital[get(rec, "DT_REFER")] = c // versões mais novas sobrescrevem
		}
		rc.Close()
	}
	return nil
}

func temValores(m map[string]float64, cods ...string) bool {
	for _, c := range cods {
		if v, ok := m[c]; ok && v != 0 {
			return true
		}
	}
	return false
}
