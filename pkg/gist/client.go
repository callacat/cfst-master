package gist

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"
	"strings"

	"controller/pkg/models"
)

type Client struct {
	token       string
	proxyPrefix string // [新增]
	httpClient  *http.Client
}

func NewClient(token, proxyPrefix string) *Client {
	return &Client{
		token:       token,
		proxyPrefix: proxyPrefix,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// [修改] 增加重试逻辑
func (c *Client) doRequestWithRetry(req *http.Request, maxRetries int) (*http.Response, error) {
	var err error
	var resp *http.Response
	for i := 0; i < maxRetries; i++ {
		resp, err = c.httpClient.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}
		log.Printf("[warn] Request to %s failed (attempt %d/%d): %v, status: %s", req.URL, i+1, maxRetries, err, resp.Status)
		time.Sleep(time.Second * time.Duration(2*i)) // Exponential backoff
	}
	return resp, err
}

// [修改] 应用代理和重试
func (c *Client) FetchDeviceResults(gistID string) ([]models.DeviceResult, error) {
	url := c.buildURL("https://api.github.com/gists/" + gistID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.doRequestWithRetry(req, 3)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var gist struct {
		Files map[string]struct {
			RawURL string `json:"raw_url"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gist); err != nil {
		return nil, err
	}

	// 查找第一个 JSON 文件
	var rawURL string
	for _, file := range gist.Files {
		if strings.HasSuffix(strings.ToLower(file.RawURL), ".json") {
			rawURL = c.buildURL(file.RawURL)
			break
		}
	}
	if rawURL == "" {
		return nil, nil // No JSON file found
	}
	
	req, _ = http.NewRequest("GET", rawURL, nil)
	data, err := c.doRequestWithRetry(req, 3)
	if err != nil {
		return nil, err
	}
	defer data.Body.Close()
	
	body, _ := io.ReadAll(data.Body)
	var drs []models.DeviceResult
	if err := json.Unmarshal(body, &drs); err != nil {
		return nil, err
	}
	return drs, nil
}


// [修改] 应用代理和重试
func (c *Client) CreateOrUpdateResultGist(gistID string, fr models.FinalResult) (string, error) {
	content, _ := json.MarshalIndent(fr, "", "  ")
	bodyMap := map[string]interface{}{
		"description": "Multi-Net 优选 IP 结果",
		"public":      false,
		"files": map[string]map[string]string{
			"selected.json": {"content": string(content)},
		},
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	var method, url string
	if gistID == "" {
		method = "POST"
		url = c.buildURL("https://api.github.com/gists")
	} else {
		method = "PATCH"
		url = c.buildURL("https://api.github.com/gists/" + gistID)
	}

	req, _ := http.NewRequest(method, url, bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequestWithRetry(req, 3)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	var respObj struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respObj); err != nil {
		return "", err
	}
	return respObj.ID, nil
}

// [新增] 统一处理 URL 代理前缀
func (c *Client) buildURL(originalURL string) string {
	if c.proxyPrefix == "" {
		return originalURL
	}
	return c.proxyPrefix + originalURL
}