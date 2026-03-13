package main

import (
	"fmt"
	"strings"
	"time"
)

func (v *Verifier) runHealthReady() bool {
	services := map[string]string{
		"users-api":          v.base.Users,
		"courses-api":        v.base.Courses,
		"course-content-api": v.base.Content,
		"enrollments-api":    v.base.Enrollments,
		"payments-api":       v.base.Payments,
		"chat-api":           v.base.Chat,
	}

	report := HealthCheckReport{
		GeneratedAt: nowRFC3339(),
		BaseURLs:    v.base,
		Services:    map[string]ServiceStatus{},
		AllPassed:   true,
	}

	for service, baseURL := range services {
		health := v.httpRequest(service+"_health", "GET", baseURL+"/health", nil, "", "200", func(result RequestResult) bool {
			return result.StatusCode == 200 && result.Error == ""
		})
		ready := v.httpRequest(service+"_ready", "GET", baseURL+"/ready", nil, "", "200", func(result RequestResult) bool {
			return result.StatusCode == 200 && result.Error == ""
		})
		metrics := v.httpRequest(service+"_metrics", "GET", baseURL+"/metrics", nil, "", "200", func(result RequestResult) bool {
			return result.StatusCode == 200 && result.Error == "" && strings.Contains(result.rawBody, "http_requests_total")
		})

		report.Services[service] = ServiceStatus{
			Health:  health,
			Ready:   ready,
			Metrics: metrics,
		}
		if !health.OK || !ready.OK || !metrics.OK {
			report.AllPassed = false
		}
	}

	v.writeJSONReport("health_ready.json", report)
	v.summary.HealthReady = report.AllPassed
	v.summary.Metrics = report.AllPassed
	if !report.AllPassed {
		v.addFailure(
			"health/ready",
			"uno o mas servicios no respondieron 200 en /health, /ready o /metrics",
			"GET /health | GET /ready | GET /metrics",
			prettyJSON(report),
			fileLink(v.root, "internal/platform/server/router.go"),
		)
	}

	return report.AllPassed
}

