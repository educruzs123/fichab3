const { get } = require('../lib/supa');

// Última cotação do Yahoo (mesmos campos do UltimaCot do Go). null se o Yahoo não responder.
async function cotacao(ticker) {
  try {
    const r = await fetch(`https://query1.finance.yahoo.com/v8/finance/chart/${encodeURIComponent(ticker)}.SA?range=1d&interval=1d`, {
      headers: { 'User-Agent': 'Mozilla/5.0' }, signal: AbortSignal.timeout(5000)
    });
    if (!r.ok) return null;
    const m = (await r.json()).chart.result[0].meta;
    if (!(m.regularMarketPrice > 0) || !m.regularMarketTime) return null;
    const hora = new Date((m.regularMarketTime + m.gmtoffset) * 1000).toISOString(); // horário da B3, sem fuso
    return {
      preco: m.regularMarketPrice, anterior: m.chartPreviousClose || 0,
      minima: m.regularMarketDayLow || 0, maxima: m.regularMarketDayHigh || 0,
      data: hora.slice(0, 10), hora: hora.slice(11, 16)
    };
  } catch (e) {
    return null;
  }
}

// As 20 maiores (tabela top20_valor_mercado) com a cotação do dia de cada uma, em paralelo.
module.exports = async (req, res) => {
  res.setHeader('Cache-Control', 'no-store');
  try {
    const l = await get('top20_valor_mercado', { select: 'posicao,ticker,empresa,valor_mercado_bi,estimado,referencia', order: 'posicao' });
    const cots = await Promise.all(l.map(x => cotacao(x.ticker)));
    res.json(l.map((x, i) => ({ ...x, cotacao: cots[i] })));
  } catch (e) {
    res.status(502).json({ erro: e.message });
  }
};
