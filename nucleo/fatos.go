package nucleo

// Fatos relevantes da CVM: conjunto IPE dos dados abertos (o mesmo do app local).

import (
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"
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

// lê os fatos relevantes de um zip do IPE, sem repetir protocolo
func FatosDoZip(b []byte) ([]FatoRelevante, error) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}
	var fs []FatoRelevante
	vistos := map[string]bool{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
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
			n := len(c)
			if n < 13 || get("Categoria") != "Fato Relevante" || vistos[c[n-3]] || !strings.HasPrefix(c[n-1], "https://") {
				continue
			}
			vistos[c[n-3]] = true
			fs = append(fs, FatoRelevante{get("Nome_Companhia"), get("CNPJ_Companhia"), strings.Join(c[7:n-5], ";"), c[n-5], c[n-1]})
		}
		rc.Close()
		if err := sc.Err(); err != nil {
			return nil, err
		}
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
		l, err := FatosDoZip(b)
		if err != nil {
			return nil, err
		}
		fs = append(fs, l...)
	}
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
