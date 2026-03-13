package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type wsSmokeClient struct {
	conn *websocket.Conn
}

func dialCourseChat(ctx context.Context, baseURL, courseID, token string) (*wsSmokeClient, map[string]any, string, error) {
	wsURL := buildWSURL(baseURL, "/api/chat/ws/courses/"+courseID, token)
	headers := map[string][]string{}
	if origin := buildWSOrigin(baseURL); origin != "" {
		headers["Origin"] = []string{origin}
	}

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		evidence := err.Error()
		if resp != nil && resp.Body != nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if strings.TrimSpace(string(body)) != "" {
				evidence = evidence + " | " + strings.TrimSpace(string(body))
			}
		}
		return nil, nil, wsURL, fmt.Errorf("%s", evidence)
	}

	client := &wsSmokeClient{conn: conn}
	hello, err := client.readJSON(10 * time.Second)
	if err != nil {
		_ = client.Close()
		return nil, nil, wsURL, err
	}

	return client, hello, wsURL, nil
}

func buildWSURL(baseURL, path, token string) string {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return ""
	}

	switch parsed.Scheme {
	case "https":
		parsed.Scheme = "wss"
	default:
		parsed.Scheme = "ws"
	}

	parsed.Path = path
	query := parsed.Query()
	query.Set("token", token)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func buildWSOrigin(baseURL string) string {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}

	return parsed.Scheme + "://" + parsed.Host
}

func (c *wsSmokeClient) sendMessage(content string) error {
	return c.conn.WriteJSON(map[string]any{
		"type":    "message",
		"content": content,
	})
}

func (c *wsSmokeClient) readJSON(timeout time.Duration) (map[string]any, error) {
	_ = c.conn.SetReadDeadline(time.Now().Add(timeout))
	var payload map[string]any
	if err := c.conn.ReadJSON(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *wsSmokeClient) Close() error {
	_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
	return c.conn.Close()
}
