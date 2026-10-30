package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

type Ponto struct {
	Data  string  `json:"data"`
	Close float64 `json:"fechamento"`
	Adj   float64 `json:"ajustado"`
	Div   float64 `json:"provento"`
}

type Serie struct {
	Pontos    []Ponto `json:"pontos"`
	BaixadoEm string  `json:"baixado_em"`
	DoCache   bool    `json:"do_cache"`
	Desatual  bool    `json:"desatualizada"`
	SimboloAV string  `json:"simbolo"`
	Fonte     string  `json:"fonte,omitempty"`
	Splits    []Split `json:"splits,omitempty"` // informados pela fonte; nil = detectar nos preços
}

var avClient = &http.Client{Timeout: 30 * time.Second}

func avGet(params url.Values) (map[string]any, []byte, error) {
	u := "https://www.alphavantage.co/query?" + params.Encode()
	resp, err := avClient.Get(u)
	if err != nil {
		return nil, nil, fmt.Errorf("sem conexão com a Alpha Vantage: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, nil, errors.New("resposta inválida da Alpha Vantage")
	}
	for _, k := range []string{"Error Message", "Note", "Information"} {
		if v, ok := m[k]; ok {
			s := fmt.Sprint(v)
			if k == "Error Message" {
				return nil, nil, errors.New("a Alpha Vantage não reconheceu o ativo")
			}
			if len(s) > 220 {
				s = s[:220] + "…"
			}
			return nil, nil, errors.New("Alpha Vantage: " + s)
		}
	}
	return m, body, nil
}

func parseSerie(m map[string]any) []Ponto {
	ts, _ := m["Monthly Adjusted Time Series"].(map[string]any)
	out := make([]Ponto, 0, len(ts))
	for d, v := range ts {
		r, _ := v.(map[string]any)
		f := func(k string) float64 { x, _ := strconv.ParseFloat(fmt.Sprint(r[k]), 64); return x }
		p := Ponto{Data: d, Close: f("4. close"), Adj: f("5. adjusted close"), Div: f("7. dividend amount")}
		if p.Close > 0 {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Data < out[j].Data })
	return out
}

func lerCache(p string) (map[string]any, time.Time, bool) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, time.Time{}, false
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return nil, time.Time{}, false
	}
	st, _ := os.Stat(p)
	t := st.ModTime()
	if s, ok := m["_baixado_em"].(string); ok {
		for _, lay := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "02/01/2006 15:04"} {
			if tt, err := time.ParseInLocation(lay, s, time.Local); err == nil {
				t = tt
				break
			}
		}
	}
	return m, t, true
}

// Série mensal ajustada (fechamento, fechamento ajustado por proventos, proventos)
func ObterCotacoes(ticker, chave, cacheDir string, forcar bool) (*Serie, error) {
	_ = os.MkdirAll(cacheDir, 0755)
	cp := filepath.Join(cacheDir, "av_"+ticker+".json")
	legado := filepath.Join(appDir, "fichas", "cache", "av_"+ticker+".json")
	m, t, ok := lerCache(cp)
	if !ok {
		m, t, ok = lerCache(legado)
	}
	fresco := ok && time.Since(t) < 12*time.Hour
	if ok && fresco && !forcar {
		return &Serie{Pontos: parseSerie(m), BaixadoEm: t.Format("02/01/2006 15:04"), DoCache: true, SimboloAV: ticker + ".SAO"}, nil
	}
	if chave == "" {
		if ok {
			return &Serie{Pontos: parseSerie(m), BaixadoEm: t.Format("02/01/2006 15:04"), DoCache: true, Desatual: true, SimboloAV: ticker + ".SAO"},
				errors.New("sem chave da Alpha Vantage: usando cotações salvas em " + t.Format("02/01/2006"))
		}
		return nil, errors.New("cadastre a chave da Alpha Vantage na configuração para ver cotação, P/L e dividend yield")
	}
	nm, body, err := avGet(url.Values{"function": {"TIME_SERIES_MONTHLY_ADJUSTED"}, "symbol": {ticker + ".SAO"}, "apikey": {chave}})
	if err != nil {
		if ok {
			return &Serie{Pontos: parseSerie(m), BaixadoEm: t.Format("02/01/2006 15:04"), DoCache: true, Desatual: true, SimboloAV: ticker + ".SAO"},
				fmt.Errorf("%v — usando cotações salvas em %s", err, t.Format("02/01/2006"))
		}
		return nil, err
	}
	_ = body
	nm["_baixado_em"] = time.Now().Format(time.RFC3339)
	if b, err := json.Marshal(nm); err == nil {
		_ = os.WriteFile(cp, b, 0644)
	}
	pts := parseSerie(nm)
	if len(pts) == 0 {
		return nil, errors.New("a Alpha Vantage não devolveu cotações para " + ticker)
	}
	return &Serie{Pontos: pts, BaixadoEm: time.Now().Format("02/01/2006 15:04"), SimboloAV: ticker + ".SAO"}, nil
}

