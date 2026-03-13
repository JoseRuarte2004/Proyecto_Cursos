package main

import "time"

const (
	composeFile = "infra/docker-compose.yml"
	envExample  = ".env.example"
	envFile     = ".env"
)

type Verifier struct {
	root       string
	reportsDir string
	env        map[string]string
	base       BaseURLs
	summary    Summary
	state      FlowState
	failures   []Failure
}

type BaseURLs struct {
	Users       string `json:"users"`
	Courses     string `json:"courses"`
	Content     string `json:"content"`
	Enrollments string `json:"enrollments"`
	Payments    string `json:"payments"`
	Chat        string `json:"chat"`
	Nginx       string `json:"nginx"`
}

type Summary struct {
	ComposeUp           bool
	HealthReady         bool
	Metrics             bool
	AuthRBAC            bool
	Courses             bool
	ContentPermissions  bool
	EnrollmentsCapacity bool
	PaymentsWebhook     bool
	RabbitEnrollment    bool
	ChatRealtime        bool
	LogsJSON            bool
}

type Failure struct {
	Section  string `json:"section"`
	Message  string `json:"message"`
	Command  string `json:"command,omitempty"`
	Evidence string `json:"evidence,omitempty"`
	FileHint string `json:"fileHint,omitempty"`
}

type FlowState struct {
	AdminToken    string
	StudentToken  string
	Student2Token string
	TeacherToken  string

	StudentID  string
	Student2ID string
	TeacherID  string
	CourseID   string
	OrderID    string
	ChatMsgID  string

	StudentEmail  string
	Student2Email string
	TeacherEmail  string
}

type RequestResult struct {
	Name            string            `json:"name"`
	Method          string            `json:"method"`
	URL             string            `json:"url"`
	Expected        string            `json:"expected,omitempty"`
	StatusCode      int               `json:"statusCode"`
	OK              bool              `json:"ok"`
	RequestHeaders  map[string]string `json:"requestHeaders,omitempty"`
	RequestBody     string            `json:"requestBody,omitempty"`
	ResponseHeaders map[string]string `json:"responseHeaders,omitempty"`
	Body            string            `json:"body,omitempty"`
	Error           string            `json:"error,omitempty"`
	DurationMs      int64             `json:"durationMs"`

	rawBody string
}

type HealthCheckReport struct {
	GeneratedAt string                   `json:"generatedAt"`
	BaseURLs    BaseURLs                 `json:"baseUrls"`
	Services    map[string]ServiceStatus `json:"services"`
	AllPassed   bool                     `json:"allPassed"`
}

type ServiceStatus struct {
	Health  RequestResult `json:"health"`
	Ready   RequestResult `json:"ready"`
	Metrics RequestResult `json:"metrics"`
}

type StepReport struct {
	GeneratedAt string                   `json:"generatedAt"`
	Steps       map[string]RequestResult `json:"steps"`
	Notes       map[string]any           `json:"notes,omitempty"`
	AllPassed   bool                     `json:"allPassed"`
}

type LogReport struct {
	GeneratedAt string              `json:"generatedAt"`
	Counts      map[string]int      `json:"counts"`
	Lines       map[string][]string `json:"lines"`
	AllPassed   bool                `json:"allPassed"`
}

type PollResult struct {
	OK       bool
	Evidence string
}

type commandResult struct {
	Stdout string
	Stderr string
	Err    error
}

func NewVerifier(root, reportsDir string) *Verifier {
	return &Verifier{
		root:       root,
		reportsDir: reportsDir,
		env:        map[string]string{},
	}
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func timestampNonce() int64 {
	return time.Now().UnixNano()
}
