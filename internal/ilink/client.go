package ilink

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"maclawbot/internal/router"
)

// iLink protocol constants
const (
	ILINK_VER = "2.1.7"   // iLink protocol version
	ILINKCV  = "65547"    // iLink client version
)

// BaseInfo contains common protocol fields for iLink requests.
type BaseInfo struct {
	ChannelVersion string `json:"channel_version"`
}

// GetUpdatesRequest is the request body for long-polling messages.
type GetUpdatesRequest struct {
	GetUpdatesBuf string   `json:"get_updates_buf"` // Cursor for incremental updates
	BaseInfo      BaseInfo `json:"base_info"`
}

// GetUpdatesResponse is the response from long-polling.
type GetUpdatesResponse struct {
	Ret           int              `json:"ret"`            // 0 on success
	ErrCode       int              `json:"errcode"`        // 0 on success
	Msgs          []router.Message `json:"msgs"`           // New messages
	GetUpdatesBuf string           `json:"get_updates_buf"` // Next cursor
}

// SendMessageRequest is the request body for sending messages.
type SendMessageRequest struct {
	Msg      router.SendMessage `json:"msg"`
	BaseInfo BaseInfo           `json:"base_info"`
}

// SendMessageResponse is the response from sending messages.
type SendMessageResponse struct {
	Ret    int    `json:"ret"`
	ErrMsg string `json:"errmsg"`
}

// Client is an iLink API client for bot operations.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewClient creates a new iLink client.
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// headers builds the common HTTP headers for iLink requests.
func (c *Client) headers(body []byte) http.Header {
	return http.Header{
		"Content-Type":            []string{"application/json"},
		"AuthorizationType":       []string{"ilink_bot_token"},
		"Content-Length":          []string{fmt.Sprintf("%d", len(body))},
		"iLink-App-Id":           []string{""},
		"iLink-App-ClientVersion": []string{ILINKCV},
		"Authorization":           []string{"Bearer " + c.Token},
	}
}

// GetUpdates long-polls for new messages from iLink.
// The server will hold the connection open until timeout or new messages arrive.
func (c *Client) GetUpdates(buf string, timeout time.Duration) (*GetUpdatesResponse, error) {
	reqBody := GetUpdatesRequest{
		GetUpdatesBuf: buf,
		BaseInfo: BaseInfo{
			ChannelVersion: ILINK_VER,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := c.BaseURL + "/ilink/bot/getupdates"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header = c.headers(body)

	// Set connection close to prevent keep-alive issues
	if timeout > 0 {
		req.Header.Set("Connection", "close")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result GetUpdatesResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// SendText sends a plain text message to a user through iLink.
func (c *Client) SendText(toUser, text, ctx string) error {
	clientID := fmt.Sprintf("hc-%d", time.Now().UnixNano())

	msg := router.SendMessage{
		FromUserID:   "",
		ToUserID:     toUser,
		ClientID:     clientID,
		MessageType:  2, // outgoing
		MessageState: 2, // sent
		ItemList: []router.Item{
			{
				Type: router.MessageTypeText,
				TextItem: &router.TextItem{
					Text: text,
				},
			},
		},
	}

	if ctx != "" {
		msg.ContextToken = ctx
	}

	reqBody := SendMessageRequest{
		Msg: msg,
		BaseInfo: BaseInfo{
			ChannelVersion: ILINK_VER,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := c.BaseURL + "/ilink/bot/sendmessage"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header = c.headers(body)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("send failed with status: %d", resp.StatusCode)
	}

	return nil
}
