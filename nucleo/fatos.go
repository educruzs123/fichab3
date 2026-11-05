package nucleo

// Fatos relevantes da CVM: conjunto IPE dos dados abertos (o mesmo do app local),
// completado pelo RAD nos dias que o arquivo ainda não cobre.

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type FatoRelevante struct {
	Empresa string `json:"empresa"`
	CNPJ    string `json:"cnpj"`
	Assunto string `json:"assunto"`
	Data    string `json:"data"`
	Link    string `json:"link"`
}

// texto de arquivo da CVM: UTF-8 ou latin-1
func latin1(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}

// lê os fatos relevantes de um zip do IPE, sem repetir protocolo;
// devolve também o CNPJ de cada código CVM do arquivo, para os fatos vindos do RAD
func FatosDoZip(b []byte) ([]FatoRelevante, map[int]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, nil, err
	}
	var fs []FatoRelevante
	cnpjs := map[int]string{}
	vistos := map[string]bool{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, nil, err
		}
		sc := bufio.NewScanner(rc)
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		var idx map[string]int
		for sc.Scan() {
			linha := strings.TrimRight(latin1(sc.Bytes()), "\r")
			if idx == nil {
				idx = map[string]int{}
				for i, h := range strings.Split(linha, ";") {
					idx[strings.TrimPrefix(strings.TrimSpace(h), "\ufeff")] = i
				}
				continue
			}
			c := strings.Split(linha, ";")
			get := func(k string) string {
				if i, ok := idx[k]; ok && i < len(c) {
					return c[i]
				}
				return ""
			}
			// contado do fim: um ';' no assunto desloca as colunas seguintes
			if cd, err := strconv.Atoi(get("Codigo_CVM")); err == nil {
				cnpjs[cd] = get("CNPJ_Companhia")
			}
			n := len(c)
			if n < 13 || get("Categoria") != "Fato Relevante" || vistos[c[n-3]] || !strings.HasPrefix(c[n-1], "https://") {
				continue
			}
			vistos[c[n-3]] = true
			fs = append(fs, FatoRelevante{get("Nome_Companhia"), get("CNPJ_Companhia"), strings.Join(c[7:n-5], ";"), c[n-5], c[n-1]})
		}
		rc.Close()
		if err := sc.Err(); err != nil {
			return nil, nil, err
		}
	}
	return fs, cnpjs, nil
}

var (
	reTag   = regexp.MustCompile(`<[^>]*>`)
	reData  = regexp.MustCompile(`(\d{2})/(\d{2})/(\d{4})`)
	reDown  = regexp.MustCompile(`OpenDownloadDocumentos\('(\d+)','(\d+)','([^']+)'`)
	reProto = regexp.MustCompile(`numProtocolo=(\d+)`)
)

// fatos relevantes entregues entre de e ate, pela consulta pública do RAD (categoria IPE_4)
func FatosDoRAD(de, ate time.Time, cnpjs map[int]string) ([]FatoRelevante, error) {
	body, _ := json.Marshal(map[string]string{
		"dataDe": de.Format("02/01/2006"), "dataAte": ate.Format("02/01/2006"), "empresa": "", "setorAtividade": "-1",
		"categoriaEmissor": "-1", "situacaoEmissor": "-1", "tipoParticipante": "-1", "dataReferencia": "",
		"categoria": "IPE_4_-1_-1", "periodo": "2", "horaIni": "", "horaFim": "", "palavraChave": "",
		"ultimaDtRef": "false", "tipoEmpresa": "0", "token": "", "versaoCaptcha": "",
	})
	req, _ := http.NewRequest("POST", "https://www.rad.cvm.gov.br/ENET/frmConsultaExternaCVM.aspx/ListarDocumentos", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (FichaB3Completa)")
	r, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("RAD respondeu %s", r.Status)
	}
	var j struct {
		D struct {
			Dados   string `json:"dados"`
			TemErro bool   `json:"temErro"`
			MsgErro string `json:"msgErro"`
		} `json:"d"`
	}
	if err := json.NewDecoder(r.Body).Decode(&j); err != nil {
		return nil, err
	}
	if j.D.TemErro {
		return nil, fmt.Errorf("RAD: %s", j.D.MsgErro)
	}
	var fs []FatoRelevante
	for _, reg := range strings.Split(j.D.Dados, "&*") {
		f := strings.Split(reg, "$&")
		if len(f) < 12 || !strings.EqualFold(strings.TrimSpace(f[7]), "Ativo") {
			continue
		}
		dl := reDown.FindStringSubmatch(f[10])
		dt := reData.FindStringSubmatch(reTag.ReplaceAllString(f[6], ""))
		if dl == nil || dt == nil {
			continue
		}
		cd, _ := strconv.Atoi(strings.ReplaceAll(strings.TrimSpace(f[0]), "-", "")) // "02680-8" no RAD é 26808 no arquivo
		fs = append(fs, FatoRelevante{
			Empresa: strings.TrimSpace(f[1]),
			CNPJ:    cnpjs[cd],
			Assunto: strings.TrimSpace(f[11]),
			Data:    dt[3] + "-" + dt[2] + "-" + dt[1],
			Link: "https://www.rad.cvm.gov.br/ENET/frmDownloadDocumento.aspx?Tela=ext&descTipo=IPE&CodigoInstituicao=1&numProtocolo=" +
				dl[3] + "&numSequencia=" + dl[1] + "&numVersao=" + dl[2],
		})
	}
	return fs, nil
}

