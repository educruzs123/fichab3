// FichaB3 — fichas de ações da B3 a partir da base local da CVM.
// Servidor local (127.0.0.1) com páginas embutidas; abre o navegador ao iniciar.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed web
var webFS embed.FS

const versao = "1.16.0"

var (
	appDir  string
	cfgPath string
	cfgMu   sync.Mutex
	cfg     Config
	banco   *Banco
	bancoMu sync.RWMutex
	supa    *Supa
	supaMu  sync.Mutex
	srv     *http.Server
)

type Config struct {
	PastaDados string `json:"pasta_dados"`
	PastaBanco string `json:"pasta_banco"`
	PastaZips  string `json:"pasta_zips"`
	ChaveAV    string `json:"chave_alpha_vantage"`
	Fonte      string `json:"fonte"` // supabase | local
	SupaURL    string `json:"supabase_url"`
	SupaChave  string `json:"supabase_chave"`
	Porta      int    `json:"porta"`
}

func defaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		PastaDados: filepath.Join(home, "Documents", "CVM DFP 2016-2025", "base"),
		PastaBanco: filepath.Join(home, "FichaB3", "banco"),
		PastaZips:  filepath.Join(home, "Documents", "CVM DFP 2016-2025", "zips"),
		Porta:      47321,
		Fonte:      "supabase",
		SupaURL:    supaURLPadrao,
		SupaChave:  supaChavePadrao,
	}
}

func loadConfig() {
	cfg = defaultConfig()
	b, err := os.ReadFile(cfgPath)
	if err == nil {
		var c Config
		if json.Unmarshal(b, &c) == nil {
			if c.PastaDados != "" {
				cfg.PastaDados = c.PastaDados
			}
			if c.PastaZips != "" {
				cfg.PastaZips = c.PastaZips
			}
			if c.PastaBanco != "" {
				cfg.PastaBanco = c.PastaBanco
			}
			cfg.ChaveAV = c.ChaveAV
			if c.Fonte != "" {
				cfg.Fonte = c.Fonte
			}
			if c.SupaURL != "" {
				cfg.SupaURL = c.SupaURL
			}
			if c.SupaChave != "" {
				cfg.SupaChave = c.SupaChave
			}
			if c.Porta > 0 {
				cfg.Porta = c.Porta
			}
		}
	}
}

func saveConfig() error {
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(cfgPath, b, 0600)
}

func expand(p string) string {
	p = strings.TrimSpace(p)
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return p
}

