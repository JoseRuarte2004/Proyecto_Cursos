package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func (v *Verifier) Run() error {
	if err := os.MkdirAll(v.reportsDir, 0o755); err != nil {
		return fmt.Errorf("no pude crear reports/: %w", err)
	}
	if err := v.cleanReportsDir(); err != nil {
		return fmt.Errorf("no pude limpiar reports/: %w", err)
	}

	if err := v.sanityCheck(); err != nil {
		return err
	}

	if err := v.ensureEnvFile(); err != nil {
		return err
	}

	envValues, err := loadEnvFile(filepath.Join(v.root, envFile))
	if err != nil {
		return err
	}
	v.env = envValues
	v.detectBaseURLs()
	v.writeJSONReport("base_urls.json", v.base)

	if err := v.ensureCommand("docker"); err != nil {
		return err
	}
	if err := v.ensureCommand("go"); err != nil {
		return err
	}

	if err := v.composeUp(); err != nil {
		v.writeSkippedReports(err.Error())
		return err
	}
	v.summary.ComposeUp = true

	if err := v.waitHealthy(); err != nil {
		v.writeSkippedReports(err.Error())
		return err
	}

	v.runHealthReady()

	authOK := v.runAuthRBAC()
	coursesOK := false
	contentOK := false
	enrollmentsOK := false
	chatOK := false

	if authOK {
		coursesOK = v.runCourses()
	} else {
		v.writeSkippedJSONReport("courses_flow.json", "skipped: auth/rbac fallo")
	}

	if coursesOK {
		contentOK = v.runContentPermissions()
	} else {
		v.writeSkippedJSONReport("content_permissions.json", "skipped: courses fallo")
	}

	if coursesOK && contentOK {
		enrollmentsOK = v.runEnrollmentsAndPayments()
	} else {
		v.writeSkippedJSONReport("enrollments_flow.json", "skipped: content/courses fallo")
		v.writeSkippedJSONReport("payments_flow.json", "skipped: content/courses fallo")
		v.writeSkippedJSONReport("webhook_idempotency.json", "skipped: content/courses fallo")
		_ = os.WriteFile(filepath.Join(v.reportsDir, "db_orders.txt"), []byte("SKIPPED\n"), 0o644)
		_ = os.WriteFile(filepath.Join(v.reportsDir, "db_enrollments.txt"), []byte("SKIPPED\n"), 0o644)
	}

	if enrollmentsOK {
		chatOK = v.runChatRealtime()
	} else {
		v.writeSkippedJSONReport("chat_flow.json", "skipped: enrollments/payments fallo")
		_ = os.WriteFile(filepath.Join(v.reportsDir, "db_chat_messages.txt"), []byte("SKIPPED\n"), 0o644)
	}

	if v.summary.ComposeUp {
		v.runLogChecks()
	} else {
		_ = os.WriteFile(filepath.Join(v.reportsDir, "log_samples.txt"), []byte("SKIPPED\n"), 0o644)
	}

	if !authOK || !coursesOK || !contentOK || !enrollmentsOK || !chatOK || len(v.failures) > 0 {
		return errors.New("verification failed")
	}

	return nil
}

