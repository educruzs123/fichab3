// Port de ficha.go para o navegador: monta a ficha de uma empresa a partir dos
// dados do Supabase (DRE, balanço, DFC, capital e trimestres) e da série de preços.
// Os nomes das chaves de saída são os mesmos do JSON que o app Go entrega ao front.
(function (raiz) {
  'use strict';

  // ---- utilitários numéricos (fp, div, sub, cresc de ficha.go) ----
  const fp = (v) => (v == null || !Number.isFinite(v) ? null : v);
  const div = (a, b) => (a == null || b == null || b === 0 ? null : fp(a / b));
  const sub = (a, b) => (a == null || b == null ? null : fp(a - b));
  const cresc = (a, b) => (a == null || b == null || b === 0 ? null : fp(a / Math.abs(b) - b / Math.abs(b)));

  const ACENTOS = { á: 'a', à: 'a', ã: 'a', â: 'a', é: 'e', ê: 'e', í: 'i', ó: 'o', õ: 'o', ô: 'o', ú: 'u', ç: 'c',
    Á: 'a', É: 'e', Í: 'i', Ó: 'o', Ú: 'u', Ç: 'c', Ã: 'a', Õ: 'o', Â: 'a', Ê: 'e', Ô: 'o' };
  const norm = (s) => String(s || '').replace(/[áàãâéêíóõôúçÁÉÍÓÚÇÃÕÂÊÔ]/g, (c) => ACENTOS[c]).toLowerCase();

  const RE = {
    ebit: /antes do resultado financeiro/,
    preTax: /antes dos tributos/,
    lucro: /(lucro|prejuizo|resultado).*(periodo|exercicio)/,
    capex: /imobilizado|intangive|ativo fixo|ativos fixos|capex|investimentos? em (bens|ativos)/,
    naoCapex: /aliena|venda|recebimento|recebid|baixa/,
    prov: /dividendo|juros sobre (o )?capital|jcp|jscp|juros s\/ ?capital|proventos/,
    naoProv: /recebid|nao controlador|minoritari|de coligad|de controlad/,
    recompra: /acoes em tesouraria|recompra|aquisicao de acoes proprias/,
  };

  // profundidade de um código de conta: "3.01.02" tem 3 níveis
  const prof = (cod) => cod.split('.').length;
  // ordem numérica dos códigos (1.10 vem depois de 1.9)
  function codLess(a, b) {
    const pa = a.split('.'), pb = b.split('.');
    for (let i = 0; i < pa.length && i < pb.length; i++) {
      const x = parseInt(pa[i], 10) || 0, y = parseInt(pb[i], 10) || 0;
      if (x !== y) return x < y;
    }
    return pa.length < pb.length;
  }
  // ano*10 + trimestre: 2025 "4T" -> 20254
  const triKey = (ano, tri) => ano * 10 + (tri ? parseInt(tri[0], 10) || 0 : 0);

  const TRI_CAMPOS = ['ano', 'tri', 'base', 'receita', 'lucro_bruto', 'ebit', 'da', 'ebitda', 'res_fin', 'lucro', 'lucro_ctrl',
    'margem_liq', 'fco', 'ativo', 'caixa', 'div_bruta', 'div_liq', 'pl', 'receita_12m', 'ebitda_12m', 'lucro_12m', 'roe_12m',
    'div_ebitda_12m', 'fco_12m'];

  // unidades (1 ON + k PN) negociadas como um só papel: quantidade de ações ÷ k
  const UNITS = {
    ALUP11: [3, '1 ON + 2 PN'], BMGB11: [3, '1 ON + 2 PN'], BPAC11: [3, '1 ON + 2 PNA'], BRBI11: [3, '1 ON + 2 PN'],
    ENGI11: [5, '1 ON + 4 PN'], IGTI11: [3, '1 ON + 2 PN'], KLBN11: [5, '1 ON + 4 PN'], RNEW11: [3, '1 ON + 2 PN'],
    SANB11: [2, '1 ON + 1 PN'], SAPR11: [5, '1 ON + 4 PN'], TAEE11: [3, '1 ON + 2 PN'],
  };
  const acoesPorUnit = (t) => (UNITS[t] ? UNITS[t][0] : 1);

  function classeTicker(t) {
    const n = t.replace(/^[^0-9]+/, '');
    if (['4', '5', '6', '7', '8'].includes(n)) return 'PN';
    if (n === '11') return 'UNIT';
    return 'ON';
  }

  const FATORES_COMUNS = [0.01, 0.02, 0.05, 0.1, 0.125, 0.2, 0.25, 1 / 3, 0.5, 1.1, 1.15, 1.2, 1.25, 1.3, 1.5, 2, 2.5, 3, 4, 5, 6, 8, 10, 20, 50, 100];

  // fator acumulado dos eventos com data em (de, ate]
  function fatorEntre(splits, de, ate) {
    let x = 1;
    for (const s of splits) if (s.data > de && s.data <= ate) x *= s.fator;
    return x;
  }

  // mediana no mesmo critério de ficha.go: com quantidade par, pega o central superior
  function mediana(x) {
    if (!x.length) return 0;
    const y = x.slice().sort((a, b) => a - b);
    return y[Math.floor(y.length / 2)];
  }
  const ultimos = (x, n) => (x.length <= n ? x : x.slice(x.length - n));

  // desdobramentos/grupamentos/bonificações pela quebra entre o fechamento e o ajustado
  function detectarSplits(pts) {
    const out = [];
    for (let i = 1; i < pts.length; i++) {
      const a = pts[i - 1], b = pts[i];
      if (a.close <= 0 || b.close <= 0 || a.adj <= 0 || b.adj <= 0) continue;
      const fa = a.adj / a.close, fb = b.adj / b.close;
      const dv = 1 - b.div / a.close;
      if (dv <= 0.3) continue;
      const q = dv / (fa / fb);
      if (q > 0.93 && q < 1.08) continue;
      let best = 0, bd = 1;
      for (const c of FATORES_COMUNS) {
        const d = Math.abs(q / c - 1);
        if (d < bd) { best = c; bd = d; }
      }
      if (bd < 0.05) {
        if (best === 1 / 3) best = Math.round(best * 1000) / 1000;
        out.push({ data: b.data, fator: best });
      }
    }
    return out;
  }

  function listaAnos(a) {
    if (!a.length) return '';
    const out = [];
    let ini = a[0], ant = a[0];
    const flush = () => out.push(ini === ant ? String(ini) : ini + '–' + ant);
    for (const x of a.slice(1)) {
      if (x === ant + 1) { ant = x; continue; }
      flush();
      ini = x; ant = x;
    }
    flush();
    return out.join(', ');
  }

  // códigos de um nível, em ordem numérica
  const codsProf = (m, p) => (m ? Object.keys(m).filter((c) => prof(c) === p).sort(codLess) : []);

  // número de ações implícito no lucro por ação (lucro dos controladores ÷ LPA)
  function acoesDe(r) {
    if (r.lpa == null || r.lucro_ctrl == null || Math.abs(r.lpa) < 0.005 || r.lucro_ctrl === 0) return null;
    const a = r.lucro_ctrl / r.lpa;
    return a <= 0 ? null : fp(a);
  }

  // LPA da classe da ação: ON, PN ou o total quando a DRE não abre por classe
  function lpaClasse(dre, nomes, classe) {
    if (!dre) return null;
    let on = null, pn = null;
    for (const c of Object.keys(dre)) {
      if (!c.startsWith('3.99.01.')) continue;
      const n = String(nomes[c] || '').trim().toUpperCase();
      const v = dre[c];
      if (n.startsWith('ON') || n.includes('ORDIN')) { if (on == null || (on === 0 && v !== 0)) on = v; }
      else if (n.startsWith('PN') || n.includes('PREFER')) { if (pn == null || (pn === 0 && v !== 0)) pn = v; }
    }
    if (on == null && dre['3.99.01.01'] !== undefined) on = dre['3.99.01.01'];
    if (classe === 'PN' && pn != null && pn !== 0) return fp(pn);
    if (on != null && on !== 0) return fp(on);
    if (pn != null && pn !== 0) return fp(pn);
    for (const c of ['3.99.01', '3.99']) {
      if (dre[c] !== undefined && dre[c] !== 0) return fp(dre[c]);
    }
    return null;
  }

  // Monta a ficha. d: {dre, bp, dfc, nomesDR, nomesBP, baseAno, tri, capital}; serie: {pontos, splits, fonte}
  function montarFicha(e, d, ticker, serie) {
    const fin = String(e.tipo || '').toLowerCase().startsWith('financ');
    const f = {
      ticker, tickers: e.tickers, empresa: e.nome, nome_curto: e.nome_curto, cnpj: e.cnpj, cod_cvm: e.cod_cvm,
      setor: e.setor, tipo: e.tipo, financeira: fin, ativa: e.ativa, classe: classeTicker(ticker),
      ult_dfp: e.ult_dfp, ult_tri: e.ult_tri, periodo_12m: '', rotulo_ebit: 'EBIT', rotulo_bruto: 'Lucro bruto',
      rotulo_receita: 'Receita líquida', anual: [], trimestres: [], cotacoes: [], splits: null, atual: {}, pl_media: null,
      notas: [], gerado_em: '', contas: {}, fonte: '',
    };
    const agora = new Date();
    const p2 = (n) => String(n).padStart(2, '0');
    f.gerado_em = `${p2(agora.getDate())}/${p2(agora.getMonth() + 1)}/${agora.getFullYear()} ${p2(agora.getHours())}:${p2(agora.getMinutes())}`;
    if (fin) {
      f.rotulo_receita = 'Receitas de intermediação';
      f.rotulo_bruto = 'Resultado bruto';
      f.rotulo_ebit = 'Resultado antes dos tributos';
    }

    const anos = Object.keys(d.dre).map((k) => parseInt(k, 10)).filter((n) => !Number.isNaN(n)).sort((a, b) => a - b);

    // splits e cotações
    let pts = [];
    if (serie) {
      pts = serie.pontos;
      f.splits = serie.splits;
      if (f.splits == null) f.splits = detectarSplits(pts);
      f.fonte_cotacao = serie.fonte || undefined;
    }
    const splits = f.splits || [];
    // splits depois da data
    const fatorApos = (data) => {
      let x = 1;
      for (const s of splits) if (s.data > data) x *= s.fator;
      return x;
    };
    const precoFimAno = (ano) => {
      let best = null;
      for (const p of pts) if (p.data.startsWith(ano + '-')) best = p;
      if (!best) return null;
      return { preco: best.close / fatorApos(best.data), data: best.data };
    };

    // rendimento de cada provento, deduzido do próprio fechamento ajustado
    const yld = new Array(pts.length).fill(0); // fração do preço
    const provA = new Array(pts.length).fill(0); // R$ por ação na base atual
    for (let i = 1; i < pts.length; i++) {
      const a = pts[i - 1], b = pts[i];
      if (a.close <= 0 || b.close <= 0 || a.adj <= 0 || b.adj <= 0) continue;
      let k = 1;
      for (const sp of splits) if (sp.data === b.data) k = sp.fator;
      let y = 1 - (a.adj / a.close) / (b.adj / b.close) * k;
      if (b.div <= 0 && y < 0.001) continue;
      if (y < 0 || y > 0.5) {
        if (b.div <= 0) continue;
        y = b.div / a.close;
      }
      yld[i] = y;
      provA[i] = y * a.close / fatorApos(a.data);
    }
    const provDataCom = (ano) => {
      let s = 0, y = 0, ok = false;
      pts.forEach((p, i) => {
        if (p.data.startsWith(ano + '-')) { ok = true; s += provA[i]; y += yld[i]; }
      });
      return ok ? { s, y } : null;
    };

    // ---- linhas anuais ----
    let rows = [];
    for (const ano of anos) {
      const k = String(ano);
      const dre = d.dre[k], bp = d.bp[k];
      const nDR = d.nomesDR[k] || {}, nBP = d.nomesBP[k] || {};
      const r = {
        ano, rotulo: k, parcial: false, base: d.baseAno[k] || '', receita: null, cresc_receita: null, lucro_bruto: null,
        ebit: null, lucro: null, lucro_ctrl: null, cresc_lucro: null, margem_bruta: null, margem_ebit: null, margem_liq: null,
        roe: null, lpa: null, acoes_mi: null, ativo: null, pl: null, pl_ctrl: null, caixa: null, div_bruta: null,
        div_liq: null, div_ebitda: null, ebitda: null, ev_ebitda: null, roa: null, fco: null, capex: null, fcl: null,
        fcl_yield: null, proventos: null, recompras: null, payout: null, prov_acao: null, preco: null, preco_data: '',
        p_l: null, p_vp: null, dy: null, dy_data_com: null, prov_data_com: null, valor_mercado_bi: null,
      };
      const get = (m, c) => (m && m[c] !== undefined ? fp(m[c]) : null);
      r.receita = get(dre, '3.01');
      r.lucro_bruto = get(dre, '3.03');
      // EBIT / resultado antes dos tributos
      for (const c of codsProf(dre, 2)) {
        const n = norm(nDR[c]);
        if ((!fin && RE.ebit.test(n)) || (fin && RE.preTax.test(n))) {
          r.ebit = get(dre, c);
          f.contas.ebit = c + ' ' + (nDR[c] || '');
          break;
        }
      }
      if (r.ebit == null && !fin) r.ebit = get(dre, '3.05');
      // lucro líquido
      let lc = '';
      if (dre['3.11'] !== undefined && RE.lucro.test(norm(nDR['3.11']))) {
        lc = '3.11';
      } else {
        for (const c of codsProf(dre, 2)) {
          const n = norm(nDR[c]);
          if (RE.lucro.test(n) && !n.includes('antes') && !n.includes('continuadas')) lc = c;
        }
      }
      if (lc !== '') {
        r.lucro = get(dre, lc);
        r.lucro_ctrl = r.lucro;
        for (const c of Object.keys(dre)) {
          if (c.startsWith(lc + '.') && prof(c) === prof(lc) + 1) {
            const n = norm(nDR[c]);
            if (n.includes('controlador') && !n.includes('nao controlador')) {
              const v = get(dre, c);
              if (v != null && (v !== 0 || r.lucro == null || r.lucro === 0)) r.lucro_ctrl = v;
            }
          }
        }
        f.contas.lucro = lc + ' ' + (nDR[lc] || '');
      }
      r.lpa = lpaClasse(dre, nDR, f.classe);
      // balanço
      r.ativo = get(bp, '1');
      for (const c of codsProf(bp, 2)) {
        if (c.startsWith('2.') && norm(nBP[c]).startsWith('patrimonio liquido')) {
          r.pl = get(bp, c);
          r.pl_ctrl = r.pl;
          let nc = null;
          for (const c2 of codsProf(bp, 3)) {
            if (!c2.startsWith(c + '.')) continue;
            const n = norm(nBP[c2]);
            if (n.includes('nao controlador') || n.includes('minoritar')) {
              nc = get(bp, c2);
            } else if (n.includes('controlador')) {
              r.pl_ctrl = get(bp, c2);
              nc = null;
              break;
            }
          }
          if (nc != null) r.pl_ctrl = sub(r.pl, nc);
          break;
        }
      }
      if (!fin) {
        let cx = 0, db = 0, tc = false, td = false;
        for (const c of codsProf(bp, 3)) {
          const n = norm(nBP[c]);
          if (c.startsWith('1.01.') && (n.startsWith('caixa e equivalentes') || n.startsWith('aplicacoes financeiras'))) {
            cx += bp[c]; tc = true;
          }
          if ((c.startsWith('2.01.') || c.startsWith('2.02.')) && n.startsWith('emprestimos e financiamentos')) {
            db += bp[c]; td = true;
          }
        }
        if (tc) r.caixa = fp(cx);
        if (td) r.div_bruta = fp(db);
        if (tc && td) r.div_liq = fp(db - cx);
      }
      // fluxo de caixa
      const cs = d.dfc[k];
      if (cs) {
        let capex = 0, prov = 0, rec = 0, tcap = false, tprov = false;
        for (const ct of cs) {
          const n = norm(ct.nome);
          if (ct.cod === '6.01') {
            r.fco = fp(ct.v);
          } else if (ct.cod.startsWith('6.02.') && prof(ct.cod) === 3) {
            if (RE.capex.test(n) && !RE.naoCapex.test(n) && ct.v < 0) { capex += -ct.v; tcap = true; }
          } else if (ct.cod.startsWith('6.03.') && prof(ct.cod) === 3) {
            if (RE.prov.test(n) && !RE.naoProv.test(n)) {
              if (ct.v < 0) prov += -ct.v;
              tprov = true;
            } else if (RE.recompra.test(n) && ct.v < 0) {
              rec += -ct.v;
            }
          }
        }
        if (tcap || !fin) r.capex = fp(capex);
        r.proventos = tprov ? fp(prov) : fp(0);
        r.recompras = fp(rec);
      }
      rows.push(r);
    }

    // LPA informado por lote de mil ações: corrige pela mediana
    const ref = [];
    for (const r of rows) { const a = acoesDe(r); if (a != null) ref.push(a); }
    const med = mediana(ultimos(ref, 3));
    for (const r of rows) {
      const a = acoesDe(r);
      if (a != null && med > 0) {
        const q = a / med;
        if (q > 300) r.lpa *= 1000;
        else if (q < 1 / 300) r.lpa /= 1000;
      }
    }

    // ações em circulação (milhões, na base atual)
    const capDe = (ano) => {
      let best = '';
      for (const dt of Object.keys(d.capital)) {
        if (dt.startsWith(ano + '-') && dt > best) best = dt;
      }
      if (best === '') return null;
      const c = d.capital[best];
      return { v: c.total - c.tes_total, dt: best, ok: c.total > 0 };
    };
    // a quantidade às vezes vem em milhares: escolhe a leitura (unidades, milhares ou milhões) mais próxima
    const perto = (c, alvo) => {
      let best = 1, bd = Infinity;
      for (const e2 of [1, 1e3, 1e6]) {
        const dd = Math.abs(Math.log(c * e2 / 1e6 / alvo));
        if (dd < bd) { best = e2; bd = dd; }
      }
      return best;
    };
    const escAno = {};
    for (const r of rows) {
      const c = capDe(r.ano);
      if (c && c.ok) {
        const a = acoesDe(r);
        if (a != null) escAno[r.ano] = perto(c.v, a);
      }
    }
    for (let pass = 0; pass < 2; pass++) {
      rows.forEach((r, i) => {
        const c = capDe(r.ano);
        if (!c || !c.ok) return;
        if (escAno[r.ano] !== undefined) return;
        for (let dlt = 1; dlt < rows.length; dlt++) {
          for (const j of [i - dlt, i + dlt]) {
            if (j < 0 || j >= rows.length) continue;
            const e2 = escAno[rows[j].ano];
            if (e2 !== undefined) {
              const cj = capDe(rows[j].ano);
              escAno[r.ano] = perto(c.v, cj.v * e2 / 1e6);
              break;
            }
          }
          if (escAno[r.ano] !== undefined) break;
        }
        if (escAno[r.ano] === undefined && pass === 1) {
          escAno[r.ano] = c.v < 1e4 ? 1e6 : c.v < 5e6 ? 1000 : 1;
        }
      });
    }
    let fonteAcoes = '';
    for (const r of rows) {
      const c = capDe(r.ano);
      if (c && c.ok) {
        r.acoes_mi = fp(c.v * (escAno[r.ano] || 0) / 1e6 * fatorApos(c.dt));
        fonteAcoes = 'capital';
      } else {
        const a = acoesDe(r);
        if (a != null) {
          r.acoes_mi = fp(a * fatorApos((r.ano + 1) + '-03-31'));
          if (fonteAcoes === '') fonteAcoes = 'lpa';
        }
      }
    }
    f.contas.acoes = fonteAcoes;
    // ações faltantes ficam com as do ano mais próximo
    rows.forEach((r, i) => {
      if (r.acoes_mi != null) return;
      for (let dlt = 1; dlt < rows.length; dlt++) {
        if (i - dlt >= 0 && rows[i - dlt].acoes_mi != null && !rows[i - dlt].parcial) { r.acoes_mi = rows[i - dlt].acoes_mi; break; }
        if (i + dlt < rows.length && rows[i + dlt].acoes_mi != null) { r.acoes_mi = rows[i + dlt].acoes_mi; break; }
      }
    });
    const nUnit = acoesPorUnit(ticker);
    if (nUnit > 1) {
      for (const r of rows) {
        if (r.acoes_mi != null) r.acoes_mi = fp(r.acoes_mi / nUnit);
        if (r.lpa != null) r.lpa = fp(r.lpa * nUnit);
      }
      f.atual.acoes_por_unit = nUnit;
      f.notas.push(`${ticker} é uma unit (${UNITS[ticker][1]}): quantidade de ações, lucro e proventos por ação e valor de mercado estão em units equivalentes (total de ações ÷ ${nUnit}).`);
    }
    let acRef = 0;
    for (let i = rows.length - 1; i >= 0; i--) if (rows[i].acoes_mi != null) { acRef = rows[i].acoes_mi; break; }
    const anosInconsist = [];
    rows.forEach((r, i) => {
      r.margem_bruta = div(r.lucro_bruto, r.receita);
      r.margem_ebit = div(r.ebit, r.receita);
      r.margem_liq = div(r.lucro, r.receita);
      if (i > 0 && rows[i - 1].ano === r.ano - 1) {
        const p = rows[i - 1];
        r.cresc_receita = cresc(r.receita, p.receita);
        r.cresc_lucro = cresc(r.lucro_ctrl, p.lucro_ctrl);
        if (r.lucro_ctrl != null && r.pl_ctrl != null && p.pl_ctrl != null) {
          r.roe = fp(r.lucro_ctrl / ((r.pl_ctrl + p.pl_ctrl) / 2));
        }
      } else if (r.lucro_ctrl != null && r.pl_ctrl != null) {
        r.roe = div(r.lucro_ctrl, r.pl_ctrl);
      }
      if (r.proventos != null && r.lucro_ctrl != null && r.lucro_ctrl > 0) r.payout = fp(r.proventos / r.lucro_ctrl);
      let ac = r.acoes_mi;
      if (ac != null && acRef > 0 && (ac / acRef > 2.5 || ac / acRef < 0.4)) {
        // reorganização societária ou outra companhia listada: valores por ação não comparáveis
        ac = null;
        anosInconsist.push(r.ano);
      }
      if (r.lpa == null && ac != null && r.lucro_ctrl != null && ac > 0) r.lpa = fp(r.lucro_ctrl / ac);
      if (r.proventos != null && ac != null && ac > 0) r.prov_acao = fp(r.proventos / ac);
      const pf = precoFimAno(r.ano);
      if (pf) {
        r.preco = fp(pf.preco);
        r.preco_data = pf.data;
        if (ac != null) {
          const vm = ac * pf.preco; // R$ mi
          r.valor_mercado_bi = fp(vm / 1000);
          if (r.lucro_ctrl != null && r.lucro_ctrl !== 0) r.p_l = fp(vm / r.lucro_ctrl);
          if (r.pl_ctrl != null && r.pl_ctrl > 0) r.p_vp = fp(vm / r.pl_ctrl);
          if (r.proventos != null && vm > 0) r.dy = fp(r.proventos / vm);
        }
        const pdc = provDataCom(r.ano);
        if (pdc) {
          r.prov_data_com = fp(pdc.s);
          r.dy_data_com = fp(pdc.y);
        }
      }
    });

    // dívida líquida / EBITDA anual (4º trimestre)
    const triPorAno = {};
    for (const t of d.tri) if (t.tri && t.tri.startsWith('4T')) triPorAno[t.ano] = t;
    for (const r of rows) {
      const t = triPorAno[r.ano];
      if (t && !fin) r.div_ebitda = fp(t.div_ebitda_12m);
    }

    // ---- trimestres e 12 meses ----
    const tris = d.tri;
    const last = tris.length ? tris[tris.length - 1] : null;
    const val = (x) => (x == null ? null : x);
    if (last) {
      f.periodo_12m = '12 meses até o ' + last.tri;
      const soma = (get) => {
        if (tris.length < 4) return null;
        let s = 0;
        for (const t of tris.slice(tris.length - 4)) {
          const v = get(t);
          if (v == null) return null;
          s += v;
        }
        return fp(s);
      };
      let rec12 = val(last.receita_12m);
      if (rec12 == null) rec12 = soma((t) => t.receita);
      let lb12 = soma((t) => t.lucro_bruto);
      let eb12 = soma((t) => t.ebit);
      let lu12 = val(last.lucro_12m);
      if (lu12 == null) lu12 = soma((t) => t.lucro);
      let lc12 = soma((t) => t.lucro_ctrl);
      if (lc12 == null) lc12 = lu12;
      if (fin && rows.length > 0) { eb12 = null; lb12 = null; }
      f.atual.doze_meses = {
        periodo: f.periodo_12m, receita: rec12, lucro_bruto: lb12, ebit: eb12, lucro: lu12, lucro_ctrl: lc12,
        margem_bruta: div(lb12, rec12), margem_ebit: div(eb12, rec12), margem_liq: div(lu12, rec12), roe: val(last.roe_12m),
        ebitda: val(last.ebitda_12m), div_liq: val(last.div_liq), div_ebitda: val(last.div_ebitda_12m), pl: val(last.pl),
        fco: val(last.fco_12m),
      };
      // linha parcial no histórico anual
      if (triKey(last.ano, last.tri) > triKey(e.ult_dfp, '4T') && rec12 != null) {
        const r = {
          ano: last.ano, rotulo: last.ano + '*', parcial: true, base: '', receita: rec12, lucro_bruto: lb12, ebit: eb12,
          lucro: lu12, lucro_ctrl: lc12, pl: val(last.pl), pl_ctrl: val(last.pl), ativo: val(last.ativo), caixa: val(last.caixa),
          div_bruta: val(last.div_bruta), div_liq: val(last.div_liq), fco: val(last.fco_12m), roe: val(last.roe_12m),
          cresc_receita: null, cresc_lucro: null, lpa: null, acoes_mi: null, div_ebitda: null, preco: null, preco_data: '',
          p_l: null, p_vp: null, dy: null, dy_data_com: null, prov_data_com: null, valor_mercado_bi: null, proventos: null,
          recompras: null, payout: null, prov_acao: null, ebitda: null, ev_ebitda: null, roa: null, fcl: null, fcl_yield: null,
          capex: null,
        };
        if (!fin) r.div_ebitda = val(last.div_ebitda_12m);
        r.margem_bruta = div(lb12, rec12);
        r.margem_ebit = div(eb12, rec12);
        r.margem_liq = div(lu12, rec12);
        if (rows.length) {
          const p = rows[rows.length - 1];
          r.cresc_receita = cresc(rec12, p.receita);
          r.cresc_lucro = cresc(lc12, p.lucro_ctrl);
          r.acoes_mi = p.acoes_mi;
        }
        rows.push(r);
      }
    }

    // cotação atual, P/L 12m e DY 12m
    if (pts.length > 0) {
      const lp = pts[pts.length - 1];
      f.atual.cotacao = lp.close;
      f.atual.cotacao_data = lp.data;
      const lpDate = new Date(lp.data + 'T00:00:00Z');
      const corte = new Date(Date.UTC(lpDate.getUTCFullYear() - 1, lpDate.getUTCMonth(), lpDate.getUTCDate()));
      let s12 = 0, y12 = 0;
      pts.forEach((p, i) => {
        const t = new Date(p.data + 'T00:00:00Z');
        if (t > corte) { s12 += provA[i]; y12 += yld[i]; }
      });
      f.atual.proventos_12m_acao = s12;
      f.atual.dy_12m = y12;
      if (pts.length > 12) {
        const ref = pts[pts.length - 13];
        f.atual.var_12m = lp.close / (ref.close / fatorEntre(splits, ref.data, lp.data)) - 1;
      }
      // ações hoje: composição do capital mais recente (ITR) ou a do último ano fechado
      let acoesHoje = null, lucro12 = null;
      if (f.atual.doze_meses) lucro12 = f.atual.doze_meses.lucro_ctrl;
      let ultCap = '';
      for (const dt of Object.keys(d.capital)) if (dt > ultCap) ultCap = dt;
      if (ultCap !== '') {
        const c = d.capital[ultCap];
        const v = c.total - c.tes_total;
        let ef = 1;
        for (let i = rows.length - 1; i >= 0; i--) {
          if (!rows[i].parcial && rows[i].acoes_mi != null) { ef = perto(v, rows[i].acoes_mi / fatorApos(ultCap)); break; }
        }
        acoesHoje = fp(v * ef / 1e6 * fatorApos(ultCap) / acoesPorUnit(ticker));
        f.atual.acoes_data = ultCap;
      }
      for (let i = rows.length - 1; i >= 0; i--) {
        if (!rows[i].parcial && rows[i].acoes_mi != null) {
          if (acoesHoje == null) acoesHoje = rows[i].acoes_mi;
          if (lucro12 == null) lucro12 = rows[i].lucro_ctrl;
          break;
        }
      }
      if (acoesHoje != null) {
        const vm = acoesHoje * lp.close;
        f.atual.acoes_mi = acoesHoje;
        f.atual.valor_mercado_bi = vm / 1000;
        if (lucro12 != null && Math.abs(lucro12) >= 0.1) { // perto de zero, o P/L explode sem significado
          f.atual.p_l = vm / lucro12;
          f.atual.lpa_12m = lucro12 / acoesHoje;
        }
        if (last && last.pl != null && last.pl > 0) f.atual.p_vp = vm / last.pl;
        const ult = rows[rows.length - 1];
        if (rows.length && ult.parcial) {
          ult.preco = fp(lp.close);
          ult.preco_data = lp.data;
          ult.valor_mercado_bi = fp(vm / 1000);
          ult.acoes_mi = acoesHoje;
          if (lucro12 != null && Math.abs(lucro12) >= 0.1) {
            ult.p_l = fp(vm / lucro12);
            ult.lpa = fp(lucro12 / acoesHoje);
          }
          if (ult.pl != null && ult.pl > 0) ult.p_vp = fp(vm / ult.pl);
          ult.dy_data_com = fp(y12);
          ult.prov_data_com = fp(s12);
        }
      }
      f.cotacoes = pts.map((p, i) => ({ mes: p.data.slice(0, 7), data: p.data, fechamento: p.close / fatorApos(p.data), ajustado: p.adj, provento: provA[i] }));
    }
    // P/L médio dos anos fechados com lucro
    const vals = [];
    for (const r of rows) if (!r.parcial && r.p_l != null && r.p_l > 0 && r.p_l < 100) vals.push(r.p_l);
    const md = mediana(vals);
    let sm = 0, n = 0;
    for (const v of vals) if (v <= md * 3 && v >= md / 3) { sm += v; n++; }
    if (n > 0) f.pl_media = fp(sm / n);

    indicadoresExtras(f, rows, tris, fin);
    f.anual = rows;

    // últimos 12 trimestres
    const idx = {};
    for (const t of tris) idx[triKey(t.ano, t.tri)] = t;
    const ini = Math.max(0, tris.length - 12);
    f.trimestres = tris.slice(ini).map((t) => {
      const o = {};
      for (const c of TRI_CAMPOS) o[c] = t[c] === undefined ? null : t[c];
      const p = idx[triKey(t.ano - 1, t.tri)];
      o.var_receita = p ? cresc(t.receita, p.receita) : null;
      o.var_lucro = p ? cresc(t.lucro_ctrl, p.lucro_ctrl) : null;
      return o;
    });

    // notas
    const bases = {};
    for (const r of rows) if (r.base) bases[r.base] = (bases[r.base] || 0) + 1;
    if (bases.individual > 0 && bases.consolidado > 0) {
      f.notas.push('Alguns anos só têm demonstrações individuais (sem consolidado) e foram usados assim.');
    } else if (bases.individual > 0) {
      f.notas.push('A companhia publica só demonstrações individuais; os números são da controladora.');
    }
    for (const sp of splits) {
      let tipo = 'desdobramento';
      if (sp.fator < 1) tipo = 'grupamento';
      else if (sp.fator < 1.9) tipo = 'bonificação';
      const fs = String(sp.fator).replace('.', ',');
      f.notas.push(`${tipo[0].toUpperCase() + tipo.slice(1)} de ${fs} detectado em ${sp.data.slice(5, 7)}/${sp.data.slice(0, 4)} (fator ${fs}): preços e valores por ação anteriores foram ajustados para a base atual.`);
    }
    if (anosInconsist.length) {
      f.notas.push(`Em ${listaAnos(anosInconsist)} o número de ações implícito no lucro por ação é muito diferente do atual (reorganização societária, incorporação ou outra companhia listada); P/L, valor de mercado, P/VP e yield desses anos foram omitidos.`);
    }
    if (fin) {
      f.notas.push('Instituição financeira: receita = receitas de intermediação financeira; margem bruta = resultado bruto da intermediação ÷ receita; não há EBITDA nem dívida líquida.');
    }
    return f;
  }

  // EBITDA, EV/EBITDA, ROA, fluxo de caixa livre e CAGR, a partir do que a ficha já calculou
  function indicadoresExtras(f, rows, tris, fin) {
    const t4 = {}, triIdx = {};
    for (const t of tris) {
      if (t.tri && t.tri.startsWith('4T')) t4[t.ano] = t;
      triIdx[triKey(t.ano, t.tri)] = t;
    }
    const last = tris.length ? tris[tris.length - 1] : null;
    // valor da firma = valor de mercado + dívida líquida; só vale com EBITDA positivo
    const ev = (vmBi, divLiq, ebitda) => {
      if (vmBi == null || divLiq == null || ebitda == null || ebitda <= 0) return null;
      return fp((vmBi * 1000 + divLiq) / ebitda);
    };
    const media = (a, b) => (a == null || b == null ? a : fp((a + b) / 2));
    rows.forEach((r, i) => {
      let ativoAnt = null;
      if (r.parcial && last) {
        const t = triIdx[triKey(last.ano - 1, last.tri)];
        if (t) ativoAnt = t.ativo;
      } else if (i > 0 && rows[i - 1].ano === r.ano - 1 && !rows[i - 1].parcial) {
        ativoAnt = rows[i - 1].ativo;
      }
      r.roa = div(r.lucro, media(r.ativo, ativoAnt));
      if (fin) return;
      if (r.parcial) {
        if (last) r.ebitda = fp(last.ebitda_12m);
      } else if (t4[r.ano]) {
        r.ebitda = fp(t4[r.ano].ebitda_12m);
      }
      r.ev_ebitda = ev(r.valor_mercado_bi, r.div_liq, r.ebitda);
      if (r.capex != null) r.fcl = sub(r.fco, r.capex);
      if (r.fcl != null && r.valor_mercado_bi != null && r.valor_mercado_bi > 0) {
        r.fcl_yield = fp(r.fcl / (r.valor_mercado_bi * 1000));
      }
    });

    // 12 meses e valor de mercado atual
    const m = f.atual.doze_meses;
    if (m && last) {
      let ativoAnt = null;
      const t = triIdx[triKey(last.ano - 1, last.tri)];
      if (t) ativoAnt = t.ativo;
      m.roa = div(m.lucro, media(last.ativo, ativoAnt));
    }
    if (!fin) {
      const vm = f.atual.valor_mercado_bi || 0;
      if (vm > 0) {
        if (last) f.atual.ev_ebitda = ev(vm, last.div_liq, fp(last.ebitda_12m));
        // FCL do último ano fechado sobre o valor de mercado de hoje
        for (let i = rows.length - 1; i >= 0; i--) {
          const r = rows[i];
          if (!r.parcial && r.fcl != null) {
            f.atual.fcl_yield = fp(r.fcl / (vm * 1000));
            f.atual.fcl_ano = r.ano;
            f.atual.fcl = r.fcl;
            break;
          }
        }
      }
    }

    // CAGR de receita e lucro dos controladores, entre anos fechados
    const fech = {};
    let fim = 0;
    for (const r of rows) {
      if (!r.parcial) {
        fech[r.ano] = r;
        if (r.ano > fim) fim = r.ano;
      }
    }
    const cagr = (get, anos) => {
      const a = fech[fim - anos], b = fech[fim];
      if (!a || !b) return null;
      const va = get(a), vb = get(b);
      if (va == null || vb == null || va <= 0 || vb <= 0) return null;
      return fp(Math.pow(vb / va, 1 / anos) - 1);
    };
    f.atual.cagr = {
      ano_fim: fim,
      receita_5a: cagr((r) => r.receita, 5), receita_10a: cagr((r) => r.receita, 10),
      lucro_5a: cagr((r) => r.lucro_ctrl, 5), lucro_10a: cagr((r) => r.lucro_ctrl, 10),
    };
  }

  const API = { montarFicha, detectarSplits, codLess, triKey, norm, mediana, fp, div, sub, cresc, acoesPorUnit, classeTicker, fatorEntre };
  if (typeof module !== 'undefined' && module.exports) module.exports = API;
  else raiz.FichaCalc = API;
})(typeof window !== 'undefined' ? window : globalThis);
