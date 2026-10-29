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
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed web
var webFS embed.FS

const versao = "1.3.0"

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
		if p == "/" || p == "/ficha" || p == "/config" {
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

func apiEmpresas(w http.ResponseWriter, r *http.Request) {
	var b interface{ lista() []*Empresa }
	if usaSupa() {
		l, err := fonteSupa().Lista(r.URL.Query().Get("atualizar") == "1")
		if err != nil {
			jsonErr(w, 502, err.Error())
			return
		}
		b = listaFixa(l)
	} else {
		bancoMu.RLock()
		bb := banco
		bancoMu.RUnlock()
		if bb == nil {
			jsonErr(w, 409, "banco não importado")
			return
		}
		b = listaFixa(bb.Empresas)
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
	}
	out := []item{}
	for _, e := range b.lista() {
		t := ""
		if len(e.Tickers) > 0 {
			t = e.Tickers[0]
		}
		out = append(out, item{t, e.Tickers, e.Nome, e.Setor, e.Tipo, e.CNPJ, e.UltDFP, e.UltTri, e.Ativa})
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
	serie, avErr := ObterCotacoes(t, chave, filepath.Join(appDir, "cache"), forcar)
	f := MontarFicha(e, d, t, serie)
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
