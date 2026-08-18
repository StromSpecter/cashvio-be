package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/google/uuid"
)

const jakartaTZ = "Asia/Jakarta"

func jakartaNow() time.Time {
	loc, err := time.LoadLocation(jakartaTZ)
	if err != nil {
		return time.Now()
	}
	return time.Now().In(loc)
}

// priceWindowOpen reports whether now is inside the 17:00-23:59 window (WIB).
func priceWindowOpen(now time.Time) bool {
	return now.Hour() >= 17 && now.Hour() <= 23
}

type goapiPriceResult struct {
	Symbol  string `json:"symbol"`
	Company struct {
		Name string `json:"name"`
	} `json:"company"`
	Date      string  `json:"date"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    int64   `json:"volume"`
	Change    float64 `json:"change"`
	ChangePct float64 `json:"change_pct"`
}

type goapiPriceResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Results []goapiPriceResult `json:"results"`
	} `json:"data"`
}

type logamPriceResult struct {
	Source       string  `json:"source"`
	Material     string  `json:"material"`
	MaterialType string  `json:"materialType"`
	Weight       float64 `json:"weight"`
	WeightUnit   string  `json:"weightUnit"`
	SellPrice    float64 `json:"sellPrice"`
	BuybackPrice float64 `json:"buybackPrice"`
	Currency     string  `json:"currency"`
	RecordedDate string  `json:"recordedDate"`
	DisplayName  string  `json:"displayName"`
}

type logamPriceResponse struct {
	Success bool               `json:"success"`
	Data    []logamPriceResult `json:"data"`
}

// GetPrices returns the latest known price per asset owned by the user.
// Stock prices are fetched from the external IDX API at most once per day,
// only within the 17:00-23:59 window. Precious metal (gold) prices are
// fetched live from the logam mulia API keyed by source.
func (s *investmentService) GetPrices(ctx context.Context, userID uuid.UUID) ([]*model.InvestmentPrice, error) {
	q := model.NewInvestmentQuery()
	q.Limit = 100
	investments, err := s.repo.GetByUserID(ctx, q, userID)
	if err != nil {
		return nil, errors.New("failed to retrieve investments")
	}

	var stockInvs, goldInvs []*model.Investment
	for _, inv := range investments {
		if inv.Type == "gold" {
			goldInvs = append(goldInvs, inv)
		} else {
			stockInvs = append(stockInvs, inv)
		}
	}

	out := make([]*model.InvestmentPrice, 0, len(investments))

	if len(goldInvs) > 0 {
		goldPrices, err := s.fetchGoldPrices(ctx, goldInvs)
		if err != nil {
			return nil, err
		}
		out = append(out, goldPrices...)
	}

	if len(stockInvs) > 0 {
		stockPrices, err := s.fetchStockPrices(ctx, stockInvs)
		if err != nil {
			return nil, err
		}
		out = append(out, stockPrices...)
	}

	return out, nil
}

func (s *investmentService) fetchStockPrices(ctx context.Context, investments []*model.Investment) ([]*model.InvestmentPrice, error) {
	symbols := distinctSymbols(investments)
	if len(symbols) == 0 {
		return []*model.InvestmentPrice{}, nil
	}

	nameBySymbol := make(map[string]string, len(investments))
	for _, inv := range investments {
		ticker := strings.ToUpper(strings.TrimSpace(inv.Ticker))
		if ticker == "" {
			continue
		}
		if _, ok := nameBySymbol[ticker]; !ok {
			nameBySymbol[ticker] = inv.Name
		}
	}

	now := jakartaNow()
	today := now.Format("2006-01-02")

	haveToday := make(map[string]bool, len(symbols))
	var needFetch []string
	for _, sym := range symbols {
		ok, err := s.stockRepo.ExistsToday(ctx, sym, today)
		if err != nil {
			return nil, errors.New("failed to check stock price")
		}
		if ok {
			haveToday[sym] = true
		} else {
			needFetch = append(needFetch, sym)
		}
	}

	if len(needFetch) > 0 && priceWindowOpen(now) {
		results, err := s.fetchPrices(ctx, needFetch)
		if err != nil {
			return nil, err
		}
		for _, r := range results {
			sym := strings.ToUpper(r.Symbol)
			haveToday[sym] = true
			if r.Company.Name != "" {
				nameBySymbol[sym] = r.Company.Name
			}
		}
	}

	latest, err := s.stockRepo.GetLatestBySymbols(ctx, symbols)
	if err != nil {
		return nil, errors.New("failed to retrieve stock prices")
	}
	priceBySymbol := make(map[string]*model.StockPrice, len(latest))
	for _, p := range latest {
		priceBySymbol[p.Symbol] = p
	}

	out := make([]*model.InvestmentPrice, 0, len(symbols))
	for _, sym := range symbols {
		ip := &model.InvestmentPrice{Symbol: sym, Name: nameBySymbol[sym]}
		if p, ok := priceBySymbol[sym]; ok {
			ip.Date = p.DateString()
			ip.Price = p.Close
			ip.Change = p.Change
			ip.ChangePct = p.ChangePct
			ip.Stale = !haveToday[sym]
		} else {
			ip.Stale = true
		}
		out = append(out, ip)
	}

	return out, nil
}

func (s *investmentService) fetchGoldPrices(ctx context.Context, investments []*model.Investment) ([]*model.InvestmentPrice, error) {
	gramsBySource := make(map[string]float64)
	for _, inv := range investments {
		source := strings.ToLower(strings.TrimSpace(inv.App))
		if source == "" {
			continue
		}
		gramsBySource[source] += inv.Units
	}

	if len(gramsBySource) == 0 {
		return []*model.InvestmentPrice{}, nil
	}

	now := jakartaNow()
	today := now.Format("2006-01-02")

	out := make([]*model.InvestmentPrice, 0, len(gramsBySource))
	for source, grams := range gramsBySource {
		entries, err := s.fetchLogamPrices(ctx, source)
		if err != nil {
			return nil, err
		}

		entry, ok := bestLogamEntry(entries, grams)
		if !ok {
			continue
		}
		perGram := perGramPrice(entry)
		if perGram <= 0 {
			continue
		}

		stale := entry.RecordedDate != today
		out = append(out, &model.InvestmentPrice{
			Symbol:    source,
			Name:      entry.DisplayName,
			Date:      entry.RecordedDate,
			Price:     perGram,
			Change:    0,
			ChangePct: 0,
			Stale:     stale,
		})
	}

	return out, nil
}

func (s *investmentService) fetchLogamPrices(ctx context.Context, source string) ([]logamPriceResult, error) {
	url := s.cfg.LogamAPI.BaseURL + "/" + source
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.New("failed to build logam price request")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, errors.New("failed to fetch logam prices")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("logam price service unavailable")
	}

	var body logamPriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, errors.New("failed to parse logam prices")
	}

	return body.Data, nil
}

// perGramPrice returns the price per gram using buyback when available,
// falling back to the sell price.
func perGramPrice(e logamPriceResult) float64 {
	if e.Weight <= 0 {
		return 0
	}
	if e.BuybackPrice > 0 {
		return e.BuybackPrice / e.Weight
	}
	return e.SellPrice / e.Weight
}

// bestLogamEntry picks the gold entry whose weight best matches the held grams.
func bestLogamEntry(entries []logamPriceResult, grams float64) (logamPriceResult, bool) {
	var best logamPriceResult
	found := false
	for _, e := range entries {
		if strings.ToLower(e.Material) != "gold" || e.Weight <= 0 || perGramPrice(e) <= 0 {
			continue
		}
		if !found {
			best = e
			found = true
			continue
		}
		if betterLogamEntry(e, best, grams) {
			best = e
		}
	}
	return best, found
}

func betterLogamEntry(candidate, current logamPriceResult, grams float64) bool {
	cd := abs(candidate.Weight - grams)
	cdCurrent := abs(current.Weight - grams)
	if cd != cdCurrent {
		return cd < cdCurrent
	}
	return candidate.Weight < current.Weight
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func (s *investmentService) fetchPrices(ctx context.Context, symbols []string) ([]goapiPriceResult, error) {
	url := s.cfg.GoAPI.PricesURL + "?symbols=" + strings.Join(symbols, ",")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.New("failed to build stock price request")
	}
	req.Header.Set("X-API-KEY", s.cfg.GoAPI.Key)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, errors.New("failed to fetch stock prices")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("stock price service unavailable")
	}

	var body goapiPriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, errors.New("failed to parse stock prices")
	}

	now := time.Now()
	rows := make([]*model.StockPrice, 0, len(body.Data.Results))
	for _, r := range body.Data.Results {
		date, err := time.Parse("2006-01-02", r.Date)
		if err != nil {
			date = now
		}
		rows = append(rows, &model.StockPrice{
			ID:        uuid.New(),
			Symbol:    strings.ToUpper(r.Symbol),
			Date:      date,
			Open:      r.Open,
			High:      r.High,
			Low:       r.Low,
			Close:     r.Close,
			Volume:    r.Volume,
			Change:    r.Change,
			ChangePct: r.ChangePct,
			FetchedAt: now,
		})
	}

	if len(rows) > 0 {
		if err := s.stockRepo.UpsertMany(ctx, rows); err != nil {
			return nil, errors.New("failed to store stock prices")
		}
	}

	return body.Data.Results, nil
}

func distinctSymbols(investments []*model.Investment) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(investments))
	for _, inv := range investments {
		ticker := strings.ToUpper(strings.TrimSpace(inv.Ticker))
		if ticker == "" || ticker == "—" || seen[ticker] {
			continue
		}
		seen[ticker] = true
		out = append(out, ticker)
	}
	return out
}
