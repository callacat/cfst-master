package gist

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp" // [新增]
	"strings"
	"time"

	"controller/pkg/models"
)

type GistFile struct {
	Filename string    `json:"filename"`
	RawURL   string    `json:"raw_url"`
}

type Gist struct {
	Files     map[string]GistFile `json:"files"`
	UpdatedAt time.Time           `json:"updated_at"`
}

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

func (c *Client) doRequestWithRetry(req *http.Request, maxRetries int) (*http.Response, error) {
	var err error
	var resp *http.Response
	for i := 0; i < maxRetries; i++ {
		resp, err = c.httpClient.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		// [修复] 增加对 resp 是否为 nil 的检查
		status := "N/A"
		if resp != nil {
			status = resp.Status
		}
		log.Printf("[warn] Request to %s failed (attempt %d/%d): %v, status: %s", req.URL, i+1, maxRetries, err, status)

		// [修复] 如果 resp 不为 nil，需要关闭它以防资源泄露
		if resp != nil {
			resp.Body.Close()
		}

		time.Sleep(time.Second * time.Duration(2*i)) // Exponential backoff
	}
	return resp, err
}

// [修改] 应用代理和重试
func (c *Client) FetchDeviceResults(gistID string, maxAgeHours int) ([]models.DeviceResult, error) {
	url := c.buildURL("https://api.github.com/gists/" + gistID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.doRequestWithRetry(req, 3)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var gist Gist
	if err := json.NewDecoder(resp.Body).Decode(&gist); err != nil {
		return nil, err
	}

	// [新增] 需求2: 过滤超过指定时间的 Gist
	if maxAgeHours > 0 && time.Since(gist.UpdatedAt) > time.Duration(maxAgeHours)*time.Hour {
		log.Printf("[info] Gist %s is too old (updated at %v), skipping.", gistID, gist.UpdatedAt)
		return nil, nil // 返回空，表示正常跳过
	}

	var allResults []models.DeviceResult
	// [新增] 正则表达式用于解析文件名, e.g., "results-cu-my-home-lt-v4.json"
	re := regexp.MustCompile(`results-(ct|cu|cm)-.*-(v4|v6)\.json`)

	// [修改] 遍历 Gist 中的所有文件
	for _, file := range gist.Files {
		matches := re.FindStringSubmatch(strings.ToLower(file.Filename))
		if len(matches) != 3 {
			log.Printf("[warn] Filename '%s' in Gist %s does not match expected format, skipping.", file.Filename, gistID)
			continue
		}

		operator := matches[1]  // e.g., "cu"
		ipVersion := matches[2] // e.g., "v4"

		// 下载文件内容
		req, _ = http.NewRequest("GET", c.buildURL(file.RawURL), nil)
		dataResp, err := c.doRequestWithRetry(req, 3)
		if err != nil {
			log.Printf("[warn] Failed to download file %s: %v", file.RawURL, err)
			continue
		}
		defer dataResp.Body.Close()

		body, _ := io.ReadAll(dataResp.Body)
		var drs []models.DeviceResult
		if err := json.Unmarshal(body, &drs); err != nil {
			log.Printf("[warn] Failed to unmarshal content from %s: %v", file.Filename, err)
			continue
		}

		// [核心] 使用从文件名解析出的信息来填充数据
		for i := range drs {
			drs[i].Operator = operator
			drs[i].IPVersion = ipVersion
		}
		allResults = append(allResults, drs...)
	}

	return allResults, nil
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