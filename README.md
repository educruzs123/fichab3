# FichaB3Completa

Fichas de ações da B3 a partir das demonstrações financeiras da CVM. O programa
sobe um servidor local em `http://127.0.0.1:47321` com as páginas embutidas no
binário e abre o navegador ao iniciar.

Para o uso no dia a dia, veja [empacotar/Leia-me.txt](empacotar/Leia-me.txt).
Este README é para quem mexe no código.

## Requisitos

- Go 1.22 ou mais novo.
- macOS para gerar o `.app` (o script usa `lipo`, `codesign` e `plutil`).

## Rodar

```bash
go run . -sem-navegador
```

Abra `http://127.0.0.1:47321`. Sem `-sem-navegador`, o navegador abre sozinho.

Opções:

| Flag | Efeito |
|------|--------|
| `-sem-navegador` | não abre o navegador |
| `-pasta-app DIR` | usa `DIR` como pasta de trabalho (padrão `~/FichaB3`) |

## Testar

```bash
go test ./...
go vet ./...
```

## Gerar o app para distribuição

```bash
sh empacotar/gerar-app.sh
```

Sai `dist/FichaB3Completa.app` (universal, arm64 + amd64) e o zip
`dist/FichaB3Completa-<versão>.zip`. A versão vem de
`empacotar/Info.plist`, então atualize-a lá antes de gerar.

## Onde ficam os dados

Tudo fica em `~/FichaB3/` (ou na pasta de `-pasta-app`):

- `config.json`: configuração (fonte dos dados, pastas, chaves).
- `banco/`: banco local da CVM, um arquivo por empresa. É lido ao iniciar
  e usado pela fonte local.
- `cache/`: cotações, válidas por 12 horas.
- `fichab3.log`: registro de funcionamento.

`config.json` tem chaves e não vai para o git (está no `.gitignore`).

## Fontes de dados

- **Supabase** (padrão, `fonte: supabase`): banco `fichas-b3`, que a CVM
  atualiza todo domingo. A URL padrão do projeto está em `supabase.go`.
- **Local** (`fonte: local`): zips da CVM baixados para uma pasta do Mac.

Cotações vêm do Supabase; `av.go` também consulta o Yahoo Finance. A chave da
Alpha Vantage é opcional, fica em `config.json` e é usada só para ações sem
cotação no banco.

## Estrutura

```
main.go        servidor HTTP, configuração e rotas
cvm.go         download e leitura dos zips da CVM
banco.go       banco local (um arquivo por empresa)
ficha.go       montagem da ficha de cada empresa
supabase.go    cliente do Supabase (balanços, cotações, logs)
av.go          cotações (Alpha Vantage e Yahoo Finance)
web/           interface: index.html, app.js, graficos.js, estilo.css
sql/           funções e tabelas do Supabase (rodar no SQL editor)
scripts/       envio manual dos PDFs de demonstrativos ao Supabase
empacotar/     ícone, Info.plist, Leia-me e gerar-app.sh
```

### Scripts

- `scripts/enviar-demonstrativos.sh`: envia os PDFs de
  `~/FichaB3/demonstrativos` ao bucket `demonstrativos`. Pede a chave
  `service_role` sem mostrá-la na tela.

### SQL

Os arquivos em `sql/` não rodam sozinhos. Aplique-os no SQL editor do
Supabase. O app só lê as funções `log_cvm_empresas` e `log_cotacoes`, e a
lista `demonstrativos`, então elas precisam existir no banco.

## Versão

Versão atual: 1.17.0 (`empacotar/Info.plist`).
