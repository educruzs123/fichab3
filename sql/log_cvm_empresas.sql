-- Log das entregas de documentos das empresas à CVM (RAD) inseridas no banco fichas-b3.
-- O esquema carga não é exposto pela API do Supabase; esta função entrega só as colunas
-- abaixo (nada de carga.config) e é o que a página Empresas do app lê em /rpc/log_cvm_empresas.
create or replace function public.log_cvm_empresas()
returns table (
  num_seq integer, empresa text, ticker text, tipo text, dt_refer date, versao integer,
  entregue_em timestamp, inserido_em timestamptz, linhas integer, status text, msg text
)
language sql stable security definer
set search_path = ''
as $$
  select r.num_seq, r.empresa,
         (select e.tickers[1] from public.empresas e
           where ltrim(e.cod_cvm, '0') = r.cd_cvm::text and cardinality(e.tickers) > 0 limit 1),
         r.tipo, r.dt_refer, r.versao, r.entregue_em, coalesce(r.fim, r.criado_em), r.linhas, r.status, r.msg
  from carga.rad_docs r
  order by coalesce(r.fim, r.criado_em) desc
$$;
revoke all on function public.log_cvm_empresas() from public;
grant execute on function public.log_cvm_empresas() to anon, authenticated;
