// 请将此文件内容完全替换为以下代码

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

// [修改] 参数 maxAgeMinutes int
func (c *Client) FetchDeviceResults(gistID string, maxAgeMinutes int) ([]models.DeviceResult, error) {
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
	
	// [修改] 使用分钟进行时间比较
	if maxAgeMinutes > 0 && time.Since(gist.UpdatedAt) > time.Duration(maxAgeMinutes)*time.Minute {
		log.Printf("[info]     Gist %s is too old (updated at %v), skipping.", gistID, gist.UpdatedAt)
		return nil, nil
	}

	var allResults []models.DeviceResult
	re := regexp.MustCompile(`results6?-(ct|cu|cm)-.*-(v4|v6)\.json`)

	for _, file := range gist.Files {
		matches := re.FindStringSubmatch(strings.ToLower(file.Filename))
		if len(matches) != 3 {
			continue
		}
		operator, ipVersion := matches[1], matches[2]
		log.Printf("[info]     + Processing matching file: %s (Operator: %s, IPVersion: %s)", file.Filename, operator, ipVersion)

		finalDownloadURL := c.buildURL(file.RawURL)
		req, _ = http.NewRequest("GET", finalDownloadURL, nil)
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

// [重构] CreateOrUpdateResultGist 现在接收一个文件名到内容的映射
func (c *Client) CreateOrUpdateResultGist(gistID string, filesToUpload map[string]string) (string, error) {
	if len(filesToUpload) == 0 {
		log.Println("[info] No files to upload to Gist. Skipping.")
		return gistID, nil
	}

	fileMap := make(map[string]map[string]string)
	for filename, content := range filesToUpload {
		fileMap[filename] = map[string]string{"content": content}
	}

	bodyMap := map[string]interface{}{
		"description": "Multi-Net 优选 IP 结果 (分线路)",
		"public":      false,
		"files":       fileMap,
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