func (v *Verifier) runAuthRBAC() bool {
	report := StepReport{
		GeneratedAt: nowRFC3339(),
		Steps:       map[string]RequestResult{},
		Notes:       map[string]any{},
		AllPassed:   true,
	}

	v.state.StudentEmail = fmt.Sprintf("verify-student-%d@example.com", timestampNonce())
	studentPassword := "student1234"
	adminEmail := v.getEnv("USERS_API_BOOTSTRAP_ADMIN_EMAIL", "admin@example.com")
	adminPassword := v.getEnv("USERS_API_BOOTSTRAP_ADMIN_PASSWORD", "admin1234")

	register := v.httpRequest(
		"register_student",
		"POST",
		v.base.Users+"/auth/register",
		map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"name":"Verify Student","email":"%s","password":"%s","phone":"111111","dni":"20000001","address":"QA 123"}`, v.state.StudentEmail, studentPassword),
		"201",
		func(result RequestResult) bool { return result.StatusCode == 201 && result.Error == "" },
	)
	report.Steps["register_student"] = register
	v.state.StudentID = stringFromMap(parseJSONMap(register.rawBody), "id")

	verifyLink, verifyEvidence := v.pollEmailLink("users-api", v.state.StudentEmail, "Verify your email", 30*time.Second, 2*time.Second)
	report.Notes["studentVerifyLinkFound"] = verifyLink != ""
	if verifyLink == "" {
		report.AllPassed = false
		v.addFailure(
			"auth/rbac",
			"no pude encontrar en logs el link de verificacion del email del student",
			"docker logs users-api",
			verifyEvidence,
			fileLink(v.root, "services/users-api/internal/app/mailer.go"),
		)
	} else {
		verifyEmail := v.httpRequest(
			"verify_student_email",
			"GET",
			verifyLink,
			nil,
			"",
			"200",
			func(result RequestResult) bool {
				return result.StatusCode == 200 && result.Error == "" && stringFromMap(parseJSONMap(result.rawBody), "status") == "verified"
			},
		)
		report.Steps["verify_student_email"] = verifyEmail

		internalVerified := v.httpRequest(
			"internal_student_email_verified",
			"GET",
			v.base.Users+"/internal/users/"+v.state.StudentID+"/email-verified",
			map[string]string{"X-Internal-Token": v.getEnv("USERS_INTERNAL_TOKEN", "internal-users")},
			"",
			"200 and emailVerified=true",
			func(result RequestResult) bool {
				payload := parseJSONMap(result.rawBody)
				return result.StatusCode == 200 && result.Error == "" && boolFromMap(payload, "emailVerified")
			},
		)
		report.Steps["internal_student_email_verified"] = internalVerified
	}

	loginStudent := v.httpRequest(
		"login_student",
		"POST",
		v.base.Users+"/auth/login",
		map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"email":"%s","password":"%s"}`, v.state.StudentEmail, studentPassword),
		"200",
		func(result RequestResult) bool { return result.StatusCode == 200 && result.Error == "" },
	)
	report.Steps["login_student"] = loginStudent
	v.state.StudentToken = stringFromMap(parseJSONMap(loginStudent.rawBody), "token")

	loginAdmin := v.httpRequest(
		"login_admin",
		"POST",
		v.base.Users+"/auth/login",
		map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"email":"%s","password":"%s"}`, adminEmail, adminPassword),
		"200",
		func(result RequestResult) bool { return result.StatusCode == 200 && result.Error == "" },
	)
	report.Steps["login_admin"] = loginAdmin
	v.state.AdminToken = stringFromMap(parseJSONMap(loginAdmin.rawBody), "token")

	meStudent := v.httpRequest(
		"me_student",
		"GET",
		v.base.Users+"/me",
		map[string]string{"Authorization": "Bearer " + v.state.StudentToken},
		"",
		"200",
		func(result RequestResult) bool { return result.StatusCode == 200 && result.Error == "" },
	)
	report.Steps["me_student"] = meStudent

	adminListStudent := v.httpRequest(
		"admin_users_student",
		"GET",
		v.base.Users+"/admin/users",
		map[string]string{"Authorization": "Bearer " + v.state.StudentToken},
		"",
		"403",
		func(result RequestResult) bool { return result.StatusCode == 403 && result.Error == "" },
	)
	report.Steps["admin_users_student"] = adminListStudent

	adminListAdmin := v.httpRequest(
		"admin_users_admin",
		"GET",
		v.base.Users+"/admin/users",
		map[string]string{"Authorization": "Bearer " + v.state.AdminToken},
		"",
		"200",
		func(result RequestResult) bool { return result.StatusCode == 200 && result.Error == "" },
	)
	report.Steps["admin_users_admin"] = adminListAdmin

	report.Notes["studentId"] = v.state.StudentID
	report.Notes["studentEmail"] = v.state.StudentEmail
	report.Notes["adminSeedEmail"] = adminEmail

	for _, step := range report.Steps {
		if !step.OK {
			report.AllPassed = false
		}
	}

	if strings.TrimSpace(v.state.AdminToken) == "" {
		report.AllPassed = false
		v.addFailure(
			"auth/rbac",
			"no se pudo obtener token admin via seed configurado",
			"POST /auth/login",
			loginAdmin.Body,
			fileLink(v.root, "services/users-api/internal/app/bootstrap_admin.go"),
		)
	}

	v.writeTokensReport()
	v.writeJSONReport("users_rbac.json", report)
	v.summary.AuthRBAC = report.AllPassed
	if !report.AllPassed {
		v.addFailure(
			"auth/rbac",
			"fallo el flujo de autenticacion o permisos basicos",
			"POST /auth/register | POST /auth/login | GET /me | GET /admin/users",
			prettyJSON(report),
			fileLink(v.root, "services/users-api/internal/app/router.go"),
		)
	}

	return report.AllPassed
}
