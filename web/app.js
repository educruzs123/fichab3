/* FichaB3Completa — páginas: início (lista), ficha, configuração. */
(function () {
  'use strict';
  const app = document.getElementById('app');
  const $ = (sel, r) => (r || document).querySelector(sel);

  // ---------- utilidades ----------
  function h(tag, attrs, ...kids) {
    const e = document.createElement(tag);
    if (attrs) for (const k in attrs) {
      const v = attrs[k];
      if (v == null || v === false) continue;
      if (k === 'class') e.className = v;
      else if (k === 'style') e.style.cssText = v;
      else if (k.startsWith('on')) e.addEventListener(k.slice(2), v);
      else e.setAttribute(k, v === true ? '' : v);
    }
    kids.flat().forEach(c => { if (c == null || c === false) return; e.appendChild(c instanceof Node ? c : document.createTextNode(String(c))); });
    return e;
  }
  const nf = (d) => new Intl.NumberFormat('pt-BR', { minimumFractionDigits: d, maximumFractionDigits: d });
  const N0 = nf(0), N1 = nf(1), N2 = nf(2);
  const ok = v => v != null && isFinite(v);
  const pct = (v, d = 1) => ok(v) ? nf(d).format(v * 100) + '%' : '—';
  const pctS = (v, d = 1) => ok(v) ? (v > 0 ? '+' : v < 0 ? '−' : '') + nf(d).format(Math.abs(v) * 100) + '%' : '—';
  const xv = v => ok(v) ? N1.format(v) + 'x' : '—';
  const rs = (v, d = 2) => ok(v) ? 'R$ ' + nf(d).format(v) : '—';
  const mi = v => { if (!ok(v)) return '—'; const a = Math.abs(v); if (a >= 1e6) return 'R$ ' + N2.format(v / 1e6) + ' tri'; if (a >= 1000) return 'R$ ' + N1.format(v / 1000) + ' bi'; return 'R$ ' + N0.format(v) + ' mi'; };
  const miN = v => ok(v) ? N0.format(v) : '—';
  const cls = v => ok(v) ? (v >= 0 ? 'up' : 'down') : '';
  const dataBR = s => s ? s.slice(8, 10) + '/' + s.slice(5, 7) + '/' + s.slice(0, 4) : '';
  async function api(url, opts) {
    const r = await fetch(url, opts);
    let j = null; try { j = await r.json(); } catch (e) { /* vazio */ }
    if (!r.ok) throw new Error((j && j.erro) || ('erro ' + r.status));
    return j;
  }
  const post = (url, body) => api(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body || {}) });
  const store = {
    get(k, d) { try { const v = localStorage.getItem(k); return v ? JSON.parse(v) : d; } catch (e) { return d; } },
    set(k, v) { try { localStorage.setItem(k, JSON.stringify(v)); } catch (e) { /* sem armazenamento */ } }
  };
  function card(...kids) { return h('section', { class: 'card secao' }, ...kids); }
  function aviso(tipo, ...kids) { return h('div', { class: 'aviso ' + tipo }, ...kids); }

  // ---------- busca no topo ----------
  let empresasCache = null;
  async function empresas() { if (!empresasCache) empresasCache = await api('/api/empresas'); return empresasCache; }
  $('#busca-topo').addEventListener('submit', async ev => {
    ev.preventDefault();
    const q = $('#busca-topo-in').value.trim();
    if (!q) return;
    try {
      const es = await empresas(), Q = q.toUpperCase();
      const e = es.find(x => x.tickers.includes(Q));
      if (e) { location.href = '/ficha?t=' + encodeURIComponent(Q); return; }
    } catch (e) { /* banco não importado */ }
    location.href = '/?q=' + encodeURIComponent(q);
  });

  // ---------- roteador ----------
  const rota = location.pathname;
  document.querySelectorAll('[data-nav]').forEach(a => { if ((rota === '/config' && a.dataset.nav === 'config') || (rota !== '/config' && a.dataset.nav === 'inicio' && rota === '/')) a.classList.add('ativo'); });
  if (rota === '/config') paginaConfig();
  else if (rota === '/ficha') paginaFicha(new URLSearchParams(location.search).get('t') || '');
  else paginaInicio(new URLSearchParams(location.search).get('q') || '');

  // ---------- início ----------
  async function paginaInicio(q0) {
    document.title = 'FichaB3Completa — empresas';
    let st;
    try { st = await api('/api/status'); } catch (e) { app.textContent = 'Erro: ' + e.message; return; }
    app.textContent = '';
    app.appendChild(h('p', { class: 'kicker' }, 'Dados da CVM'));
    app.appendChild(h('h1', null, 'Fichas de ações da B3'));
    if (st.fonte === 'supabase') {
      if (!st.banco_pronto) {
        app.appendChild(aviso('erro', h('b', null, 'Não consegui ler o banco do Supabase. '), (st.supabase && st.supabase.erro) || '', ' ', h('a', { href: '/config' }, 'Ver configuração')));
        return;
      }
      const sm = st.supabase.meta || {};
      app.appendChild(h('p', { class: 'sub' }, `${sm.n_empresas_com_ticker} empresas com ticker · demonstrações anuais ${sm.ano_min}–${sm.ano_max} · trimestres até ${sm.ult_tri} · banco Supabase atualizado em ${new Date(sm.importado_em).toLocaleString('pt-BR')}`));
    } else if (!st.banco_pronto) {
      app.appendChild(h('p', { class: 'lede' }, 'Para começar, monte o banco local com os dados da CVM. Depois, basta atualizar a cada temporada de resultados.'));
      app.appendChild(aviso('info', h('b', null, 'Banco ainda não importado. '), 'Abra a ', h('a', { href: '/config' }, 'configuração'), ' e clique em “Baixar da CVM e atualizar”.'));
      return;
    }
    if (st.fonte !== 'supabase') {
      const m = st.banco;
      app.appendChild(h('p', { class: 'sub' }, `${m.n_empresas} empresas · demonstrações anuais ${m.ano_min}–${m.ano_max} · trimestres até ${m.ult_tri} · banco importado em ${m.importado_em}`));
    }
    const rec = store.get('recentes', []);
    if (rec.length) app.appendChild(h('p', { class: 'acoes', style: 'margin:0 0 14px' }, h('span', { class: 'status', style: 'align-self:center' }, 'Recentes:'), rec.slice(0, 10).map(t => h('a', { class: 'chip', href: '/ficha?t=' + encodeURIComponent(t) }, t))));
    const inp = h('input', { type: 'search', placeholder: 'Filtrar por ticker, nome ou setor', value: q0, style: 'width:100%;font:inherit;padding:9px 12px;border-radius:10px;border:1px solid var(--ln);background:var(--card);color:var(--tx);margin-bottom:12px' });
    app.appendChild(inp);
    const so = h('label', { class: 'status', style: 'display:inline-flex;gap:6px;align-items:center;margin:0 0 10px' }, h('input', { type: 'checkbox', checked: true }), 'Só companhias ativas na CVM');
    app.appendChild(so);
    const corpo = h('tbody');
    const cont = h('p', { class: 'status' });
    app.appendChild(h('div', { class: 'tabela-wrap', style: 'max-height:none' }, h('table', { class: 't lista' }, h('thead', null, h('tr', null, h('th', null, 'Ticker'), h('th', null, 'Empresa'), h('th', null, 'Setor'), h('th', null, 'Tipo'), h('th', { class: 'n' }, 'Último DFP'), h('th', { class: 'n' }, 'Último tri'))), corpo)));
    app.appendChild(cont);
    let es = [];
    try { es = await empresas(); } catch (e) { cont.textContent = e.message; return; }
    const norm = s => (s || '').normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
    function desenhar() {
      const q = norm(inp.value.trim()), soAt = so.querySelector('input').checked;
      const lista = es.filter(e => (!soAt || e.ativa) && (!q || norm(e.tickers.join(' ') + ' ' + e.nome + ' ' + e.setor).includes(q)));
      corpo.textContent = '';
      lista.slice(0, 600).forEach(e => {
        const tr = h('tr', { onclick: () => { location.href = '/ficha?t=' + encodeURIComponent(e.ticker); } },
          h('td', null, h('a', { href: '/ficha?t=' + encodeURIComponent(e.ticker) }, e.ticker), e.tickers.length > 1 ? h('span', { class: 'status' }, ' ' + e.tickers.slice(1).join(' ')) : null),
          h('td', null, e.nome), h('td', null, e.setor || '—'), h('td', null, e.tipo || '—'), h('td', { class: 'n' }, e.ult_dfp || '—'), h('td', { class: 'n' }, e.ult_tri || '—'));
        corpo.appendChild(tr);
      });
      cont.textContent = lista.length + ' empresas' + (lista.length > 600 ? ' (mostrando 600; refine a busca)' : '');
    }
    inp.addEventListener('input', desenhar);
    so.querySelector('input').addEventListener('change', desenhar);
    desenhar();
    inp.focus();
  }

  // ---------- ficha ----------
  async function paginaFicha(t, atualizar) {
    t = t.toUpperCase();
    document.title = t + ' — FichaB3Completa';
    if (!atualizar) app.textContent = '';
    if (!atualizar) app.appendChild(h('p', { class: 'carregando' }, 'Montando a ficha de ' + t + '…'));
    let f;
    try { f = await api('/api/ficha?t=' + encodeURIComponent(t) + (atualizar ? '&atualizar=1' : '')); }
    catch (e) {
      app.textContent = '';
      app.appendChild(h('h1', null, t));
      app.appendChild(aviso('erro', e.message, ' ', /banco/.test(e.message) ? h('a', { href: '/config' }, 'Ir para a configuração') : h('a', { href: '/' }, 'Voltar à lista')));
      return;
    }
    const rec = store.get('recentes', []).filter(x => x !== f.ticker); rec.unshift(f.ticker); store.set('recentes', rec.slice(0, 12));
    document.title = f.ticker + ' — ' + (f.nome_curto || f.empresa);
    app.textContent = '';
    const A = f.anual || [], fech = A.filter(r => !r.parcial), ult = fech[fech.length - 1] || {}, ant = fech[fech.length - 2] || {};
    const at = f.atual || {}, m12 = at.doze_meses || {};
    const cats = A.map(r => r.rotulo), parc = A.map(r => r.parcial);
    const fin = f.financeira;
    const C1 = 'var(--a)', C2 = 'var(--b)', C3 = 'var(--c)', C4 = 'var(--a2)';

    // cabeçalho
    app.appendChild(h('p', { class: 'kicker' }, [f.setor, f.tipo, f.ativa ? null : 'cancelada na CVM'].filter(Boolean).join(' · ')));
    app.appendChild(h('h1', null, (f.nome_curto || f.empresa) + ' · ' + f.ticker));
    app.appendChild(h('p', { class: 'sub' }, h('span', { class: 'chips' }, f.tickers.map(x => h('a', { class: x === f.ticker ? 'chip' : 'chip cinza', href: '/ficha?t=' + x }, x))), '  ',
      `${f.empresa} · CNPJ ${f.cnpj} · DFP até ${f.ult_dfp} · trimestres até ${f.ult_tri || '—'}` + (at.cotacao_data ? ` · cotação de ${dataBR(at.cotacao_data)}` : '')));
    const barra = h('div', { class: 'acoes noprint', style: 'margin:-6px 0 18px' },
      h('button', { class: 'btn', onclick: () => paginaFicha(f.ticker, true) }, 'Atualizar cotação'),
      h('button', { class: 'btn', onclick: () => baixarCSV(f) }, 'Baixar tabela (CSV)'),
      h('button', { class: 'btn', onclick: () => window.print() }, 'Imprimir / PDF'));
    app.appendChild(barra);
    if (f.aviso_cotacao) app.appendChild(aviso('alerta', f.aviso_cotacao, ' ', /chave/.test(f.aviso_cotacao) ? h('a', { href: '/config' }, 'Configurar') : null));

    // frase de abertura
    const lede = h('p', { class: 'lede' });
    const b = t => h('b', null, t);
    const add = (...xs) => xs.flat(3).forEach(x => { if (x != null && x !== '') lede.append(x); });
    if (ult.receita != null) {
      add(`Em ${ult.ano}, a companhia teve ${fin ? 'receitas de intermediação de' : 'receita líquida de'} `, b(mi(ult.receita)), ult.cresc_receita != null ? ` (${pctS(ult.cresc_receita)} sobre ${ult.ano - 1})` : '', ' e lucro de ', b(mi(ult.lucro_ctrl)), '.');
      if (m12.lucro_ctrl != null && f.ult_tri) add(` Nos ${f.periodo_12m.replace('12 meses', '12 meses')}, lucrou `, b(mi(m12.lucro_ctrl)), m12.roe != null ? [', com ROE de ', b(pct(m12.roe))] : '', '.');
      if (at.p_l != null) add(' A ação negocia a ', b(xv(at.p_l).replace('x', ' vezes')), ' o lucro', f.pl_media != null ? [' (média histórica: ', b(xv(f.pl_media)), ')'] : '', at.dy_12m != null ? [', com dividend yield de ', b(pct(at.dy_12m)), ' em 12 meses'] : '', '.');
      app.appendChild(lede);
    }

    // indicadores de mercado
    const tile = (k, v, d, c) => h('div', { class: 'tile' }, h('div', { class: 'k' }, k), h('div', { class: 'v ' + (c || '') }, v), h('div', { class: 'd' }, d || ''));
    const t1 = h('div', { class: 'tiles' });
    t1.append(
      tile('Cotação', rs(at.cotacao), at.var_12m != null ? h('span', null, h('b', { class: cls(at.var_12m) }, pctS(at.var_12m)), ' em 12 meses') : (at.cotacao_data ? dataBR(at.cotacao_data) : 'sem cotação')),
      tile('P/L (12 meses)', xv(at.p_l), f.pl_media != null ? h('span', null, 'média ', h('b', null, xv(f.pl_media))) : ''),
      tile('P/VP', xv(at.p_vp), 'sobre o PL do ' + (f.ult_tri || 'último balanço')),
      tile('Dividend yield 12m', pct(at.dy_12m), at.proventos_12m_acao != null ? h('span', null, h('b', null, rs(at.proventos_12m_acao)), ' por ação') : ''),
      tile('Valor de mercado', at.valor_mercado_bi != null ? 'R$ ' + N1.format(at.valor_mercado_bi) + ' bi' : '—', at.acoes_mi != null ? N0.format(at.acoes_mi) + ' mi de ações' + (f.contas.acoes === 'capital' ? '' : ' (estimado)') : ''),
      tile('ROE 12m', pct(m12.roe), h('span', null, ult.ano + ': ', h('b', null, pct(ult.roe)), ' · ' + ant.ano + ': ', h('b', null, pct(ant.roe)))));
    app.appendChild(t1);
    const t2 = h('div', { class: 'tiles' });
    const hist = (k) => h('span', null, (ult.ano || '') + ': ', h('b', null, pct(ult[k])), ' · ' + (ant.ano || '') + ': ', h('b', null, pct(ant[k])));
    t2.append(tile(fin ? 'Margem bruta da intermediação' : 'Margem bruta', pct(m12.margem_bruta ?? ult.margem_bruta), m12.margem_bruta == null && ult.ano ? h('span', null, 'em ' + ult.ano + ' · ' + (ant.ano || '') + ': ', h('b', null, pct(ant.margem_bruta))) : hist('margem_bruta')));
    if (!fin) t2.append(tile('Margem EBIT', pct(m12.margem_ebit), hist('margem_ebit')));
    t2.append(tile('Margem líquida', pct(m12.margem_liq), hist('margem_liq')));
    if (!fin) t2.append(tile('Dív. líquida / EBITDA', xv(m12.div_ebitda), m12.div_liq != null ? h('span', null, 'dívida líquida ', h('b', null, mi(m12.div_liq))) : ''));
    else t2.append(tile('Patrimônio líquido', mi(m12.pl), 'no ' + (f.ult_tri || '')));
    app.appendChild(t2);
    app.appendChild(h('p', { class: 'desc' }, `Valores grandes: ${f.periodo_12m || 'últimos 12 meses'} (indicadores trimestrais da CVM). Abaixo de cada um, os anos fechados (DFP). ROE = lucro dos controladores ÷ patrimônio líquido médio.` + (f.contas.acoes === 'capital' ? ' Ações em circulação pela composição do capital informada à CVM (sem tesouraria).' : ' Ações estimadas pelo lucro ÷ lucro por ação.')));

    // escala dos gráficos em R$ bi ou mi
    const maxRec = Math.max(...A.map(r => Math.abs(r.receita || 0)), 1);
    const emBi = maxRec >= 3000, div = emBi ? 1000 : 1, un = emBi ? 'R$ bilhões' : 'R$ milhões';
    const fv = v => ok(v) ? (emBi ? N1 : N0).format(v) : '';
    const fa = v => (emBi ? (Math.abs(v) < 10 ? N1 : N0) : N0).format(v);

    // receita
    const gR = h('div', { class: 'graf' });
    app.appendChild(card(h('h2', null, `${f.rotulo_receita}, ${cats[0] || ''}–${cats[cats.length - 1] || ''}`),
      h('p', { class: 'desc' }, `${un} por ano, DFP consolidada da CVM. ${A.some(r => r.parcial) ? cats[cats.length - 1] + ' é parcial (barra clara): ' + f.periodo_12m + '.' : ''} Abaixo do valor, o crescimento sobre o ano anterior.`), gR));
    G.categorias(gR, { cats, parcial: parc, series: [{ nome: f.rotulo_receita, cor: C1, vals: A.map(r => ok(r.receita) ? r.receita / div : null) }], fmtEsq: fa, legenda: false,
      rotulos: { fmt: fv, amostra: emBi ? '000,0' : '00.000', sub: i => A[i].cresc_receita != null ? { t: pctS(A[i].cresc_receita), cor: A[i].cresc_receita >= 0 ? 'var(--ok)' : 'var(--bad)' } : null },
      titulo: i => A[i].parcial ? cats[i] + ' · ' + f.periodo_12m : cats[i], tooltip: i => [[f.rotulo_receita, mi(A[i].receita), C1], ['Crescimento', pctS(A[i].cresc_receita)]] });

    // lucro e margem líquida
    const gL = h('div', { class: 'graf' });
    app.appendChild(card(h('h2', null, 'Lucro líquido e margem líquida'), h('p', { class: 'desc' }, `Barras: lucro atribuído aos controladores (${un}); linha: lucro líquido ÷ receita (eixo à direita).`), gL));
    G.categorias(gL, { cats, parcial: parc, series: [{ nome: 'Lucro dos controladores', cor: C1, corNeg: 'var(--bad)', vals: A.map(r => ok(r.lucro_ctrl) ? r.lucro_ctrl / div : null) }, { nome: 'Margem líquida', tipo: 'linha', eixo: 'dir', cor: C2, vals: A.map(r => r.margem_liq) }],
      fmtEsq: fa, fmtDir: v => N0.format(v * 100) + '%', zero: true,
      tooltip: i => [['Lucro dos controladores', mi(A[i].lucro_ctrl), C1], ['Lucro líquido total', mi(A[i].lucro)], ['Margem líquida', pct(A[i].margem_liq), C2], ['Variação do lucro', pctS(A[i].cresc_lucro)]] });

    // margens e ROE
    const gM = h('div', { class: 'graf' }), gRoe = h('div', { class: 'graf' });
    const sM = [{ nome: fin ? 'Margem bruta da intermediação' : 'Margem bruta', tipo: 'linha', cor: C1, vals: A.map(r => r.margem_bruta) }];
    if (!fin) sM.push({ nome: 'Margem EBIT', tipo: 'linha', cor: C3, vals: A.map(r => r.margem_ebit) });
    sM.push({ nome: 'Margem líquida', tipo: 'linha', cor: C2, vals: A.map(r => r.margem_liq) });
    app.appendChild(h('div', { class: 'grade2 secao' },
      h('section', { class: 'card' }, h('h2', null, 'Margens'), h('p', { class: 'desc' }, fin ? 'Resultado bruto da intermediação e lucro líquido sobre as receitas de intermediação.' : 'Lucro bruto, EBIT (resultado antes do resultado financeiro e dos impostos) e lucro líquido sobre a receita líquida.'), gM),
      h('section', { class: 'card' }, h('h2', null, 'Retorno sobre o patrimônio (ROE)'), h('p', { class: 'desc' }, 'Lucro dos controladores ÷ patrimônio líquido médio do ano.'), gRoe)));
    G.categorias(gM, { cats, parcial: parc, series: sM, fmtEsq: v => N0.format(v * 100) + '%', zero: false, tooltip: i => sM.map(x => [x.nome, pct(x.vals[i]), x.cor]) });
    G.categorias(gRoe, { cats, parcial: parc, series: [{ nome: 'ROE', cor: C3, corNeg: 'var(--bad)', vals: A.map(r => r.roe) }], fmtEsq: v => N0.format(v * 100) + '%', legenda: false, rotulos: { fmt: v => N0.format(v * 100) + '%', amostra: '00%' }, tooltip: i => [['ROE', pct(A[i].roe), C3], ['Lucro dos controladores', mi(A[i].lucro_ctrl)], ['PL dos controladores', mi(A[i].pl_ctrl)]] });

    // P/L
    const temPreco = (f.cotacoes || []).length > 0;
    const gP = h('div', { class: 'graf' });
    const Apl = A.filter(r => r.p_l != null || r.preco != null);
    const tetoPL = f.pl_media != null ? Math.max(40, f.pl_media * 4) : 100;
    const foraPL = Apl.filter(r => r.p_l != null && (r.p_l <= 0 || r.p_l > tetoPL)).map(r => r.rotulo + (r.p_l <= 0 ? ' (prejuízo)' : ' (' + xv(r.p_l) + ')'));
    app.appendChild(card(h('h2', null, 'Preço / lucro (P/L)'), h('p', { class: 'desc' }, 'Preço de fechamento do último pregão de cada ano ÷ lucro por ação do ano (ajustados por desdobramentos). ', A.some(r => r.parcial) ? `${cats[cats.length - 1]}: cotação atual sobre o lucro dos ${f.periodo_12m}. ` : '', f.pl_media != null ? ['Linha tracejada: média dos anos com lucro, ', h('b', null, xv(f.pl_media)), '. '] : '', foraPL.length ? 'Fora do gráfico (lucro muito baixo ou prejuízo): ' + foraPL.join(', ') + '.' : '', !temPreco ? ' Precisa da chave da Alpha Vantage.' : ''), gP));
    G.categorias(gP, { cats: Apl.map(r => r.rotulo), parcial: Apl.map(r => r.parcial), series: [{ nome: 'P/L', tipo: 'linha', cor: C1, vals: Apl.map(r => (r.p_l != null && r.p_l > 0 && r.p_l <= tetoPL) ? r.p_l : null) }], fmtEsq: v => N0.format(v) + 'x', legenda: false,
      refs: f.pl_media != null ? [{ valor: f.pl_media, rotulo: 'média ' + xv(f.pl_media) }] : [], rotulos: { fmt: v => N1.format(v) + 'x', amostra: '00,0x' }, vazio: temPreco ? 'Sem lucro por ação para calcular o P/L.' : 'Cadastre a chave da Alpha Vantage para ver o P/L.',
      tooltip: i => { const r = Apl[i]; return [['P/L', r.p_l != null && r.p_l < 0 ? 'prejuízo' : xv(r.p_l), C1], [r.parcial ? 'Cotação atual' : 'Preço no fim do ano', rs(r.preco)], ['Lucro por ação', rs(r.lpa)], ['P/VP', xv(r.p_vp)], ['Valor de mercado', r.valor_mercado_bi != null ? 'R$ ' + N1.format(r.valor_mercado_bi) + ' bi' : null]]; } });

    // cotação
    if (temPreco) secaoCotacao(f);

    // proventos
    const gV = h('div', { class: 'graf' });
    const Ap = A.filter(r => !r.parcial || r.dy_data_com != null);
    app.appendChild(card(h('h2', null, 'Proventos e dividend yield'), h('p', { class: 'desc' }, 'Barras: dividendos e JCP pagos em caixa no ano (fluxo de caixa da DFP) por ação, na base atual de ações; linha: dividend yield sobre o valor de mercado no fim do ano. ',
      A.some(r => r.parcial) ? `${cats[cats.length - 1]}: proventos com data-ex nos últimos 12 meses (Alpha Vantage) sobre a cotação atual. ` : '', 'No detalhe, o payout (proventos ÷ lucro).'), gV));
    G.categorias(gV, { cats: Ap.map(r => r.rotulo), parcial: Ap.map(r => r.parcial), series: [{ nome: 'Proventos por ação (R$)', cor: C4, vals: Ap.map(r => r.parcial ? r.prov_data_com : r.prov_acao) }, { nome: 'Dividend yield', tipo: 'linha', eixo: 'dir', cor: C2, vals: Ap.map(r => r.parcial ? r.dy_data_com : r.dy) }],
      fmtEsq: v => 'R$ ' + N2.format(v), fmtDir: v => N0.format(v * 100) + '%',
      tooltip: i => { const r = Ap[i]; return [['Pagos no ano', r.parcial ? null : mi(r.proventos)], ['Por ação', rs(r.parcial ? r.prov_data_com : r.prov_acao), C4], ['Dividend yield', pct(r.parcial ? r.dy_data_com : r.dy), C2], ['Payout', pct(r.payout)], ['Recompras', r.recompras ? mi(r.recompras) : null], ['Por ação pela data-ex (AV)', r.parcial ? null : rs(r.prov_data_com)]]; } });

    // endividamento
    if (!fin) {
      const gD = h('div', { class: 'graf' });
      app.appendChild(card(h('h2', null, 'Endividamento'), h('p', { class: 'desc' }, `Barras: dívida líquida = empréstimos e financiamentos − caixa e aplicações (${un}); linha: dívida líquida ÷ EBITDA dos 12 meses (eixo à direita). Valor negativo = caixa líquido.`), gD));
      G.categorias(gD, { cats, parcial: parc, series: [{ nome: 'Dívida líquida', cor: C1, corNeg: C3, vals: A.map(r => ok(r.div_liq) ? r.div_liq / div : null) }, { nome: 'Dív. líq. / EBITDA', tipo: 'linha', eixo: 'dir', cor: C2, vals: A.map(r => r.div_ebitda) }],
        fmtEsq: fa, fmtDir: v => N1.format(v) + 'x', tooltip: i => [['Dívida bruta', mi(A[i].div_bruta)], ['Caixa e aplicações', mi(A[i].caixa)], ['Dívida líquida', mi(A[i].div_liq), C1], ['Dív. líq. / EBITDA', xv(A[i].div_ebitda), C2]] });
      // fluxo de caixa
      const gF = h('div', { class: 'graf' });
      app.appendChild(card(h('h2', null, 'Geração de caixa'), h('p', { class: 'desc' }, `Caixa das operações (FCO), investimentos em imobilizado e intangível (capex) e proventos pagos, ${un}.`), gF));
      G.categorias(gF, { cats: fech.map(r => r.rotulo), series: [{ nome: 'FCO', cor: C1, vals: fech.map(r => ok(r.fco) ? r.fco / div : null) }, { nome: 'Capex', cor: C2, vals: fech.map(r => ok(r.capex) ? r.capex / div : null) }, { nome: 'Proventos pagos', cor: C4, vals: fech.map(r => ok(r.proventos) ? r.proventos / div : null) }],
        fmtEsq: fa, tooltip: i => { const r = fech[i]; return [['FCO', mi(r.fco), C1], ['Capex', mi(r.capex), C2], ['FCO − capex', mi(ok(r.fco) && ok(r.capex) ? r.fco - r.capex : null)], ['Proventos pagos', mi(r.proventos), C4], ['Recompras', r.recompras ? mi(r.recompras) : null]]; } });
    }

    // tabela anual
    const colsA = [['Ano', r => r.rotulo, 'txt'], [f.rotulo_receita, r => miN(r.receita)], ['Cresc.', r => pctS(r.cresc_receita)], [f.rotulo_bruto, r => miN(r.lucro_bruto)]];
    if (!fin) colsA.push(['EBIT', r => miN(r.ebit)]); else colsA.push(['Res. antes trib.', r => miN(r.ebit)]);
    colsA.push(['Lucro ctrl.', r => miN(r.lucro_ctrl)], ['Mg. bruta', r => pct(r.margem_bruta)]);
    if (!fin) colsA.push(['Mg. EBIT', r => pct(r.margem_ebit)]);
    colsA.push(['Mg. líq.', r => pct(r.margem_liq)], ['ROE', r => pct(r.roe)], ['LPA', r => rs(r.lpa)], ['Ativo total', r => miN(r.ativo)], ['PL', r => miN(r.pl)]);
    if (!fin) colsA.push(['Caixa', r => miN(r.caixa)], ['Dív. bruta', r => miN(r.div_bruta)], ['Dív. líq.', r => miN(r.div_liq)], ['DL/EBITDA', r => xv(r.div_ebitda)], ['FCO', r => miN(r.fco)], ['Capex', r => miN(r.capex)]);
    colsA.push(['Proventos', r => miN(r.proventos)], ['Payout', r => pct(r.payout)], ['Preço', r => rs(r.preco)], ['P/L', r => xv(r.p_l)], ['P/VP', r => xv(r.p_vp)], ['DY', r => pct(r.parcial ? r.dy_data_com : r.dy)]);
    app.appendChild(card(h('h2', null, 'Histórico anual'), h('p', { class: 'desc' }, 'Valores em R$ milhões, exceto por ação, preços e percentuais. Linha em itálico: período parcial (12 meses).'),
      h('div', { class: 'tabela-wrap' }, h('table', { class: 't' }, h('thead', null, h('tr', null, colsA.map((c, i) => h('th', { class: i ? 'n' : '' }, c[0])))),
        h('tbody', null, A.slice().reverse().map(r => h('tr', { class: r.parcial ? 'parcial' : '' }, colsA.map((c, i) => h('td', { class: i ? 'n' : '' }, c[1](r))))))))));

    // trimestres
    const T = f.trimestres || [];
    if (T.length) {
      const colsT = [['Trimestre', t => t.tri], [f.rotulo_receita, t => miN(t.receita)], ['Var. a/a', t => pctS(t.var_receita)]];
      if (!fin) colsT.push(['EBITDA', t => miN(t.ebitda)]);
      colsT.push(['Lucro ctrl.', t => miN(t.lucro_ctrl)], ['Var. a/a', t => pctS(t.var_lucro)], ['Mg. líq.', t => pct(t.margem_liq)], ['ROE 12m', t => pct(t.roe_12m)], ['PL', t => miN(t.pl)]);
      if (!fin) colsT.push(['Dív. líq.', t => miN(t.div_liq)], ['DL/EBITDA 12m', t => xv(t.div_ebitda_12m)], ['FCO', t => miN(t.fco)]);
      const gT = h('div', { class: 'graf' });
      app.appendChild(card(h('h2', null, 'Últimos trimestres'), h('p', { class: 'desc' }, 'Resultados de cada trimestre (ITR/DFP da CVM), R$ milhões. Barras: ' + f.rotulo_receita.toLowerCase() + '; linha: margem líquida.'), gT,
        h('div', { class: 'tabela-wrap', style: 'margin-top:12px' }, h('table', { class: 't' }, h('thead', null, h('tr', null, colsT.map((c, i) => h('th', { class: i ? 'n' : '' }, c[0])))),
          h('tbody', null, T.slice().reverse().map(t => h('tr', null, colsT.map((c, i) => h('td', { class: i ? 'n' : '' }, c[1](t))))))))));
      G.categorias(gT, { cats: T.map(t => t.tri), series: [{ nome: f.rotulo_receita, cor: C1, vals: T.map(t => ok(t.receita) ? t.receita / div : null) }, { nome: 'Margem líquida', tipo: 'linha', eixo: 'dir', cor: C2, vals: T.map(t => t.margem_liq) }], fmtEsq: fa, fmtDir: v => N0.format(v * 100) + '%', altura: 240,
        tooltip: i => [[f.rotulo_receita, mi(T[i].receita), C1], ['Var. a/a', pctS(T[i].var_receita)], ['Lucro dos controladores', mi(T[i].lucro_ctrl)], ['Margem líquida', pct(T[i].margem_liq), C2]] });
    }

    // notas
    const notas = (f.notas || []).slice();
    notas.push(f.fonte === 'supabase' ? 'Demonstrações financeiras: banco fichas-b3 no Supabase, carregado dos arquivos DFP/ITR do portal de dados abertos da CVM; valores em R$ milhões.' : 'Demonstrações financeiras: arquivos DFP/ITR da CVM importados da sua pasta local; valores em R$ convertidos de milhares para milhões.');
    if (f.fonte_cotacao) notas.push('Cotações: ' + f.fonte_cotacao + '. Preço sem ajuste por proventos, ajustado por desdobramentos; “com proventos reinvestidos” usa o fechamento ajustado.');
    notas.push(f.contas.acoes === 'capital' ? 'Ações em circulação: composição do capital (total menos tesouraria) informada nos DFP/ITR da CVM; valor de mercado = ações × cotação do ticker escolhido; P/L = valor de mercado ÷ lucro dos controladores.' : 'Ações em circulação estimadas como lucro dos controladores ÷ lucro por ação da classe ' + (f.classe === 'UNIT' ? 'ON' : f.classe) + ' (sem os zips da CVM); P/L, valor de mercado e yield dependem dessa estimativa.');
    app.appendChild(h('div', { class: 'rodape' }, h('b', null, 'Notas e fontes'), h('ul', null, notas.map(n => h('li', null, n))), h('p', null, 'Ficha gerada em ' + f.gerado_em + ' pelo FichaB3Completa.')));
  }

  function secaoCotacao(f) {
    const pts = f.cotacoes.map(c => ({ d: new Date(c.data + 'T12:00:00'), mes: c.mes, v: c.fechamento, a: c.ajustado, p: c.provento }));
    const ultimo = pts[pts.length - 1];
    let per = store.get('cot_periodo', '5a');
    const seg = h('div', { class: 'seg', role: 'group', 'aria-label': 'Intervalo' });
    const opts = [['1a', '1 ano'], ['3a', '3 anos'], ['5a', '5 anos'], ['10a', '10 anos'], ['tudo', 'Tudo']];
    const btns = opts.map(([k, l]) => h('button', { type: 'button', 'aria-pressed': String(k === per), onclick: () => { per = k; store.set('cot_periodo', k); desenhar(); } }, l));
    btns.forEach(b => seg.appendChild(b));
    const g1 = h('div', { class: 'graf' }), g2 = h('div', { class: 'graf' });
    const info = h('p', { class: 'desc' }), info2 = h('p', { class: 'desc' });
    app.appendChild(card(h('div', { class: 'cab' }, h('h2', null, 'Cotação mensal'), seg), info, g1,
      h('h3', null, 'Com proventos reinvestidos'), info2, g2));
    function desenhar() {
      btns.forEach((b, i) => b.setAttribute('aria-pressed', String(opts[i][0] === per)));
      const anos = { '1a': 1, '3a': 3, '5a': 5, '10a': 10 }[per];
      let s = pts;
      if (anos) { const lim = new Date(ultimo.d); lim.setFullYear(lim.getFullYear() - anos); s = pts.filter(p => p.d >= lim); }
      if (s.length < 2) s = pts;
      const v0 = s[0], vp = ultimo.v / v0.v - 1, vt = ultimo.a / v0.a - 1;
      info.textContent = '';
      info.append('Fechamento do último pregão de cada mês, em R$, ajustado por desdobramentos. Último: ', h('b', null, rs(ultimo.v)), ' em ' + dataBR(f.atual.cotacao_data) + '. No intervalo: ', h('b', { class: cls(vp) }, pctS(vp)), '.');
      info2.textContent = '';
      info2.append('Quanto viraria R$ 1.000 aplicados em ' + v0.mes.slice(5) + '/' + v0.mes.slice(0, 4) + ': só pelo preço ', h('b', { class: cls(vp) }, pctS(vp)), '; reinvestindo dividendos e JCP ', h('b', { class: cls(vt) }, pctS(vt)), '.');
      G.tempo(g1, { pts: s, series: [{ nome: 'Fechamento', key: 'v', cor: 'var(--a)' }], area: true, fmt: v => 'R$ ' + (v < 10 ? N2 : N0).format(v),
        tooltip: (p) => { const i = pts.indexOf(p); const r = [['Fechamento', rs(p.v)]]; if (i >= 12) r.push(['Variação em 12 meses', pctS(p.v / pts[i - 12].v - 1)]); if (p.p) r.push(['Provento no mês', rs(p.p, 4)]); return r; } });
      const idx = s.map(p => ({ d: p.d, so: 1000 * p.v / v0.v, com: 1000 * p.a / v0.a }));
      G.tempo(g2, { pts: idx, series: [{ nome: 'Só preço', key: 'so', cor: 'var(--eixo)' }, { nome: 'Com proventos reinvestidos', key: 'com', cor: 'var(--c)', largura: 2.5 }], fmt: v => 'R$ ' + N0.format(v), ref: 1000, zero: false, rotFinal: true, altura: 240,
        tooltip: p => [['Com proventos', 'R$ ' + N0.format(p.com) + ' (' + pctS(p.com / 1000 - 1) + ')', 'var(--c)'], ['Só preço', 'R$ ' + N0.format(p.so) + ' (' + pctS(p.so / 1000 - 1) + ')', 'var(--eixo)']] });
    }
    desenhar();
  }

  function baixarCSV(f) {
    const cols = Object.keys(f.anual[0] || {});
    const esc = v => { if (v == null) return ''; if (typeof v === 'number') return String(v).replace('.', ','); const s = String(v); return /[;"\n]/.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s; };
    const linhas = [cols.join(';')].concat(f.anual.map(r => cols.map(c => esc(r[c])).join(';')));
    const blob = new Blob(['\ufeff' + linhas.join('\n')], { type: 'text/csv;charset=utf-8' });
    const a = h('a', { href: URL.createObjectURL(blob), download: f.ticker.toLowerCase() + '_anual.csv' });
    document.body.appendChild(a); a.click(); a.remove();
  }

  // ---------- configuração ----------
  async function paginaConfig() {
    document.title = 'Configuração — FichaB3Completa';
    let st;
    try { st = await api('/api/status'); } catch (e) { app.textContent = 'Erro: ' + e.message; return; }
    app.textContent = '';
    app.appendChild(h('p', { class: 'kicker' }, 'Configuração'));
    app.appendChild(h('h1', null, 'Fonte dos dados e cotações'));
    app.appendChild(h('p', { class: 'lede' }, 'O FichaB3Completa lê as demonstrações financeiras da CVM no banco do Supabase (ou de arquivos no Mac) e busca as cotações na Alpha Vantage.'));
    const msg = h('div');
    app.appendChild(msg);
    const flash = (tipo, t) => { msg.textContent = ''; msg.appendChild(aviso(tipo, t)); window.scrollTo({ top: 0, behavior: 'smooth' }); };
    const escolher = async (titulo) => { const r = await api('/api/escolher-pasta?titulo=' + encodeURIComponent(titulo)); return r.pasta || null; };

    // 0. fonte dos dados
    const local = h('div');
    const fonteAtual = st.config.fonte || 'supabase';
    const opt = (v, t, d) => h('label', { style: 'display:flex;gap:10px;align-items:flex-start;padding:10px 12px;border:1px solid var(--ln);border-radius:10px;cursor:pointer' },
      h('input', { type: 'radio', name: 'fonte', value: v, checked: fonteAtual === v, onchange: async () => { try { await post('/api/config', { fonte: v }); location.reload(); } catch (e) { flash('erro', e.message); } } }),
      h('span', null, h('b', null, t), h('br'), h('span', { class: 'status' }, d)));
    const inSU = h('input', { type: 'text', value: st.config.supabase_url || '', spellcheck: 'false' });
    const inSK = h('input', { type: 'text', value: st.config.supabase_chave || '', spellcheck: 'false' });
    const supaSt = h('div', { class: 'status', style: 'margin-top:10px' });
    const pintarSupa = s => {
      supaSt.textContent = '';
      if (!s) return;
      if (s.erro) { supaSt.append(h('b', { style: 'color:var(--bad)' }, '● Sem acesso. '), s.erro); return; }
      const m = s.meta || {};
      supaSt.append(h('b', { style: 'color:var(--ok)' }, '● Conectado. '), `${m.n_empresas_com_ticker} empresas com ticker (${m.n_empresas} com balanços) · DFP ${m.ano_min}–${m.ano_max} · trimestres até ${m.ult_tri} · última carga da CVM em ${new Date(m.importado_em).toLocaleString('pt-BR')}.`);
    };
    pintarSupa(st.supabase);
    const blocoSupa = h('div', { style: fonteAtual === 'supabase' ? '' : 'display:none' },
      h('div', { class: 'form', style: 'margin-top:14px' },
        h('div', { class: 'campo' }, h('label', null, 'URL do projeto'), h('div', { class: 'linha' }, inSU)),
        h('div', { class: 'campo' }, h('label', null, 'Chave publicável (somente leitura)'), h('div', { class: 'linha' }, inSK,
          h('button', { class: 'btn', onclick: async () => { try { await post('/api/config', { supabase_url: inSU.value, supabase_chave: inSK.value }); flash('info', 'Conexão salva.'); pintarSupa((await api('/api/status')).supabase); } catch (e) { flash('erro', e.message); } } }, 'Salvar'),
          h('button', { class: 'btn', onclick: async () => { supaSt.textContent = 'Testando…'; try { const r = await post('/api/testar-supabase', { url: inSU.value, chave: inSK.value }); pintarSupa({ meta: r.meta }); } catch (e) { pintarSupa({ erro: e.message }); } } }, 'Testar')),
          h('div', { class: 'ajuda' }, 'O banco no Supabase se atualiza sozinho com os dados da CVM todo domingo. Não precisa baixar nada no Mac.'))),
      supaSt);
    app.appendChild(card(h('h2', null, 'Fonte dos dados'),
      h('p', { class: 'desc' }, 'De onde o FichaB3Completa lê as demonstrações financeiras.'),
      h('div', { style: 'display:grid;gap:8px' },
        opt('supabase', 'Banco no Supabase (recomendado)', 'Projeto fichas-b3: todas as companhias da CVM, atualizado automaticamente.'),
        opt('local', 'Arquivos da CVM no Mac', 'Baixa os zips para uma pasta do Mac e monta um banco local.')),
      blocoSupa));
    app.appendChild(local);
    if (fonteAtual === 'supabase') local.style.display = 'none';
    const appendLocal = el => local.appendChild(el);

    // progresso compartilhado
    const barra = h('div', { class: 'barra' }, h('div'));
    const etapa = h('div', { class: 'status' });
    const botoes = [];
    let poll = null;
    const travar = v => botoes.forEach(b => { b.disabled = v; });
    const acompanhar = () => {
      clearInterval(poll); travar(true);
      poll = setInterval(async () => {
        let p; try { p = await api('/api/importar/progresso'); } catch (e) { return; }
        barra.firstChild.style.width = (p.pct || 0).toFixed(0) + '%';
        etapa.textContent = p.rodando ? `${p.etapa}… ${N0.format(p.pct || 0)}%` : '';
        if (!p.rodando) {
          clearInterval(poll); travar(false);
          if (p.erro) { flash('erro', 'A atualização falhou: ' + p.erro); barra.firstChild.style.width = '0'; }
          else if (p.concluiu) { flash('info', 'Banco atualizado: ' + p.resumo + '.'); empresasCache = null; const s2 = await api('/api/status'); pintarBanco(s2); pintarZips(s2.zips); }
        }
      }, 700);
    };
    const iniciar = async (url, extra) => {
      try { await post('/api/config', Object.assign({ pasta_zips: inZips.value, pasta_banco: inBanco.value }, extra || {})); await post(url); etapa.textContent = 'Iniciando…'; acompanhar(); }
      catch (e) { flash('erro', e.message); }
    };

    // 1. CVM
    const inZips = h('input', { type: 'text', value: st.config.pasta_zips || '', spellcheck: 'false' });
    const zipsSt = h('div', { class: 'status', style: 'margin-top:6px' });
    const pintarZips = z => { zipsSt.textContent = ''; if (!z) return; zipsSt.append(z.n_dfp ? h('span', { style: 'color:var(--ok);font-weight:700' }, '✓ ') : h('span', { style: 'color:var(--b);font-weight:700' }, '– '), z.n_dfp ? `Nesta pasta: ${z.n_dfp} zips DFP (${z.anos}) e ${z.n_itr} ITR.` : (z.existe ? 'Ainda não há zips da CVM nesta pasta; o primeiro download traz tudo (cerca de 600 MB).' : 'A pasta será criada no primeiro download.')); };
    pintarZips(st.zips);
    const btnCVM = h('button', { class: 'btn pri', onclick: () => iniciar('/api/atualizar-cvm?baixar=1') }, 'Baixar da CVM e atualizar');
    const btnSem = h('button', { class: 'btn', onclick: () => iniciar('/api/atualizar-cvm?baixar=0') }, 'Atualizar sem baixar');
    botoes.push(btnCVM, btnSem);
    appendLocal(card(h('h2', null, '1. Atualizar com os dados da CVM (arquivos no Mac)'),
      h('p', { class: 'desc' }, 'Baixa de dados.cvm.gov.br os arquivos de demonstrações anuais (DFP), trimestrais (ITR), cadastro e tickers (FCA), e remonta o banco. Só baixa o que mudou: anos antigos não são baixados de novo. Use depois de cada temporada de resultados (DFP até o fim de março; ITR em meados de maio, agosto e novembro).'),
      h('div', { class: 'form' }, h('div', { class: 'campo' }, h('label', null, 'Pasta onde guardar os arquivos da CVM'), h('div', { class: 'linha' }, inZips,
        h('button', { class: 'btn', onclick: async () => { try { const p = await escolher('Escolha a pasta para os arquivos da CVM'); if (p) { inZips.value = p; await post('/api/config', { pasta_zips: p }); pintarZips((await api('/api/status')).zips); } } catch (e) { flash('erro', e.message); } } }, 'Escolher…'),
        h('button', { class: 'btn', onclick: () => post('/api/abrir-pasta?qual=zips') }, 'Abrir no Finder')), zipsSt)),
      h('div', { style: 'margin-top:14px' }, barra, etapa),
      h('div', { class: 'acoes', style: 'margin-top:8px' }, btnCVM, btnSem)));

    // 2. banco
    const inBanco = h('input', { type: 'text', value: st.config.pasta_banco, spellcheck: 'false' });
    const estado = h('div', { class: 'status', style: 'margin-top:10px' });
    const pintarBanco = s => {
      estado.textContent = '';
      if (s.banco_pronto) {
        const m = s.banco;
        const fonte = m.fonte === 'cvm' ? 'zips da CVM' : 'CSVs consolidados';
        estado.append(h('b', { style: 'color:var(--ok)' }, '● Banco pronto. '), `${m.n_empresas} empresas com ticker · DFP ${m.ano_min}–${m.ano_max} · trimestres até ${m.ult_tri} · fonte: ${fonte} · ${N1.format(m.tamanho_mb)} MB · atualizado em ${m.importado_em}.`);
      } else estado.append(h('b', { style: 'color:var(--b)' }, '● Banco ainda não montado. '), 'Clique em “Baixar da CVM e atualizar”.');
    };
    pintarBanco(st);
    appendLocal(card(h('h2', null, '2. Banco de dados local'),
      h('p', { class: 'desc' }, 'Onde fica o banco montado (um arquivo compactado por empresa). O banco antigo só é substituído quando a atualização termina.'),
      h('div', { class: 'form' }, h('div', { class: 'campo' }, h('label', null, 'Pasta do banco'), h('div', { class: 'linha' }, inBanco,
        h('button', { class: 'btn', onclick: async () => { try { const p = await escolher('Escolha onde guardar o banco'); if (p) inBanco.value = p + '/banco'; } catch (e) { flash('erro', e.message); } } }, 'Escolher…'),
        h('button', { class: 'btn', onclick: async () => { try { await post('/api/config', { pasta_banco: inBanco.value }); pintarBanco(await api('/api/status')); flash('info', 'Pasta do banco salva.'); } catch (e) { flash('erro', e.message); } } }, 'Salvar'),
        h('button', { class: 'btn', onclick: () => post('/api/abrir-pasta?qual=banco') }, 'Abrir no Finder')))), estado));

    // 3. Alpha Vantage
    const inChave = h('input', { type: 'password', placeholder: st.config.tem_chave ? 'chave salva: ' + st.config.chave_mascarada : 'cole aqui a sua chave', autocomplete: 'off', spellcheck: 'false' });
    const resCh = h('div', { class: 'status', style: 'margin-top:8px' });
    app.appendChild(card(h('h2', null, 'Cotações (Alpha Vantage)'),
      h('p', { class: 'desc' }, 'Usada para cotação, P/L, valor de mercado, dividend yield e o gráfico de preço. As cotações se atualizam sozinhas: cada ficha guarda a cotação por 12 horas, e o botão “Atualizar cotação” da ficha busca na hora. A chave gratuita permite 25 consultas por dia.'),
      h('div', { class: 'form' }, h('div', { class: 'campo' }, h('label', null, 'Chave da API'), h('div', { class: 'linha' }, inChave,
        h('button', { class: 'btn', onclick: async () => { try { await post('/api/config', { chave_alpha_vantage: inChave.value }); const s2 = await api('/api/status'); inChave.value = ''; inChave.placeholder = s2.config.tem_chave ? 'chave salva: ' + s2.config.chave_mascarada : 'cole aqui a sua chave'; flash('info', s2.config.tem_chave ? 'Chave salva.' : 'Chave removida.'); } catch (e) { flash('erro', e.message); } } }, 'Salvar'),
        h('button', { class: 'btn', onclick: async () => { resCh.textContent = 'Testando…'; try { const r = await post('/api/testar-chave', { chave: inChave.value }); resCh.textContent = '✓ A chave funciona: PETR4 a ' + rs(r.preco) + ' em ' + dataBR(r.data) + '.'; } catch (e) { resCh.textContent = '✗ ' + e.message; } } }, 'Testar')),
        h('div', { class: 'ajuda' }, 'Não tem? Peça uma gratuita em ', h('a', { href: 'https://www.alphavantage.co/support/#api-key', target: '_blank', rel: 'noopener' }, 'alphavantage.co'), '.'), resCh)),
      h('div', { class: 'acoes', style: 'margin-top:10px' }, h('button', { class: 'btn', onclick: async () => { try { const r = await post('/api/limpar-cache'); flash('info', r.removidos + ' cotações guardadas foram apagadas.'); } catch (e) { flash('erro', e.message); } } }, 'Limpar cotações guardadas'))));

    // 4. alternativa: CSVs consolidados
    const inDados = h('input', { type: 'text', value: st.config.pasta_dados, spellcheck: 'false' });
    const lista = h('ul', { class: 'arquivos' });
    const pintarArquivos = arqs => { lista.textContent = ''; arqs.forEach(a => lista.appendChild(h('li', null, h('span', { class: 's ' + (a.existe ? 'ok' : 'no') }, a.existe ? '✓' : '✗'), h('span', null, h('b', null, a.nome), ' — ' + a.descricao), h('span', { class: 'm' }, a.existe ? (N0.format(a.tamanho / 1e6) + ' MB · ' + a.alterado) : 'não encontrado')))); };
    pintarArquivos(st.arquivos);
    const btnCSV = h('button', { class: 'btn', onclick: () => iniciar('/api/importar', { pasta_dados: inDados.value }) }, 'Importar dos CSVs');
    botoes.push(btnCSV);
    appendLocal(h('details', { class: 'card secao' }, h('summary', { style: 'cursor:pointer;font-weight:650' }, 'Alternativa: importar dos CSVs consolidados'),
      h('p', { class: 'desc', style: 'margin-top:10px' }, 'Para usar os arquivos DRE, BP_Ativo, BP_Passivo, DFC e Trimestral_indicadores montados à parte, em vez dos zips da CVM. A quantidade de ações continua vindo dos zips, se houver.'),
      h('div', { class: 'form' }, h('div', { class: 'campo' }, h('div', { class: 'linha' }, inDados,
        h('button', { class: 'btn', onclick: async () => { try { const p = await escolher('Escolha a pasta com os CSVs consolidados'); if (p) { inDados.value = p; await post('/api/config', { pasta_dados: p }); pintarArquivos((await api('/api/status')).arquivos); } } catch (e) { flash('erro', e.message); } } }, 'Escolher…')))), lista,
      h('div', { class: 'acoes', style: 'margin-top:10px' }, btnCSV)));

    // 5. sobre
    app.appendChild(card(h('h2', null, 'Sobre'),
      h('p', { class: 'desc' }, `FichaB3Completa ${st.versao}. Configuração e registros em ${st.pasta_app}. O programa fica rodando em segundo plano enquanto você usa as fichas; abrir o app de novo só reabre esta janela.`),
      h('div', { class: 'acoes' }, h('button', { class: 'btn', onclick: () => post('/api/abrir-pasta?qual=app') }, 'Abrir pasta de configuração'),
        h('button', { class: 'btn perigo', onclick: async () => { if (!confirm('Encerrar o FichaB3Completa? Para voltar, abra o app de novo.')) return; try { await post('/api/encerrar'); } catch (e) { /* já saiu */ } app.textContent = ''; app.appendChild(h('h1', null, 'FichaB3Completa encerrado')); app.appendChild(h('p', { class: 'lede' }, 'Pode fechar esta aba. Para voltar, abra o FichaB3Completa em Aplicativos.')); } }, 'Encerrar FichaB3Completa'))));
    if (st.importando) acompanhar();
  }
})();
