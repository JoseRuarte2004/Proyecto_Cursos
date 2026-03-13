package main

import "time"

const (
	composeFile = "infra/docker-compose.yml"
	envExample  = ".env.example"
	envFile     = ".env"
)

type Runner struct {
	root       string
	reportsDir string
	env        map[string]string
	base       BaseURLs
	report     IntegrationReport
	state      FlowState
	summary    Summary
	failures   []Failure
}

type BaseURLs struct {
	Users       string `json:"users"`
	Courses     string `json:"courses"`
	Content     string `json:"content"`
	Enrollments string `json:"enrollments"`
	Payments    string `json:"payments"`
}

type FlowState struct {
	AdminToken   string
	StudentToken string
	TeacherToken string
	StudentID    string
	TeacherID    string
	CourseID     string
	OrderID      string
	StudentEmail string
	TeacherEmail string
}

type Summary struct {
	ComposeUp   bool
	HealthReady bool
	AuthRBAC    bool
	Courses     bool
	Content     bool
	Enrollments bool
	Payments    bool
	Contracts   bool
}

type Failure struct {
	Section  string `json:"section"`
	Message  string `json:"message"`
	Command  string `json:"command,omitempty"`
	Evidence string `json:"evidence,omitempty"`
}

type IntegrationReport struct {
	GeneratedAt string                   `json:"generatedAt"`
	BaseURLs    BaseURLs                 `json:"baseUrls"`
	Steps       map[string]RequestResult `json:"steps"`
	Contracts   ContractChecks           `json:"contracts"`
	Notes       map[string]any           `json:"notes,omitempty"`
	AllPassed   bool                     `json:"allPassed"`
}

type ContractChecks struct {
	ErrorEnvelopeValid bool     `json:"errorEnvelopeValid"`
	SuccessJSONValid   bool     `json:"successJsonValid"`
	MetricsOK          bool     `json:"metricsOk"`
	Failures           []string `json:"failures,omitempty"`
}

type RequestResult struct {
	Name        string            `json:"name"`
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	StatusCode  int               `json:"statusCode"`
	OK          bool              `json:"ok"`
	Headers     map[string]string `json:"headers,omitempty"`
	RequestBody string            `json:"requestBody,omitempty"`
	Body        string            `json:"body,omitempty"`
	Error       string            `json:"error,omitempty"`
	DurationMs  int64             `json:"durationMs"`

	rawBody string
}

type commandResult struct {
	Stdout string
	Stderr string
	Err    error
}

func NewRunner(root, reportsDir string) *Runner {
	return &Runner{
		root:       root,
		reportsDir: reportsDir,
		env:        map[string]string{},
		report: IntegrationReport{
			GeneratedAt: nowRFC3339(),
			Steps:       map[string]RequestResult{},
			Notes:       map[string]any{},
			AllPassed:   true,
		},
	}
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func nonce() int64 {
	return time.Now().UnixNano()
}
