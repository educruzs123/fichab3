// Fatos relevantes da CVM (IPE), os 20 mais recentes. O site não pode baixar os zips da CVM
// direto do navegador (a CVM não libera CORS), então a função faz o download e a leitura.
// Espelha baixarFatos e fatosDoZip de cvm.go; guarda o resultado por 6 horas.
const JSZip = require('jszip');

const CVM = 'https://dados.cvm.gov.br/dados/CIA_ABERTA/';
const SEIS_HORAS = 6 * 60 * 60 * 1000;
let cache = null;
let cacheEm = 0;

function latin1(buf) {
  return new TextDecoder('latin1').decode(buf);
}

// lê os fatos relevantes de um CSV do IPE, sem repetir protocolo (fatosDoZip)
function fatosDoCSV(texto, vistos, fs) {
  const linhas = texto.split('\n');
  const hdr = linhas[0].replace(/\r$/, '').split(';');
  const idx = {};
  hdr.forEach((h, i) => { idx[h.trim().replace(/^﻿/, '')] = i; });
  for (const linha of linhas.slice(1)) {
    const s = linha.replace(/\r$/, '');
    if (!s) continue;
    const c = s.split(';');
    const n = c.length;
    const get = (k) => (k in idx && idx[k] < n ? c[idx[k]] : '');
    // contado do fim: um ';' no assunto desloca as colunas seguintes
    if (n < 13 || get('Categoria') !== 'Fato Relevante' || vistos.has(c[n - 3]) || !c[n - 1].startsWith('https://')) continue;
    vistos.add(c[n - 3]);
    fs.push({ empresa: get('Nome_Companhia'), cnpj: get('CNPJ_Companhia'), assunto: c.slice(7, n - 5).join(';'), data: c[n - 5], link: c[n - 1] });
  }
}

async function baixar() {
  const fs = [];
  const vistos = new Set();
  const ano = new Date().getFullYear();
  for (let y = ano; y >= ano - 1 && fs.length < 20; y--) {
    const r = await fetch(`${CVM}DOC/IPE/DADOS/ipe_cia_aberta_${y}.zip`);
    if (r.status === 404 && y === ano) continue; // em janeiro a CVM pode não ter o arquivo do ano novo
    if (!r.ok) throw new Error(`CVM respondeu ${r.status} ao baixar os fatos relevantes`);
    const zip = await JSZip.loadAsync(await r.arrayBuffer());
    for (const nome of Object.keys(zip.files)) {
      const arq = zip.files[nome];
      if (arq.dir) continue;
      fatosDoCSV(latin1(await arq.async('uint8array')), vistos, fs);
    }
  }
  // a data não tem hora; no mesmo dia, o link com protocolo maior foi entregue depois
  fs.sort((a, b) => (a.data !== b.data ? (a.data > b.data ? -1 : 1) : a.link > b.link ? -1 : a.link < b.link ? 1 : 0));
  return fs.slice(0, 20);
}

module.exports = async (req, res) => {
  try {
    if (!cache || Date.now() - cacheEm >= SEIS_HORAS) {
      try {
        cache = await baixar();
        cacheEm = Date.now();
      } catch (e) {
        if (!cache) throw e; // sem lista anterior, o erro vai para o front
      }
    }
    res.setHeader('Cache-Control', 'public, s-maxage=21600, stale-while-revalidate=86400');
    res.status(200).json(cache);
  } catch (e) {
    res.status(502).json({ erro: e.message });
  }
};
