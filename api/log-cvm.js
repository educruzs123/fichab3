const { get } = require('../lib/supa');

// Função public.log_cvm_empresas (sql/log_cvm_empresas.sql), lida pelo app.
module.exports = async (req, res) => {
  res.setHeader('Cache-Control', 'no-store');
  try {
    res.json(await get('rpc/log_cvm_empresas'));
  } catch (e) {
    res.status(502).json({ erro: e.message });
  }
};
