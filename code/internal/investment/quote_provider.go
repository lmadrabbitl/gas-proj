package investment

import (
	"context"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultQuoteCacheTTL = 30 * time.Minute

var (
	googlePriceRe     = regexp.MustCompile(`R\$ ?([0-9]+(?:[.,][0-9]+)*)`)
	googleTimestampRe = regexp.MustCompile(`([A-Z][a-z]{2} \d{1,2}, \d{1,2}:\d{2}:\d{2} [AP]M GMT[+-]\d+)`)
	b3PriceRe         = regexp.MustCompile(`asset__info__value[^>]*>\s*([0-9.,]+)\s*<`)
	b3UpdatedAtRe     = regexp.MustCompile(`Atualizado às (\d{2}/\d{2}/\d{4} \d{2}h\d{2})`)
)

type QuoteProvider interface {
	FetchQuotes(ctx context.Context, tickers []string) (map[string]AssetQuote, error)
}

type AssetQuote struct {
	Ticker       string
	CurrentPrice int64
	Timestamp    time.Time
}

type GoogleFinanceQuoteProviderConfig struct {
	Client   *http.Client
	Now      func() time.Time
	CacheTTL time.Duration
}

type GoogleFinanceQuoteProvider struct {
	client   *http.Client
	now      func() time.Time
	cacheTTL time.Duration

	mu    sync.RWMutex
	cache map[string]cachedAssetQuote
}

type cachedAssetQuote struct {
	quote     AssetQuote
	fetchedAt time.Time
}

func NewGoogleFinanceQuoteProvider(cfg GoogleFinanceQuoteProviderConfig) *GoogleFinanceQuoteProvider {
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	nowFn := cfg.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	ttl := cfg.CacheTTL
	if ttl <= 0 {
		ttl = defaultQuoteCacheTTL
	}
	return &GoogleFinanceQuoteProvider{
		client:   client,
		now:      nowFn,
		cacheTTL: ttl,
		cache:    make(map[string]cachedAssetQuote),
	}
}

func (p *GoogleFinanceQuoteProvider) FetchQuotes(ctx context.Context, tickers []string) (map[string]AssetQuote, error) {
	out := make(map[string]AssetQuote, len(tickers))
	if len(tickers) == 0 {
		return out, nil
	}

	now := p.now()
	missing := make([]string, 0, len(tickers))
	for _, rawTicker := range tickers {
		ticker := normalizeAssetCode(rawTicker)
		if ticker == "" {
			continue
		}
		if cached, ok := p.loadCached(ticker, now); ok {
			out[ticker] = cached
			continue
		}
		missing = append(missing, ticker)
	}

	var outMu sync.Mutex
	var wg sync.WaitGroup
	for _, ticker := range missing {
		ticker := ticker
		wg.Add(1)
		go func() {
			defer wg.Done()
			quote, err := p.fetchQuote(ctx, ticker)
			if err != nil {
				log.Printf("investment quote unavailable for %s: %v", ticker, err)
				return
			}
			outMu.Lock()
			out[ticker] = quote
			outMu.Unlock()
			p.storeCached(ticker, quote, now)
		}()
	}
	wg.Wait()
	return out, nil
}

func (p *GoogleFinanceQuoteProvider) loadCached(ticker string, now time.Time) (AssetQuote, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	cached, ok := p.cache[ticker]
	if !ok || now.Sub(cached.fetchedAt) >= p.cacheTTL || cached.quote.CurrentPrice <= 0 {
		return AssetQuote{}, false
	}
	return cached.quote, true
}

func (p *GoogleFinanceQuoteProvider) storeCached(ticker string, quote AssetQuote, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache[ticker] = cachedAssetQuote{quote: quote, fetchedAt: now}
}

func (p *GoogleFinanceQuoteProvider) fetchQuote(ctx context.Context, ticker string) (AssetQuote, error) {
	url, parser := quoteSourceForTicker(ticker)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return AssetQuote{}, fmt.Errorf("build quote request for %s: %w", ticker, err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 gas-proj/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return AssetQuote{}, fmt.Errorf("request quote for %s: %w", ticker, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return AssetQuote{}, fmt.Errorf("quote request failed for %s with status %s", ticker, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return AssetQuote{}, fmt.Errorf("read quote body for %s: %w", ticker, err)
	}
	return parser(ticker, string(body), p.now())
}

type quoteParser func(ticker string, body string, now time.Time) (AssetQuote, error)

func quoteSourceForTicker(ticker string) (string, quoteParser) {
	if strings.EqualFold(ticker, "SPYI11") {
		return fmt.Sprintf("https://borainvestir.b3.com.br/cotacoes/etfs/%s/", strings.ToUpper(ticker)), parseB3Quote
	}
	return fmt.Sprintf("https://www.google.com/finance/beta/quote/%s:BVMF", strings.ToUpper(ticker)), parseGoogleFinanceQuote
}

func parseGoogleFinanceQuote(ticker string, body string, now time.Time) (AssetQuote, error) {
	text := normalizeQuoteWhitespace(stripHTML(body))

	priceMatch := googlePriceRe.FindStringSubmatch(text)
	if len(priceMatch) < 2 {
		return AssetQuote{}, fmt.Errorf("parse Google Finance price for %s", ticker)
	}
	price, err := parseBrazilianPriceToCents(priceMatch[1])
	if err != nil {
		return AssetQuote{}, fmt.Errorf("parse Google Finance price for %s: %w", ticker, err)
	}

	timestamp := now
	timestampMatch := googleTimestampRe.FindStringSubmatch(text)
	if len(timestampMatch) >= 2 {
		parsed, err := parseGoogleFinanceTimestamp(timestampMatch[1], now.Location(), now.Year())
		if err == nil {
			timestamp = parsed
		}
	}

	return AssetQuote{
		Ticker:       ticker,
		CurrentPrice: price,
		Timestamp:    timestamp,
	}, nil
}

func parseB3Quote(ticker string, body string, now time.Time) (AssetQuote, error) {
	priceMatch := b3PriceRe.FindStringSubmatch(body)
	if len(priceMatch) < 2 {
		return AssetQuote{}, fmt.Errorf("parse B3 price for %s", ticker)
	}
	price, err := parseBrazilianPriceToCents(priceMatch[1])
	if err != nil {
		return AssetQuote{}, fmt.Errorf("parse B3 price for %s: %w", ticker, err)
	}

	timestamp := now
	timestampMatch := b3UpdatedAtRe.FindStringSubmatch(body)
	if len(timestampMatch) >= 2 {
		parsed, err := time.ParseInLocation("02/01/2006 15h04", timestampMatch[1], time.Local)
		if err == nil {
			timestamp = parsed
		}
	}

	return AssetQuote{
		Ticker:       ticker,
		CurrentPrice: price,
		Timestamp:    timestamp,
	}, nil
}

func stripHTML(value string) string {
	tagRe := regexp.MustCompile(`<[^>]+>`)
	noTags := tagRe.ReplaceAllString(value, "\n")
	return html.UnescapeString(noTags)
}

func normalizeQuoteWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func parseGoogleFinanceTimestamp(value string, fallbackLoc *time.Location, year int) (time.Time, error) {
	replacer := strings.NewReplacer("GMT-3", "-03:00", "GMT-2", "-02:00", "GMT-4", "-04:00", "GMT+0", "+00:00")
	clean := replacer.Replace(strings.TrimSpace(value))
	parsed, err := time.Parse("Jan 2, 3:04:05 PM -07:00", clean)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(year, parsed.Month(), parsed.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, parsed.Location()).In(fallbackLoc), nil
}

func parseBrazilianPriceToCents(value string) (int64, error) {
	clean := strings.TrimSpace(value)
	switch {
	case strings.Contains(clean, ",") && strings.Contains(clean, "."):
		clean = strings.ReplaceAll(clean, ".", "")
		clean = strings.ReplaceAll(clean, ",", ".")
	case strings.Contains(clean, ","):
		clean = strings.ReplaceAll(clean, ",", ".")
	}

	parsed, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, err
	}
	return int64(parsed*100 + 0.5), nil
}
