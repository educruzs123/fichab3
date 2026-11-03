const { get } = require('../lib/supa');

// Mesma forma de /api/status do app Go, só com o que a página inicial usa.
module.exports = async (req, res) => {
  res.setHeader('Cache-Control', 'no-store');
  const sp = { url: process.env.SUPABASE_URL || 'https://crdjqnivmxuzxihgeohm.supabase.co' };
  try {
    const m = await get('meta', { select: '*' });
    if (!m.length) throw new Error('o banco do Supabase ainda não tem carga concluída');
    sp.meta = m[0];
    res.json({ fonte: 'supabase', hospedagem: 'vercel', banco_pronto: true, supabase: sp });
  } catch (e) {
    sp.erro = e.message;
    res.json({ fonte: 'supabase', hospedagem: 'vercel', banco_pronto: false, supabase: sp });
  }
};