func (v *Verifier) cleanReportsDir() error {
	entries, err := os.ReadDir(v.reportsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(v.reportsDir, entry.Name())
		if entry.IsDir() {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
			continue
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	}

	return nil
}

func (v *Verifier) sanityCheck() error {
	required := []string{
		composeFile,
		"infra/nginx",
		"services/users-api",
		"services/courses-api",
		"services/course-content-api",
		"services/enrollments-api",
		"services/payments-api",
		"services/chat-api",
		"scripts/verify.ps1",
		"scripts/verify.sh",
		"Makefile",
		envExample,
	}

	lines := make([]string, 0, len(required)+2)
	for _, path := range required {
		fullPath := filepath.Join(v.root, path)
		if _, err := os.Stat(fullPath); err != nil {
			return fmt.Errorf("sanity check: falta %s: %w", path, err)
		}
		lines = append(lines, "OK "+path)
	}

	routerPath := filepath.Join(v.root, "internal/platform/server/router.go")
	routerContent, err := os.ReadFile(routerPath)
	if err != nil {
		return fmt.Errorf("sanity check: no pude leer %s: %w", routerPath, err)
	}
	if !strings.Contains(string(routerContent), `Get("/health"`) || !strings.Contains(string(routerContent), `Get("/ready"`) {
		return fmt.Errorf("sanity check: %s no define /health y /ready", routerPath)
	}
	lines = append(lines, "OK shared /health /ready")

	_ = os.WriteFile(filepath.Join(v.reportsDir, "sanity_check.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	return nil
}

func (v *Verifier) ensureEnvFile() error {
	target := filepath.Join(v.root, envFile)
	if _, err := os.Stat(target); err == nil {
		return nil
	}

	data, err := os.ReadFile(filepath.Join(v.root, envExample))
	if err != nil {
		return fmt.Errorf("no pude leer %s: %w", envExample, err)
	}

	if err := os.WriteFile(target, data, 0o644); err != nil {
		return fmt.Errorf("no pude crear %s: %w", envFile, err)
	}

	return nil
}

func loadEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no pude leer %s: %w", path, err)
	}

	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
	}

	return values, nil
}

func (v *Verifier) detectBaseURLs() {
	v.base = BaseURLs{
		Users:       "http://localhost:" + v.getEnv("USERS_API_PORT", "8081"),
		Courses:     "http://localhost:" + v.getEnv("COURSES_API_PORT", "8082"),
		Content:     "http://localhost:" + v.getEnv("COURSE_CONTENT_API_PORT", "8083"),
		Enrollments: "http://localhost:" + v.getEnv("ENROLLMENTS_API_PORT", "8084"),
		Payments:    "http://localhost:" + v.getEnv("PAYMENTS_API_PORT", "8085"),
		Chat:        "http://localhost:" + v.getEnv("CHAT_API_PORT", "8090"),
		Nginx:       "http://localhost:" + v.getEnv("NGINX_PORT", "8080"),
	}
}

func (v *Verifier) getEnv(key, fallback string) string {
	if value := strings.TrimSpace(v.env[key]); value != "" {
		return value
	}
	return fallback
}

func (v *Verifier) ensureCommand(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("comando requerido no encontrado: %s", name)
	}
	return nil
}

func (v *Verifier) composeUp() error {
	result := v.runCommand(10*time.Minute, "docker", "compose", "-f", composeFile, "up", "-d", "--build")
	_ = os.WriteFile(filepath.Join(v.reportsDir, "compose_up.txt"), []byte(joinOutput(result)), 0o644)
	if result.Err != nil {
		v.addFailure("compose up", "docker compose up -d --build fallo", "docker compose -f infra/docker-compose.yml up -d --build", joinOutput(result), "[infra/docker-compose.yml]("+filepath.ToSlash(filepath.Join(v.root, "infra/docker-compose.yml"))+")")
		return fmt.Errorf("compose up fallo")
	}

	return nil
}

