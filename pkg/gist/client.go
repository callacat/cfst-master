// 请将文件 pkg/gist/client.go 的内容完全替换为以下代码

package gist

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"controller/pkg/models"
)

const browserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"

type Client struct {
	token       string
	proxyPrefix string
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
	// ... (此函数无需修改，保持原样)
	req.Header.Set("User-Agent", browserUserAgent)
	var err error
	var resp *http.Response
	for i := 0; i < maxRetries; i++ {
		resp, err = c.httpClient.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}
		status := "N/A"
		if resp != nil {
			status = resp.Status
		}
		log.Printf("[warn] Request to %s failed (attempt %d/%d): %v, status: %s", req.URL, i+1, maxRetries, err, status)
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(time.Second * time.Duration(2*i))
	}
	return resp, err
}

// [核心修改] FetchDeviceResults 适配新的数据格式和文件名解析逻辑
func (c *Client) FetchDeviceResults(gistID string, maxAgeHours int) ([]models.DeviceResult, error) {
	log.Printf("[info] ---> Fetching data from Gist ID: %s", gistID)
	apiRequestURL := c.buildURL("https://api.github.com/gists/" + gistID)
	req, _ := http.NewRequest("GET", apiRequestURL, nil)
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.doRequestWithRetry(req, 3)
	if err != nil || resp == nil {
		return nil, fmt.Errorf("failed to fetch Gist metadata for %s: %v", gistID, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Gist metadata response body for %s: %v", gistID, err)
	}

	var gist struct {
		Files map[string]struct {
			Filename string `json:"filename"`
			RawURL   string `json:"raw_url"`
		} `json:"files"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	if err := json.Unmarshal(bodyBytes, &gist); err != nil {
		return nil, fmt.Errorf("failed to decode Gist JSON for %s: %v", gistID, err)
	}

	if maxAgeHours > 0 && time.Since(gist.UpdatedAt) > time.Duration(maxAgeHours)*time.Hour {
		log.Printf("[info]     Gist %s is too old (updated at %v), skipping.", gistID, gist.UpdatedAt)
		return nil, nil
	}

	var allResults []models.DeviceResult
	// [修改] 使用新的正则表达式来捕获 operator 和 ipVersion
	re := regexp.MustCompile(`results6?-(ct|cu|cm)-.*-(v4|v6)\.json`)

	for _, file := range gist.Files {
		matches := re.FindStringSubmatch(strings.ToLower(file.Filename))
		if len(matches) != 3 { // 需要匹配到 (文件名, operator, ipVersion) 这3个部分
			continue
		}
		operator, ipVersion := matches[1], matches[2]
		log.Printf("[info]     + Processing matching file: %s (Operator: %s, IPVersion: %s)", file.Filename, operator, ipVersion)

		finalDownloadURL := c.buildURL(file.RawURL)
		req, _ = http.NewRequest("GET", finalDownloadURL, nil) // Re-create request for retry logic
		dataResp, err := c.doRequestWithRetry(req, 3)
		if err != nil || dataResp == nil {
			log.Printf("[warn]       Failed to download content for %s. Skipping.", file.Filename)
			continue
		}
		defer dataResp.Body.Close()

		body, _ := io.ReadAll(dataResp.Body)
		var data struct {
			Results []models.DeviceResult `json:"results"`
		}
		if err := json.Unmarshal(body, &data); err != nil {
			log.Printf("[warn]       Failed to unmarshal content from %s. Error: %v", file.Filename, err)
			continue
		}

		// [修改] 将从文件名中解析出的信息填充到每条记录中
		for i := range data.Results {
			data.Results[i].Operator = operator
			data.Results[i].IPVersion = ipVersion
		}

		allResults = append(allResults, data.Results...)
		log.Printf("[info]       Successfully processed %s, found %d valid results.", file.Filename, len(data.Results))
	}
	log.Printf("[info] <--- Finished fetching Gist %s, total valid results gathered: %d", gistID, len(allResults))
	return allResults, nil
}

// CreateOrUpdateResultGist 无需修改，保持原样
func (c *Client) CreateOrUpdateResultGist(gistID string, fr models.FinalResult) (string, error) {
	// ... (此函数无需修改，保持原样)
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
		log.Println("[info] ---> Creating new result Gist...")
	} else {
		method = "PATCH"
		url = c.buildURL("https://api.github.com/gists/" + gistID)
		log.Printf("[info] ---> Updating result Gist: %s", gistID)
	}

	req, _ := http.NewRequest(method, url, bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequestWithRetry(req, 3)
	if err != nil || resp == nil {
		return nil, fmt.Errorf("failed to create/update result Gist after retries: %v", err)
	}
	defer resp.Body.Close()

	var respObj struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respObj); err != nil {
		return "", err
	}
	log.Println("[info] <--- Successfully created/updated result Gist.")
	return respObj.ID, nil
}

func (c *Client) buildURL(originalURL string) string {
	if c.proxyPrefix == "" {
		return originalURL
	}
	return c.proxyPrefix + originalURL
}