func abrirNavegador(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("/usr/bin/open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func main() {
	semNavegador := flag.Bool("sem-navegador", false, "não abrir o navegador")
	pastaApp := flag.String("pasta-app", "", "pasta de trabalho (padrão ~/FichaB3)")
	flag.Parse()

	home, _ := os.UserHomeDir()
	appDir = filepath.Join(home, "FichaB3")
	if *pastaApp != "" {
		appDir = expand(*pastaApp)
	}
	_ = os.MkdirAll(appDir, 0755)
	cfgPath = filepath.Join(appDir, "config.json")
	if lf, err := os.OpenFile(filepath.Join(appDir, "fichab3.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		log.SetOutput(io.MultiWriter(lf, os.Stderr))
	}
	loadConfig()
	_ = saveConfig() // grava os campos novos (fonte Supabase) na configuração existente
	supa = NovoSupa(cfg.SupaURL, cfg.SupaChave)

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Porta)
	url := "http://" + addr + "/"
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// já está rodando: só abre o navegador
		log.Printf("porta %d em uso, abrindo a janela existente", cfg.Porta)
		if !*semNavegador {
			abrirNavegador(url)
		}
		return
	}

	go func() {
		if b, err := CarregarBanco(cfg.PastaBanco); err == nil {
			bancoMu.Lock()
			banco = b
			bancoMu.Unlock()
			log.Printf("banco carregado: %d empresas", len(b.Empresas))
		} else {
			log.Printf("banco ainda não importado: %v", err)
		}
	}()

	mux := http.NewServeMux()
	sub, _ := fs.Sub(webFS, "web")
	static := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "/" || p == "/ficha" || p == "/itr" || p == "/config" {
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFileFS(w, r, sub, "index.html")
			return
		}
		static.ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/status", apiStatus)
	mux.HandleFunc("/api/config", apiConfig)
	mux.HandleFunc("/api/escolher-pasta", apiEscolherPasta)
	mux.HandleFunc("/api/importar", apiImportar)
	mux.HandleFunc("/api/importar/progresso", apiProgresso)
	mux.HandleFunc("/api/atualizar-cvm", apiAtualizarCVM)
	mux.HandleFunc("/api/testar-supabase", apiTestarSupabase)
	mux.HandleFunc("/api/empresas", apiEmpresas)
	mux.HandleFunc("/api/log-cvm", apiLogCVM)
	mux.HandleFunc("/api/log-cotacoes", apiLogCotacoes)
	mux.HandleFunc("/api/cotacoes-diarias", apiCotacoesDiarias)
	mux.HandleFunc("/api/demonstrativos", apiDemonstrativos)
	mux.HandleFunc("/demonstrativos/", servirDemonstrativo)
	mux.HandleFunc("/api/itr", apiITR)
	mux.HandleFunc("/api/ficha", apiFicha)
	mux.HandleFunc("/api/testar-chave", apiTestarChave)
	mux.HandleFunc("/api/limpar-cache", apiLimparCache)
	mux.HandleFunc("/api/abrir-pasta", apiAbrirPasta)
	mux.HandleFunc("/api/encerrar", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "use POST", 405)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		go func() {
			time.Sleep(300 * time.Millisecond)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctx)
			os.Exit(0)
		}()
	})

	srv = &http.Server{Handler: localOnly(mux)}
	log.Printf("FichaB3Completa %s em %s (dados: %s)", versao, url, cfg.PastaDados)
	if !*semNavegador {
		go func() { time.Sleep(400 * time.Millisecond); abrirNavegador(url) }()
	}
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// Recusa pedidos que não venham do próprio computador ou de outra origem.
func localOnly(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if host != "127.0.0.1" && host != "::1" {
			http.Error(w, "somente local", 403)
			return
		}
		if o := r.Header.Get("Origin"); o != "" && !strings.HasPrefix(o, "http://127.0.0.1:") && !strings.HasPrefix(o, "http://localhost:") {
			http.Error(w, "origem não permitida", 403)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func jsonErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"erro": msg})
}

type arqInfo struct {
	Nome      string `json:"nome"`
	Existe    bool   `json:"existe"`
	Tamanho   int64  `json:"tamanho"`
	Alterado  string `json:"alterado"`
	Descricao string `json:"descricao"`
	Opcional  bool   `json:"opcional"`
}

func apiStatus(w http.ResponseWriter, r *http.Request) {
	cfgMu.Lock()
	c := cfg
	cfgMu.Unlock()
	bancoMu.RLock()
	b := banco
	bancoMu.RUnlock()
	st := map[string]any{
		"versao":       versao,
		"pasta_app":    appDir,
		"fonte":        c.Fonte,
		"config":       map[string]any{"pasta_dados": c.PastaDados, "pasta_banco": c.PastaBanco, "pasta_zips": c.PastaZips, "tem_chave": c.ChaveAV != "", "chave_mascarada": mascarar(c.ChaveAV), "porta": c.Porta, "fonte": c.Fonte, "supabase_url": c.SupaURL, "supabase_chave": c.SupaChave},
		"arquivos":     verificarArquivos(c.PastaDados),
		"zips":         verificarZips(c.PastaZips),
		"banco_pronto": b != nil,
		"importando":   progressoAtual().Rodando,
	}
	if b != nil {
		st["banco"] = b.Meta
	}
	if c.Fonte == "supabase" {
		sp := map[string]any{"url": c.SupaURL}
		if m, err := fonteSupa().Meta(); err != nil {
			sp["erro"] = err.Error()
			st["banco_pronto"] = false
		} else {
			sp["meta"] = m
			st["banco_pronto"] = true
		}
		st["supabase"] = sp
	}
	writeJSON(w, st)
}

func mascarar(k string) string {
	if len(k) <= 4 {
		return strings.Repeat("•", len(k))
	}
	return strings.Repeat("•", len(k)-4) + k[len(k)-4:]
}

func apiConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", 405)
		return
	}
	var in struct {
		PastaDados *string `json:"pasta_dados"`
		PastaBanco *string `json:"pasta_banco"`
		PastaZips  *string `json:"pasta_zips"`
		ChaveAV    *string `json:"chave_alpha_vantage"`
		Fonte      *string `json:"fonte"`
		SupaURL    *string `json:"supabase_url"`
		SupaChave  *string `json:"supabase_chave"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		jsonErr(w, 400, "pedido inválido")
		return
	}
	cfgMu.Lock()
	mudouBanco := false
	if in.PastaDados != nil {
		cfg.PastaDados = expand(*in.PastaDados)
	}
	if in.PastaBanco != nil && expand(*in.PastaBanco) != cfg.PastaBanco {
		cfg.PastaBanco = expand(*in.PastaBanco)
		mudouBanco = true
	}
	if in.PastaZips != nil {
		cfg.PastaZips = expand(*in.PastaZips)
	}
	if in.ChaveAV != nil {
		cfg.ChaveAV = strings.TrimSpace(*in.ChaveAV)
	}
	mudouSupa := false
	if in.Fonte != nil && (*in.Fonte == "supabase" || *in.Fonte == "local") {
		cfg.Fonte = *in.Fonte
	}
	if in.SupaURL != nil && strings.TrimSpace(*in.SupaURL) != "" {
		cfg.SupaURL = strings.TrimRight(strings.TrimSpace(*in.SupaURL), "/")
		mudouSupa = true
	}
	if in.SupaChave != nil && strings.TrimSpace(*in.SupaChave) != "" {
		cfg.SupaChave = strings.TrimSpace(*in.SupaChave)
		mudouSupa = true
	}
	if mudouSupa {
		supaMu.Lock()
		supa = NovoSupa(cfg.SupaURL, cfg.SupaChave)
		supaMu.Unlock()
	}
	err := saveConfig()
	pb := cfg.PastaBanco
	cfgMu.Unlock()
	if err != nil {
		jsonErr(w, 500, "não consegui salvar a configuração: "+err.Error())
		return
	}
	if mudouBanco {
		b, err := CarregarBanco(pb)
		bancoMu.Lock()
		if err == nil {
			banco = b
		} else {
			banco = nil
		}
		bancoMu.Unlock()
	}
	writeJSON(w, map[string]any{"ok": true})
}

func apiEscolherPasta(w http.ResponseWriter, r *http.Request) {
	if runtime.GOOS != "darwin" {
		jsonErr(w, 400, "o seletor de pastas só funciona no macOS; digite o caminho")
		return
	}
	prompt := r.URL.Query().Get("titulo")
	if prompt == "" {
		prompt = "Escolha a pasta"
	}
	prompt = strings.ReplaceAll(prompt, `"`, "'")
	script := fmt.Sprintf(`tell me to activate
set p to POSIX path of (choose folder with prompt "%s")
return p`, prompt)
	out, err := exec.Command("/usr/bin/osascript", "-e", script).Output()
	if err != nil {
		writeJSON(w, map[string]any{"cancelado": true})
		return
	}
	p := strings.TrimRight(strings.TrimSpace(string(out)), "/")
	writeJSON(w, map[string]any{"pasta": p})
}

func apiAbrirPasta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", 405)
		return
	}
	cfgMu.Lock()
	p := cfg.PastaDados
	if r.URL.Query().Get("qual") == "app" {
		p = appDir
	} else if r.URL.Query().Get("qual") == "zips" {
		p = cfg.PastaZips
	} else if r.URL.Query().Get("qual") == "banco" {
		p = cfg.PastaBanco
	}
	cfgMu.Unlock()
	if runtime.GOOS == "darwin" {
		_ = exec.Command("/usr/bin/open", p).Start()
	}
	writeJSON(w, map[string]any{"ok": true})
}

