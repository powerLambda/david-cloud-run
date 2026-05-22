package optimizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// feishuBaseURL is the Feishu API root. Overridable in tests.
var feishuBaseURL = "https://open.feishu.cn"

func fetchToken(ctx context.Context, httpCli *http.Client, appID, appSecret string) (string, error) {
	body, err := json.Marshal(tokenReq{AppID: appID, AppSecret: appSecret})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		feishuBaseURL+"/open-apis/auth/v3/tenant_access_token/internal",
		bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := httpCli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tr tokenResp
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	if tr.Code != 0 {
		return "", fmt.Errorf("feishu token error: code=%d msg=%s", tr.Code, tr.Msg)
	}
	return tr.TenantAccessToken, nil
}

func updateRecord(ctx context.Context, httpCli *http.Client, token, baseID, tableID, recordID string, fields map[string]interface{}) error {
	bodyBytes, err := json.Marshal(map[string]interface{}{"fields": fields})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/open-apis/bitable/v1/apps/%s/tables/%s/records/%s",
		feishuBaseURL, baseID, tableID, recordID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpCli.Do(req)
	if err != nil {
		return fmt.Errorf("updateRecord: %w", err)
	}
	defer resp.Body.Close()

	var ur updateResp
	if err := json.NewDecoder(resp.Body).Decode(&ur); err != nil {
		return err
	}
	if ur.Code != 0 {
		return fmt.Errorf("feishu update error: code=%d msg=%s", ur.Code, ur.Msg)
	}
	return nil
}

func searchRecords(ctx context.Context, httpCli *http.Client, token, baseID, tableID string, body searchReqBody) ([]record, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/open-apis/bitable/v1/apps/%s/tables/%s/records/search",
		feishuBaseURL, baseID, tableID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var sr searchResp
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}
	if sr.Code != 0 {
		return nil, fmt.Errorf("feishu bitable error: code=%d msg=%s", sr.Code, sr.Msg)
	}
	return sr.Data.Items, nil
}
