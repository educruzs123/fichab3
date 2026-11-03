package nucleo

// Ajudantes das funções do Vercel (pasta api/): o mesmo formato de resposta do app local.

import (
	"encoding/json"
	"net/http"
)

// SupaPadrao: o banco do Supabase com a chave publicável (somente leitura), como no app local.
func SupaPadrao() *Supa { return NovoSupa(SupaURLPadrao, SupaChavePadrao) }

func Responde(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func Erro(w http.ResponseWriter, codigo int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(codigo)
	_ = json.NewEncoder(w).Encode(map[string]any{"erro": msg})
}