func apiImportar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", 405)
		return
	}
	cfgMu.Lock()
	pd, pb, pz := cfg.PastaDados, cfg.PastaBanco, cfg.PastaZips
	cfgMu.Unlock()
	if err := IniciarImportacao(pd, pb, pz); err != nil {
		jsonErr(w, 409, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func apiProgresso(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, progressoAtual())
}

func fonteSupa() *Supa {
	supaMu.Lock()
	defer supaMu.Unlock()
	return supa
}

func usaSupa() bool {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	return cfg.Fonte == "supabase"
}

func apiTestarSupabase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", 405)
		return
	}
	var in struct {
		URL   string `json:"url"`
		Chave string `json:"chave"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	s := fonteSupa()
	if strings.TrimSpace(in.URL) != "" || strings.TrimSpace(in.Chave) != "" {
		u, k := in.URL, in.Chave
		if strings.TrimSpace(u) == "" {
			u = s.URL
		}
		if strings.TrimSpace(k) == "" {
			k = s.Chave
		}
		s = NovoSupa(u, k)
	}
	m, err := s.Meta()
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	l, err := s.Lista(true)
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "meta": m, "empresas": len(l)})
}

var reData = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// /api/itr?t=VALE3 lista os ITRs com demonstrações completas; com &dt=2026-06-30 traz as contas
func apiITR(w http.ResponseWriter, r *http.Request) {
	if !usaSupa() {
		jsonErr(w, 409, "as demonstrações de cada ITR só existem com a fonte Supabase")
		return
	}
	t := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("t")))
	dt := r.URL.Query().Get("dt")
	if dt != "" && !reData.MatchString(dt) {
		jsonErr(w, 400, "data inválida: use AAAA-MM-DD")
		return
	}
	s := fonteSupa()
	e, err := s.PorTicker(t)
	if err != nil {
		jsonErr(w, 502, err.Error())
		return
	}
	if e == nil {
		jsonErr(w, 404, "ticker não encontrado no banco do Supabase: "+t)
		return
	}
	itrs, err := s.ITRs(e)
	if err != nil {
		jsonErr(w, 502, err.Error())
		return
	}
	out := map[string]any{"ticker": t, "empresa": e.NomeCurto, "tickers": e.Tickers, "itrs": itrs}
	if dt != "" {
		dre, bp, dfc, base, err := s.ITR(e, dt)
		if err != nil {
			jsonErr(w, 502, err.Error())
			return
		}
		if len(dre)+len(bp)+len(dfc) == 0 {
			jsonErr(w, 404, "o banco não tem as demonstrações de "+t+" em "+dt)
			return
		}
		out["dt"], out["base"], out["dre"], out["bp"], out["dfc"] = dt, base, dre, bp, dfc
	}
	writeJSON(w, out)
}

var (
	reTickerPasta = regexp.MustCompile(`^[A-Z0-9]{4,6}$`)
	reArquivoPDF  = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9_.-]*\.pdf$`)
)

// /api/demonstrativos?t=TIMS3 lista os PDFs do Supabase e os de ~/FichaB3/demonstrativos/TIMS3 que não estejam lá
func apiDemonstrativos(w http.ResponseWriter, r *http.Request) {
	t := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("t")))
	out := []map[string]any{}
	if !reTickerPasta.MatchString(t) {
		writeJSON(w, out)
		return
	}
	noBanco := map[string]bool{}
	if usaSupa() {
		l, err := fonteSupa().Demonstrativos(t)
		if err != nil {
			log.Printf("demonstrativos de %s no Supabase: %v", t, err)
		}
		for _, a := range l {
			noBanco[a["nome"].(string)] = true
			out = append(out, a)
		}
	}
	itens, _ := os.ReadDir(filepath.Join(appDir, "demonstrativos", t))
	for _, it := range itens {
		if info, err := it.Info(); err == nil && info.Mode().IsRegular() && reArquivoPDF.MatchString(it.Name()) && !noBanco[it.Name()] {
			out = append(out, map[string]any{"nome": it.Name(), "tamanho": info.Size(), "url": "/demonstrativos/" + t + "/" + it.Name()})
		}
	}
	writeJSON(w, out)
}

// entrega só PDFs diretamente dentro da pasta do ticker; nome e ticker validados contra as listas acima
func servirDemonstrativo(w http.ResponseWriter, r *http.Request) {
	partes := strings.Split(strings.TrimPrefix(r.URL.Path, "/demonstrativos/"), "/")
	if len(partes) != 2 || !reTickerPasta.MatchString(partes[0]) || !reArquivoPDF.MatchString(partes[1]) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	http.ServeFile(w, r, filepath.Join(appDir, "demonstrativos", partes[0], partes[1]))
}

