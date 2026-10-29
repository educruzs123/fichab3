/* Gráficos SVG simples, sem dependências. */
(function () {
  const NS = 'http://www.w3.org/2000/svg';
  const tip = () => document.getElementById('tip');
  const renders = new Map();
  let rt = null;
  window.addEventListener('resize', () => { clearTimeout(rt); rt = setTimeout(() => renders.forEach((fn, el) => { if (document.body.contains(el)) fn(); else renders.delete(el); }), 150); });

  function s(tag, attrs, parent) {
    const e = document.createElementNS(NS, tag);
    for (const k in attrs) if (attrs[k] != null) e.setAttribute(k, attrs[k]);
    if (parent) parent.appendChild(e);
    return e;
  }
  function txt(parent, x, y, t, attrs) { const e = s('text', Object.assign({ x, y }, attrs || {}), parent); e.textContent = t; return e; }

  function niceTicks(min, max, n) {
    if (min === max) { max = min + 1; }
    const span = max - min, step0 = span / Math.max(1, n);
    const mag = Math.pow(10, Math.floor(Math.log10(step0)));
    const r = step0 / mag;
    const step = (r >= 5 ? 10 : r >= 2 ? 5 : r >= 1 ? 2 : 1) * mag;
    const lo = Math.floor(min / step) * step, hi = Math.ceil(max / step) * step;
    const out = [];
    for (let v = lo; v <= hi + step / 2; v += step) out.push(+v.toFixed(10));
    return out;
  }
  function escala(d0, d1, r0, r1) { const k = (r1 - r0) / ((d1 - d0) || 1); return v => r0 + (v - d0) * k; }
  function medir(svg, t) { const e = txt(svg, -999, -999, t); const w = e.getComputedTextLength(); e.remove(); return w; }

  function mostrarTip(ev, titulo, linhas) {
    const t = tip(); t.textContent = '';
    if (titulo) { const h = document.createElement('div'); h.className = 't'; h.textContent = titulo; t.appendChild(h); }
    linhas.forEach(([l, v, cor]) => {
      if (v == null || v === '') return;
      const d = document.createElement('div'); d.className = 'l';
      const a = document.createElement('span');
      if (cor) { const i = document.createElement('i'); i.style.cssText = 'display:inline-block;width:8px;height:8px;border-radius:2px;margin-right:6px;background:' + cor; a.appendChild(i); }
      a.appendChild(document.createTextNode(l));
      const b = document.createElement('span'); b.textContent = v;
      d.appendChild(a); d.appendChild(b); t.appendChild(d);
    });
    t.style.display = 'block';
    let x = ev.clientX + 14, y = ev.clientY + 14;
    const w = t.offsetWidth, h = t.offsetHeight;
    if (x + w > window.innerWidth - 8) x = ev.clientX - w - 14;
    if (y + h > window.innerHeight - 8) y = ev.clientY - h - 14;
    t.style.left = Math.max(4, x) + 'px'; t.style.top = Math.max(4, y) + 'px';
  }
  function esconderTip() { tip().style.display = 'none'; }

  function legenda(el, series) {
    const lg = document.createElement('div'); lg.className = 'leg';
    series.filter(x => x.nome).forEach(x => { const sp = document.createElement('span'); const i = document.createElement('i'); i.style.background = x.cor; if (x.tipo === 'linha') { i.style.height = '3px'; i.style.verticalAlign = '3px'; } sp.appendChild(i); sp.appendChild(document.createTextNode(x.nome)); lg.appendChild(sp); });
    el.appendChild(lg);
  }

  /* Categorias (anos): barras e/ou linhas, eixo esquerdo e direito. */
  function categorias(el, o) {
    const fn = () => {
      el.textContent = '';
      const cats = o.cats || [];
      const temDado = o.series.some(x => x.vals.some(v => v != null && isFinite(v)));
      if (!cats.length || !temDado) { const d = document.createElement('div'); d.className = 'vazio'; d.textContent = o.vazio || 'Sem dados para este gráfico.'; el.appendChild(d); return; }
      if (o.legenda !== false && o.series.length > 1) legenda(el, o.series);
      const W = Math.max(300, el.clientWidth), H = o.altura || 280;
      const svg = s('svg', { viewBox: `0 0 ${W} ${H}`, width: '100%', role: 'img', 'aria-label': o.aria || '' }, el);
      const esq = o.series.filter(x => x.eixo !== 'dir'), dir = o.series.filter(x => x.eixo === 'dir');
      const ext = (ss, extra) => { let mn = Infinity, mx = -Infinity; ss.forEach(x => x.vals.forEach(v => { if (v != null && isFinite(v)) { mn = Math.min(mn, v); mx = Math.max(mx, v); } })); (extra || []).forEach(v => { if (v != null) { mn = Math.min(mn, v); mx = Math.max(mx, v); } }); if (!isFinite(mn)) { mn = 0; mx = 1; } if (ss.some(x => x.tipo !== 'linha') || o.zero !== false) { mn = Math.min(0, mn); mx = Math.max(0, mx); } return [mn, mx]; };
      const refE = (o.refs || []).filter(r => r.eixo !== 'dir').map(r => r.valor), refD = (o.refs || []).filter(r => r.eixo === 'dir').map(r => r.valor);
      const [e0, e1] = ext(esq, refE), tE = niceTicks(e0, e1, 5);
      const fE = o.fmtEsq || (v => v), fD = o.fmtDir || (v => v);
      let tD = null;
      if (dir.length) { const [d0, d1] = ext(dir, refD); tD = niceTicks(d0, d1, 5); }
      const mt = o.rotulos ? 34 : 14, mb = 26;
      const ml = Math.max(...tE.map(v => medir(svg, fE(v)))) + 12;
      const mr = tD ? Math.max(...tD.map(v => medir(svg, fD(v)))) + 12 : 8;
      const yE = escala(tE[0], tE[tE.length - 1], H - mb, mt);
      const yD = tD ? escala(tD[0], tD[tD.length - 1], H - mb, mt) : null;
      const n = cats.length, step = (W - ml - mr) / n;
      const cx = i => ml + step * (i + .5);
      const g = s('g', {}, svg);
      tE.forEach(v => { s('line', { x1: ml, x2: W - mr, y1: yE(v), y2: yE(v), stroke: v === 0 ? 'var(--eixo)' : 'var(--grade)', 'stroke-width': 1 }, g); txt(g, ml - 6, yE(v) + 4, fE(v), { 'text-anchor': 'end' }); });
      if (tD) tD.forEach(v => txt(g, W - mr + 6, yD(v) + 4, fD(v), { 'text-anchor': 'start' }));
      (o.refs || []).forEach(r => { const y = (r.eixo === 'dir' ? yD : yE)(r.valor); s('line', { x1: ml, x2: W - mr, y1: y, y2: y, stroke: r.cor || 'var(--eixo)', 'stroke-dasharray': '5,4', 'stroke-width': 1.2 }, g); if (r.rotulo) txt(g, ml + 4, y - 5, r.rotulo, { 'text-anchor': 'start', fill: r.cor || 'var(--mu)' }); });
      const barras = o.series.filter(x => x.tipo !== 'linha');
      const nb = barras.length, bwTot = Math.min(step * .72, nb * 46), bw = bwTot / Math.max(1, nb);
      barras.forEach((x, bi) => {
        const y = x.eixo === 'dir' ? yD : yE;
        x.vals.forEach((v, i) => {
          if (v == null || !isFinite(v)) return;
          const x0 = cx(i) - bwTot / 2 + bi * bw + 1, y0 = y(Math.max(0, v)), h = Math.abs(y(v) - y(0));
          const parc = o.parcial && o.parcial[i];
          s('rect', { x: x0, y: y0, width: Math.max(1, bw - 2), height: Math.max(.5, h), rx: 3, fill: (x.corNeg && v < 0) ? x.corNeg : x.cor, 'fill-opacity': parc ? .35 : 1, stroke: parc ? x.cor : null, 'stroke-dasharray': parc ? '4,3' : null }, g);
        });
      });
      o.series.filter(x => x.tipo === 'linha').forEach(x => {
        const y = x.eixo === 'dir' ? yD : yE;
        let d = '', on = false;
        x.vals.forEach((v, i) => { if (v == null || !isFinite(v)) { on = false; return; } d += (on ? 'L' : 'M') + cx(i).toFixed(1) + ',' + y(v).toFixed(1); on = true; });
        s('path', { d, fill: 'none', stroke: x.cor, 'stroke-width': 2.2, 'stroke-linejoin': 'round', 'stroke-linecap': 'round', 'stroke-dasharray': x.tracejado ? '5,4' : null }, g);
        x.vals.forEach((v, i) => { if (v == null || !isFinite(v)) return; const parc = o.parcial && o.parcial[i]; s('circle', { cx: cx(i), cy: y(v), r: 3.2, fill: parc ? 'var(--card)' : x.cor, stroke: x.cor, 'stroke-width': parc ? 2 : 0 }, g); });
      });
      if (o.rotulos) {
        const sr = o.series[o.rotulos.serie || 0], y = sr.eixo === 'dir' ? yD : yE;
        const larg = medir(svg, o.rotulos.amostra || '000,0') + 6;
        if (larg <= step) sr.vals.forEach((v, i) => {
          if (v == null || !isFinite(v)) return;
          const top = sr.tipo === 'linha' ? y(v) - 9 : Math.min(y(v), y(0)) - (o.rotulos.sub ? 17 : 5);
          txt(g, cx(i), top, (o.rotulos.fmt || fE)(v), { 'text-anchor': 'middle', class: 'val' });
          if (o.rotulos.sub) { const t2 = o.rotulos.sub(i); if (t2) txt(g, cx(i), top + 13, t2.t, { 'text-anchor': 'middle', fill: t2.cor || 'var(--mu)' }); }
        });
      }
      const lw = medir(svg, cats[n - 1] || '2026*') + 8, every = Math.max(1, Math.ceil(lw / step));
      cats.forEach((c, i) => { if ((n - 1 - i) % every === 0) txt(g, cx(i), H - mb + 17, c, { 'text-anchor': 'middle' }); });
      const hl = s('rect', { y: mt, height: H - mt - mb, fill: 'var(--a3)', opacity: .45, display: 'none' }, svg);
      svg.insertBefore(hl, g);
      cats.forEach((c, i) => {
        const r = s('rect', { x: ml + step * i, y: mt, width: step, height: H - mt - mb, fill: 'transparent' }, svg);
        r.addEventListener('mousemove', ev => { hl.setAttribute('x', ml + step * i); hl.setAttribute('width', step); hl.removeAttribute('display'); mostrarTip(ev, o.titulo ? o.titulo(i) : c, o.tooltip ? o.tooltip(i) : o.series.map(x => [x.nome, x.vals[i] == null ? null : (x.eixo === 'dir' ? fD : fE)(x.vals[i]), x.cor])); });
        r.addEventListener('mouseleave', () => { hl.setAttribute('display', 'none'); esconderTip(); });
      });
    };
    renders.set(el, fn); fn();
  }

  /* Série temporal (datas): linhas, com área opcional na primeira. */
  const MES = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];
  function tempo(el, o) {
    const fn = () => {
      el.textContent = '';
      const pts = o.pts || [];
      if (pts.length < 2) { const d = document.createElement('div'); d.className = 'vazio'; d.textContent = o.vazio || 'Sem dados.'; el.appendChild(d); return; }
      if (o.series.length > 1) legenda(el, o.series.map(x => Object.assign({ tipo: 'linha' }, x)));
      const W = Math.max(300, el.clientWidth), H = o.altura || 280, mt = 14, mb = 26;
      const svg = s('svg', { viewBox: `0 0 ${W} ${H}`, width: '100%' }, el);
      let mn = Infinity, mx = -Infinity;
      pts.forEach(p => o.series.forEach(x => { const v = p[x.key]; if (v != null && isFinite(v)) { mn = Math.min(mn, v); mx = Math.max(mx, v); } }));
      if (o.ref != null) { mn = Math.min(mn, o.ref); mx = Math.max(mx, o.ref); }
      if (o.zero !== false) mn = Math.min(0, mn);
      const tk = niceTicks(mn, mx, 5), fmt = o.fmt || (v => v);
      const ml = Math.max(...tk.map(v => medir(svg, fmt(v)))) + 12;
      const mr = o.rotFinal ? Math.max(...o.series.map(x => medir(svg, fmt(pts[pts.length - 1][x.key] || 0)))) + 14 : 10;
      const t0 = pts[0].d.getTime(), t1 = pts[pts.length - 1].d.getTime();
      const X = escala(t0, t1, ml, W - mr), Y = escala(tk[0], tk[tk.length - 1], H - mb, mt);
      const g = s('g', {}, svg);
      tk.forEach(v => { s('line', { x1: ml, x2: W - mr, y1: Y(v), y2: Y(v), stroke: v === 0 ? 'var(--eixo)' : 'var(--grade)' }, g); txt(g, ml - 6, Y(v) + 4, fmt(v), { 'text-anchor': 'end' }); });
      if (o.ref != null) s('line', { x1: ml, x2: W - mr, y1: Y(o.ref), y2: Y(o.ref), stroke: 'var(--eixo)', 'stroke-dasharray': '4,4' }, g);
      // eixo x
      const curto = (t1 - t0) < 1000 * 86400 * 800;
      const ticks = [];
      const d0 = new Date(pts[0].d);
      if (curto) { for (let d = new Date(d0.getFullYear(), d0.getMonth() + 1, 1); d.getTime() <= t1; d.setMonth(d.getMonth() + 1)) if (d.getMonth() % 3 === 0) ticks.push(new Date(d)); }
      else { for (let a = d0.getFullYear() + 1; a <= new Date(t1).getFullYear(); a++) ticks.push(new Date(a, 0, 1)); }
      const lbl = d => curto ? MES[d.getMonth()] + '/' + String(d.getFullYear()).slice(2) : String(d.getFullYear());
      const lw = medir(svg, curto ? 'out/26' : '2026') + 10, every = Math.max(1, Math.ceil(lw / ((W - ml - mr) / Math.max(1, ticks.length))));
      ticks.forEach((d, i) => { if ((ticks.length - 1 - i) % every === 0) txt(g, X(d.getTime()), H - mb + 17, lbl(d), { 'text-anchor': 'middle' }); });
      o.series.forEach((x, si) => {
        let d = '', on = false;
        pts.forEach(p => { const v = p[x.key]; if (v == null || !isFinite(v)) { on = false; return; } d += (on ? 'L' : 'M') + X(p.d.getTime()).toFixed(1) + ',' + Y(v).toFixed(1); on = true; });
        if (si === 0 && o.area) s('path', { d: d + `L${X(t1)},${Y(Math.max(0, tk[0]))}L${X(t0)},${Y(Math.max(0, tk[0]))}Z`, fill: x.cor, 'fill-opacity': .1 }, g);
        s('path', { d, fill: 'none', stroke: x.cor, 'stroke-width': x.largura || 2, 'stroke-linejoin': 'round' }, g);
      });
      if (o.rotFinal) {
        const lp = pts[pts.length - 1];
        const ls = o.series.map(x => ({ v: lp[x.key], c: x.cor })).filter(z => z.v != null).sort((a, b) => b.v - a.v);
        let prev = -Infinity;
        ls.forEach(z => { let y = Y(z.v) + 4; if (y - prev < 14) y = prev + 14; prev = y; txt(g, W - mr + 6, y, fmt(z.v), { fill: z.c, 'font-weight': 600 }); });
      }
      const rule = s('line', { y1: mt, y2: H - mb, stroke: 'var(--eixo)', display: 'none' }, svg);
      const dots = o.series.map(x => s('circle', { r: 4, fill: x.cor, stroke: 'var(--card)', 'stroke-width': 2, display: 'none' }, svg));
      const cap = s('rect', { x: ml, y: mt, width: W - ml - mr, height: H - mt - mb, fill: 'transparent' }, svg);
      cap.addEventListener('mousemove', ev => {
        const r = svg.getBoundingClientRect(), mx = (ev.clientX - r.left) * (W / r.width);
        const tt = t0 + (mx - ml) / (W - ml - mr) * (t1 - t0);
        let bi = 0, bd = Infinity; pts.forEach((p, i) => { const dd = Math.abs(p.d.getTime() - tt); if (dd < bd) { bd = dd; bi = i; } });
        const p = pts[bi], xx = X(p.d.getTime());
        rule.setAttribute('x1', xx); rule.setAttribute('x2', xx); rule.removeAttribute('display');
        o.series.forEach((x, si) => { const v = p[x.key]; if (v == null) { dots[si].setAttribute('display', 'none'); return; } dots[si].setAttribute('cx', xx); dots[si].setAttribute('cy', Y(v)); dots[si].removeAttribute('display'); });
        mostrarTip(ev, MES[p.d.getMonth()] + '/' + p.d.getFullYear(), o.tooltip ? o.tooltip(p, bi) : o.series.map(x => [x.nome, fmt(p[x.key]), x.cor]));
      });
      cap.addEventListener('mouseleave', () => { rule.setAttribute('display', 'none'); dots.forEach(d => d.setAttribute('display', 'none')); esconderTip(); });
    };
    renders.set(el, fn); fn();
  }

  window.G = { categorias, tempo, mostrarTip, esconderTip };
})();
