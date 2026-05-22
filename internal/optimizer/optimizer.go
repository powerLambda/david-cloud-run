package optimizer

import (
	"context"
	"log"
	"net/http"
)

// FetchPortfolioCode authenticates to Feishu, queries the Bitable table using
// the hardcoded view and field constants, and returns one PortfolioItem per record.
// Each item maps field name → text value extracted from the first FieldValue element.
func FetchPortfolioCode(ctx context.Context, cfg Config) ([]PortfolioItem, error) {
	httpCli := &http.Client{}

	token, err := fetchToken(ctx, httpCli, cfg.AppID, cfg.AppSecret)
	if err != nil {
		return nil, err
	}

	body := searchReqBody{
		ViewID:          bitableViewID,
		FieldNames:      portfolioFieldNames,
		AutomaticFields: false,
	}

	records, err := searchRecords(ctx, httpCli, token, bitableBaseID, bitableTableID, body)
	if err != nil {
		return nil, err
	}

	items := make([]PortfolioItem, 0, len(records))
	for _, r := range records {
		item := make(PortfolioItem, len(portfolioFieldNames))
		for _, name := range portfolioFieldNames {
			if vals, ok := r.Fields[name]; ok && len(vals) > 0 {
				item[name] = vals[0].Text
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// QueryPortfolioPrice enriches a slice of PortfolioItems with current market prices.
// It is independent of FetchPortfolioCode and can be called with any []PortfolioItem.
func QueryPortfolioPrice(ctx context.Context, items []PortfolioItem) ([]PortfolioItemWithPrice, error) {
	httpCli := &http.Client{}
	result := make([]PortfolioItemWithPrice, 0, len(items))
	for _, item := range items {
		code := item["证券代码"]
		price, err := fetchPrice(ctx, httpCli, code)
		if err != nil {
			log.Printf("optimizer: price fetch error for %q: %v", code, err)
		}
		result = append(result, PortfolioItemWithPrice{
			Name:  item["证券名称"],
			Code:  code,
			Price: price,
		})
	}
	return result, nil
}
