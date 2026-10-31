#!/bin/sh
# Envia os PDFs de ~/FichaB3/demonstrativos ao bucket "demonstrativos" do Supabase.
# A chave de serviço é digitada aqui, sem aparecer na tela, e não fica salva.
set -eu
URL=https://crdjqnivmxuzxihgeohm.supabase.co/storage/v1/object/demonstrativos
printf 'Cole a chave service_role (não aparece ao colar) e tecle Enter: '
stty -echo; read -r CHAVE; stty echo; echo
[ -n "$CHAVE" ] || { echo "Chave vazia."; exit 1; }
falhas=0
for f in "$HOME"/FichaB3/demonstrativos/*/*.pdf; do
  caminho=$(basename "$(dirname "$f")")/$(basename "$f")
  cod=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$URL/$caminho" \
    -H "Authorization: Bearer $CHAVE" -H "apikey: $CHAVE" \
    -H "Content-Type: application/pdf" -H "x-upsert: true" --data-binary @"$f")
  echo "$cod $caminho"
  [ "$cod" = 200 ] || falhas=$((falhas + 1))
done
echo "Falhas: $falhas"
[ "$falhas" = 0 ]
