// Leitura do Supabase (PostgREST) com a chave publicável, a mesma que o app usa.
const URL_BASE = process.env.SUPABASE_URL || 'https://crdjqnivmxuzxihgeohm.supabase.co';
const CHAVE = process.env.SUPABASE_KEY || 'sb_publishable_ij_YqC0j1gEBTiPNXW5Erg_zF6fjnZi';

async function get(caminho, params = {}) {
  const qs = new URLSearchParams(params).toString();
  const r = await fetch(`${URL_BASE}/rest/v1/${caminho}${qs ? '?' + qs : ''}`, { headers: { apikey: CHAVE, Authorization: 'Bearer ' + CHAVE } });
  if (!r.ok) throw new Error(`Supabase ${r.status}: ${await r.text()}`);
  return r.json();
}

// todas as linhas (a API devolve no máximo 1.000 por vez)
async function todos(caminho, params = {}) {
  let out = [];
  for (let off = 0; ; off += 1000) {
    const pg = await get(caminho, { ...params, limit: 1000, offset: off });
    out = out.concat(pg);
    if (pg.length < 1000) return out;
  }
}

module.exports = { get, todos };
