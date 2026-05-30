package optimizer

type screenshotReq struct {
	URL         string             `json:"url"`
	Viewport    screenshotViewport `json:"viewport"`
	Options     screenshotOptions  `json:"options"`
	GotoOptions screenshotGoto     `json:"gotoOptions"`
}

type screenshotViewport struct {
	Width             int  `json:"width"`
	Height            int  `json:"height"`
	DeviceScaleFactor int  `json:"deviceScaleFactor"`
	IsMobile          bool `json:"isMobile"`
}

type screenshotOptions struct {
	Type    string `json:"type"`
	Quality int    `json:"quality"`
}

type screenshotGoto struct {
	WaitUntil string `json:"waitUntil"`
}

type feishuImageResp struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data feishuImageData `json:"data"`
}

type feishuImageData struct {
	ImageKey string `json:"image_key"`
}

type botPostMsg struct {
	MsgType string         `json:"msg_type"`
	Content botPostContent `json:"content"`
}

type botPostContent struct {
	Post botPostLangs `json:"post"`
}

type botPostLangs struct {
	ZhCn botPostBody `json:"zh_cn"`
}

type botPostBody struct {
	Title   string         `json:"title"`
	Content [][]botPostElem `json:"content"`
}

type botPostElem struct {
	Tag      string `json:"tag"`
	Text     string `json:"text,omitempty"`
	ImageKey string `json:"image_key,omitempty"`
}

type botPostResp struct {
	StatusCode    int    `json:"StatusCode"`
	StatusMessage string `json:"StatusMessage"`
}