var (
	fatosMu    sync.Mutex
	fatosCache []FatoRelevante
	fatosEm    time.Time
)

// os 20 mais recentes; a CVM publica o arquivo do ano, e em janeiro pode faltar o ano novo
func baixarFatos() ([]FatoRelevante, error) {
	var fs []FatoRelevante
	cnpjs := map[int]string{}
	cli := &http.Client{Timeout: 50 * time.Second}
	ano := time.Now().Year()
	for y := ano; y >= ano-1 && len(fs) < 20; y-- {
		r, err := cli.Get(fmt.Sprintf("https://dados.cvm.gov.br/dados/CIA_ABERTA/DOC/IPE/DADOS/ipe_cia_aberta_%d.zip", y))
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(r.Body)
		r.Body.Close()
		if r.StatusCode == 404 && y == ano {
			continue
		}
		if err != nil || r.StatusCode != 200 {
			return nil, fmt.Errorf("CVM respondeu %s ao baixar os fatos relevantes", r.Status)
		}
		l, cs, err := FatosDoZip(b)
		if err != nil {
			return nil, err
		}
		fs = append(fs, l...)
		for k, v := range cs {
			if _, ok := cnpjs[k]; !ok {
				cnpjs[k] = v
			}
		}
	}
	// o arquivo aberto atrasa dias; o RAD cobre do último dia dele até hoje (no máximo 15 dias)
	hoje := time.Now().UTC().Add(-3 * time.Hour)
	de := hoje.AddDate(0, 0, -15)
	for _, f := range fs {
		if t, err := time.Parse("2006-01-02", f.Data); err == nil && t.After(de) {
			de = t
		}
	}
	if rad, err := FatosDoRAD(de, hoje, cnpjs); err == nil {
		vistos := map[string]bool{}
		for _, f := range fs {
			vistos[reProto.FindString(f.Link)] = true
		}
		for _, f := range rad {
			if p := reProto.FindString(f.Link); !vistos[p] {
				vistos[p] = true
				fs = append(fs, f)
			}
		}
	} // se o RAD falhar, fica só o arquivo aberto
	// a data não tem hora; no mesmo dia, o link com protocolo maior foi entregue depois
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Data != fs[j].Data {
			return fs[i].Data > fs[j].Data
		}
		return fs[i].Link > fs[j].Link
	})
	if len(fs) > 20 {
		fs = fs[:20]
	}
	return fs, nil
}

// guarda por 6 horas; se o download falhar, usa a última lista
func UltimosFatos() ([]FatoRelevante, error) {
	fatosMu.Lock()
	defer fatosMu.Unlock()
	if fatosCache != nil && time.Since(fatosEm) < 6*time.Hour {
		return fatosCache, nil
	}
	fs, err := baixarFatos()
	if err != nil {
		if fatosCache != nil {
			return fatosCache, nil
		}
		return nil, err
	}
	fatosCache, fatosEm = fs, time.Now()
	return fs, nil
}