func apiCotacoesDiarias(w http.ResponseWriter, r *http.Request) {
	if !usaSupa() {
		jsonErr(w, 409, "as cotações diárias só existem com a fonte Supabase")
		return
	}
	t := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("t")))
	l, err := fonteSupa().CotacoesDiarias(t)
	if err != nil {
		jsonErr(w, 502, err.Error())
		return
	}
	// último pregão do Yahoo, se o banco ainda não o tiver
	if u := UltimaCotacao(t, false); u != nil && len(l) > 0 {
		var ult struct {
			Data string `json:"data"`
		}
		if json.Unmarshal(l[len(l)-1], &ult) == nil && u.Data > ult.Data {
			if b, err := json.Marshal(map[string]any{"data": u.Data, "fechamento": u.Preco}); err == nil {
				l = append(l, b)
			}
		}
	}
	writeJSON(w, l)
}

func apiLogCotacoes(w http.ResponseWriter, r *http.Request) {
	if !usaSupa() {
		jsonErr(w, 409, "o log de cotações só existe com a fonte Supabase")
		return
	}
	l, err := fonteSupa().LogCotacoes()
	if err != nil {
		jsonErr(w, 502, err.Error())
		return
	}
	writeJSON(w, l)
}

func apiLogCVM(w http.ResponseWriter, r *http.Request) {
	if !usaSupa() {
		jsonErr(w, 409, "o log de inserções da CVM só existe com a fonte Supabase")
		return
	}
	l, err := fonteSupa().LogCVM()
	if err != nil {
		jsonErr(w, 502, err.Error())
		return
	}
	writeJSON(w, l)
}

func apiEmpresas(w http.ResponseWriter, r *http.Request) {
	var b interface{ lista() []*Empresa }
	ult12 := func(e *Empresa) (Ult12, bool) { return Ult12{}, false }
	var merc map[string]Mercado
	var acoes map[int]Acoes
	if usaSupa() {
		forcar := r.URL.Query().Get("atualizar") == "1"
		l, err := fonteSupa().Lista(forcar)
		if err != nil {
			jsonErr(w, 502, err.Error())
			return
		}
		b = listaFixa(l)
		if res, err := fonteSupa().Ultimos12(forcar); err == nil {
			ult12 = func(e *Empresa) (Ult12, bool) { u, ok := res[e.ID]; return u, ok }
		} else {
			log.Printf("lista: sem os números de 12 meses do Supabase: %v", err)
		}
		if m, a, err := fonteSupa().Mercados(forcar); err == nil {
			merc, acoes = m, a
		} else {
			log.Printf("lista: sem cotações do Supabase: %v", err)
		}
	} else {
		bancoMu.RLock()
		bb := banco
		bancoMu.RUnlock()
		if bb == nil {
			jsonErr(w, 409, "banco não importado")
			return
		}
		b = listaFixa(bb.Empresas)
		res := bb.Ultimos12()
		ult12 = func(e *Empresa) (Ult12, bool) { u, ok := res[e.CNPJ]; return u, ok }
	}
	type item struct {
		Ticker  string   `json:"ticker"`
		Tickers []string `json:"tickers"`
		Nome    string   `json:"nome"`
		Setor   string   `json:"setor"`
		Tipo    string   `json:"tipo"`
		CNPJ    string   `json:"cnpj"`
		UltDFP  int      `json:"ult_dfp"`
		UltTri  string   `json:"ult_tri"`
		Ativa   bool     `json:"ativa"`
		*Ult12
		Cotacao *float64 `json:"cotacao,omitempty"`
		Var12   *float64 `json:"var_12m,omitempty"`
		DY12    *float64 `json:"dy_12m,omitempty"`
		PLx     *float64 `json:"p_l,omitempty"`
	}
	out := []item{}
	for _, e := range b.lista() {
		t := ""
		if len(e.Tickers) > 0 {
			t = e.Tickers[0]
		}
		it := item{t, e.Tickers, e.Nome, e.Setor, e.Tipo, e.CNPJ, e.UltDFP, e.UltTri, e.Ativa, nil, nil, nil, nil, nil}
		u, temU := ult12(e)
		if temU {
			it.Ult12 = &u
		}
		if m, ok := merc[t]; ok {
			it.Cotacao, it.Var12, it.DY12 = fp(m.Preco), m.Var12, fp(m.DY12)
			// lucro mínimo de R$ 100 mil: perto de zero, o P/L explode sem significado
			if a := acoes[e.ID]; a.Qt > 0 && temU && u.LucroCtrl != nil && *u.LucroCtrl >= 0.1 && u.PL != nil && *u.PL > 0 {
				vm := m.Preco * a.Qt * fatorEntre(m.Splits, a.Data, "9999") / acoesPorUnit(t) / 1e6 // R$ milhões, na base de ações de hoje
				// a CVM às vezes informa as ações em milhares: valor de mercado 1000× menor que o normal
				if vm / *u.PL < 0.02 {
					vm *= 1000
				}
				it.PLx = fp(vm / *u.LucroCtrl)
			}
		}
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ticker < out[j].Ticker })
	writeJSON(w, out)
}

