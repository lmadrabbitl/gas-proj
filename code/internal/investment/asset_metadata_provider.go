package investment

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const defaultInvestidor10BaseURL = "https://investidor10.com.br"

var (
	cnpjRe            = regexp.MustCompile(`\b\d{2}[.\s]?\d{3}[.\s]?\d{3}[\/\s]?\d{4}-?\d{2}\b`)
	percentageLikeRe  = regexp.MustCompile(`^[A-Z0-9.-]+\s*·\s*-?[0-9]+(?:[.,][0-9]+)?%$`)
	purePercentRe     = regexp.MustCompile(`^-?[0-9]+(?:[.,][0-9]+)?%$`)
	moneyLikePrefixRe = regexp.MustCompile(`^(R\$|US\$|€|\$)\s*`)
	titleTagRe        = regexp.MustCompile(`(?is)<title>(.*?)</title>`)
)

type AssetMetadataProvider interface {
	FetchAssetMetadata(ctx context.Context, ticker string, assetType AssetType) (*AssetMetadata, error)
}

type AssetMetadata struct {
	Name      string
	CNPJ      *string
	Source    string
	UpdatedAt time.Time
}

type Investidor10AssetMetadataProviderConfig struct {
	BaseURL string
	Client  *http.Client
	Now     func() time.Time
}

type Investidor10AssetMetadataProvider struct {
	baseURL string
	client  *http.Client
	now     func() time.Time
}

type metadataSourceCandidate struct {
	Path   string
	Source string
}

func NewInvestidor10AssetMetadataProvider(cfg Investidor10AssetMetadataProviderConfig) *Investidor10AssetMetadataProvider {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultInvestidor10BaseURL
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	nowFn := cfg.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	return &Investidor10AssetMetadataProvider{
		baseURL: baseURL,
		client:  client,
		now:     nowFn,
	}
}

func (p *Investidor10AssetMetadataProvider) FetchAssetMetadata(ctx context.Context, ticker string, assetType AssetType) (*AssetMetadata, error) {
	normalizedTicker := normalizeAssetCode(ticker)
	var attempts []string
	for _, candidate := range metadataSourceCandidates(normalizedTicker, assetType) {
		body, statusCode, err := p.fetch(ctx, candidate.Path)
		if err != nil {
			attempts = append(attempts, fmt.Sprintf("%s: %v", candidate.Path, err))
			continue
		}
		if statusCode == http.StatusNotFound {
			attempts = append(attempts, fmt.Sprintf("%s: not found", candidate.Path))
			continue
		}
		metadata, err := parseInvestidor10AssetMetadata(normalizedTicker, body)
		if err != nil {
			attempts = append(attempts, fmt.Sprintf("%s: %v", candidate.Path, err))
			continue
		}
		metadata.Source = candidate.Source
		metadata.UpdatedAt = p.now().UTC()
		return metadata, nil
	}
	return nil, fmt.Errorf("metadata not found for %s after trying %s", normalizedTicker, strings.Join(attempts, "; "))
}

func metadataSourceCandidates(ticker string, assetType AssetType) []metadataSourceCandidate {
	slug := strings.ToLower(strings.TrimSpace(ticker))
	switch assetType {
	case AssetTypeStock:
		return []metadataSourceCandidate{
			{Path: "/acoes/" + slug + "/", Source: "investidor10:acoes"},
			{Path: "/bdrs/" + slug + "/", Source: "investidor10:bdrs"},
		}
	case AssetTypeETF:
		return []metadataSourceCandidate{
			{Path: "/etfs/" + slug + "/", Source: "investidor10:etfs"},
			{Path: "/fiis/" + slug + "/", Source: "investidor10:fiis"},
			{Path: "/bdrs/" + slug + "/", Source: "investidor10:bdrs"},
			{Path: "/acoes/" + slug + "/", Source: "investidor10:acoes"},
		}
	default:
		return []metadataSourceCandidate{
			{Path: "/fiis/" + slug + "/", Source: "investidor10:fiis"},
			{Path: "/etfs/" + slug + "/", Source: "investidor10:etfs"},
			{Path: "/bdrs/" + slug + "/", Source: "investidor10:bdrs"},
			{Path: "/acoes/" + slug + "/", Source: "investidor10:acoes"},
		}
	}
}

