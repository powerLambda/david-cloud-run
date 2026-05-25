package optimizer

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

// fieldText extracts a string value from a Feishu Bitable field's raw JSON.
// Text/URL fields return [{"text":"...","type":"..."}]; number fields return a bare number.
func fieldText(raw json.RawMessage) string {
	var vals []FieldValue
	if err := json.Unmarshal(raw, &vals); err == nil && len(vals) > 0 {
		return vals[0].Text
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

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
		item := make(PortfolioItem, len(portfolioFieldNames)+1)
		item["record_id"] = r.RecordID
		for _, name := range portfolioFieldNames {
			if raw, ok := r.Fields[name]; ok {
				item[name] = fieldText(raw)
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// UpdatePortfolioPrice writes the current price for each item back to the Feishu Bitable
// "当前市价" column, identified by each item's RecordID. Errors per record are logged
// but do not abort remaining updates.
func UpdatePortfolioPrice(ctx context.Context, cfg Config, items []PortfolioItemWithPrice) error {
	httpCli := &http.Client{}
	token, err := fetchToken(ctx, httpCli, cfg.AppID, cfg.AppSecret)
	if err != nil {
		return err
	}
	records := make([]batchUpdateItem, len(items))
	for i, item := range items {
		records[i] = batchUpdateItem{
			RecordID: item.RecordID,
			Fields:   map[string]interface{}{fieldCurrentPrice: item.Price},
		}
	}
	return batchUpdateRecords(ctx, httpCli, token, bitableBaseID, bitableTableID, records)
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
			RecordID: item["record_id"],
			Name:     item["证券名称"],
			Code:     code,
			Price:    price,
		})
	}
	return result, nil
}
