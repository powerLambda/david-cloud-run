package optimizer

import "encoding/json"

type tokenReq struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

type tokenResp struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
}

type searchReqBody struct {
	ViewID          string   `json:"view_id,omitempty"`
	FieldNames      []string `json:"field_names,omitempty"`
	AutomaticFields bool     `json:"automatic_fields"`
}

// FieldValue is one element in a Bitable field array, e.g. {"text":"159136_SZ","type":"text"}.
type FieldValue struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type searchResp struct {
	Code int        `json:"code"`
	Msg  string     `json:"msg"`
	Data searchData `json:"data"`
}

type searchData struct {
	HasMore bool     `json:"has_more"`
	Items   []record `json:"items"`
	Total   int      `json:"total"`
}

type record struct {
	RecordID string                     `json:"record_id"`
	Fields   map[string]json.RawMessage `json:"fields"`
}

// PortfolioItem holds extracted text values keyed by field name,
// e.g. {"证券名称": "A50ETF广发", "证券代码": "sz159136"}.
type PortfolioItem map[string]string

// PortfolioItemWithPrice combines bitable data with the current market price.
type PortfolioItemWithPrice struct {
	RecordID string  `json:"record_id"`
	Name     string  `json:"name"`
	Code     string  `json:"code"`
	Price    float64 `json:"price"`
}

type batchUpdateItem struct {
	RecordID string                 `json:"record_id"`
	Fields   map[string]interface{} `json:"fields"`
}

type batchUpdateReqBody struct {
	Records []batchUpdateItem `json:"records"`
}

type batchUpdateResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}