func (p *Investidor10AssetMetadataProvider) fetch(ctx context.Context, path string) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+path, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 gas-proj/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("request %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", resp.StatusCode, fmt.Errorf("read %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return "", resp.StatusCode, fmt.Errorf("request %s failed with status %s body=%q", path, resp.Status, strings.TrimSpace(string(body[:min(len(body), 512)])))
	}
	return string(body), resp.StatusCode, nil
}

func parseInvestidor10AssetMetadata(ticker string, body string) (*AssetMetadata, error) {
	lines := splitMeaningfulLines(stripHTML(body))
	cnpj := extractCNPJ(body)
	name := extractNameFromMetadataBody(ticker, lines)
	if name == "" {
		name = extractNameFromTitleTag(ticker, body)
	}
	if name == "" && cnpj == nil {
		return nil, fmt.Errorf("no useful metadata found")
	}
	if name == "" {
		name = ticker
	}
	return &AssetMetadata{
		Name: name,
		CNPJ: cnpj,
	}, nil
}

func splitMeaningfulLines(body string) []string {
	rawLines := strings.Split(body, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		normalized := strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if normalized == "" {
			continue
		}
		lines = append(lines, normalized)
	}
	return lines
}

func extractNameFromMetadataBody(ticker string, lines []string) string {
	labels := []string{"Nome da Empresa:", "Razão Social", "Nome:"}
	for _, label := range labels {
		for index, line := range lines {
			if !strings.Contains(line, label) {
				continue
			}
			candidate := strings.TrimSpace(strings.TrimPrefix(line, label))
			if candidate != "" && normalizeAssetCode(candidate) != ticker && !isInvalidNameCandidate(candidate) {
				return candidate
			}
			for next := index + 1; next < len(lines); next++ {
				candidate = strings.TrimSpace(lines[next])
				if candidate == "" {
					continue
				}
				if normalizeAssetCode(candidate) == ticker || isInvalidNameCandidate(candidate) {
					continue
				}
				if looksLikeMetadataLabel(candidate) {
					break
				}
				return candidate
			}
		}
	}
	return ""
}

func extractNameFromTitleTag(ticker string, body string) string {
	match := titleTagRe.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	title := strings.Join(strings.Fields(stripHTML(match[1])), " ")
	parts := strings.Split(title, " - ")
	if len(parts) < 2 {
		return ""
	}
	first := normalizeAssetCode(parts[0])
	second := strings.TrimSpace(parts[1])
	if first != ticker || second == "" || normalizeAssetCode(second) == ticker || isInvalidNameCandidate(second) {
		return ""
	}
	switch strings.ToUpper(second) {
	case "FII", "ETF", "AÇÃO", "ACOES", "AÇÕES":
		return ""
	default:
		return second
	}
}

func isInvalidNameCandidate(candidate string) bool {
	if candidate == "Resumo" || candidate == "Sobre" || candidate == "Cotação" {
		return true
	}
	if moneyLikePrefixRe.MatchString(candidate) {
		return true
	}
	if purePercentRe.MatchString(candidate) || percentageLikeRe.MatchString(candidate) {
		return true
	}
	return false
}

func looksLikeMetadataLabel(candidate string) bool {
	upper := strings.ToUpper(candidate)
	if strings.HasSuffix(candidate, ":") {
		return true
	}
	switch upper {
	case "CNPJ", "RAZÃO SOCIAL", "NOME DA EMPRESA", "NOME", "PÚBLICO-ALVO", "MANDATO", "SEGMENTO", "TIPO DE FUNDO", "PRAZO DE DURAÇÃO", "TIPO DE GESTÃO", "TAXA DE ADMINISTRAÇÃO":
		return true
	default:
		return false
	}
}

func extractCNPJ(body string) *string {
	match := cnpjRe.FindString(body)
	if match == "" {
		return nil
	}
	digits := onlyDigits(match)
	if len(digits) != 14 {
		return nil
	}
	return &digits
}

func onlyDigits(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
