-- Situação da atualização de cotações, uma linha por ticker ativo, para a página Empresas.
-- O esquema carga não é exposto pela API do Supabase; esta função entrega só as colunas abaixo
-- e é lida pelo app em /rpc/log_cotacoes. Não é histórico: carga.cot_tickers guarda só a última execução.
create or replace function public.log_cotacoes()
returns table (
  ticker text, empresa text, atualizado_em timestamptz, status text, msg text,
  ult_pregao date, fechamento numeric
)
language sql stable security definer
set search_path = ''
as $$
  select c.ticker, e.nome_curto, c.ult_atualizacao, c.status, c.msg, u.data, u.fechamento
  from carga.cot_tickers c
  join public.tickers t on t.ticker = c.ticker and t.ativo
  left join public.empresas e on e.id = t.empresa_id
  left join lateral (
    select q.data, q.fechamento from public.cotacoes q
    where q.ticker = c.ticker order by q.data desc limit 1
  ) u on true
  order by c.ult_atualizacao desc nulls last, c.ticker
$$;
revoke all on function public.log_cotacoes() from public;
grant execute on function public.log_cotacoes() to anon, authenticated;