func apiFicha(w http.ResponseWriter, r *http.Request) {
	t := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("t")))
	forcar := r.URL.Query().Get("atualizar") == "1"
	var e *Empresa
	var d *EmpData
	var err error
	if usaSupa() {
		s := fonteSupa()
		if e, err = s.PorTicker(t); err != nil {
			jsonErr(w, 502, err.Error())
			return
		}
		if e == nil {
			jsonErr(w, 404, "ticker não encontrado no banco do Supabase: "+t)
			return
		}
		if d, err = s.Dados(e, forcar); err != nil {
			jsonErr(w, 502, "erro ao ler a empresa no Supabase: "+err.Error())
			return
		}
	} else {
		bancoMu.RLock()
		b := banco
		bancoMu.RUnlock()
		if b == nil {
			jsonErr(w, 409, "banco não importado")
			return
		}
		if e = b.PorTicker(t); e == nil {
			jsonErr(w, 404, "ticker não encontrado na base: "+t)
			return
		}
		if d, err = b.Dados(e.CNPJ); err != nil {
			jsonErr(w, 500, "erro ao ler dados da empresa: "+err.Error())
			return
		}
	}
	cfgMu.Lock()
	chave := cfg.ChaveAV
	cfgMu.Unlock()
	var serie *Serie
	var avErr error
	if usaSupa() {
		serie, avErr = fonteSupa().Cotacoes(t, forcar)
		if avErr != nil {
			log.Printf("cotações do Supabase para %s: %v; tentando a Alpha Vantage", t, avErr)
		}
	}
	if serie == nil {
		serie, avErr = ObterCotacoes(t, chave, filepath.Join(appDir, "cache"), forcar)
	}
	ult := UltimaCotacao(t, forcar)
	serie = comUltimaCotacao(serie, ult)
	f := MontarFicha(e, d, t, serie)
	if ult != nil && f.Atual["cotacao_data"] == ult.Data {
		f.Atual["cotacao_hora"] = ult.Hora
		if ult.Anterior > 0 {
			f.Atual["var_dia"] = ult.Preco/ult.Anterior - 1
			f.Atual["cotacao_anterior"] = ult.Anterior
		}
		f.FonteCot += "; última cotação: Yahoo Finance"
	}
	if usaSupa() {
		f.Fonte = "supabase"
	} else {
		f.Fonte = "local"
	}
	if avErr != nil {
		f.AvisoCotacao = avErr.Error()
	}
	writeJSON(w, f)
}

func apiTestarChave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", 405)
		return
	}
	var in struct {
		Chave string `json:"chave"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	k := strings.TrimSpace(in.Chave)
	if k == "" {
		cfgMu.Lock()
		k = cfg.ChaveAV
		cfgMu.Unlock()
	}
	if k == "" {
		jsonErr(w, 400, "informe a chave")
		return
	}
	p, data, err := TestarChave(k)
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "preco": p, "data": data})
}

func apiLimparCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", 405)
		return
	}
	dir := filepath.Join(appDir, "cache")
	n := 0
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "av_") {
			if os.Remove(filepath.Join(dir, e.Name())) == nil {
				n++
			}
		}
	}
	writeJSON(w, map[string]any{"ok": true, "removidos": n})
}

func apiAtualizarCVM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", 405)
		return
	}
	cfgMu.Lock()
	pz, pb := cfg.PastaZips, cfg.PastaBanco
	cfgMu.Unlock()
	if err := IniciarAtualizacaoCVM(pz, pb, r.URL.Query().Get("baixar") != "0"); err != nil {
		jsonErr(w, 409, err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

type listaFixa []*Empresa

func (l listaFixa) lista() []*Empresa { return l }
