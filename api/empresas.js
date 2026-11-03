const { todos } = require('../lib/supa');

// Só o que a busca do topo usa: tickers, nome e se a empresa está ativa na CVM.
module.exports = async (req, res) => {
  res.setHeader('Cache-Control', 'public, max-age=900');
  try {
    const rows = await todos('empresas', { select: 'id,nome,nome_curto,setor,ativa,tickers', tickers: 'neq.{}', ult_dfp: 'not.is.null', order: 'id' });
    res.json(rows.map(r => ({ ...r, nome: r.nome_curto || r.nome })));
  } catch (e) {
    res.status(502).json({ erro: e.message });
  }
};
