-- Lista dos PDFs do bucket demonstrativos, de leitura pública: o app monta os links a partir dela.
-- Cada arquivo novo no bucket precisa de uma linha aqui.
create table if not exists public.demonstrativos (
  ticker text not null,
  arquivo text not null,
  tamanho bigint not null,
  primary key (ticker, arquivo)
);
alter table public.demonstrativos enable row level security;
create policy "leitura publica" on public.demonstrativos for select to anon, authenticated using (true);
