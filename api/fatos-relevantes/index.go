package handler

import (
	"net/http"

	"fichab3/nucleo"
)

// /api/fatos-relevantes: os 20 fatos relevantes mais recentes da CVM.
func Handler(w http.ResponseWriter, r *http.Request) {
	l, err := nucleo.UltimosFatos()
	if err != nil {
		nucleo.Erro(w, 502, err.Error())
		return
	}
	nucleo.Responde(w, l)
}
