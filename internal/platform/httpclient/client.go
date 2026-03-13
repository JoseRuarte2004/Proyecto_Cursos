package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"proyecto-cursos/internal/platform/internalauth"
	"proyecto-cursos/internal/platform/metrics"
	"proyecto-cursos/internal/platform/requestid"
)

const (
	defaultTimeout = 3 * time.Second
	maxRetries     = 2
)

type Client struct {
	baseURL       string
	service       string
	dependency    string
	internalToken string
	httpClient    *http.Client
}

func New(baseURL, service, dependency string, opts ...Option) *Client {
	client := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		service:    strings.TrimSpace(service),
		dependency: strings.TrimSpace(dependency),
		httpClient: &http.Client{Timeout: defaultTimeout},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

type Option func(*Client)

func WithTimeout(timeout time.Duration) Option {
	return func(client *Client) {
		if timeout > 0 {
			client.httpClient.Timeout = timeout
		}
	}
}

func WithInternalToken(token string) Option {
	return func(client *Client) {
		client.internalToken = strings.TrimSpace(token)
	}
}

type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

func (c *Client) Do(ctx context.Context, method, path string, body []byte, headers map[string]string) (*Response, error) {
	url := c.baseURL + path
	attempt := 0

	for {
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}

		for key, value := range headers {
			req.Header.Set(key, value)
		}
		if requestID := requestid.FromContext(ctx); requestID != "" && req.Header.Get(requestid.Header) == "" {
			req.Header.Set(requestid.Header, requestID)
		}
		if c.internalToken != "" && req.Header.Get(internalauth.Header) == "" {
			req.Header.Set(internalauth.Header, c.internalToken)
		}

		resp, err := c.httpClient.Do(req)
		if err == nil {
			defer resp.Body.Close()

			payload, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				return nil, readErr
			}

			result := &Response{
				StatusCode: resp.StatusCode,
				Headers:    resp.Header.Clone(),
				Body:       payload,
			}
			if resp.StatusCode >= http.StatusInternalServerError && attempt < maxRetries {
				c.recordDependencyError()
				if waitErr := waitBackoff(ctx, attempt); waitErr != nil {
					return result, waitErr
				}
				attempt++
				continue
			}

			if resp.StatusCode >= http.StatusInternalServerError {
				c.recordDependencyError()
			}

			return result, nil
		}

		if !shouldRetry(err) || attempt >= maxRetries {
			c.recordDependencyError()
			return nil, err
		}

		c.recordDependencyError()
		if waitErr := waitBackoff(ctx, attempt); waitErr != nil {
			return nil, waitErr
		}
		attempt++
	}
}

func (c *Client) GetJSON(ctx context.Context, path string, headers map[string]string, out any) (int, error) {
	resp, err := c.Do(ctx, http.MethodGet, path, nil, headers)
	if err != nil {
		return 0, err
	}

	if out != nil && len(resp.Body) > 0 {
		if err := json.Unmarshal(resp.Body, out); err != nil {
			return resp.StatusCode, err
		}
	}

	return resp.StatusCode, nil
}

func (c *Client) DoJSON(ctx context.Context, method, path string, body any, headers map[string]string, out any) (*Response, error) {
	var payload []byte
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = data
	}

	if headers == nil {
		headers = map[string]string{}
	}
	if body != nil && headers["Content-Type"] == "" {
		headers["Content-Type"] = "application/json"
	}

	resp, err := c.Do(ctx, method, path, payload, headers)
	if err != nil {
		return nil, err
	}

	if out != nil && len(resp.Body) > 0 {
		if err := json.Unmarshal(resp.Body, out); err != nil {
			return nil, err
		}
	}

	return resp, nil
}

func (c *Client) recordDependencyError() {
	if c.service == "" || c.dependency == "" {
		return
	}

	metrics.RecordDependencyError(c.service, c.dependency)
}

func shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return false
}

func waitBackoff(ctx context.Context, attempt int) error {
	delay := time.Duration(attempt+1) * 150 * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
