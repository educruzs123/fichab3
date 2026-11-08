package main

// Os tipos e a lógica da ficha moraram para o pacote nucleo (também usado pelo Vercel).
// Aqui ficam só os apelidos, para o restante do app seguir igual.

import "fichab3/nucleo"

type (
	Empresa    = nucleo.Empresa
	Conta      = nucleo.Conta
	Tri        = nucleo.Tri
	EmpData    = nucleo.EmpData
	Capital    = nucleo.Capital
	Ult12      = nucleo.Ult12
	Supa       = nucleo.Supa
	Ponto      = nucleo.Ponto
	Serie      = nucleo.Serie
	Split      = nucleo.Split
	UltimaCot  = nucleo.UltimaCot
	Mercado    = nucleo.Mercado
	Acoes      = nucleo.Acoes
	ContaITR   = nucleo.ContaITR
	Top20Linha = nucleo.Top20Linha
	Ficha      = nucleo.Ficha
	AnoRow     = nucleo.AnoRow
	TriOut     = nucleo.TriOut
	CotOut     = nucleo.CotOut
)

var (
	triKey           = nucleo.TriKey
	profundidade     = nucleo.Profundidade
	codLess          = nucleo.CodLess
	num              = nucleo.Num
	temValores       = nucleo.TemValores
	ult12De          = nucleo.Ult12De
	codsProf         = nucleo.CodsProf
	acoesPorUnit     = nucleo.AcoesPorUnit
	classeTicker     = nucleo.ClasseTicker
	NovoSupa         = nucleo.NovoSupa
	MontarFicha      = nucleo.MontarFicha
	detectarSplits   = nucleo.DetectarSplits
	ObterCotacoes    = nucleo.ObterCotacoes
	TestarChave      = nucleo.TestarChave
	UltimaCotacao    = nucleo.UltimaCotacao
	comUltimaCotacao = nucleo.ComUltimaCotacao
	supaURLPadrao    = nucleo.SupaURLPadrao
	supaChavePadrao  = nucleo.SupaChavePadrao
	reEBIT           = nucleo.ReEBIT
	reLucro          = nucleo.ReLucro
	fp               = nucleo.Fp
	norm             = nucleo.Norm
	div              = nucleo.Div
	fatorEntre       = nucleo.FatorEntre
	tickerOposto     = nucleo.TickerOposto
	vmDasClasses     = nucleo.VmDasClasses
)