func TestarChave(chave string) (float64, string, error) {
	m, _, err := avGet(url.Values{"function": {"GLOBAL_QUOTE"}, "symbol": {"PETR4.SAO"}, "apikey": {chave}})
	if err != nil {
		return 0, "", err
	}
	q, _ := m["Global Quote"].(map[string]any)
	if q == nil || len(q) == 0 {
		return 0, "", errors.New("a chave respondeu, mas sem cotação (tente de novo em 1 minuto)")
	}
	p, _ := strconv.ParseFloat(fmt.Sprint(q["05. price"]), 64)
	return p, fmt.Sprint(q["07. latest trading day"]), nil
}

// Última cotação no Yahoo Finance (a mesma fonte do yfinance), guardada por 5 minutos.
type UltimaCot struct {
	Preco float64
	Data  string // AAAA-MM-DD do pregão, no horário da bolsa
	Hora  string // HH:MM
	em    time.Time
}

var (
	ultCotMu sync.Mutex
	ultCot   = map[string]*UltimaCot{}
)

// nil se o Yahoo não responder ou não conhecer o ticker: a ficha segue com os dados do banco.
func UltimaCotacao(ticker string, forcar bool) *UltimaCot {
	ultCotMu.Lock()
	c, ok := ultCot[ticker]
	ultCotMu.Unlock()
	if ok && !forcar && time.Since(c.em) < 5*time.Minute {
		if c.Preco <= 0 {
			return nil
		}
		return c
	}
	n := &UltimaCot{em: time.Now()}
	req, _ := http.NewRequest("GET", "https://query1.finance.yahoo.com/v8/finance/chart/"+url.PathEscape(ticker)+".SA?range=1d&interval=1d", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	cli := &http.Client{Timeout: 5 * time.Second}
	if resp, err := cli.Do(req); err == nil {
		defer resp.Body.Close()
		var j struct {
			Chart struct {
				Result []struct {
					Meta struct {
						Preco  float64 `json:"regularMarketPrice"`
						Tempo  int64   `json:"regularMarketTime"`
						Offset int     `json:"gmtoffset"`
					} `json:"meta"`
				} `json:"result"`
			} `json:"chart"`
		}
		if resp.StatusCode == 200 && json.NewDecoder(resp.Body).Decode(&j) == nil && len(j.Chart.Result) > 0 {
			m := j.Chart.Result[0].Meta
			if m.Preco > 0 && m.Tempo > 0 {
				t := time.Unix(m.Tempo, 0).In(time.FixedZone("B3", m.Offset))
				n.Preco, n.Data, n.Hora = m.Preco, t.Format("2006-01-02"), t.Format("15:04")
			}
		}
	}
	ultCotMu.Lock()
	ultCot[ticker] = n
	ultCotMu.Unlock()
	if n.Preco <= 0 {
		return nil
	}
	return n
}

// Série com a última cotação no fim: troca o mês corrente ou abre um mês novo. Copia os
// pontos porque a série pode estar em cache. O ajustado acompanha o preço, para não
// parecer provento na conta de rendimento.
func comUltimaCotacao(s *Serie, u *UltimaCot) *Serie {
	if s == nil || u == nil || len(s.Pontos) == 0 {
		return s
	}
	ult := s.Pontos[len(s.Pontos)-1]
	if u.Data <= ult.Data || ult.Close <= 0 {
		return s
	}
	c := *s
	c.Pontos = append([]Ponto(nil), s.Pontos...)
	n := Ponto{Data: u.Data, Close: u.Preco, Adj: ult.Adj * u.Preco / ult.Close}
	if u.Data[:7] == ult.Data[:7] {
		n.Div = ult.Div
		c.Pontos[len(c.Pontos)-1] = n
	} else {
		c.Pontos = append(c.Pontos, n)
	}
	return &c
}
