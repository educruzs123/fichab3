package nucleo

// Funções usadas pelo app local (raiz) e pelo Vercel. Ficam aqui, junto da lógica que usam.

func TriKey(ano int, tri string) int                       { return triKey(ano, tri) }
func Profundidade(cod string) int                          { return profundidade(cod) }
func CodLess(a, b string) bool                             { return codLess(a, b) }
func Num(s string) *float64                                { return num(s) }
func TemValores(m map[string]float64, cods ...string) bool { return temValores(m, cods...) }
func Ult12De(tris []Tri) Ult12                             { return ult12De(tris) }
func CodsProf(m map[string]float64, prof int) []string     { return codsProf(m, prof) }
func AcoesPorUnit(ticker string) float64                   { return acoesPorUnit(ticker) }
func ClasseTicker(t string) string                         { return classeTicker(t) }

var (
	SupaURLPadrao   = supaURLPadrao
	SupaChavePadrao = supaChavePadrao
	ReEBIT          = reEBIT
	ReLucro         = reLucro
)

func Fp(v float64) *float64                          { return fp(v) }
func Norm(s string) string                           { return norm(s) }
func Div(a, b *float64) *float64                     { return div(a, b) }
func FatorEntre(sp []Split, de, ate string) float64  { return fatorEntre(sp, de, ate) }
func DetectarSplits(p []Ponto) []Split               { return detectarSplits(p) }
func ComUltimaCotacao(s *Serie, u *UltimaCot) *Serie { return comUltimaCotacao(s, u) }
