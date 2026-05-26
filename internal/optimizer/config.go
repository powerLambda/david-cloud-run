package optimizer

import (
	"os"
	"strings"
	"time"
)

const (
	bitableBaseID    = "AXiQbIeOIabJxcsDS6ScdMg5nub" // Feishu Bitable app token
	bitableTableID   = "tbl0UyerHzv2pDbY"            // Feishu Bitable table ID
	bitableViewID    = "vewgMuvngQ"
	fieldCurrentPrice = "当前市价"
	requestTimeout  = 15 * time.Second
	snapshotTimeout = 60 * time.Second
)

var portfolioFieldNames = []string{"证券名称", "证券代码"}

type Config struct {
	AppID            string
	AppSecret        string
	BrowserlessURL   string
	BrowserlessToken string
}

func LoadConfig() Config {
	return Config{
		AppID:            strings.TrimSpace(os.Getenv("FEISHU_APP_ID")),
		AppSecret:        strings.TrimSpace(os.Getenv("FEISHU_APP_SECRET")),
		BrowserlessURL:   strings.TrimSpace(os.Getenv("BROWSERLESS_URL")),
		BrowserlessToken: strings.TrimSpace(os.Getenv("BROWSERLESS_TOKEN")),
	}
}
