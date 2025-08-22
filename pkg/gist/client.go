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

// [新增] 定义一个标准的浏览器 User-Agent
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
	// [修改] 为所有请求设置 User-Agent
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

func (c *Client) FetchDeviceResults(gistID string, maxAgeHours int) ([]models.DeviceResult, error) {
	log.Printf("[info] ---> Fetching data from Gist ID: %s", gistID)
	url := c.buildURL("https://api.github.com/gists/" + gistID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.doRequestWithRetry(req, 3)
	if err != nil {
		log.Printf("[error] Failed to fetch Gist %s after all retries: %v", gistID, err)
		return nil, err
	}
	if resp == nil {
		log.Printf("[error] Received a nil response for Gist %s after all retries.", gistID)
		return nil, fmt.Errorf("received nil response for Gist %s", gistID)
	}
	defer resp.Body.Close()

	var gist struct {
		Files     map[string]struct{ Filename, RawURL string } `json:"files"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gist); err != nil {
		log.Printf("[error] Failed to decode Gist JSON for %s: %v", gistID, err)
		return nil, err
	}

	if maxAgeHours > 0 && time.Since(gist.UpdatedAt) > time.Duration(maxAgeHours)*time.Hour {
		log.Printf("[info]     Gist %s is too old (updated at %v), skipping.", gistID, gist.UpdatedAt)
		return nil, nil
	}
	var allResults []models.DeviceResult
	re := regexp.MustCompile(`results6?-(ct|cu|cm)-.*-(v4|v6)\.json`)

	log.Printf("[info]     Found %d file(s) in Gist %s. Processing them now...", len(gist.Files), gistID)
	for _, file := range gist.Files {
		matches := re.FindStringSubmatch(strings.ToLower(file.Filename))
		if len(matches) != 3 {
			log.Printf("[info]     - Skipping file '%s' as it does not match required name format.", file.Filename)
			continue
		}
		log.Printf("[info]     + Processing matching file: %s", file.Filename)
		operator, ipVersion := matches[1], matches[2]

		// 注意：这里的 NewRequest 创建的 req 会在 doRequestWithRetry 中被设置 User-Agent
		req, _ = http.NewRequest("GET", c.buildURL(file.RawURL), nil)
		dataResp, err := c.doRequestWithRetry(req, 3)
		if err != nil || dataResp == nil {
			log.Printf("[warn]       Failed to download content from %s. Skipping file.", file.RawURL)
			continue
		}
		defer dataResp.Body.Close()
		
		body, _ := io.ReadAll(dataResp.Body)
		var drs []models.DeviceResult
		if err := json.Unmarshal(body, &drs); err != nil {
			log.Printf("[warn]       Failed to unmarshal content from %s. Error: %v", file.Filename, err)
			// 仍然保留调试日志，以防万一
			bodyStr := string(body)
			debugContent := bodyStr
			if len(bodyStr) > 500 {
				debugContent = bodyStr[:500]
			}
			log.Printf("[debug]      Received unexpected content for %s:\n--BEGIN--\n%s\n---END---", file.Filename, debugContent)
			continue
		}
		
		for i := range drs {
			drs[i].Operator = operator
			drs[i].IPVersion = ipVersion
		}
		allResults = append(allResults, drs...)
		log.Printf("[info]       Successfully processed file %s, found %d valid results.", file.Filename, len(drs))
	}
	log.Printf("[info] <--- Finished fetching Gist %s, total valid results gathered: %d", gistID, len(allResults))
	return allResults, nil
}

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
		log.Printf("[error] Failed to create/update result Gist after retries.")
		return "", fmt.Errorf("failed to create/update result Gist after retries: %v", err)
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