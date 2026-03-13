package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"proyecto-cursos/internal/platform/requestid"
)

func (r *Runner) ensureReportsDir() error {
	return os.MkdirAll(r.reportsDir, 0o755)
}

func (r *Runner) ensureEnvFile() error {
	target := filepath.Join(r.root, envFile)
	if _, err := os.Stat(target); err == nil {
		return nil
	}

	data, err := os.ReadFile(filepath.Join(r.root, envExample))
	if err != nil {
		return err
	}

	return os.WriteFile(target, data, 0o644)
}

func (r *Runner) loadEnv() error {
	data, err := os.ReadFile(filepath.Join(r.root, envFile))
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		r.env[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
	}

	r.base = BaseURLs{
		Users:       "http://localhost:" + r.getEnv("USERS_API_PORT", "8081"),
		Courses:     "http://localhost:" + r.getEnv("COURSES_API_PORT", "8082"),
		Content:     "http://localhost:" + r.getEnv("COURSE_CONTENT_API_PORT", "8083"),
		Enrollments: "http://localhost:" + r.getEnv("ENROLLMENTS_API_PORT", "8084"),
		Payments:    "http://localhost:" + r.getEnv("PAYMENTS_API_PORT", "8085"),
	}
	r.report.BaseURLs = r.base

	return nil
}

func (r *Runner) getEnv(key, fallback string) string {
	if value := strings.TrimSpace(r.env[key]); value != "" {
		return value
	}
	return fallback
}

func (r *Runner) ensureCommand(name string) error {
	_, err := exec.LookPath(name)
	return err
}

func (r *Runner) runCommand(timeout time.Duration, name string, args ...string) commandResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.root

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}

	return commandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Err:    err,
	}
}

func joinOutput(result commandResult) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(result.Stdout) != "" {
		parts = append(parts, result.Stdout)
	}
	if strings.TrimSpace(result.Stderr) != "" {
		parts = append(parts, result.Stderr)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func (r *Runner) httpRequest(name, method, url string, headers map[string]string, body string) RequestResult {
	start := time.Now()
	req, err := http.NewRequestWithContext(context.Background(), method, url, strings.NewReader(body))
	if err != nil {
		return RequestResult{Name: name, Method: method, URL: url, Error: err.Error(), RequestBody: body}
	}

	if headers == nil {
		headers = map[string]string{}
	}
	if headers[requestid.Header] == "" {
		headers[requestid.Header] = requestid.New()
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return RequestResult{
			Name:        name,
			Method:      method,
			URL:         url,
			Headers:     sanitizeHeaders(headers),
			RequestBody: sanitizeBody(body),
			Error:       err.Error(),
			DurationMs:  time.Since(start).Milliseconds(),
		}
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(resp.Body)
	result := RequestResult{
		Name:        name,
		Method:      method,
		URL:         url,
		StatusCode:  resp.StatusCode,
		Headers:     sanitizeHeaders(headers),
		RequestBody: sanitizeBody(body),
		Body:        sanitizeBody(string(rawBody)),
		DurationMs:  time.Since(start).Milliseconds(),
		rawBody:     string(rawBody),
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.OK = true
	}
	return result
}

func sanitizeHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}

	result := map[string]string{}
	for key, value := range headers {
		lower := strings.ToLower(key)
		if lower == "authorization" && strings.HasPrefix(value, "Bearer ") {
			result[key] = "Bearer " + redactToken(strings.TrimPrefix(value, "Bearer "))
			continue
		}
		if strings.Contains(lower, "token") {
			result[key] = redactToken(value)
			continue
		}
		result[key] = value
	}
	return result
}

func sanitizeBody(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return body
	}

	var payload any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return body
	}

	data, err := json.Marshal(redactPayload(payload))
	if err != nil {
		return body
	}
	return string(data)
}

func redactPayload(payload any) any {
	switch typed := payload.(type) {
	case map[string]any:
		result := map[string]any{}
		for key, value := range typed {
			if strings.Contains(strings.ToLower(key), "token") {
				if text, ok := value.(string); ok {
					result[key] = redactToken(text)
					continue
				}
			}
			result[key] = redactPayload(value)
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, redactPayload(item))
		}
		return result
	default:
		return payload
	}
}

func redactToken(token string) string {
	token = strings.TrimSpace(token)
	if len(token) <= 10 {
		return token
	}
	return token[:6] + "..." + token[len(token)-4:]
}

func parseJSONMap(raw string) map[string]any {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return payload
}

func parseJSONArray(raw string) []map[string]any {
	var payload []map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return payload
}

func stringFromMap(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func floatFromMap(payload map[string]any, key string) float64 {
	if payload == nil {
		return 0
	}
	value, ok := payload[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	default:
		return 0
	}
}

func boolFromMap(payload map[string]any, key string) bool {
	if payload == nil {
		return false
	}
	value, _ := payload[key].(bool)
	return value
}

func containsItemByID(raw, id string) bool {
	for _, item := range parseJSONArray(raw) {
		if stringFromMap(item, "id") == id || stringFromMap(item, "courseId") == id {
			return true
		}
	}
	return false
}

func findEnrollmentStatus(raw, courseID string) string {
	for _, item := range parseJSONArray(raw) {
		if stringFromMap(item, "courseId") == courseID {
			return stringFromMap(item, "status")
		}
	}
	return ""
}

func poll(timeout, interval time.Duration, fn func() (bool, string)) (bool, string) {
	deadline := time.Now().Add(timeout)
	last := ""
	for time.Now().Before(deadline) {
		ok, evidence := fn()
		last = evidence
		if ok {
			return true, evidence
		}
		time.Sleep(interval)
	}
	return false, last
}

func asJSON(raw string) error {
	var payload any
	return json.Unmarshal([]byte(raw), &payload)
}

func intString(value int) string {
	return strconv.Itoa(value)
}
