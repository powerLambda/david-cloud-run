package optimizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"time"
)

const snapshotTargetURL = "https://dapanyuntu.com/"

var shanghaiLoc = time.FixedZone("Asia/Shanghai", 8*60*60)

// takeScreenshot calls the browserless REST screenshot API and returns JPEG bytes.
func takeScreenshot(ctx context.Context, httpCli *http.Client, browserlessURL, token string) ([]byte, error) {
	body, err := json.Marshal(screenshotReq{
		URL: snapshotTargetURL,
		Viewport: screenshotViewport{
			Width:             1920,
			Height:            1012,
			DeviceScaleFactor: 1,
			IsMobile:          false,
		},
		Options: screenshotOptions{
			Type:    "jpeg",
			Quality: 100,
		},
		GotoOptions: screenshotGoto{
			WaitUntil: "networkidle2",
		},
	})
	if err != nil {
		return nil, err
	}

	endpoint := browserlessURL + "/screenshot?token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("browserless screenshot: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("browserless screenshot status %d: %s", resp.StatusCode, msg)
	}

	return io.ReadAll(resp.Body)
}

// uploadFeishuImage uploads imageData as a JPEG to the Feishu IM image API and returns the image_key.
func uploadFeishuImage(ctx context.Context, httpCli *http.Client, feishuToken string, imageData []byte) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	if err := mw.WriteField("image_type", "message"); err != nil {
		return "", err
	}
	fw, err := mw.CreateFormFile("image", "snapshot.jpg")
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(imageData); err != nil {
		return "", err
	}
	mw.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		feishuBaseURL+"/open-apis/im/v1/images",
		&buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+feishuToken)

	resp, err := httpCli.Do(req)
	if err != nil {
		return "", fmt.Errorf("feishu image upload: %w", err)
	}
	defer resp.Body.Close()

	var ir feishuImageResp
	if err := json.NewDecoder(resp.Body).Decode(&ir); err != nil {
		return "", err
	}
	if ir.Code != 0 {
		return "", fmt.Errorf("feishu image upload error: code=%d msg=%s", ir.Code, ir.Msg)
	}
	return ir.Data.ImageKey, nil
}

func sendMarketSnapshotToGroup(ctx context.Context, httpCli *http.Client, webhookURL, imageKey string, t time.Time) error {
	msg := botPostMsg{
		MsgType: "post",
		Content: botPostContent{
			Post: botPostLangs{
				ZhCn: botPostBody{
					Title:   t.In(shanghaiLoc).Format("2006-01-02 15:04") + " A股大盘云图",
					Content: [][]botPostElem{{{Tag: "img", ImageKey: imageKey}}},
				},
			},
		},
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := httpCli.Do(req)
	if err != nil {
		return fmt.Errorf("feishu webhook: %w", err)
	}
	defer resp.Body.Close()

	var wr botPostResp
	if err := json.NewDecoder(resp.Body).Decode(&wr); err != nil {
		return err
	}
	if wr.StatusCode != 0 {
		return fmt.Errorf("feishu webhook error: code=%d msg=%s", wr.StatusCode, wr.StatusMessage)
	}
	return nil
}

// TakeMarketSnapshot takes a screenshot of the market page and uploads it to Feishu,
// returning the image_key.
func TakeMarketSnapshot(ctx context.Context, cfg Config) (string, error) {
	httpCli := &http.Client{}

	token, err := fetchToken(ctx, httpCli, cfg.AppID, cfg.AppSecret)
	if err != nil {
		return "", fmt.Errorf("fetch feishu token: %w", err)
	}

	imageData, err := takeScreenshot(ctx, httpCli, cfg.BrowserlessURL, cfg.BrowserlessToken)
	if err != nil {
		return "", err
	}
	log.Printf("market snapshot: screenshot size=%d bytes", len(imageData))
	if err := os.WriteFile("/tmp/market_snapshot_latest.jpg", imageData, 0644); err != nil {
		log.Printf("market snapshot: write diagnostic file: %v", err)
	}

	imageKey, err := uploadFeishuImage(ctx, httpCli, token, imageData)
	if err != nil {
		return "", err
	}
	log.Printf("market snapshot: uploaded image_key=%s", imageKey)

	if cfg.SnapshotWebhookURL != "" {
		if err := sendMarketSnapshotToGroup(ctx, httpCli, cfg.SnapshotWebhookURL, imageKey, time.Now()); err != nil {
			log.Printf("market snapshot: send to group failed: %v", err)
		} else {
			log.Printf("market snapshot: sent to group via webhook")
		}
	}

	return imageKey, nil
}
