// Port de supabase.go e das rotas /api/* do main.go para o navegador (site estático).
// Lê o banco fichas-b3 pela API REST com a chave publicável (somente leitura) e monta
// as respostas no mesmo formato que o app Go entrega ao front. Fora de 127.0.0.1,
// intercepta o fetch para as rotas /api/*; no app instalado nada muda.
(function (raiz) {
  'use strict';

  const Calc = typeof module !== 'undefined' && module.exports ? require('./ficha.js') : raiz.FichaCalc;

  const SUPA_URL = 'https://crdjqnivmxuzxihgeohm.supabase.co';
  const SUPA_CHAVE = 'sb_publishable_ij_YqC0j1gEBTiPNXW5Erg_zF6fjnZi';
  const PAGINA = 1000;
  const MIN = 60 * 1000;

  const cache = { lista: null, listaEm: 0, porTk: {}, res12: null, res12Em: 0, merc: null, mercEm: 0, cot: {}, dados: {}, ultima: {} };

  class Erro extends Error {
    constructor(status, msg) { super(msg); this.status = status; }
  }

  // ---- cliente REST (get / supaTodos) ----
  async function consulta(caminho, params, offset) {
    const q = new URLSearchParams(params);
    if (offset != null) { q.set('limit', String(PAGINA)); q.set('offset', String(offset)); }
    let r;
    try {
      r = await fetch(SUPA_URL + '/rest/v1/' + caminho + '?' + q.toString(), {
        headers: { apikey: SUPA_CHAVE, Accept: 'application/json' },
      });
    } catch (e) {
      throw new Erro(502, 'sem conexão com o Supabase: ' + e.message);
    }
    const txt = await r.text();
    if (!r.ok) {
      let msg = '';
      try { msg = JSON.parse(txt).message || ''; } catch (e) { /* corpo que não é JSON */ }
      if (!msg) msg = txt.trim().slice(0, 200);
      if (r.status === 401 || r.status === 403) throw new Erro(502, `o Supabase recusou a chave (${r.status}): ${msg}`);
      throw new Erro(502, `Supabase respondeu ${r.status}: ${msg}`);
    }
    return JSON.parse(txt);
  }

  // todas as páginas de uma consulta (a API devolve no máximo 1.000 linhas por vez)
  async function todos(caminho, params) {
    let out = [];
    for (let off = 0; ; off += PAGINA) {
      const pg = await consulta(caminho, params, off);
      out = out.concat(pg);
      if (pg.length < PAGINA) return out;
    }
  }

  const sv = (x) => (x == null ? '' : x);
  const num = (x) => (x == null ? 0 : x);
  const soDigitos = (t) => t.replace(/[0-9]+$/, '');

  // ---- empresas ----
  async function lista(forcar) {
    if (!forcar && cache.lista && Date.now() - cache.listaEm < 15 * MIN) return cache.lista;
    const rows = await todos('empresas', {
      select: 'id,cnpj,cod_cvm,nome,nome_curto,setor,tipo,ativa,tickers,ult_dfp,ult_tri',
      tickers: 'neq.{}', ult_dfp: 'not.is.null', order: 'id',
    });
    const porTk = {};
    const l = rows.map((r) => {
      const e = {
        id: r.id, cnpj: sv(r.cnpj), cod_cvm: sv(r.cod_cvm), nome: sv(r.nome), nome_curto: sv(r.nome_curto), setor: sv(r.setor),
        tipo: sv(r.tipo), tickers: r.tickers || [], ult_dfp: num(r.ult_dfp), ult_tri: sv(r.ult_tri), ativa: !!r.ativa,
      };
      if (!e.nome_curto) e.nome_curto = e.nome;
      for (const t of e.tickers) if (!(t in porTk)) porTk[t] = e;
      return e;
    });
    l.sort((a, b) => (a.tickers[0] < b.tickers[0] ? -1 : a.tickers[0] > b.tickers[0] ? 1 : 0));
    cache.lista = l; cache.porTk = porTk; cache.listaEm = Date.now();
    return l;
  }

  async function porTicker(t) {
    const l = await lista(false);
    if (cache.porTk[t]) return cache.porTk[t];
    const raiz2 = soDigitos(t);
    for (const e of l) {
      for (const x of e.tickers) if (raiz2 !== '' && soDigitos(x) === raiz2) return e;
    }
    return null;
  }

  // último trimestre de cada empresa (por id), com os números de 12 meses
  async function ultimos12(forcar) {
    if (!forcar && cache.res12 && Date.now() - cache.res12Em < 15 * MIN) return cache.res12;
    const rows = await todos('trimestres', {
      select: 'empresa_id,ano,tri,receita_12m,lucro_12m,lucro_ctrl,roe_12m,div_ebitda_12m,pl',
      ano: 'gte.' + (new Date().getFullYear() - 2), order: 'empresa_id,dt_refer',
    });
    const porEmp = {};
    for (const l of rows) (porEmp[l.empresa_id] = porEmp[l.empresa_id] || []).push(l);
    const r = {};
    for (const id of Object.keys(porEmp)) r[id] = ult12De(porEmp[id]);
    cache.res12 = r; cache.res12Em = Date.now();
    return r;
  }

  // mesmo cálculo de banco.go: número de 12 meses do último trimestre (com ROE/patrimônio ajustados)
  function ult12De(tris) {
    const t = tris[tris.length - 1];
    const u = {
      tri: t.tri, receita_12m: t.receita_12m ?? null, lucro_12m: t.lucro_12m ?? null, margem_liq_12m: Calc.div(t.lucro_12m ?? null, t.receita_12m ?? null),
      roe_12m: t.roe_12m ?? null, div_ebitda_12m: t.div_ebitda_12m ?? null,
    };
    const pl = t.pl ?? null;
    u.lucroCtrl = u.lucro_12m;
    if (tris.length >= 4) {
      let soma = 0;
      for (const x of tris.slice(tris.length - 4)) {
        if (x.lucro_ctrl == null) return u;
        soma += x.lucro_ctrl;
      }
      u.lucroCtrl = Calc.fp(soma);
    }
    if ((pl != null && pl <= 0) || (u.roe_12m != null && u.roe_12m > 0 && t.lucro_12m != null && t.lucro_12m < 0)) u.roe_12m = null;
    u.pl = pl;
    return u;
  }

  // ---- empresa: demonstrações e trimestres (Dados) ----
  async function dados(e, forcar) {
    const c = cache.dados[e.id];
    if (c && !forcar && Date.now() - c.em < 30 * MIN) return c.d;
    const id = String(e.id);
    const contas = (tab) => todos(tab, {
      select: 'ano,cod_conta,valor,contas_nome(nome)', empresa_id: 'eq.' + id, order: 'ano,cod_conta',
    });
    const [dre, bp, dfc, bases, tri, caps] = await Promise.all([
      contas('dre'), contas('bp'), contas('dfc'),
      consulta('base_ano', { select: 'ano,base', empresa_id: 'eq.' + id }, null),
      todos('trimestres', { select: '*', empresa_id: 'eq.' + id, order: 'dt_refer' }),
      consulta('capital', { select: 'dt_refer,qt_on,qt_pn,qt_total,tes_on,tes_pn,tes_total', empresa_id: 'eq.' + id, order: 'dt_refer' }, null),
    ]);
    const d = { cnpj: e.cnpj, dre: {}, bp: {}, dfc: {}, nomesDR: {}, nomesBP: {}, baseAno: {}, tri: [], capital: {} };
    const encher = (rows, vals, nomes) => {
      for (const r of rows) {
        if (r.valor == null) continue;
        const a = String(r.ano);
        if (!vals[a]) { vals[a] = {}; nomes[a] = {}; }
        vals[a][r.cod_conta] = r.valor;
        if (r.contas_nome) nomes[a][r.cod_conta] = r.contas_nome.nome;
      }
    };
    encher(dre, d.dre, d.nomesDR);
    encher(bp, d.bp, d.nomesBP);
    for (const r of dfc) {
      if (r.valor == null) continue;
      const a = String(r.ano);
      (d.dfc[a] = d.dfc[a] || []).push({ cod: r.cod_conta, nome: r.contas_nome ? r.contas_nome.nome : '', v: r.valor });
    }
    for (const a of Object.keys(d.dfc)) d.dfc[a].sort((x, y) => (Calc.codLess(x.cod, y.cod) ? -1 : 1));
    for (const b of bases) d.baseAno[String(b.ano)] = b.base;
    d.tri = tri.slice().sort((x, y) => Calc.triKey(x.ano, x.tri) - Calc.triKey(y.ano, y.tri));
    for (const k of caps) {
      const total = num(k.qt_total);
      if (total > 0) d.capital[k.dt_refer] = { on: num(k.qt_on), pn: num(k.qt_pn), total, tes_on: num(k.tes_on), tes_pn: num(k.tes_pn), tes_total: num(k.tes_total) };
    }
    cache.dados[e.id] = { d, em: Date.now() };
    return d;
  }

  // ---- cotações mensais (Cotacoes) ----
  async function cotacoes(ticker) {
    const rows = await todos('cotacoes_mensais', {
      select: 'data,fechamento,ajustado,dividendos,split', ticker: 'eq.' + ticker, order: 'mes',
    });
    if (!rows.length) throw new Erro(404, 'sem cotações no Supabase para ' + ticker);
    const splits = [];
    for (const r of rows) if (r.split > 0 && r.split !== 1) splits.push({ data: r.data, fator: r.split });
    const pontos = [];
    for (const r of rows) {
      if (!(r.fechamento > 0)) continue;
      const f = Calc.fatorEntre(splits, r.data, '9999');
      pontos.push({ data: r.data, close: r.fechamento * f, adj: r.ajustado, div: r.dividendos * f });
    }
    const ult = rows[rows.length - 1].data;
    const br = ult.slice(8, 10) + '/' + ult.slice(5, 7) + '/' + ult.slice(0, 4);
    return { pontos, splits, simbolo: ticker, baixadoEm: br, fonte: 'banco fichas-b3 no Supabase (cotações mensais até ' + br + ')' };
  }

  async function cotacoesDoTicker(ticker, forcar) {
    const c = cache.cot[ticker];
    if (c && !forcar && Date.now() - c.em < 30 * MIN) return c.serie;
    const serie = await cotacoes(ticker);
    cache.cot[ticker] = { serie, em: Date.now() };
    return serie;
  }

  // ---- mercado: cotação, variação e proventos de 12 meses (Mercados) ----
  function dataMenos(anos, meses, dataISO) {
    const d = new Date(dataISO + 'T00:00:00Z');
    d.setUTCFullYear(d.getUTCFullYear() + anos);
    d.setUTCMonth(d.getUTCMonth() + meses);
    return d.toISOString().slice(0, 10);
  }

  async function mercados(forcar) {
    if (!forcar && cache.merc && Date.now() - cache.mercEm < 15 * MIN) return cache.merc;
    const hoje = new Date().toISOString().slice(0, 10);
    const cs = await todos('cotacoes_mensais', {
      select: 'ticker,data,fechamento,ajustado,dividendos,split', data: 'gte.' + dataMenos(-1, -2, hoje), order: 'ticker,mes',
    });
    const porTk = {};
    for (const c of cs) if (c.fechamento > 0) (porTk[c.ticker] = porTk[c.ticker] || []).push(c);
    const merc = {};
    for (const tk of Object.keys(porTk)) {
      const l = porTk[tk];
      const u = l[l.length - 1];
      const corte = dataMenos(-1, 0, u.data);
      const m = { preco: u.fechamento, var12: null, dy12: 0, splits: [] };
      l.forEach((c, i) => {
        if (c.split > 0 && c.split !== 1) m.splits.push({ data: c.data, fator: c.split });
        if (c.data <= corte) { m.var12 = Calc.fp(u.fechamento / c.fechamento - 1); return; }
        if (i === 0 || c.ajustado <= 0 || l[i - 1].ajustado <= 0) return;
        // a coluna de proventos é incompleta: o rendimento sai do salto no fechamento ajustado
        const a = l[i - 1];
        let y = 1 - (a.ajustado / a.fechamento) / (c.ajustado / c.fechamento);
        if (y < 0 || y > 0.5) y = c.dividendos / a.fechamento;
        if (y >= 0.001 || c.dividendos > 0) m.dy12 += y;
      });
      merc[tk] = m;
    }
    const caps = await todos('capital', {
      select: 'empresa_id,dt_refer,qt_total,tes_total', dt_refer: 'gte.' + dataMenos(-2, 0, hoje), order: 'empresa_id,dt_refer',
    });
    const acoes = {};
    for (const c of caps) {
      const total = num(c.qt_total);
      if (total > 0) acoes[c.empresa_id] = { qt: total - num(c.tes_total), data: c.dt_refer }; // ordenado por data: fica a última
    }
    cache.merc = { merc, acoes }; cache.mercEm = Date.now();
    return cache.merc;
  }

  // ---- ITRs ----
  async function itrs(e) {
    const l = await consulta('bp_tri', { select: 'dt_refer', empresa_id: 'eq.' + e.id, cod_conta: 'eq.1', order: 'dt_refer' }, null);
    const out = [];
    for (const x of l) if (out.length === 0 || out[out.length - 1] !== x.dt_refer) out.push(x.dt_refer);
    return out;
  }

  async function itr(e, dt) {
    const ler = async (tab, sel) => {
      const rows = await todos(tab, { select: sel + ',contas_nome(nome)', empresa_id: 'eq.' + e.id, dt_refer: 'eq.' + dt, order: 'cod_conta' });
      let base = '';
      const out = rows.map((r) => {
        base = r.base !== undefined ? r.base : base;
        const c = { cod: r.cod_conta, nome: r.contas_nome ? r.contas_nome.nome : '', valor: r.valor == null ? null : r.valor };
        if (r.dt_ini) c.dt_ini = r.dt_ini;
        return c;
      });
      return { out, base };
    };
    const [dre, bp, dfc] = await Promise.all([
      ler('dre_tri', 'cod_conta,valor,dt_ini,base'), ler('bp_tri', 'cod_conta,valor,base'), ler('dfc_tri', 'cod_conta,valor,dt_ini,base'),
    ]);
    return { dre: dre.out, bp: bp.out, dfc: dfc.out, base: dre.base };
  }

  async function cotacoesDiarias(t) {
    const ini = new Date(); ini.setFullYear(ini.getFullYear() - 5);
    return todos('cotacoes', {
      select: 'data,fechamento', ticker: 'eq.' + t, data: 'gte.' + ini.toISOString().slice(0, 10), order: 'data',
    });
  }

  async function demonstrativos(t) {
    const l = await consulta('demonstrativos', { select: 'arquivo,tamanho', ticker: 'eq.' + t }, null);
    return l.map((a) => ({
      nome: a.arquivo, tamanho: a.tamanho,
      url: SUPA_URL + '/storage/v1/object/public/demonstrativos/' + encodeURIComponent(t) + '/' + encodeURIComponent(a.arquivo),
    }));
  }

  async function meta() {
    const m = await consulta('meta', { select: '*' }, null);
    if (!m.length) throw new Erro(502, 'o banco do Supabase ainda não tem carga concluída');
    return m[0];
  }

  // ---- cotação do dia (Yahoo, pelo proxy /api/cotacao) ----
  async function ultimaCotacao(t, forcar) {
    const c = cache.ultima[t];
    if (c && !forcar && Date.now() - c.em < 5 * MIN) return c.u;
    let u = null;
    try {
      const r = await fetch('/api/cotacao?t=' + encodeURIComponent(t));
      if (r.ok) u = await r.json();
    } catch (e) { u = null; }
    cache.ultima[t] = { u, em: Date.now() };
    return u;
  }

  // Ficha.Atual, fixa para o último preço de hoje (comUltimaCotacao)
  function comUltimaCotacao(serie, u) {
    if (!serie || !u || !serie.pontos.length) return serie;
    const ult = serie.pontos[serie.pontos.length - 1];
    if (u.data <= ult.data || ult.close <= 0) return serie;
    const c = Object.assign({}, serie, { pontos: serie.pontos.slice() });
    const n = { data: u.data, close: u.preco, adj: ult.adj * u.preco / ult.close, div: 0 };
    if (u.data.slice(0, 7) === ult.data.slice(0, 7)) {
      n.div = ult.div;
      c.pontos[c.pontos.length - 1] = n;
    } else {
      c.pontos.push(n);
    }
    return c;
  }

  // ---- rotas /api/* ----
  const jsonOk = (x) => x;
  const CONFIG = {
    fonte: 'supabase', supabase_url: SUPA_URL, supabase_chave: SUPA_CHAVE, pasta_zips: '', pasta_banco: '', pasta_dados: '',
    tem_chave: false, chave_mascarada: '', porta: 0,
  };
  const soNoApp = () => new Erro(409, 'Disponível só no app instalado. No site, a conexão com o Supabase é fixa.');

  async function status() {
    const sp = { url: SUPA_URL };
    let pronto = true;
    try { sp.meta = await meta(); } catch (e) { sp.erro = e.message; pronto = false; }
    return { versao: 'site', fonte: 'supabase', pasta_app: '', config: CONFIG, arquivos: [], zips: [], banco_pronto: pronto, importando: false, supabase: sp };
  }

  async function empresas(params) {
    const forcar = params.get('atualizar') === '1';
    const l = await lista(forcar);
    let res12 = {}, mercs = { merc: {}, acoes: {} };
    try { res12 = await ultimos12(forcar); } catch (e) { /* sem os números de 12 meses: a lista segue sem eles */ }
    try { mercs = await mercados(forcar); } catch (e) { /* sem cotações: a lista segue sem preço */ }
    const out = [];
    for (const e of l) {
      const t = e.tickers[0] || '';
      const it = {
        ticker: t, tickers: e.tickers, nome: e.nome, setor: e.setor, tipo: e.tipo, cnpj: e.cnpj, ult_dfp: e.ult_dfp,
        ult_tri: e.ult_tri, ativa: e.ativa,
      };
      const u = res12[e.id];
      if (u) {
        Object.assign(it, { tri: u.tri, receita_12m: u.receita_12m, lucro_12m: u.lucro_12m, margem_liq_12m: u.margem_liq_12m, roe_12m: u.roe_12m, div_ebitda_12m: u.div_ebitda_12m });
      }
      const m = mercs.merc[t];
      if (m) {
        it.cotacao = Calc.fp(m.preco);
        it.var_12m = m.var12 == null ? undefined : m.var12;
        it.dy_12m = Calc.fp(m.dy12);
        // lucro mínimo de R$ 100 mil: perto de zero, o P/L explode sem significado
        const a = mercs.acoes[e.id];
        if (a && a.qt > 0 && u && u.lucroCtrl != null && u.lucroCtrl >= 0.1 && u.pl != null && u.pl > 0) {
          let vm = m.preco * a.qt * Calc.fatorEntre(m.splits, a.data, '9999') / Calc.acoesPorUnit(t) / 1e6; // R$ milhões
          // a CVM às vezes informa as ações em milhares: valor de mercado 1000× menor que o normal
          if (vm / u.pl < 0.02) vm *= 1000;
          it.p_l = Calc.fp(vm / u.lucroCtrl);
        }
      }
      for (const k of Object.keys(it)) if (it[k] === undefined) delete it[k];
      out.push(it);
    }
    out.sort((a, b) => (a.ticker < b.ticker ? -1 : a.ticker > b.ticker ? 1 : 0));
    return out;
  }

  async function ficha(params) {
    const t = (params.get('t') || '').trim().toUpperCase();
    const atualizar = params.get('atualizar') === '1';
    const e = await porTicker(t);
    if (!e) throw new Erro(404, 'ticker não encontrado no banco do Supabase: ' + t);
    let d;
    try { d = await dados(e, atualizar); } catch (err) { throw new Erro(502, 'erro ao ler a empresa no Supabase: ' + err.message); }
    let serie = null, avisoCot = '';
    try { serie = await cotacoesDoTicker(t, atualizar); } catch (err) { avisoCot = err.message; }
    const ult = await ultimaCotacao(t, atualizar);
    serie = comUltimaCotacao(serie, ult);
    const f = Calc.montarFicha(e, d, t, serie);
    if (ult && f.atual.cotacao_data === ult.data) {
      f.atual.cotacao_hora = ult.hora;
      if (ult.anterior > 0) {
        f.atual.var_dia = ult.preco / ult.anterior - 1;
        f.atual.cotacao_anterior = ult.anterior;
      }
      f.fonte_cotacao = (f.fonte_cotacao || '') + '; última cotação: Yahoo Finance';
    }
    f.fonte = 'supabase';
    if (avisoCot) f.aviso_cotacao = avisoCot;
    return f;
  }

  async function itrRota(params) {
    const t = (params.get('t') || '').trim().toUpperCase();
    const dt = params.get('dt') || '';
    if (dt && !/^\d{4}-\d{2}-\d{2}$/.test(dt)) throw new Erro(400, 'data inválida: use AAAA-MM-DD');
    const e = await porTicker(t);
    if (!e) throw new Erro(404, 'ticker não encontrado no banco do Supabase: ' + t);
    const lst = await itrs(e);
    const out = { ticker: t, empresa: e.nome_curto, tickers: e.tickers, itrs: lst };
    if (dt) {
      const r = await itr(e, dt);
      if (r.dre.length + r.bp.length + r.dfc.length === 0) throw new Erro(404, 'o banco não tem as demonstrações de ' + t + ' em ' + dt);
      Object.assign(out, { dt, base: r.base, dre: r.dre, bp: r.bp, dfc: r.dfc });
    }
    return out;
  }

  async function demonstrativosRota(params) {
    const t = (params.get('t') || '').trim().toUpperCase();
    if (!/^[A-Z0-9]{4,6}$/.test(t)) return [];
    try { return await demonstrativos(t); } catch (e) { return []; }
  }

  async function cotacoesDiariasRota(params) {
    const t = (params.get('t') || '').trim().toUpperCase();
    const l = await cotacoesDiarias(t);
    const u = await ultimaCotacao(t, false);
    if (u && l.length) {
      const ult = l[l.length - 1];
      if (u.data > ult.data) l.push({ data: u.data, fechamento: u.preco });
    }
    return l;
  }

  // roteia um pedido /api/... para os dados locais; devolve o objeto a ser serializado
  async function rotear(url, opts) {
    const [caminho, qs] = url.split('?');
    const params = new URLSearchParams(qs || '');
    const metodo = ((opts && opts.method) || 'GET').toUpperCase();
    switch (caminho) {
      case '/api/status': return status();
      case '/api/empresas': return empresas(params);
      case '/api/ficha': return ficha(params);
      case '/api/itr': return itrRota(params);
      case '/api/demonstrativos': return demonstrativosRota(params);
      case '/api/cotacoes-diarias': return cotacoesDiariasRota(params);
      case '/api/log-cotacoes': return consultaLog('log_cotacoes');
      case '/api/log-cvm': return consultaLog('log_cvm_empresas');
      case '/api/testar-supabase': return { ok: true, meta: await meta() };
      case '/api/config':
        if (metodo === 'GET') return CONFIG;
        throw soNoApp();
      case '/api/fatos-relevantes': {
        const r = await fetch('/api/fatos');
        const j = await r.json();
        if (!r.ok) throw new Erro(r.status, j.erro || 'erro ao ler os fatos relevantes');
        return j;
      }
      default: throw soNoApp();
    }
  }

  async function consultaLog(funcao) {
    return todos('rpc/' + funcao, {});
  }

  // ---- interceptação do fetch fora do app instalado (127.0.0.1) ----
  function ativarShim() {
    if (typeof window === 'undefined' || !window.fetch || window.location.hostname === '127.0.0.1') return;
    const fetchOriginal = window.fetch.bind(window);
    window.fetch = async function (input, opts) {
      const url = typeof input === 'string' ? input : input.url;
      if (!url.startsWith('/api/') || url.startsWith('/api/fatos?') || url === '/api/fatos' || url.startsWith('/api/cotacao?')) {
        return fetchOriginal(input, opts);
      }
      const cab = { 'Content-Type': 'application/json' };
      try {
        const data = jsonOk(await rotear(url, opts));
        return new Response(JSON.stringify(data), { status: 200, headers: cab });
      } catch (e) {
        return new Response(JSON.stringify({ erro: e.message }), { status: e.status || 500, headers: cab });
      }
    };
  }
  ativarShim();

  const API = { rotear, ult12De, comUltimaCotacao, dataMenos, consulta, todos, lista, porTicker, dados, cotacoesDoTicker, mercados, ultimos12 };
  if (typeof module !== 'undefined' && module.exports) module.exports = API;
  else raiz.DadosSupa = API;
})(typeof window !== 'undefined' ? window : globalThis);
