const { get } = require('../lib/supa');

// Função public.log_cotacoes (sql/log_cotacoes.sql), lida pelo app.
module.exports = async (req, res) => {
  res.setHeader('Cache-Control', 'no-store');
  try {
    res.json(await get('rpc/log_cotacoes'));
  } catch (e) {
    res.status(502).json({ erro: e.message });
  }
};
