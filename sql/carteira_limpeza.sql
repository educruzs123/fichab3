-- Apaga do banco tudo o que não é da carteira (rodar uma vez, depois de sql/carteira.sql).
set statement_timeout = '15min';
create temp table k as select id from public.empresas where cnpj in (select cnpj from public.carteira);
delete from public.dre where empresa_id not in (select id from k);
delete from public.bp where empresa_id not in (select id from k);
delete from public.dfc where empresa_id not in (select id from k);
delete from public.dre_tri where empresa_id not in (select id from k);
delete from public.bp_tri where empresa_id not in (select id from k);
delete from public.dfc_tri where empresa_id not in (select id from k);
delete from public.trimestres where empresa_id not in (select id from k);
delete from public.capital where empresa_id not in (select id from k);
delete from public.base_ano where empresa_id not in (select id from k);
delete from carga.q_metricas where empresa_id not in (select id from k);
delete from public.tickers where ticker not in (select ticker from public.carteira);
delete from public.empresas where id not in (select id from k);
delete from public.cotacoes where ticker not in (select ticker from public.carteira);
delete from public.cotacoes_mensais where ticker not in (select ticker from public.carteira);
delete from carga.cot_tickers where ticker not in (select ticker from public.carteira);
delete from carga.rad_docs where cd_cvm::text not in (select ltrim(cod_cvm, '0') from public.empresas where cod_cvm is not null);
delete from public.demonstrativos where ticker not in (select ticker from public.carteira);
delete from public.top20_valor_mercado where ticker not in (select ticker from public.carteira);
-- nome_id não tem índice: um anti-join por hash em vez de not exists por linha
delete from public.contas_nome where id not in (
  select nome_id from public.dre where nome_id is not null union select nome_id from public.bp where nome_id is not null
  union select nome_id from public.dfc where nome_id is not null union select nome_id from public.dre_tri where nome_id is not null
  union select nome_id from public.bp_tri where nome_id is not null union select nome_id from public.dfc_tri where nome_id is not null);
-- recalcula trimestres, tickers das empresas e a tabela meta
select carga.finalizar();
