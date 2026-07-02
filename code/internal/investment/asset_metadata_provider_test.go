package investment

import "testing"

func TestParseInvestidor10AssetMetadataStock(t *testing.T) {
	t.Parallel()

	body := `<html><body><h1>WEGE3</h1><h2>WEG S.A.</h2><div>Nome da Empresa: WEG S.A.</div><div>CNPJ: 84.429.695/0001-11</div></body></html>`

	metadata, err := parseInvestidor10AssetMetadata("WEGE3", body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if metadata.Name != "WEG S.A." {
		t.Fatalf("expected name WEG S.A., got %q", metadata.Name)
	}
	if metadata.CNPJ == nil || *metadata.CNPJ != "84429695000111" {
		t.Fatalf("expected cnpj 84429695000111, got %+v", metadata.CNPJ)
	}
}

func TestParseInvestidor10AssetMetadataETFWithoutCNPJ(t *testing.T) {
	t.Parallel()

	body := `<html><head><title>SPYI11 - Buena Vista US High Income ETF - Investidor10</title></head><body><h1>SPYI11</h1></body></html>`

	metadata, err := parseInvestidor10AssetMetadata("SPYI11", body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if metadata.Name != "Buena Vista US High Income ETF" {
		t.Fatalf("expected ETF name, got %q", metadata.Name)
	}
	if metadata.CNPJ != nil {
		t.Fatalf("expected nil cnpj, got %+v", metadata.CNPJ)
	}
}

func TestParseInvestidor10AssetMetadataSkipsPercentageNoise(t *testing.T) {
	t.Parallel()

	body := `<html><head><title>SPYI11 - Buena Vista US High Income ETF - Investidor10</title></head><body><div>SPYI11</div><div>SPYI11 · 0,88%</div></body></html>`

	metadata, err := parseInvestidor10AssetMetadata("SPYI11", body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if metadata.Name != "Buena Vista US High Income ETF" {
		t.Fatalf("expected ETF name, got %q", metadata.Name)
	}
}

func TestParseInvestidor10AssetMetadataPrefersLabeledName(t *testing.T) {
	t.Parallel()

	body := `<html><body><div>BPAC11</div><div>BPAC11 · 1,32%</div><div>Nome da Empresa: Banco BTG Pactual S.A.</div><div>CNPJ: 30.306.294/0001-45</div></body></html>`

	metadata, err := parseInvestidor10AssetMetadata("BPAC11", body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if metadata.Name != "Banco BTG Pactual S.A." {
		t.Fatalf("expected labeled name, got %q", metadata.Name)
	}
	if metadata.CNPJ == nil || *metadata.CNPJ != "30306294000145" {
		t.Fatalf("expected cnpj 30306294000145, got %+v", metadata.CNPJ)
	}
}

func TestParseInvestidor10AssetMetadataReadsNextLineAfterLabel(t *testing.T) {
	t.Parallel()

	body := `<html><body><div>Razão Social</div><div>BTG Pactual Logística Fundo de Investimento Imobiliário</div><div>CNPJ</div><div>11.839.593/0001-09</div></body></html>`

	metadata, err := parseInvestidor10AssetMetadata("BTLG11", body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if metadata.Name != "BTG Pactual Logística Fundo de Investimento Imobiliário" {
		t.Fatalf("expected multiline labeled name, got %q", metadata.Name)
	}
	if metadata.CNPJ == nil || *metadata.CNPJ != "11839593000109" {
		t.Fatalf("expected cnpj 11839593000109, got %+v", metadata.CNPJ)
	}
}
