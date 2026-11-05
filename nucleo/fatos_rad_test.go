package nucleo

import (
	"os"
	"strings"
	"testing"
)

// rede: roda só com FATOS_REDE=1
func TestUltimosFatosComRAD(t *testing.T) {
	if os.Getenv("FATOS_REDE") == "" {
		t.Skip("defina FATOS_REDE=1")
	}
	fs, err := UltimosFatos()
	if err != nil {
		t.Fatal(err)
	}
	vistos := map[string]bool{}
	for _, f := range fs {
		t.Log(f.Data, f.CNPJ, f.Empresa, "|", f.Assunto)
		if f.CNPJ == "" {
			t.Errorf("sem CNPJ: %s", f.Empresa)
		}
		p := reProto.FindString(f.Link)
		if p == "" || vistos[p] || !strings.HasPrefix(f.Link, "https://") {
			t.Errorf("link ruim ou repetido: %s", f.Link)
		}
		vistos[p] = true
	}
	if len(fs) != 20 {
		t.Errorf("esperava 20, veio %d", len(fs))
	}
}
