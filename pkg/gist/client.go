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

// ... Client struct, NewClient, doRequestWithRetry, buildURL (无变化) ...
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


// [修改] 重构 FetchDeviceResults，增加日志并修复 Bug
func (c *Client) FetchDeviceResults(gistID string, maxAgeHours int) ([]models.DeviceResult, error) {
	log.Printf("[info] ---> Fetching data from Gist ID: %s", gistID)
	url := c.buildURL("https://api.github.com/gists/" + gistID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.doRequestWithRetry(req, 3)
	// [修复] 检查 resp 和 err，防止 nil pointer panic
	if err != nil {
		log.Printf("[error] Failed to fetch Gist %s after retries: %v", gistID, err)
		return nil, err
	}
	if resp == nil {
		log.Printf("[error] Received nil response for Gist %s after retries", gistID)
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
		log.Printf("[info] Gist %s is too old (updated at %v), skipping.", gistID, gist.UpdatedAt)
		return nil, nil
	}

	var allResults []models.DeviceResult
	re := regexp.MustCompile(`results-(ct|cu|cm)-.*-(v4|v6)\.json`)

	for _, file := range gist.Files {
		log.Printf("[info]     Processing file: %s", file.Filename)
		matches := re.FindStringSubmatch(strings.ToLower(file.Filename))
		if len(matches) != 3 {
			continue
		}
		operator, ipVersion := matches[1], matches[2]

		req, _ = http.NewRequest("GET", c.buildURL(file.RawURL), nil)
		dataResp, err := c.doRequestWithRetry(req, 3)
		if err != nil || dataResp == nil {
			log.Printf("[warn]     Failed to download content from %s. Skipping file.", file.RawURL)
			continue
		}
		defer dataResp.Body.Close()
		
		body, _ := io.ReadAll(dataResp.Body)
		var drs []models.DeviceResult
		if err := json.Unmarshal(body, &drs); err != nil {
			log.Printf("[warn]     Failed to unmarshal content from %s. Skipping file.", file.Filename)
			continue
		}
		
		for i := range drs {
			drs[i].Operator = operator
			drs[i].IPVersion = ipVersion
		}
		allResults = append(allResults, drs...)
		log.Printf("[info]     Successfully processed file %s, found %d results.", file.Filename, len(drs))
	}
	log.Printf("[info] <--- Finished fetching Gist %s, total results found: %d", gistID, len(allResults))
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
		log.Printf("[error] Failed to create/update result Gist")
		return "", fmt.Errorf("failed to create/update result Gist")
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