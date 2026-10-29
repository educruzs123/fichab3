package main

// Fonte "CVM direto": baixa os zips do portal de dados abertos da CVM e monta o
// banco a partir deles, sem depender dos CSVs consolidados.

import (
	"archive/zip"
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var urlCVM = func() string {
	if u := os.Getenv("FICHAB3_URL_CVM"); u != "" { // só para testes
		return u
	}
	return "https://dados.cvm.gov.br/dados/CIA_ABERTA/"
}()

// ---------- download ----------

type arqCVM struct {
	nome, url string
	historico bool // anos fechados: só baixa se não existir
}

func listaDownloads() []arqCVM {
	ano := time.Now().Year()
	out := []arqCVM{{"cad_cia_aberta.csv", urlCVM + "CAD/DADOS/cad_cia_aberta.csv", false}}
	for y := 2010; y <= ano; y++ {
		n := fmt.Sprintf("dfp_cia_aberta_%d.zip", y)
		out = append(out, arqCVM{n, urlCVM + "DOC/DFP/DADOS/" + n, y < ano-2})
	}
	for y := 2011; y <= ano; y++ {
		n := fmt.Sprintf("itr_cia_aberta_%d.zip", y)
		out = append(out, arqCVM{n, urlCVM + "DOC/ITR/DADOS/" + n, y < ano-1})
	}
	for y := ano - 1; y <= ano; y++ {
		n := fmt.Sprintf("fca_cia_aberta_%d.zip", y)
		out = append(out, arqCVM{n, urlCVM + "DOC/FCA/DADOS/" + n, false})
	}
	return out
}

var dlClient = &http.Client{Timeout: 15 * time.Minute}

// baixa um arquivo se mudou no servidor; devolve (baixou, bytes, erro)
func baixarArquivo(a arqCVM, pasta string, onBytes func(n, total int64)) (bool, int64, error) {
	dest := filepath.Join(pasta, a.nome)
	st, errSt := os.Stat(dest)
	if errSt == nil && a.historico {
		return false, 0, nil
	}
	// confere antes pelo cabeçalho (data e tamanho), caso o servidor ignore If-Modified-Since
	if errSt == nil {
		if hr, err := http.NewRequest("HEAD", a.url, nil); err == nil {
			hr.Header.Set("User-Agent", "FichaB3Completa/"+versao)
			if hresp, err := dlClient.Do(hr); err == nil {
				hresp.Body.Close()
				lm, e2 := http.ParseTime(hresp.Header.Get("Last-Modified"))
				if hresp.StatusCode == http.StatusOK && e2 == nil && !lm.After(st.ModTime()) && (hresp.ContentLength <= 0 || hresp.ContentLength == st.Size()) {
					return false, 0, nil
				}
				if hresp.StatusCode == http.StatusNotFound {
					return false, 0, nil
				}
			}
		}
	}
	req, _ := http.NewRequest("GET", a.url, nil)
	req.Header.Set("User-Agent", "FichaB3Completa/"+versao)
	if errSt == nil {
		req.Header.Set("If-Modified-Since", st.ModTime().UTC().Format(http.TimeFormat))
	}
	resp, err := dlClient.Do(req)
	if err != nil {
		return false, 0, fmt.Errorf("sem conexão com dados.cvm.gov.br: %v", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotModified:
		return false, 0, nil
	case resp.StatusCode == http.StatusNotFound:
		return false, 0, nil // ainda não publicado
	case resp.StatusCode != http.StatusOK:
		return false, 0, fmt.Errorf("%s: resposta %d da CVM", a.nome, resp.StatusCode)
	}
	tmp := dest + ".baixando"
	f, err := os.Create(tmp)
	if err != nil {
		return false, 0, err
	}
	total := resp.ContentLength
	var n int64
	buf := make([]byte, 256*1024)
	for {
		k, er := resp.Body.Read(buf)
		if k > 0 {
			if _, ew := f.Write(buf[:k]); ew != nil {
				f.Close()
				os.Remove(tmp)
				return false, n, ew
			}
			n += int64(k)
			onBytes(n, total)
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			f.Close()
			os.Remove(tmp)
			return false, n, fmt.Errorf("%s: download interrompido: %v", a.nome, er)
		}
	}
	f.Close()
	if strings.HasSuffix(a.nome, ".zip") {
		if zr, err := zip.OpenReader(tmp); err != nil {
			os.Remove(tmp)
			return false, n, fmt.Errorf("%s: arquivo baixado está corrompido", a.nome)
		} else {
			zr.Close()
		}
	}
	if err := os.Rename(tmp, dest); err != nil {
		return false, n, err
	}
	if lm, err := http.ParseTime(resp.Header.Get("Last-Modified")); err == nil {
		_ = os.Chtimes(dest, lm, lm)
	}
	return true, n, nil
}

// Baixa o que mudou e reimporta o banco a partir dos zips.
func IniciarAtualizacaoCVM(pastaZips, pastaBanco string, baixar bool) error {
	progMu.Lock()
	if prog.Rodando {
		progMu.Unlock()
		return errors.New("já existe uma atualização em andamento")
	}
	prog = Progresso{Rodando: true, Etapa: "Preparando", Inicio: time.Now().Format("15:04:05")}
	progMu.Unlock()
	go func() {
		t0 := time.Now()
		falha := func(err error) {
			log.Printf("atualização da CVM falhou: %v", err)
			setProg(func(p *Progresso) { p.Rodando = false; p.Erro = err.Error() })
		}
		if err := os.MkdirAll(pastaZips, 0755); err != nil {
			falha(fmt.Errorf("não consegui criar a pasta dos zips: %w", err))
			return
		}
		novos := []string{}
		var avisos []string
		if baixar {
			lista := listaDownloads()
			for i, a := range lista {
				i, a := i, a
				setProg(func(p *Progresso) {
					p.Etapa = fmt.Sprintf("Conferindo na CVM: %s (%d de %d)", a.nome, i+1, len(lista))
					p.Pct = float64(i) / float64(len(lista)) * 40
				})
				ok, _, err := baixarArquivo(a, pastaZips, func(n, total int64) {
					setProg(func(p *Progresso) {
						if total > 0 {
							p.Etapa = fmt.Sprintf("Baixando %s: %.0f de %.0f MB", a.nome, float64(n)/1e6, float64(total)/1e6)
						} else {
							p.Etapa = fmt.Sprintf("Baixando %s: %.0f MB", a.nome, float64(n)/1e6)
						}
					})
				})
				if err != nil {
					if i == 0 && strings.Contains(err.Error(), "sem conexão") {
						falha(err)
						return
					}
					avisos = append(avisos, err.Error())
					continue
				}
				if ok {
					novos = append(novos, a.nome)
				}
			}
		}
		setProg(func(p *Progresso) { p.Pct = 40; p.Etapa = "Lendo os zips da CVM" })
		meta, err := importarZips(pastaZips, pastaBanco)
		if err != nil {
			falha(err)
			return
		}
		meta.Segundos = time.Since(t0).Seconds()
		meta.Fonte = "cvm"
		meta.Baixados = novos
		if err := gravarMeta(pastaBanco, meta); err != nil {
			falha(err)
			return
		}
		b, err := CarregarBanco(pastaBanco)
		if err != nil {
			falha(fmt.Errorf("banco gravado mas não abriu: %w", err))
			return
		}
		bancoMu.Lock()
		banco = b
		bancoMu.Unlock()
		res := fmt.Sprintf("%d empresas, DFP %d–%d, trimestres até %s", meta.NEmpresas, meta.AnoMin, meta.AnoMax, meta.UltTri)
		if baixar {
			if len(novos) == 0 {
				res += "; nenhum arquivo novo na CVM"
			} else {
				res += "; baixados: " + strings.Join(novos, ", ")
			}
		}
		if len(avisos) > 0 {
			res += ". Avisos: " + strings.Join(avisos, "; ")
		}
		log.Printf("atualização da CVM concluída: %s", res)
		setProg(func(p *Progresso) {
			p.Rodando = false
			p.Pct = 100
			p.Etapa = "Concluído"
			p.Concluiu = time.Now().Format("02/01/2006 15:04")
			p.Resumo = res + fmt.Sprintf(" (%.0f s)", meta.Segundos)
		})
	}()
	return nil
}

// ---------- leitura dos zips ----------

func latin1(b []byte) string {
	ascii := true
	for _, c := range b {
		if c >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii || utf8.Valid(b) {
		return string(b)
	}
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}

// lê um CSV (separado por ;) de dentro do zip, linha a linha, chamando fn com os campos
func lerCSVZip(f *zip.File, fn func(get func(string) string, campos []string)) error {
	return lerCSVZipF(f, nil, fn)
}

// aceita (opcional) recebe o primeiro campo cru da linha e decide se vale a pena processá-la
func lerCSVZipF(f *zip.File, aceita func(primeiro []byte) bool, fn func(get func(string) string, campos []string)) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	br := bufio.NewReaderSize(rc, 1<<20)
	linha, err := br.ReadString('\n')
	if err != nil && linha == "" {
		return err
	}
	hdr := strings.Split(strings.TrimRight(latin1([]byte(linha)), "\r\n"), ";")
	idx := map[string]int{}
	for i, h := range hdr {
		idx[strings.TrimPrefix(strings.TrimSpace(h), "\ufeff")] = i
	}
	nh := len(hdr)
	var campos []string
	get := func(k string) string {
		if i, ok := idx[k]; ok && i < len(campos) {
			return campos[i]
		}
		return ""
	}
	for {
		b, err := br.ReadSlice('\n')
		if len(b) > 0 && aceita != nil {
			if i := bytes.IndexByte(b, ';'); i < 0 || !aceita(b[:i]) {
				b = nil
			}
		}
		if len(b) > 0 {
			s := strings.TrimRight(latin1(b), "\r\n")
			campos = strings.Split(s, ";")
			if len(campos) > nh {
				// ';' dentro de um texto: junta o excedente no campo de descrição
				if i, ok := idx["DS_CONTA"]; ok {
					ext := len(campos) - nh
					m := strings.Join(campos[i:i+ext+1], ";")
					campos = append(append(append([]string{}, campos[:i]...), m), campos[i+ext+1:]...)
				}
			}
			fn(get, campos)
		}
		if err == bufio.ErrBufferFull {
			// linha maior que o buffer: descarta o resto
			for err == bufio.ErrBufferFull {
				_, err = br.ReadSlice('\n')
			}
			continue
		}
		if err != nil {
			break
		}
	}
	return nil
}

var reTicker = regexp.MustCompile(`^[A-Z0-9]{4}(3|4|5|6|7|8|11)$`)

type cadInfo struct {
	nome, comercial, setor, sit, codcvm string
}

func lerCadastro(p string) map[string]cadInfo {
	out := map[string]cadInfo{}
	f, err := os.Open(p)
	if err != nil {
		return out
	}
	defer f.Close()
	br := bufio.NewReaderSize(f, 1<<20)
	l, _ := br.ReadString('\n')
	hdr := strings.Split(strings.TrimRight(latin1([]byte(l)), "\r\n"), ";")
	idx := map[string]int{}
	for i, h := range hdr {
		idx[strings.TrimPrefix(h, "\ufeff")] = i
	}
	for {
		b, err := br.ReadBytes('\n')
		if len(b) > 0 {
			c := strings.Split(strings.TrimRight(latin1(b), "\r\n"), ";")
			g := func(k string) string {
				if i, ok := idx[k]; ok && i < len(c) {
					return strings.TrimSpace(c[i])
				}
				return ""
			}
			cn := g("CNPJ_CIA")
			ci := cadInfo{g("DENOM_SOCIAL"), g("DENOM_COMERC"), g("SETOR_ATIV"), g("SIT"), g("CD_CVM")}
			if old, ok := out[cn]; !ok || (old.sit != "ATIVO" && ci.sit == "ATIVO") {
				out[cn] = ci
			}
		}
		if err != nil {
			break
		}
	}
	return out
}

type tkInfo struct {
	t     string
	ativo bool
	ano   string
}

// tickers de ações (FCA, valor mobiliário), do FCA mais recente para o mais antigo
func lerTickers(zips []string) map[string][]tkInfo {
	out := map[string][]tkInfo{}
	sort.Sort(sort.Reverse(sort.StringSlice(zips)))
	for _, z := range zips {
		zr, err := zip.OpenReader(z)
		if err != nil {
			continue
		}
		for _, f := range zr.File {
			if !strings.Contains(f.Name, "valor_mobiliario") {
				continue
			}
			_ = lerCSVZip(f, func(g func(string) string, _ []string) {
				t := strings.ToUpper(strings.TrimSpace(g("Codigo_Negociacao")))
				if !reTicker.MatchString(t) || !strings.EqualFold(g("Mercado"), "Bolsa") {
					return
				}
				cn := g("CNPJ_Companhia")
				for _, x := range out[cn] {
					if x.t == t {
						return
					}
				}
				out[cn] = append(out[cn], tkInfo{t, g("Data_Fim_Negociacao") == "", g("Data_Referencia")})
			})
		}
		zr.Close()
	}
	return out
}

// ordena: ativos primeiro; ON, PN, units
func ordenarTickers(ts []tkInfo) []string {
	cl := func(t string) int {
		n := strings.TrimLeftFunc(t, func(r rune) bool { return r < '0' || r > '9' })
		v, _ := strconv.Atoi(n)
		return v
	}
	sort.SliceStable(ts, func(i, j int) bool {
		if ts[i].ativo != ts[j].ativo {
			return ts[i].ativo
		}
		return cl(ts[i].t) < cl(ts[j].t)
	})
	out := []string{}
	for _, x := range ts {
		out = append(out, x.t)
	}
	return out
}

// valores trimestrais crus: chave base|dt_refer|ini -> código -> valor (R$ mi)
type qstore struct {
	dre, dfc        map[string]map[string]float64
	bp              map[string]map[string]float64
	nDRE, nDFC, nBP map[string]map[string]string
}

func novoQ() *qstore {
	return &qstore{map[string]map[string]float64{}, map[string]map[string]float64{}, map[string]map[string]float64{},
		map[string]map[string]string{}, map[string]map[string]string{}, map[string]map[string]string{}}
}

var internTab = map[string]string{}

func intern(s string) string {
	if v, ok := internTab[s]; ok {
		return v
	}
	internTab[s] = s
	return s
}

// contas trimestrais necessárias para os indicadores
func qDRE(cod, nome string) bool {
	p := profundidade(cod)
	if strings.HasPrefix(cod, "3.99") {
		return false
	}
	return p <= 2 || (p == 3 && strings.Contains(norm(nome), "controlador"))
}
func qBP(cod string) bool {
	p := profundidade(cod)
	return cod == "1" || (p == 2 && strings.HasPrefix(cod, "2.")) || (p == 3 && (strings.HasPrefix(cod, "1.01.") || strings.HasPrefix(cod, "2.01.") || strings.HasPrefix(cod, "2.02.")))
}
func qDFC(cod, nome string) bool {
	if cod == "6.01" {
		return true
	}
	if profundidade(cod) == 4 && strings.HasPrefix(cod, "6.01.01.") {
		n := norm(nome)
		return strings.Contains(n, "deprecia") || strings.Contains(n, "amortiza") || strings.Contains(n, "exaust")
	}
	return false
}

func put(v map[string]map[string]float64, n map[string]map[string]string, k, cod, nome string, x float64) {
	k, cod, nome = intern(k), intern(cod), intern(nome)
	m := v[k]
	if m == nil {
		m = map[string]float64{}
		v[k] = m
		n[k] = map[string]string{}
	}
	m[cod] = x
	n[k][cod] = nome
}

// contas que interessam (para não guardar tudo)
func guardarDRE(cod string) bool { return true }
func guardarBP(cod string) bool  { return profundidade(cod) <= 3 }
func guardarDFC(cod, nome string) bool {
	p := profundidade(cod)
	if p <= 3 {
		return true
	}
	if p == 4 && strings.HasPrefix(cod, "6.01.01.") {
		n := norm(nome)
		return strings.Contains(n, "deprecia") || strings.Contains(n, "amortiza") || strings.Contains(n, "exaust")
	}
	return false
}

func escala(e string) float64 {
	switch strings.ToUpper(strings.TrimSpace(e)) {
	case "UNIDADE":
		return 1e-6
	case "MIL":
		return 1e-3
	}
	return 1e-3
}

func importarZips(pastaZips, pastaBanco string) (MetaBanco, error) {
	meta := MetaBanco{PastaZips: pastaZips, Fonte: "cvm"}
	dfps, _ := filepath.Glob(filepath.Join(pastaZips, "dfp_cia_aberta_*.zip"))
	itrs, _ := filepath.Glob(filepath.Join(pastaZips, "itr_cia_aberta_*.zip"))
	fcas, _ := filepath.Glob(filepath.Join(pastaZips, "fca_cia_aberta_*.zip"))
	sort.Strings(dfps)
	sort.Strings(itrs)
	if len(dfps) == 0 {
		return meta, fmt.Errorf("nenhum dfp_cia_aberta_AAAA.zip em %s — use “Baixar da CVM e atualizar”", pastaZips)
	}
	if len(fcas) == 0 {
		return meta, fmt.Errorf("falta o fca_cia_aberta_AAAA.zip (de onde vêm os tickers) em %s", pastaZips)
	}
	setProg(func(p *Progresso) { p.Etapa = "Lendo cadastro e tickers"; p.Pct = 41 })
	cad := lerCadastro(filepath.Join(pastaZips, "cad_cia_aberta.csv"))
	tks := lerTickers(fcas)
	emps := map[string]*acum{}
	qs := map[string]*qstore{}
	for cn, ts := range tks {
		e := &Empresa{CNPJ: cn, Tickers: ordenarTickers(ts)}
		if c, ok := cad[cn]; ok {
			e.Nome, e.NomeCurto, e.Setor, e.CodCVM, e.Ativa = c.nome, c.comercial, c.setor, c.codcvm, c.sit == "ATIVO"
		}
		emps[cn] = &acum{emp: e, dre: map[string]map[string]float64{}, bp: map[string]map[string]float64{}, dfc: map[string]map[string]Conta{},
			nomesDR: map[string]map[string]string{}, nomesBP: map[string]map[string]string{}}
		qs[cn] = novoQ()
	}
	if len(emps) == 0 {
		return meta, errors.New("nenhum ticker encontrado nos arquivos FCA")
	}

	type parte struct {
		demo string // DRE, BPA, BPP, DFC_MI, composicao_capital
		base string
	}
	classif := func(nome string) (parte, bool) {
		n := strings.TrimSuffix(filepath.Base(nome), ".csv")
		for _, d := range []string{"DRE", "BPA", "BPP", "DFC_MI"} {
			for _, b := range []string{"con", "ind"} {
				if strings.Contains(n, "_"+d+"_"+b+"_") {
					bb := "consolidado"
					if b == "ind" {
						bb = "individual"
					}
					return parte{d, bb}, true
				}
			}
		}
		if strings.Contains(n, "composicao_capital") {
			return parte{"cap", ""}, true
		}
		return parte{}, false
	}
	anoMin, anoMax := 9999, 0
	total := len(dfps) + len(itrs)
	for zi, z := range append(append([]string{}, dfps...), itrs...) {
		ehDFP := zi < len(dfps)
		zi, z := zi, z
		setProg(func(p *Progresso) {
			p.Etapa = "Lendo " + filepath.Base(z)
			p.Pct = 42 + float64(zi)/float64(total)*50
		})
		zr, err := zip.OpenReader(z)
		if err != nil {
			log.Printf("%s: %v", z, err)
			continue
		}
		for _, f := range zr.File {
			pt, ok := classif(f.Name)
			if !ok {
				continue
			}
			if pt.demo == "cap" {
				_ = lerCSVZip(f, func(g func(string) string, _ []string) {
					a := emps[g("CNPJ_CIA")]
					if a == nil {
						return
					}
					fl := func(k string) float64 { v, _ := strconv.ParseFloat(strings.Replace(g(k), ",", ".", 1), 64); return v }
					c := Capital{ON: fl("QT_ACAO_ORDIN_CAP_INTEGR"), PN: fl("QT_ACAO_PREF_CAP_INTEGR"), Total: fl("QT_ACAO_TOTAL_CAP_INTEGR"),
						TesON: fl("QT_ACAO_ORDIN_TESOURO"), TesPN: fl("QT_ACAO_PREF_TESOURO"), TesTot: fl("QT_ACAO_TOTAL_TESOURO")}
					if c.Total > 0 {
						if a.capital == nil {
							a.capital = map[string]Capital{}
						}
						a.capital[g("DT_REFER")] = c
					}
				})
				continue
			}
			_ = lerCSVZipF(f, func(p []byte) bool { _, ok := emps[string(p)]; return ok }, func(g func(string) string, _ []string) {
				cn := g("CNPJ_CIA")
				a := emps[cn]
				if a == nil {
					return
				}
				ord := g("ORDEM_EXERC")
				if !strings.HasPrefix(ord, "Ú") && !strings.HasPrefix(ord, "U") {
					return // PENÚLTIMO
				}
				cod, nome := g("CD_CONTA"), g("DS_CONTA")
				vs := g("VL_CONTA")
				v, err := strconv.ParseFloat(strings.Replace(vs, ",", ".", 1), 64)
				if err != nil {
					return
				}
				if !strings.HasPrefix(cod, "3.99") {
					v *= escala(g("ESCALA_MOEDA"))
				}
				dtRef := g("DT_REFER")
				if len(dtRef) < 10 {
					return
				}
				q := qs[cn]
				switch pt.demo {
				case "DRE":
					if !guardarDRE(cod) {
						return
					}
					if ehDFP {
						ano := dtRef[:4]
						k := pt.base + "|" + ano
						m := a.dre[k]
						if m == nil {
							m = map[string]float64{}
							a.dre[k] = m
							a.nomesDR[k] = map[string]string{}
						}
						m[intern(cod)] = v
						a.nomesDR[k][intern(cod)] = intern(nome)
						if n, _ := strconv.Atoi(ano); n > 0 {
							if n < anoMin {
								anoMin = n
							}
							if n > anoMax {
								anoMax = n
							}
						}
					}
					if qDRE(cod, nome) {
						put(q.dre, q.nDRE, pt.base+"|"+dtRef+"|"+g("DT_INI_EXERC"), cod, nome, v)
					}
				case "BPA", "BPP":
					if !guardarBP(cod) {
						return
					}
					if ehDFP {
						k := pt.base + "|" + dtRef[:4]
						m := a.bp[k]
						if m == nil {
							m = map[string]float64{}
							a.bp[k] = m
							a.nomesBP[k] = map[string]string{}
						}
						m[intern(cod)] = v
						a.nomesBP[k][intern(cod)] = intern(nome)
					}
					if qBP(cod) {
						put(q.bp, q.nBP, pt.base+"|"+dtRef, cod, nome, v)
					}
				case "DFC_MI":
					if !guardarDFC(cod, nome) {
						return
					}
					if ehDFP && profundidade(cod) <= 3 {
						k := pt.base + "|" + dtRef[:4]
						m := a.dfc[k]
						if m == nil {
							m = map[string]Conta{}
							a.dfc[k] = m
						}
						m[intern(cod)] = Conta{Cod: intern(cod), Nome: intern(nome), V: v}
					}
					if qDFC(cod, nome) {
						put(q.dfc, q.nDFC, pt.base+"|"+dtRef+"|"+g("DT_INI_EXERC"), cod, nome, v)
					}
				}
			})
		}
		zr.Close()
	}
	meta.AnoMin, meta.AnoMax = anoMin, anoMax
	meta.ZipsLidos = total

	// trimestres calculados a partir dos ITR (1º a 3º) e do DFP (4º)
	setProg(func(p *Progresso) { p.Etapa = "Calculando trimestres"; p.Pct = 93 })
	ultKey := 0
	for cn, a := range emps {
		a.tri = calcularTrimestres(qs[cn])
		if n := len(a.tri); n > 0 {
			t := a.tri[n-1]
			if k := triKey(t.Ano, t.Tri); k > ultKey {
				ultKey = k
				meta.UltTri = t.Tri
			}
		}
		// tipo: instituição financeira pela estrutura da DRE
		a.emp.Tipo = "Não financeira"
		if s := norm(a.emp.Setor); strings.Contains(s, "banco") || strings.Contains(s, "segurad") || strings.Contains(s, "financ") || strings.Contains(s, "intermediac") {
			a.emp.Tipo = "Financeira"
		}
		for _, nm := range a.nomesDR {
			if strings.Contains(norm(nm["3.01"]), "intermediacao financeira") {
				a.emp.Tipo = "Financeira"
				break
			}
		}
		if a.emp.NomeCurto == "" {
			a.emp.NomeCurto = a.emp.Nome
		}
	}
	qs = nil
	internTab = map[string]string{}
	n, nb, err := gravarBanco(emps, pastaBanco)
	if err != nil {
		return meta, err
	}
	meta.NEmpresas = n
	meta.TamanhoMB = float64(nb) / 1e6
	meta.ImportadoEm = time.Now().Format("02/01/2006 15:04")
	return meta, nil
}

type ytd struct {
	receita, bruto, ebit, resfin, lucro, ctrl, da, fco *float64
}

func calcYTD(dre map[string]float64, nDRE map[string]string, dfc map[string]float64, nDFC map[string]string) ytd {
	var y ytd
	g := func(m map[string]float64, c string) *float64 {
		if v, ok := m[c]; ok {
			return fp(v)
		}
		return nil
	}
	y.receita = g(dre, "3.01")
	y.bruto = g(dre, "3.03")
	lc := ""
	cods := codsProf(dre, 2)
	for _, c := range cods {
		n := norm(nDRE[c])
		if y.ebit == nil && reEBIT.MatchString(n) {
			y.ebit = g(dre, c)
		}
		if strings.HasPrefix(n, "resultado financeiro") {
			y.resfin = g(dre, c)
		}
		if reLucro.MatchString(n) && !strings.Contains(n, "antes") && !strings.Contains(n, "continuadas") {
			lc = c
		}
	}
	if _, ok := dre["3.11"]; ok && reLucro.MatchString(norm(nDRE["3.11"])) {
		lc = "3.11"
	}
	if lc != "" {
		y.lucro = g(dre, lc)
		y.ctrl = y.lucro
		for c, v := range dre {
			if strings.HasPrefix(c, lc+".") && profundidade(c) == profundidade(lc)+1 {
				n := norm(nDRE[c])
				if strings.Contains(n, "controlador") && !strings.Contains(n, "nao controlador") && (v != 0 || y.lucro == nil || *y.lucro == 0) {
					y.ctrl = fp(v)
				}
			}
		}
	}
	if dfc != nil {
		y.fco = g(dfc, "6.01")
		s, tem := 0.0, false
		for c, v := range dfc {
			if strings.HasPrefix(c, "6.01.01.") {
				n := norm(nDFC[c])
				if strings.Contains(n, "deprecia") || strings.Contains(n, "amortiza") || strings.Contains(n, "exaust") {
					s += abs(v)
					tem = true
				}
			}
		}
		if tem {
			y.da = fp(s)
		}
	}
	return y
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func subp(a, b *float64) *float64 {
	if a == nil {
		return nil
	}
	if b == nil {
		return nil
	}
	return fp(*a - *b)
}

type balanco struct{ ativo, caixa, divb, divl, pl *float64 }

func calcBP(bp map[string]float64, n map[string]string) balanco {
	var b balanco
	if v, ok := bp["1"]; ok {
		b.ativo = fp(v)
	}
	cx, db := 0.0, 0.0
	tc, td := false, false
	for _, c := range codsProf(bp, 2) {
		if strings.HasPrefix(c, "2.") && strings.HasPrefix(norm(n[c]), "patrimonio liquido") {
			b.pl = fp(bp[c])
		}
	}
	for _, c := range codsProf(bp, 3) {
		nn := norm(n[c])
		if strings.HasPrefix(c, "1.01.") && (strings.HasPrefix(nn, "caixa e equivalentes") || strings.HasPrefix(nn, "aplicacoes financeiras")) {
			cx += bp[c]
			tc = true
		}
		if (strings.HasPrefix(c, "2.01.") || strings.HasPrefix(c, "2.02.")) && strings.HasPrefix(nn, "emprestimos e financiamentos") {
			db += bp[c]
			td = true
		}
	}
	if tc {
		b.caixa = fp(cx)
	}
	if td {
		b.divb = fp(db)
	}
	if tc && td {
		b.divl = fp(db - cx)
	}
	return b
}

func calcularTrimestres(q *qstore) []Tri {
	// datas de referência disponíveis (por base), escolhendo consolidado quando houver
	type ref struct{ dt, base string }
	refs := map[string]string{} // dt -> base
	for k, m := range q.dre {
		p := strings.Split(k, "|")
		if len(p) < 3 || !(temValores(m, "3.01", "3.11") || temValores(q.bp[p[0]+"|"+p[1]], "1")) {
			continue // demonstração zerada (ex.: consolidado de quem não tem controladas)
		}
		if b, ok := refs[p[1]]; !ok || (b == "individual" && p[0] == "consolidado") {
			refs[p[1]] = p[0]
		}
	}
	// YTD = linha com o menor início de exercício para a data
	iniDe := map[string]string{}
	ytdDe := func(base, dt string) (ytd, bool) {
		ini := ""
		for k := range q.dre {
			p := strings.Split(k, "|")
			if len(p) == 3 && p[0] == base && p[1] == dt && (ini == "" || p[2] < ini) {
				ini = p[2]
			}
		}
		if ini == "" {
			return ytd{}, false
		}
		iniDe[dt] = ini
		kd := base + "|" + dt + "|" + ini
		// DFC: também a de menor início
		fi := ""
		for k := range q.dfc {
			p := strings.Split(k, "|")
			if len(p) == 3 && p[0] == base && p[1] == dt && (fi == "" || p[2] < fi) {
				fi = p[2]
			}
		}
		var dfc map[string]float64
		var ndfc map[string]string
		if fi != "" {
			dfc, ndfc = q.dfc[base+"|"+dt+"|"+fi], q.nDFC[base+"|"+dt+"|"+fi]
		}
		return calcYTD(q.dre[kd], q.nDRE[kd], dfc, ndfc), true
	}
	dts := []string{}
	for dt := range refs {
		dts = append(dts, dt)
	}
	sort.Strings(dts)
	type tq struct {
		ano, q int
		dt     string
		y      ytd
		bal    balanco
		base   string
	}
	var lst []tq
	for _, dt := range dts {
		base := refs[dt]
		y, ok := ytdDe(base, dt)
		if !ok {
			continue
		}
		ano, _ := strconv.Atoi(dt[:4])
		mes, _ := strconv.Atoi(dt[5:7])
		qq := (mes + 2) / 3
		var bal balanco
		if m, ok := q.bp[base+"|"+dt]; ok {
			bal = calcBP(m, q.nBP[base+"|"+dt])
		}
		lst = append(lst, tq{ano, qq, dt, y, bal, base})
	}
	// valor do trimestre = acumulado − acumulado anterior do mesmo exercício
	// (pelo início do exercício, o que também serve a quem não fecha o ano em dezembro)
	meses := func(d string) int {
		if len(d) < 7 {
			return 0
		}
		a, _ := strconv.Atoi(d[:4])
		m, _ := strconv.Atoi(d[5:7])
		return a*12 + m
	}
	var out []Tri
	for i, t := range lst {
		r := Tri{Ano: t.ano, Tri: fmt.Sprintf("%dT%02d", t.q, t.ano%100), Base: t.base}
		ini := iniDe[t.dt]
		primeiro := meses(t.dt)-meses(ini) <= 3 // 1º trimestre do exercício
		var prev tq
		temPrev := false
		for j := i - 1; j >= 0; j-- {
			if iniDe[lst[j].dt] == ini {
				prev, temPrev = lst[j], meses(t.dt)-meses(lst[j].dt) == 3
				break
			}
		}
		disc := func(a, b *float64) *float64 {
			if primeiro {
				return a
			}
			if !temPrev {
				return nil
			}
			return subp(a, b)
		}
		r.Receita = disc(t.y.receita, prev.y.receita)
		r.LucroBruto = disc(t.y.bruto, prev.y.bruto)
		r.EBIT = disc(t.y.ebit, prev.y.ebit)
		r.ResFin = disc(t.y.resfin, prev.y.resfin)
		r.Lucro = disc(t.y.lucro, prev.y.lucro)
		r.LucroCtrl = disc(t.y.ctrl, prev.y.ctrl)
		r.DA = disc(t.y.da, prev.y.da)
		r.FCO = disc(t.y.fco, prev.y.fco)
		if r.EBIT != nil && r.DA != nil {
			r.EBITDA = fp(*r.EBIT + *r.DA)
		}
		r.MargemLiq = div(r.Lucro, r.Receita)
		r.Ativo, r.Caixa, r.DivBruta, r.DivLiq, r.PL = t.bal.ativo, t.bal.caixa, t.bal.divb, t.bal.divl, t.bal.pl
		out = append(out, r)
	}
	// 12 meses: soma dos últimos 4 trimestres consecutivos
	for i := range out {
		if i < 3 {
			continue
		}
		ok := true
		for j := i - 2; j <= i; j++ {
			a, b := out[j-1], out[j]
			if triKey(b.Ano, b.Tri)-triKey(a.Ano, a.Tri) != 1 && !(a.Tri[0] == '4' && b.Tri[0] == '1' && b.Ano == a.Ano+1) {
				ok = false
			}
		}
		if !ok {
			continue
		}
		soma := func(get func(t Tri) *float64) *float64 {
			s := 0.0
			for j := i - 3; j <= i; j++ {
				v := get(out[j])
				if v == nil {
					return nil
				}
				s += *v
			}
			return fp(s)
		}
		r := &out[i]
		r.Receita12 = soma(func(t Tri) *float64 { return t.Receita })
		r.EBITDA12 = soma(func(t Tri) *float64 { return t.EBITDA })
		r.Lucro12 = soma(func(t Tri) *float64 { return t.Lucro })
		r.FCO12 = soma(func(t Tri) *float64 { return t.FCO })
		lc12 := soma(func(t Tri) *float64 { return t.LucroCtrl })
		if lc12 != nil && r.PL != nil {
			pl0 := out[i-3].PL
			if i >= 4 {
				pl0 = out[i-4].PL
			}
			if pl0 != nil && *pl0+*r.PL != 0 {
				r.ROE12 = fp(*lc12 / ((*pl0 + *r.PL) / 2))
			} else if *r.PL != 0 {
				r.ROE12 = fp(*lc12 / *r.PL)
			}
		}
		if r.DivLiq != nil && r.EBITDA12 != nil && *r.EBITDA12 != 0 {
			r.DivEBITDA12 = fp(*r.DivLiq / *r.EBITDA12)
		}
	}
	return out
}
