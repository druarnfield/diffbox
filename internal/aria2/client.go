package aria2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

type Client struct {
	url        string
	secret     string
	counter    uint64
	httpClient *http.Client
}

type Request struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      string        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type Response struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *RPCError       `json:"error"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type DownloadStatus struct {
	GID             string `json:"gid"`
	Status          string `json:"status"`
	TotalLength     string `json:"totalLength"`
	CompletedLength string `json:"completedLength"`
	DownloadSpeed   string `json:"downloadSpeed"`
	ErrorCode       string `json:"errorCode,omitempty"`
	ErrorMessage    string `json:"errorMessage,omitempty"`
}

func NewClient(host string, port int, secret string) *Client {
	return &Client{
		url:    fmt.Sprintf("http://%s:%d/jsonrpc", host, port),
		secret: secret,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) call(method string, params ...interface{}) (json.RawMessage, error) {
	id := fmt.Sprintf("%d", atomic.AddUint64(&c.counter, 1))

	// Prepend token if secret is set
	if c.secret != "" {
		params = append([]interface{}{"token:" + c.secret}, params...)
	}

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var rpcResp Response
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// AddURI adds a download by URL, returns GID
func (c *Client) AddURI(url string, dir string, filename string, headers map[string]string) (string, error) {
	options := map[string]interface{}{
		"dir": dir,
		"out": filename,
	}

	if len(headers) > 0 {
		headerList := make([]string, 0, len(headers))
		for k, v := range headers {
			headerList = append(headerList, fmt.Sprintf("%s: %s", k, v))
		}
		options["header"] = headerList
	}

	result, err := c.call("aria2.addUri", []string{url}, options)
	if err != nil {
		return "", err
	}

	var gid string
	if err := json.Unmarshal(result, &gid); err != nil {
		return "", fmt.Errorf("unmarshal gid: %w", err)
	}

	return gid, nil
}

// TellStatus gets download status by GID
func (c *Client) TellStatus(gid string) (*DownloadStatus, error) {
	result, err := c.call("aria2.tellStatus", gid)
	if err != nil {
		return nil, err
	}

	var status DownloadStatus
	if err := json.Unmarshal(result, &status); err != nil {
		return nil, fmt.Errorf("unmarshal status: %w", err)
	}

	return &status, nil
}

// TellActive gets all active downloads
func (c *Client) TellActive() ([]DownloadStatus, error) {
	result, err := c.call("aria2.tellActive")
	if err != nil {
		return nil, err
	}

	var statuses []DownloadStatus
	if err := json.Unmarshal(result, &statuses); err != nil {
		return nil, fmt.Errorf("unmarshal statuses: %w", err)
	}

	return statuses, nil
}

// Pause pauses a download
func (c *Client) Pause(gid string) error {
	_, err := c.call("aria2.pause", gid)
	return err
}

// Remove removes a download
func (c *Client) Remove(gid string) error {
	_, err := c.call("aria2.remove", gid)
	return err
}

// GetVersion checks aria2 is running
func (c *Client) GetVersion() (string, error) {
	result, err := c.call("aria2.getVersion")
	if err != nil {
		return "", err
	}

	var version struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(result, &version); err != nil {
		return "", fmt.Errorf("unmarshal version: %w", err)
	}

	return version.Version, nil
}