func (v *Verifier) waitHealthy() error {
	services := []string{
		"postgres",
		"redis",
		"rabbitmq",
		"users-api",
		"courses-api",
		"course-content-api",
		"enrollments-api",
		"payments-api",
		"chat-api",
		"nginx",
	}

	deadline := time.Now().Add(6 * time.Minute)
	var last string
	for time.Now().Before(deadline) {
		allHealthy := true
		lines := make([]string, 0, len(services))
		for _, service := range services {
			containerID, err := v.composeContainerID(service)
			if err != nil {
				allHealthy = false
				lines = append(lines, fmt.Sprintf("%s: container id no disponible (%v)", service, err))
				continue
			}

			result := v.runCommand(30*time.Second, "docker", "inspect", "--format", "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}", containerID)
			status := strings.TrimSpace(result.Stdout)
			if status == "" {
				status = strings.TrimSpace(result.Stderr)
			}
			lines = append(lines, fmt.Sprintf("%s: %s", service, status))
			if result.Err != nil || status != "healthy" {
				allHealthy = false
			}
		}

		last = strings.Join(lines, "\n")
		_ = os.WriteFile(filepath.Join(v.reportsDir, "compose_health.txt"), []byte(last), 0o644)
		if allHealthy {
			return nil
		}

		time.Sleep(5 * time.Second)
	}

	v.addFailure("compose health", "los contenedores no llegaron a healthy dentro del timeout", "docker compose -f infra/docker-compose.yml ps", last, "[infra/docker-compose.yml]("+filepath.ToSlash(filepath.Join(v.root, "infra/docker-compose.yml"))+")")
	return fmt.Errorf("healthchecks no llegaron a healthy")
}

func (v *Verifier) composeContainerID(service string) (string, error) {
	result := v.runCommand(30*time.Second, "docker", "compose", "-f", composeFile, "ps", "-q", service)
	if result.Err != nil {
		return "", fmt.Errorf("%s", joinOutput(result))
	}

	id := strings.TrimSpace(result.Stdout)
	if id == "" {
		return "", fmt.Errorf("docker compose ps -q %s devolvio vacio", service)
	}
	return id, nil
}

func (v *Verifier) dockerLogs(service string, tail int) (string, error) {
	containerID, err := v.composeContainerID(service)
	if err != nil {
		return "", err
	}

	result := v.runCommand(1*time.Minute, "docker", "logs", "--tail", strconv.Itoa(tail), containerID)
	if result.Err != nil {
		return joinOutput(result), result.Err
	}

	return joinOutput(result), nil
}

func (v *Verifier) queryPostgres(query string) (string, error) {
	result := v.runCommand(
		2*time.Minute,
		"docker", "compose", "-f", composeFile,
		"exec", "-T",
		"-e", "PGPASSWORD="+v.getEnv("POSTGRES_PASSWORD", "postgres"),
		"postgres",
		"psql",
		"-U", v.getEnv("POSTGRES_USER", "postgres"),
		"-d", v.getEnv("POSTGRES_DB", "platform_dev"),
		"-At",
		"-F", "|",
		"-c", query,
	)
	if result.Err != nil {
		return joinOutput(result), result.Err
	}

	return strings.TrimSpace(result.Stdout), nil
}

func (v *Verifier) runCommand(timeout time.Duration, name string, args ...string) commandResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = v.root
	cmd.Env = append(os.Environ(), "COMPOSE_BAKE=false")

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

func (v *Verifier) httpRequest(name, method, url string, headers map[string]string, body string, expected string, okFn func(RequestResult) bool) RequestResult {
	start := time.Now()
	req, err := http.NewRequestWithContext(context.Background(), method, url, strings.NewReader(body))
	if err != nil {
		return RequestResult{Name: name, Method: method, URL: url, Expected: expected, Error: err.Error(), RequestBody: body}
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
			Name:           name,
			Method:         method,
			URL:            url,
			Expected:       expected,
			RequestHeaders: sanitizeHeaders(headers),
			RequestBody:    sanitizeBody(body),
			Error:          err.Error(),
			DurationMs:     time.Since(start).Milliseconds(),
		}
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(resp.Body)
	result := RequestResult{
		Name:            name,
		Method:          method,
		URL:             url,
		Expected:        expected,
		StatusCode:      resp.StatusCode,
		RequestHeaders:  sanitizeHeaders(headers),
		RequestBody:     sanitizeBody(body),
		ResponseHeaders: captureHeaders(resp.Header),
		Body:            sanitizeBody(string(rawBody)),
		DurationMs:      time.Since(start).Milliseconds(),
		rawBody:         string(rawBody),
	}
	if okFn != nil {
		result.OK = okFn(result)
	}
	return result
}

