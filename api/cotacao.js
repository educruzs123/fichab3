// Cotação do dia de um ticker (Yahoo Finance), como UltimaCotacao em av.go.
// O Yahoo não libera CORS para o navegador, por isso a consulta passa por esta função.
const RE_TICKER = /^[A-Z0-9]{4,6}$/;

module.exports = async (req, res) => {
  const t = (new URL(req.url, 'http://x').searchParams.get('t') || '').trim().toUpperCase();
  if (!RE_TICKER.test(t)) return res.status(400).json({ erro: 'ticker inválido' });
  res.setHeader('Cache-Control', 'public, s-maxage=300, stale-while-revalidate=600');
  try {
    const r = await fetch(`https://query1.finance.yahoo.com/v8/finance/chart/${encodeURIComponent(t)}.SA?range=1d&interval=1d`,
      { headers: { 'User-Agent': 'Mozilla/5.0' } });
    if (!r.ok) return res.status(200).json(null);
    const j = await r.json();
    const result = j && j.chart && j.chart.result && j.chart.result[0];
    if (!result) return res.status(200).json(null);
    const m = result.meta;
    if (!(m.regularMarketPrice > 0) || !(m.regularMarketTime > 0)) return res.status(200).json(null);
    // horário da bolsa: o instante deslocado pelo fuso informado pelo Yahoo
    const hora = new Date((m.regularMarketTime + (m.gmtoffset || 0)) * 1000).toISOString();
    return res.status(200).json({ preco: m.regularMarketPrice, anterior: m.chartPreviousClose || 0, data: hora.slice(0, 10), hora: hora.slice(11, 16) });
  } catch (e) {
    return res.status(200).json(null);
  }
};
