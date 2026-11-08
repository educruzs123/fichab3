package main

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestFatosDoZip(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	w, _ := z.Create("ipe.csv")
	w.Write([]byte("CNPJ_Companhia;Nome_Companhia;Codigo_CVM;Data_Referencia;Categoria;Tipo;Especie;Assunto;Data_Entrega;Tipo_Apresentacao;Protocolo_Entrega;Versao;Link_Download\n" +
		"1;A;1;2026-01-01;Fato Relevante;;;Venda; com ponto e v\xedrgula;2026-01-02;AP;P1;1;https://cvm/1\n" +
		"1;A;1;2026-01-01;Fato Relevante;;;Venda; com ponto e v\xedrgula;2026-01-02;AP;P1;1;https://cvm/1\n" +
		"2;B;2;2026-01-01;Assembleia;AGE;Ata;Outra;2026-01-03;AP;P2;1;https://cvm/2\n" +
		"3;C;3;2026-01-01;Fato Relevante;;;Sem link;2026-01-03;AP;P3;1;javascript:x\n"))
	z.Close()
	fs, err := fatosDoZip(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 || fs[0].Assunto != "Venda; com ponto e vírgula" || fs[0].Data != "2026-01-02" || fs[0].Link != "https://cvm/1" {
		t.Fatalf("%+v", fs)
	}
}