func captureHeaders(header http.Header) map[string]string {
	if len(header) == 0 {
		return nil
	}

	result := map[string]string{}
	for _, key := range []string{"Content-Type", "Idempotency-Key"} {
		if value := header.Get(key); value != "" {
			result[key] = sanitizeHeaderValue(key, value)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func sanitizeHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}

	sanitized := map[string]string{}
	for key, value := range headers {
		sanitized[key] = sanitizeHeaderValue(key, value)
	}
	return sanitized
}

func sanitizeHeaderValue(key, value string) string {
	lower := strings.ToLower(key)
	if lower == "authorization" {
		if strings.HasPrefix(value, "Bearer ") {
			return "Bearer " + redactToken(strings.TrimPrefix(value, "Bearer "))
		}
		return redactToken(value)
	}
	if strings.Contains(lower, "token") {
		return redactToken(value)
	}
	return value
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

	redacted := redactPayload(payload)
	data, err := json.Marshal(redacted)
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
				text, ok := value.(string)
				if ok {
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
	if token == "" {
		return ""
	}
	if len(token) <= 10 {
		return token[:2] + "..." + token[len(token)-2:]
	}
	return token[:6] + "..." + token[len(token)-4:]
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

func (v *Verifier) writeJSONReport(name string, payload any) {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(v.reportsDir, name), data, 0o644)
}

func (v *Verifier) writeSkippedJSONReport(name, reason string) {
	v.writeJSONReport(name, map[string]any{
		"generatedAt": nowRFC3339(),
		"allPassed":   false,
		"error":       reason,
	})
}

func (v *Verifier) writeSkippedReports(reason string) {
	if reason == "" {
		reason = "runtime checks skipped"
	}
	v.writeSkippedJSONReport("health_ready.json", reason)
	v.writeSkippedJSONReport("users_rbac.json", reason)
	v.writeSkippedJSONReport("courses_flow.json", reason)
	v.writeSkippedJSONReport("content_permissions.json", reason)
	v.writeSkippedJSONReport("enrollments_flow.json", reason)
	v.writeSkippedJSONReport("payments_flow.json", reason)
	v.writeSkippedJSONReport("webhook_idempotency.json", reason)
	v.writeSkippedJSONReport("chat_flow.json", reason)
	_ = os.WriteFile(filepath.Join(v.reportsDir, "tokens_redacted.txt"), []byte("SKIPPED: "+reason+"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(v.reportsDir, "db_orders.txt"), []byte("SKIPPED: "+reason+"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(v.reportsDir, "db_enrollments.txt"), []byte("SKIPPED: "+reason+"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(v.reportsDir, "db_chat_messages.txt"), []byte("SKIPPED: "+reason+"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(v.reportsDir, "log_samples.txt"), []byte("SKIPPED: "+reason+"\n"), 0o644)
}

func (v *Verifier) addFailure(section, message, command, evidence, fileHint string) {
	v.failures = append(v.failures, Failure{
		Section:  section,
		Message:  message,
		Command:  command,
		Evidence: evidence,
		FileHint: fileHint,
	})
}

func (v *Verifier) writeTokensReport() {
	lines := []string{
		"admin=" + redactToken(v.state.AdminToken),
		"student=" + redactToken(v.state.StudentToken),
		"teacher=" + redactToken(v.state.TeacherToken),
		"student2=" + redactToken(v.state.Student2Token),
	}
	_ = os.WriteFile(filepath.Join(v.reportsDir, "tokens_redacted.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func (v *Verifier) poll(timeout, interval time.Duration, fn func() PollResult) PollResult {
	deadline := time.Now().Add(timeout)
	last := PollResult{}
	for time.Now().Before(deadline) {
		last = fn()
		if last.OK {
			return last
		}
		time.Sleep(interval)
	}
	return last
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
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func floatFromMap(payload map[string]any, key string) float64 {
	if payload == nil {
		return 0
	}
	switch typed := payload[key].(type) {
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

func containsCourseByID(raw, courseID string) bool {
	items := parseJSONArray(raw)
	for _, item := range items {
		if stringFromMap(item, "id") == courseID || stringFromMap(item, "courseId") == courseID {
			return true
		}
	}
	return false
}

func findEnrollmentStatus(raw, courseID string) string {
	items := parseJSONArray(raw)
	for _, item := range items {
		if stringFromMap(item, "courseId") == courseID {
			return stringFromMap(item, "status")
		}
	}
	return ""
}

func findUserIDByEmail(raw, email string) string {
	items := parseJSONArray(raw)
	for _, item := range items {
		if strings.EqualFold(stringFromMap(item, "email"), email) {
			return stringFromMap(item, "id")
		}
	}
	return ""
}

func prettyJSON(value any) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return ""
	}
	return string(data)
}

func fileLink(root, rel string) string {
	return "[" + rel + "](" + filepath.ToSlash(filepath.Join(root, rel)) + ")"
}

func (v *Verifier) pollEmailLink(service, email, subject string, timeout, interval time.Duration) (string, string) {
	result := v.poll(timeout, interval, func() PollResult {
		output, err := v.dockerLogs(service, 500)
		if err != nil {
			return PollResult{OK: false, Evidence: output}
		}

		link := findLoggedEmailLink(output, email, subject)
		return PollResult{OK: strings.TrimSpace(link) != "", Evidence: output}
	})
	if !result.OK {
		return "", result.Evidence
	}

	return findLoggedEmailLink(result.Evidence, email, subject), result.Evidence
}

func findLoggedEmailLink(output, email, subject string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		payload := parseJSONMap(line)
		if payload == nil {
			continue
		}
		if stringFromMap(payload, "msg") != "email queued" {
			continue
		}
		if !strings.EqualFold(stringFromMap(payload, "to"), email) {
			continue
		}
		if strings.TrimSpace(subject) != "" && stringFromMap(payload, "subject") != subject {
			continue
		}

		return stringFromMap(payload, "link")
	}

	return ""
}

func (v *Verifier) PrintSummary() {
	printCheck := func(ok bool, label string) {
		if ok {
			fmt.Printf("OK   %s\n", label)
			return
		}
		fmt.Printf("FAIL %s\n", label)
	}

	printCheck(v.summary.ComposeUp, "compose up")
	printCheck(v.summary.HealthReady, "health/ready")
	printCheck(v.summary.Metrics, "metrics")
	printCheck(v.summary.AuthRBAC, "auth/rbac")
	printCheck(v.summary.Courses, "courses")
	printCheck(v.summary.ContentPermissions, "content permisos")
	printCheck(v.summary.EnrollmentsCapacity, "enrollments cupo")
	printCheck(v.summary.PaymentsWebhook, "payments + webhook idempotencia")
	printCheck(v.summary.RabbitEnrollment, "rabbit consume + enrollment active")
	printCheck(v.summary.ChatRealtime, "chat realtime + history")
	printCheck(v.summary.LogsJSON, "logs JSON")

	if len(v.failures) == 0 {
		fmt.Println("TODO OK")
		return
	}

	fmt.Println("FAIL")
	for i, failure := range v.failures {
		fmt.Printf("%d. [%s] %s\n", i+1, failure.Section, failure.Message)
		if failure.Command != "" {
			fmt.Printf("   comando: %s\n", failure.Command)
		}
		if failure.Evidence != "" {
			fmt.Printf("   evidencia: %s\n", singleLine(failure.Evidence))
		}
		if failure.FileHint != "" {
			fmt.Printf("   revisar: %s\n", failure.FileHint)
		}
	}
}

func singleLine(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " | ")
	return strings.TrimSpace(value)
}
