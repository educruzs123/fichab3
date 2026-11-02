-- Carteira: os únicos tickers mantidos no banco (planilha de outubro de 2026).
-- carga.processar só grava empresas cujo CNPJ está aqui e só os tickers listados; assim a carga
-- semanal e a do RAD não trazem de volta as outras empresas. cnpj nulo = BDR, sem dados na CVM.
create table if not exists public.carteira (
  ticker text primary key,
  empresa text not null,
  setor text,
  cnpj text
);
alter table public.carteira enable row level security;
create policy "leitura publica" on public.carteira for select to anon, authenticated using (true);

insert into public.carteira (ticker, empresa, setor) values
('BBDC4','Banco Bradesco','Bancos'),('BBDC3','Banco Bradesco','Bancos'),('BRBI11','BR Partners','Serviços Financeiros'),
('STOC34','Stone','Serviços Financeiros'),('LREN3','Lojas Renner','Varejo'),('BMEB3','Banco Mercantil','Bancos'),
('BBAS3','Banco do Brasil','Bancos'),('TIMS3','Tim','Telecomunicações'),('BPAC11','BTG','Bancos'),('ALOS3','Allos','Shoppings'),
('VULC3','Vulcabras','Calçados'),('BMEB4','Banco Mercantil','Bancos'),('BRSR6','Banco Banrisul','Bancos'),('WIZC3','Wiz','Seguros'),
('LEVE3','Mahle Metal Leve','Automóveis e Motocicletas'),('MULT3','Multiplan','Shoppings'),('SAPR4','Sanepar','Saneamento'),
('RANI3','Irani','Papel e Celulose'),('SAPR11','Sanepar','Saneamento'),('LOGG3','Log','Exploração de imóveis'),
('BAZA3','Banco da Amazônia','Bancos'),('ITSA4','Itausa','Holding'),('PINE4','Banco Pine','Bancos'),('PSSA3','Porto Seguro','Seguros'),
('PASS3','Compass','Oléo, gás e biocomb.'),('ABCB4','Banco ABC Brasil','Bancos'),('BMGB4','Banco BMG','Bancos'),
('PAGS34','Pagbank','Serviços Financeiros'),('DEXP3','Dexxos','Industrial'),('MDIA3','MDias Branco','Alimentos'),
('B3SA3','B3','Serviços Financeiros'),('SAPR3','Sanepar','Saneamento'),('JHSF3','JHSF','Imobiliário'),('SBSP3','Sabesp','Saneamento'),
('AGRO3','Brasilagro','Agronegócio'),('ITUB4','Banco Itau','Bancos'),('POMO3','Marcopolo','Industrial'),('CPFE3','CPFL Energia','Energia'),
('SAUD3','Bradsaúde','Saúde'),('FESA4','Ferbasa','Siderurgia e Metalurgia'),('CMIG4','Cemig','Energia'),('BRAP4','Bradespar','Holding'),
('KLBN4','Klabin','Papel e Celulose'),('CSUD3','CSU','Tecnologia'),('EQPA3','Equatorial Pará','Energia'),('IRBR3','IRB','Seguros'),
('GOAU3','Metalúrgica Gerdau','Siderurgia e Metalurgia'),('CGAS5','Comgás','Oléo, gás e biocomb.'),('VIVT3','Telefonica Vivo','Telecomunicações'),
('KLBN11','Klabin','Papel e Celulose'),('CGRA4','Grazziotin','Varejo'),('POMO4','Marcopolo','Industrial'),('AXIA3','Axia Energia','Energia'),
('CSMG3','Copasa','Saneamento'),('BRAP3','Bradespar','Holding'),('GGBR3','Gerdau','Siderurgia e Metalurgia'),('FIQE3','Unifique','Telecomunicações'),
('CLSC4','Celesc','Energia'),('ISAE4','ISA Energia','Energia'),('KLBN3','Klabin','Papel e Celulose'),('VALE3','Vale','Mineração'),
('GRND3','Grendene','Calçados'),('RECV3','Petroreconcâvo','Oléo, gás e biocomb.'),('UNIP6','Unipar','Químicos'),('SHUL4','Shulz','Bens Industriais'),
('SLCE3','SLC Agrícola','Agronegócio'),('ITUB3','Banco Itau','Bancos'),('CPLE3','Copel','Energia'),('SANB4','Banco Santander','Bancos'),
('SANB11','Banco Santander','Bancos'),('ALUP11','Alupar','Energia'),('SANB3','Banco Santander','Bancos'),('PETR4','Petrobras','Oléo, gás e biocomb.'),
('CEBR5','CEB','Energia'),('SUZB3','Suzano','Papel e Celulose'),('CEBR3','CEB','Energia'),('VBBR3','Vibra Energia','Oléo, gás e biocomb.'),
('BBSE3','BB Seguridade','Seguros'),('ABEV3','Ambev','Bebidas'),('CXSE3','Caixa Seguridade','Seguros'),('EGIE3','Engie Brasil','Energia'),
('TAEE3','Taesa','Energia'),('CEBR6','CEB','Energia'),('TGMA3','Tegma','Logistica'),('TAEE11','Taesa','Energia'),('TAEE4','Taesa','Energia'),
('FLRY3','Fleury','Saúde'),('BEES3','Banestes','Bancos'),('CMIN3','CSN Mineração','Mineração'),('KEPL3','Kepler Weber','Agronegócio')
on conflict (ticker) do nothing;

-- tickers que o FCA não traz: Klabin ON/PN e o código novo da Equatorial Pará
insert into public.tickers (ticker, empresa_id, ativo)
select v.t, e.id, true from (values ('KLBN3', '89.637.490/0001-45'), ('KLBN4', '89.637.490/0001-45'), ('EQPA3', '04.895.728/0001-80')) v(t, cnpj)
join public.empresas e on e.cnpj = v.cnpj
on conflict (ticker) do nothing;

update public.carteira c set cnpj = e.cnpj
from public.tickers t join public.empresas e on e.id = t.empresa_id
where t.ticker = c.ticker;

-- filtro da carteira em carga.processar (troca no texto da própria função, que é longa)
do $$
declare d text;
begin
  select pg_get_functiondef('carga.processar(integer)'::regprocedure) into d;
  d := replace(d, $a$coalesce(cnpj_cia, '') <> ''$a$, 'cnpj_cia in (select cnpj from public.carteira)');
  d := replace(d, $a$coalesce(cnpj_companhia, '') <> ''$a$, 'cnpj_companhia in (select cnpj from public.carteira)');
  d := replace(d, $a$v.mercado = 'Bolsa'$a$, $a$v.mercado = 'Bolsa' and v.tk in (select ticker from public.carteira)$a$);
  execute d;
end $$;
