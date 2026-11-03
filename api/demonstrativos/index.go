package handler

import (
	"log"
	"net/http"
	"regexp"
	"strings"

	"fichab3/nucleo"
)

var reTicker = regexp.MustCompile(`^[A-Z0-9]{4,6}$`)

// /api/demonstrativos?t=TIMS3: os PDFs do bucket público do Supabase.
func Handler(w http.ResponseWriter, r *http.Request) {
	t := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("t")))
	out := []map[string]any{}
	if !reTicker.MatchString(t) {
		nucleo.Responde(w, out)
		return
	}
	l, err := nucleo.SupaPadrao().Demonstrativos(t)
	if err != nil {
		log.Printf("demonstrativos de %s no Supabase: %v", t, err)
	}
	out = append(out, l...)
	nucleo.Responde(w, out)
}
