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

// GetPrices returns the latest known price per ticker owned by the user.
// Prices are fetched from the external API at most once per day, and only
// within the 17:00-23:59 window. Outside the window, the last stored price
// is returned (marked stale when it is not from today).
func (s *investmentService) GetPrices(ctx context.Context, userID uuid.UUID) ([]*model.InvestmentPrice, error) {
	q := model.NewInvestmentQuery()
	q.Limit = 100
	investments, err := s.repo.GetByUserID(ctx, q, userID)
	if err != nil {
		return nil, errors.New("failed to retrieve investments")
	}

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
