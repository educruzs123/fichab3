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

  // barra com a ponta do dado arredondada (4px) e o lado da base reto
  function barra(g, x, w, yv, y0, attrs) {
    const h = Math.abs(yv - y0), r = Math.min(4, w / 2, h), t = yv < y0 ? 1 : -1; // t: barra cresce para cima
    const ponta = yv, d = `M${x},${y0}V${ponta + t * r}Q${x},${ponta} ${x + r},${ponta}H${x + w - r}Q${x + w},${ponta} ${x + w},${ponta + t * r}V${y0}Z`;
    return s('path', Object.assign({ d: h < .5 ? `M${x},${y0}h${w}v-.5h-${w}Z` : d }, attrs), g);
  }
  // marcador com anel na cor do fundo, para ficar legível sobre linhas e barras
  function ponto(g, x, y, cor, vazado) {
    return s('circle', { cx: x, cy: y, r: 4, fill: vazado ? 'var(--card)' : cor, stroke: vazado ? cor : 'var(--card)', 'stroke-width': 2 }, g);
  }

  /* Categorias (anos): barras e/ou linhas numa escala só. */
  function categorias(el, o) {
    const fn = () => {
      el.textContent = '';
      const cats = o.cats || [];
      const temDado = o.series.some(x => x.vals.some(v => v != null && isFinite(v)));
      if (!cats.length || !temDado) { const d = document.createElement('div'); d.className = 'vazio'; d.textContent = o.vazio || 'Sem dados para este gráfico.'; el.appendChild(d); return; }
      if (o.legenda !== false && o.series.length > 1) legenda(el, o.series);
      const W = Math.max(300, el.clientWidth), H = o.altura || 280;
      const svg = s('svg', { viewBox: `0 0 ${W} ${H}`, width: '100%', role: 'img', 'aria-label': o.aria || '' }, el);
      let mn = Infinity, mx = -Infinity;
      o.series.forEach(x => x.vals.forEach(v => { if (v != null && isFinite(v)) { mn = Math.min(mn, v); mx = Math.max(mx, v); } }));
      (o.refs || []).forEach(r => { mn = Math.min(mn, r.valor); mx = Math.max(mx, r.valor); });
      if (o.series.some(x => x.tipo !== 'linha') || o.zero !== false) { mn = Math.min(0, mn); mx = Math.max(0, mx); }
      const tk = niceTicks(mn, mx, 5), fE = o.fmtEsq || (v => v);
      const mt = o.rotulos ? 34 : 14, mb = 26, mr = 8;
      const ml = Math.max(...tk.map(v => medir(svg, fE(v)))) + 12;
      const Y = escala(tk[0], tk[tk.length - 1], H - mb, mt);
      const n = cats.length, step = (W - ml - mr) / n;
      const cx = i => ml + step * (i + .5);
      const g = s('g', {}, svg);
      tk.forEach(v => { s('line', { x1: ml, x2: W - mr, y1: Y(v), y2: Y(v), stroke: v === 0 ? 'var(--eixo)' : 'var(--grade)', 'stroke-width': 1 }, g); txt(g, ml - 6, Y(v) + 4, fE(v), { 'text-anchor': 'end' }); });
      (o.refs || []).forEach(r => { const y = Y(r.valor); s('line', { x1: ml, x2: W - mr, y1: y, y2: y, stroke: 'var(--eixo)', 'stroke-dasharray': '5,4', 'stroke-width': 1 }, g); if (r.rotulo) txt(g, ml + 4, y - 5, r.rotulo, { 'text-anchor': 'start' }); });
      const barras = o.series.filter(x => x.tipo !== 'linha');
      const nb = barras.length, gap = 2, bw = Math.max(2, Math.min(24, (step * .7 - gap * (nb - 1)) / Math.max(1, nb)));
      const bwTot = nb * bw + gap * (nb - 1);
      barras.forEach((x, bi) => {
        x.vals.forEach((v, i) => {
          if (v == null || !isFinite(v)) return;
          const parc = o.parcial && o.parcial[i];
          barra(g, cx(i) - bwTot / 2 + bi * (bw + gap), bw, Y(v), Y(0), { fill: (x.corNeg && v < 0) ? x.corNeg : x.cor, 'fill-opacity': parc ? .4 : 1 });
        });
      });
      o.series.filter(x => x.tipo === 'linha').forEach(x => {
        let d = '', on = false;
        x.vals.forEach((v, i) => { if (v == null || !isFinite(v)) { on = false; return; } d += (on ? 'L' : 'M') + cx(i).toFixed(1) + ',' + Y(v).toFixed(1); on = true; });
        s('path', { d, fill: 'none', stroke: x.cor, 'stroke-width': 2, 'stroke-linejoin': 'round', 'stroke-linecap': 'round' }, g);
        x.vals.forEach((v, i) => { if (v == null || !isFinite(v)) return; ponto(g, cx(i), Y(v), x.cor, o.parcial && o.parcial[i]); });
      });
      if (o.rotulos) {
        const sr = o.series[o.rotulos.serie || 0];
        const larg = medir(svg, o.rotulos.amostra || '000,0') + 6;
        if (larg <= step) sr.vals.forEach((v, i) => {
          if (v == null || !isFinite(v)) return;
          const top = sr.tipo === 'linha' ? Y(v) - 10 : Math.min(Y(v), Y(0)) - (o.rotulos.sub ? 17 : 5);
          txt(g, cx(i), top, (o.rotulos.fmt || fE)(v), { 'text-anchor': 'middle', class: 'val' });
          if (o.rotulos.sub) { const t2 = o.rotulos.sub(i); if (t2) txt(g, cx(i), top + 13, t2.t, { 'text-anchor': 'middle', fill: t2.cor || 'var(--mu)' }); }
        });
      }
      const lw = medir(svg, cats[n - 1] || '2026*') + 8, every = Math.max(1, Math.ceil(lw / step));
      cats.forEach((c, i) => { if ((n - 1 - i) % every === 0) txt(g, cx(i), H - mb + 17, c, { 'text-anchor': 'middle' }); });
      const hl = s('rect', { y: mt, height: H - mt - mb, rx: 4, fill: 'var(--a3)', opacity: .45, display: 'none' }, svg);
      svg.insertBefore(hl, g);
      cats.forEach((c, i) => {
        const r = s('rect', { x: ml + step * i, y: mt, width: step, height: H - mt - mb, fill: 'transparent' }, svg);
        r.addEventListener('mousemove', ev => { hl.setAttribute('x', ml + step * i); hl.setAttribute('width', step); hl.removeAttribute('display'); mostrarTip(ev, o.titulo ? o.titulo(i) : c, o.tooltip ? o.tooltip(i) : o.series.map(x => [x.nome, x.vals[i] == null ? null : fE(x.vals[i]), x.cor])); });
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
      const mr = o.rotFinal ? Math.max(...o.series.map(x => medir(svg, fmt(pts[pts.length - 1][x.key] || 0)))) + 16 : 10;
      const t0 = pts[0].d.getTime(), t1 = pts[pts.length - 1].d.getTime();
      const X = escala(t0, t1, ml, W - mr), Y = escala(tk[0], tk[tk.length - 1], H - mb, mt);
      const g = s('g', {}, svg);
      tk.forEach(v => { s('line', { x1: ml, x2: W - mr, y1: Y(v), y2: Y(v), stroke: v === 0 ? 'var(--eixo)' : 'var(--grade)' }, g); txt(g, ml - 6, Y(v) + 4, fmt(v), { 'text-anchor': 'end' }); });
      if (o.ref != null) s('line', { x1: ml, x2: W - mr, y1: Y(o.ref), y2: Y(o.ref), stroke: 'var(--eixo)', 'stroke-dasharray': '5,4' }, g);
      // eixo x
      const dias = (t1 - t0) / 86400000, semanal = dias < 100, mensal = dias < 400, curto = dias < 800;
      const ticks = [];
      const d0 = new Date(pts[0].d);
      if (semanal) { for (let d = new Date(d0); d.getTime() <= t1; d.setDate(d.getDate() + 7)) ticks.push(new Date(d)); }
      else if (curto) { for (let d = new Date(d0.getFullYear(), d0.getMonth() + 1, 1); d.getTime() <= t1; d.setMonth(d.getMonth() + 1)) if (mensal || d.getMonth() % 3 === 0) ticks.push(new Date(d)); }
      else { for (let a = d0.getFullYear() + 1; a <= new Date(t1).getFullYear(); a++) ticks.push(new Date(a, 0, 1)); }
      const lbl = d => semanal ? String(d.getDate()).padStart(2, '0') + '/' + String(d.getMonth() + 1).padStart(2, '0') : curto ? MES[d.getMonth()] + '/' + String(d.getFullYear()).slice(2) : String(d.getFullYear());
      const lw = medir(svg, curto ? 'out/26' : '2026') + 10, every = Math.max(1, Math.ceil(lw / ((W - ml - mr) / Math.max(1, ticks.length))));
      ticks.forEach((d, i) => { if ((ticks.length - 1 - i) % every === 0) txt(g, X(d.getTime()), H - mb + 17, lbl(d), { 'text-anchor': 'middle' }); });
      o.series.forEach((x, si) => {
        let d = '', on = false;
        pts.forEach(p => { const v = p[x.key]; if (v == null || !isFinite(v)) { on = false; return; } d += (on ? 'L' : 'M') + X(p.d.getTime()).toFixed(1) + ',' + Y(v).toFixed(1); on = true; });
        if (si === 0 && o.area) s('path', { d: d + `L${X(t1)},${Y(Math.max(0, tk[0]))}L${X(t0)},${Y(Math.max(0, tk[0]))}Z`, fill: x.cor, 'fill-opacity': .1 }, g);
        s('path', { d, fill: 'none', stroke: x.cor, 'stroke-width': 2, 'stroke-linejoin': 'round', 'stroke-linecap': 'round' }, g);
      });
      if (o.rotFinal) {
        const lp = pts[pts.length - 1];
        const ls = o.series.map(x => ({ v: lp[x.key], c: x.cor })).filter(z => z.v != null).sort((a, b) => b.v - a.v);
        let prev = -Infinity;
        ls.forEach(z => { ponto(g, X(t1), Y(z.v), z.c); let y = Y(z.v) + 4; if (y - prev < 14) y = prev + 14; prev = y; txt(g, W - mr + 8, y, fmt(z.v), { class: 'val' }); });
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
        mostrarTip(ev, o.titulo ? o.titulo(p) : MES[p.d.getMonth()] + '/' + p.d.getFullYear(), o.tooltip ? o.tooltip(p, bi) : o.series.map(x => [x.nome, fmt(p[x.key]), x.cor]));
      });
      cap.addEventListener('mouseleave', () => { rule.setAttribute('display', 'none'); dots.forEach(d => d.setAttribute('display', 'none')); esconderTip(); });
    };
    renders.set(el, fn); fn();
  }

  window.G = { categorias, tempo, mostrarTip, esconderTip };
})();
