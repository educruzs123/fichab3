-- Bucket público de PDFs de demonstrativos (leitura aberta; escrita só com a chave de serviço)
-- e lista das 20 empresas de maior valor de mercado, de leitura pública.
insert into storage.buckets (id, name, public, file_size_limit, allowed_mime_types)
values ('demonstrativos', 'demonstrativos', true, 52428800, array['application/pdf'])
on conflict (id) do nothing;

create table if not exists public.top20_valor_mercado (
  posicao smallint primary key check (posicao between 1 and 20),
  ticker text not null unique,
  empresa text not null,
  valor_mercado_bi numeric not null,
  estimado boolean not null default false,
  referencia date not null
);
comment on table public.top20_valor_mercado is 'As 20 maiores empresas por valor de mercado (R$ bilhões). estimado = calculado fora da ficha do app.';
alter table public.top20_valor_mercado enable row level security;
create policy "leitura publica" on public.top20_valor_mercado for select to anon, authenticated using (true);
