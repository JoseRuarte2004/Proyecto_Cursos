package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (r *Runner) Run() error {
	if err := r.ensureReportsDir(); err != nil {
		return err
	}
	if err := r.ensureEnvFile(); err != nil {
		return err
	}
	if err := r.loadEnv(); err != nil {
		return err
	}
	if err := r.ensureCommand("docker"); err != nil {
		return err
	}

	if err := r.composeUp(); err != nil {
		_ = r.writeReport()
		return err
	}
	r.summary.ComposeUp = true

	if err := r.waitHTTPReady(); err != nil {
		_ = r.writeReport()
		return err
	}
	r.summary.HealthReady = true

	r.runFlow()

	if err := r.writeReport(); err != nil {
		return err
	}

	if len(r.failures) > 0 || !r.report.AllPassed {
		return errors.New("integration failed")
	}

	return nil
}

func (r *Runner) composeUp() error {
	result := r.runCommand(10*time.Minute, "docker", "compose", "-f", composeFile, "up", "-d", "--build")
	if err := os.WriteFile(filepath.Join(r.reportsDir, "integration_compose_up.txt"), []byte(joinOutput(result)), 0o644); err != nil {
		return err
	}
	if result.Err != nil {
		r.addFailure("compose up", "docker compose up -d --build fallo", "docker compose -f infra/docker-compose.yml up -d --build", joinOutput(result))
		return errors.New("compose up fallo")
	}
	return nil
}

func (r *Runner) waitHTTPReady() error {
	services := map[string]string{
		"users-api":          r.base.Users,
		"courses-api":        r.base.Courses,
		"course-content-api": r.base.Content,
		"enrollments-api":    r.base.Enrollments,
		"payments-api":       r.base.Payments,
	}

	deadline := time.Now().Add(6 * time.Minute)
	last := []string{}
	for time.Now().Before(deadline) {
		allReady := true
		last = last[:0]
		for service, baseURL := range services {
			health := r.httpRequest(service+"_health_wait", http.MethodGet, baseURL+"/health", nil, "")
			ready := r.httpRequest(service+"_ready_wait", http.MethodGet, baseURL+"/ready", nil, "")
			last = append(last, fmt.Sprintf("%s health=%d ready=%d", service, health.StatusCode, ready.StatusCode))
			if health.StatusCode != http.StatusOK || ready.StatusCode != http.StatusOK {
				allReady = false
			}
		}
		if allReady {
			return nil
		}
		time.Sleep(5 * time.Second)
	}

	evidence := strings.Join(last, "\n")
	_ = os.WriteFile(filepath.Join(r.reportsDir, "integration_wait.txt"), []byte(evidence), 0o644)
	r.addFailure("health/ready", "los servicios no llegaron a responder 200 en /health y /ready", "GET /health | GET /ready", evidence)
	return errors.New("wait health/ready fallo")
}

func (r *Runner) queryPostgres(query string) (string, error) {
	result := r.runCommand(
		2*time.Minute,
		"docker", "compose", "-f", composeFile,
		"exec", "-T",
		"-e", "PGPASSWORD="+r.getEnv("POSTGRES_PASSWORD", "postgres"),
		"postgres",
		"psql",
		"-U", r.getEnv("POSTGRES_USER", "postgres"),
		"-d", r.getEnv("POSTGRES_DB", "platform_dev"),
		"-At",
		"-F", "|",
		"-c", query,
	)
	if result.Err != nil {
		return joinOutput(result), result.Err
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (r *Runner) runFlow() {
	r.checkHealthAndMetrics()
	r.runAuthAndRBAC()
	if r.summary.AuthRBAC {
		r.runCoursesAndContent()
	}
	if r.summary.Courses && r.summary.Content {
		r.runEnrollmentsAndPayments()
	}
	r.validateContracts()
}

func (r *Runner) checkHealthAndMetrics() {
	services := map[string]string{
		"users-api":          r.base.Users,
		"courses-api":        r.base.Courses,
		"course-content-api": r.base.Content,
		"enrollments-api":    r.base.Enrollments,
		"payments-api":       r.base.Payments,
	}

	healthOK := true
	metricsOK := true
	for service, baseURL := range services {
		health := r.httpRequest(service+"_health", http.MethodGet, baseURL+"/health", nil, "")
		health.OK = health.StatusCode == http.StatusOK && health.Error == ""
		ready := r.httpRequest(service+"_ready", http.MethodGet, baseURL+"/ready", nil, "")
		ready.OK = ready.StatusCode == http.StatusOK && ready.Error == ""
		metrics := r.httpRequest(service+"_metrics", http.MethodGet, baseURL+"/metrics", nil, "")
		metrics.OK = metrics.StatusCode == http.StatusOK && metrics.Error == "" && strings.Contains(metrics.rawBody, "http_requests_total")

		r.report.Steps[health.Name] = health
		r.report.Steps[ready.Name] = ready
		r.report.Steps[metrics.Name] = metrics

		if !health.OK || !ready.OK {
			healthOK = false
		}
		if !metrics.OK {
			metricsOK = false
		}
	}

	r.summary.HealthReady = r.summary.HealthReady && healthOK
	r.report.Contracts.MetricsOK = metricsOK
}

func (r *Runner) runAuthAndRBAC() {
	adminEmail := r.getEnv("USERS_API_BOOTSTRAP_ADMIN_EMAIL", "admin@example.com")
	adminPassword := r.getEnv("USERS_API_BOOTSTRAP_ADMIN_PASSWORD", "admin1234")

	loginAdmin := r.httpRequest(
		"login_admin",
		http.MethodPost,
		r.base.Users+"/auth/login",
		map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"email":"%s","password":"%s"}`, adminEmail, adminPassword),
	)
	loginAdmin.OK = loginAdmin.StatusCode == http.StatusOK && loginAdmin.Error == ""
	r.report.Steps[loginAdmin.Name] = loginAdmin
	r.state.AdminToken = stringFromMap(parseJSONMap(loginAdmin.rawBody), "token")
	r.report.Notes["adminSeedEmail"] = adminEmail

	r.state.StudentEmail = fmt.Sprintf("integration-student-%d@example.com", nonce())
	registerStudent := r.httpRequest(
		"register_student",
		http.MethodPost,
		r.base.Users+"/auth/register",
		map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"name":"Integration Student","email":"%s","password":"student1234","phone":"111","dni":"301","address":"QA 1"}`, r.state.StudentEmail),
	)
	registerStudent.OK = registerStudent.StatusCode == http.StatusCreated && registerStudent.Error == ""
	r.report.Steps[registerStudent.Name] = registerStudent
	r.state.StudentID = stringFromMap(parseJSONMap(registerStudent.rawBody), "id")

	loginStudent := r.httpRequest(
		"login_student",
		http.MethodPost,
		r.base.Users+"/auth/login",
		map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"email":"%s","password":"student1234"}`, r.state.StudentEmail),
	)
	loginStudent.OK = loginStudent.StatusCode == http.StatusOK && loginStudent.Error == ""
	r.report.Steps[loginStudent.Name] = loginStudent
	r.state.StudentToken = stringFromMap(parseJSONMap(loginStudent.rawBody), "token")

	meStudent := r.httpRequest(
		"me_student",
		http.MethodGet,
		r.base.Users+"/me",
		map[string]string{"Authorization": "Bearer " + r.state.StudentToken},
		"",
	)
	meStudent.OK = meStudent.StatusCode == http.StatusOK && meStudent.Error == ""
	r.report.Steps[meStudent.Name] = meStudent

	adminUsersStudent := r.httpRequest(
		"admin_users_as_student",
		http.MethodGet,
		r.base.Users+"/admin/users",
		map[string]string{"Authorization": "Bearer " + r.state.StudentToken},
		"",
	)
	adminUsersStudent.OK = adminUsersStudent.StatusCode == http.StatusForbidden && adminUsersStudent.Error == ""
	r.report.Steps[adminUsersStudent.Name] = adminUsersStudent

	adminUsersAdmin := r.httpRequest(
		"admin_users_as_admin",
		http.MethodGet,
		r.base.Users+"/admin/users",
		map[string]string{"Authorization": "Bearer " + r.state.AdminToken},
		"",
	)
	adminUsersAdmin.OK = adminUsersAdmin.StatusCode == http.StatusOK && adminUsersAdmin.Error == ""
	r.report.Steps[adminUsersAdmin.Name] = adminUsersAdmin

	r.summary.AuthRBAC = loginAdmin.OK && registerStudent.OK && loginStudent.OK && meStudent.OK && adminUsersStudent.OK && adminUsersAdmin.OK && r.state.AdminToken != ""
	if !r.summary.AuthRBAC {
		r.addFailure("auth/rbac", "fallo el flujo de auth o RBAC base", "users-api auth flow", stringifySteps(
			loginAdmin, registerStudent, loginStudent, meStudent, adminUsersStudent, adminUsersAdmin,
		))
	}
}

func (r *Runner) runCoursesAndContent() {
	r.state.TeacherEmail = fmt.Sprintf("integration-teacher-%d@example.com", nonce())

	registerTeacher := r.httpRequest(
		"register_teacher",
		http.MethodPost,
		r.base.Users+"/auth/register",
		map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"name":"Integration Teacher","email":"%s","password":"teacher1234","phone":"222","dni":"302","address":"QA 2"}`, r.state.TeacherEmail),
	)
	registerTeacher.OK = registerTeacher.StatusCode == http.StatusCreated && registerTeacher.Error == ""
	r.report.Steps[registerTeacher.Name] = registerTeacher
	r.state.TeacherID = stringFromMap(parseJSONMap(registerTeacher.rawBody), "id")

	promoteTeacher := r.httpRequest(
		"promote_teacher",
		http.MethodPatch,
		r.base.Users+"/admin/users/"+r.state.TeacherID+"/role",
		map[string]string{
			"Authorization": "Bearer " + r.state.AdminToken,
			"Content-Type":  "application/json",
		},
		`{"role":"teacher"}`,
	)
	promoteTeacher.OK = promoteTeacher.StatusCode == http.StatusOK && promoteTeacher.Error == ""
	r.report.Steps[promoteTeacher.Name] = promoteTeacher

	loginTeacher := r.httpRequest(
		"login_teacher",
		http.MethodPost,
		r.base.Users+"/auth/login",
		map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"email":"%s","password":"teacher1234"}`, r.state.TeacherEmail),
	)
	loginTeacher.OK = loginTeacher.StatusCode == http.StatusOK && loginTeacher.Error == ""
	r.report.Steps[loginTeacher.Name] = loginTeacher
	r.state.TeacherToken = stringFromMap(parseJSONMap(loginTeacher.rawBody), "token")

	createCourse := r.httpRequest(
		"create_course",
		http.MethodPost,
		r.base.Courses+"/courses",
		map[string]string{
			"Authorization": "Bearer " + r.state.AdminToken,
			"Content-Type":  "application/json",
		},
		`{"title":"QA Integration Course","description":"Flujo E2E","category":"backend","price":100.5,"currency":"USD","capacity":1,"status":"draft"}`,
	)
	createCourse.OK = createCourse.StatusCode == http.StatusCreated && createCourse.Error == ""
	r.report.Steps[createCourse.Name] = createCourse
	r.state.CourseID = stringFromMap(parseJSONMap(createCourse.rawBody), "id")

	publishCourse := r.httpRequest(
		"publish_course",
		http.MethodPatch,
		r.base.Courses+"/courses/"+r.state.CourseID,
		map[string]string{
			"Authorization": "Bearer " + r.state.AdminToken,
			"Content-Type":  "application/json",
		},
		`{"status":"published"}`,
	)
	publishCourse.OK = publishCourse.StatusCode == http.StatusOK && publishCourse.Error == ""
	r.report.Steps[publishCourse.Name] = publishCourse

	assignTeacher := r.httpRequest(
		"assign_teacher",
		http.MethodPost,
		r.base.Courses+"/courses/"+r.state.CourseID+"/teachers",
		map[string]string{
			"Authorization": "Bearer " + r.state.AdminToken,
			"Content-Type":  "application/json",
		},
		fmt.Sprintf(`{"teacherId":"%s"}`, r.state.TeacherID),
	)
	assignTeacher.OK = assignTeacher.StatusCode == http.StatusCreated && assignTeacher.Error == ""
	r.report.Steps[assignTeacher.Name] = assignTeacher

	teacherCourses := r.httpRequest(
		"teacher_courses",
		http.MethodGet,
		r.base.Courses+"/teacher/me/courses",
		map[string]string{"Authorization": "Bearer " + r.state.TeacherToken},
		"",
	)
	teacherCourses.OK = teacherCourses.StatusCode == http.StatusOK && containsItemByID(teacherCourses.rawBody, r.state.CourseID)
	r.report.Steps[teacherCourses.Name] = teacherCourses

	publicCourses := r.httpRequest(
		"public_courses",
		http.MethodGet,
		r.base.Courses+"/courses?limit=10&offset=0",
		nil,
		"",
	)
	publicCourses.OK = publicCourses.StatusCode == http.StatusOK && containsItemByID(publicCourses.rawBody, r.state.CourseID)
	r.report.Steps[publicCourses.Name] = publicCourses

	lesson1 := r.httpRequest(
		"create_lesson_1",
		http.MethodPost,
		r.base.Content+"/courses/"+r.state.CourseID+"/lessons",
		map[string]string{
			"Authorization": "Bearer " + r.state.AdminToken,
			"Content-Type":  "application/json",
		},
		`{"title":"Lesson 1","description":"Intro","orderIndex":1,"videoUrl":"https://video.test/1"}`,
	)
	lesson1.OK = lesson1.StatusCode == http.StatusCreated && lesson1.Error == ""
	r.report.Steps[lesson1.Name] = lesson1

	lesson2 := r.httpRequest(
		"create_lesson_2",
		http.MethodPost,
		r.base.Content+"/courses/"+r.state.CourseID+"/lessons",
		map[string]string{
			"Authorization": "Bearer " + r.state.AdminToken,
			"Content-Type":  "application/json",
		},
		`{"title":"Lesson 2","description":"Deep Dive","orderIndex":2,"videoUrl":"https://video.test/2"}`,
	)
	lesson2.OK = lesson2.StatusCode == http.StatusCreated && lesson2.Error == ""
	r.report.Steps[lesson2.Name] = lesson2

	teacherLessons := r.httpRequest(
		"teacher_lessons",
		http.MethodGet,
		r.base.Content+"/courses/"+r.state.CourseID+"/lessons",
		map[string]string{"Authorization": "Bearer " + r.state.TeacherToken},
		"",
	)
	teacherLessons.OK = teacherLessons.StatusCode == http.StatusOK && teacherLessons.Error == ""
	r.report.Steps[teacherLessons.Name] = teacherLessons

	studentLessonsBefore := r.httpRequest(
		"student_lessons_before_enrollment",
		http.MethodGet,
		r.base.Content+"/courses/"+r.state.CourseID+"/lessons",
		map[string]string{"Authorization": "Bearer " + r.state.StudentToken},
		"",
	)
	studentLessonsBefore.OK = studentLessonsBefore.StatusCode != http.StatusOK && studentLessonsBefore.Error == ""
	r.report.Steps[studentLessonsBefore.Name] = studentLessonsBefore

	r.summary.Courses = registerTeacher.OK && promoteTeacher.OK && loginTeacher.OK && createCourse.OK && publishCourse.OK && assignTeacher.OK && teacherCourses.OK && publicCourses.OK
	r.summary.Content = lesson1.OK && lesson2.OK && teacherLessons.OK && studentLessonsBefore.OK

	if !r.summary.Courses {
		r.addFailure("courses", "fallo el flujo de cursos", "courses flow", stringifySteps(
			registerTeacher, promoteTeacher, loginTeacher, createCourse, publishCourse, assignTeacher, teacherCourses, publicCourses,
		))
	}
	if !r.summary.Content {
		r.addFailure("content", "fallo el flujo de contenido/permisos", "content flow", stringifySteps(
			lesson1, lesson2, teacherLessons, studentLessonsBefore,
		))
	}
}

func (r *Runner) runEnrollmentsAndPayments() {
	availabilityBefore := r.httpRequest(
		"availability_before",
		http.MethodGet,
		r.base.Enrollments+"/courses/"+r.state.CourseID+"/availability",
		nil,
		"",
	)
	availabilityBefore.OK = availabilityBefore.StatusCode == http.StatusOK &&
		int(floatFromMap(parseJSONMap(availabilityBefore.rawBody), "capacity")) == 1 &&
		int(floatFromMap(parseJSONMap(availabilityBefore.rawBody), "available")) == 1
	r.report.Steps[availabilityBefore.Name] = availabilityBefore

	reserve := r.httpRequest(
		"reserve_enrollment",
		http.MethodPost,
		r.base.Enrollments+"/enrollments/reserve",
		map[string]string{
			"Authorization": "Bearer " + r.state.StudentToken,
			"Content-Type":  "application/json",
		},
		fmt.Sprintf(`{"courseId":"%s"}`, r.state.CourseID),
	)
	reserve.OK = reserve.StatusCode == http.StatusCreated && reserve.Error == ""
	r.report.Steps[reserve.Name] = reserve

	duplicateReserve := r.httpRequest(
		"duplicate_reserve",
		http.MethodPost,
		r.base.Enrollments+"/enrollments/reserve",
		map[string]string{
			"Authorization": "Bearer " + r.state.StudentToken,
			"Content-Type":  "application/json",
		},
		fmt.Sprintf(`{"courseId":"%s"}`, r.state.CourseID),
	)
	duplicateReserve.OK = duplicateReserve.StatusCode == http.StatusConflict && duplicateReserve.Error == ""
	r.report.Steps[duplicateReserve.Name] = duplicateReserve

	createOrder := r.httpRequest(
		"create_order",
		http.MethodPost,
		r.base.Payments+"/orders",
		map[string]string{
			"Authorization":   "Bearer " + r.state.StudentToken,
			"Content-Type":    "application/json",
			"Idempotency-Key": fmt.Sprintf("integration-order-%d", nonce()),
		},
		fmt.Sprintf(`{"courseId":"%s","provider":"mercadopago"}`, r.state.CourseID),
	)
	createOrder.OK = createOrder.StatusCode == http.StatusCreated && stringFromMap(parseJSONMap(createOrder.rawBody), "orderId") != ""
	r.report.Steps[createOrder.Name] = createOrder
	r.state.OrderID = stringFromMap(parseJSONMap(createOrder.rawBody), "orderId")

	webhook1 := r.httpRequest(
		"webhook_paid_first",
		http.MethodPost,
		r.base.Payments+"/webhooks/mercadopago",
		map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"orderId":"%s","providerPaymentId":"pay_%d","status":"paid"}`, r.state.OrderID, nonce()),
	)
	webhook1.OK = webhook1.StatusCode == http.StatusOK && boolFromMap(parseJSONMap(webhook1.rawBody), "published")
	r.report.Steps[webhook1.Name] = webhook1

	webhook2 := r.httpRequest(
		"webhook_paid_second",
		http.MethodPost,
		r.base.Payments+"/webhooks/mercadopago",
		map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"orderId":"%s","providerPaymentId":"pay_repeat_%d","status":"paid"}`, r.state.OrderID, nonce()),
	)
	webhook2.OK = webhook2.StatusCode == http.StatusOK && !boolFromMap(parseJSONMap(webhook2.rawBody), "published")
	r.report.Steps[webhook2.Name] = webhook2

	meActiveOK, meEvidence := poll(45*time.Second, 2*time.Second, func() (bool, string) {
		me := r.httpRequest(
			"student_enrollments_after_paid_poll",
			http.MethodGet,
			r.base.Enrollments+"/me/enrollments",
			map[string]string{"Authorization": "Bearer " + r.state.StudentToken},
			"",
		)
		if findEnrollmentStatus(me.rawBody, r.state.CourseID) == "active" {
			me.OK = true
			r.report.Steps["student_enrollments_after_paid"] = me
			return true, me.rawBody
		}
		return false, me.rawBody
	})
	if !meActiveOK {
		r.addFailure("enrollments", "el enrollment no paso a active via HTTP", "GET /me/enrollments", meEvidence)
	}

	orderSQL := fmt.Sprintf(`SELECT id, status FROM orders WHERE id = '%s';`, r.state.OrderID)
	orderOK, orderEvidence := poll(30*time.Second, 2*time.Second, func() (bool, string) {
		output, err := r.queryPostgres(orderSQL)
		return err == nil && strings.Contains(output, "|paid"), output
	})
	r.report.Notes["orderQuery"] = orderEvidence
	if !orderOK {
		r.addFailure("payments", "la orden no quedo en paid en Postgres", orderSQL, orderEvidence)
	}

	enrollmentSQL := fmt.Sprintf(`SELECT user_id, course_id, status FROM enrollments WHERE user_id = '%s' AND course_id = '%s';`, r.state.StudentID, r.state.CourseID)
	enrollmentOK, enrollmentEvidence := poll(30*time.Second, 2*time.Second, func() (bool, string) {
		output, err := r.queryPostgres(enrollmentSQL)
		return err == nil && strings.Contains(output, "|active"), output
	})
	r.report.Notes["enrollmentQuery"] = enrollmentEvidence
	if !enrollmentOK {
		r.addFailure("enrollments", "el enrollment no quedo active en Postgres", enrollmentSQL, enrollmentEvidence)
	}

	studentLessonsAfter := r.httpRequest(
		"student_lessons_after_payment",
		http.MethodGet,
		r.base.Content+"/courses/"+r.state.CourseID+"/lessons",
		map[string]string{"Authorization": "Bearer " + r.state.StudentToken},
		"",
	)
	studentLessonsAfter.OK = studentLessonsAfter.StatusCode == http.StatusOK && studentLessonsAfter.Error == ""
	r.report.Steps[studentLessonsAfter.Name] = studentLessonsAfter

	r.summary.Enrollments = availabilityBefore.OK && reserve.OK && duplicateReserve.OK && meActiveOK && enrollmentOK
	r.summary.Payments = createOrder.OK && webhook1.OK && webhook2.OK && orderOK && studentLessonsAfter.OK

	if !r.summary.Enrollments {
		r.addFailure("enrollments", "fallo el flujo de reserva/activacion", "enrollments flow", stringifySteps(
			availabilityBefore, reserve, duplicateReserve,
		))
	}
	if !r.summary.Payments {
		r.addFailure("payments", "fallo el flujo de pagos/webhook/idempotencia", "payments flow", stringifySteps(
			createOrder, webhook1, webhook2, studentLessonsAfter,
		))
	}
}

func (r *Runner) validateContracts() {
	errorEnvelopeValid := true
	successJSONValid := true
	failures := make([]string, 0)

	for name, step := range r.report.Steps {
		switch {
		case strings.HasSuffix(name, "_metrics"):
			if step.StatusCode != http.StatusOK {
				failures = append(failures, name+": /metrics no devolvio 200")
			}
		case step.StatusCode >= 200 && step.StatusCode < 300:
			if strings.TrimSpace(step.rawBody) == "" {
				successJSONValid = false
				failures = append(failures, name+": respuesta exitosa vacia")
				continue
			}
			if err := asJSON(step.rawBody); err != nil {
				successJSONValid = false
				failures = append(failures, name+": respuesta exitosa no es JSON valido")
			}
		case step.StatusCode >= 400:
			payload := parseJSONMap(step.rawBody)
			errorPayload, ok := payload["error"].(map[string]any)
			if !ok || stringFromMap(errorPayload, "code") == "" || stringFromMap(errorPayload, "message") == "" || stringFromMap(payload, "requestId") == "" {
				errorEnvelopeValid = false
				failures = append(failures, name+": error envelope invalido")
			}
		}
	}

	r.report.Contracts.ErrorEnvelopeValid = errorEnvelopeValid
	r.report.Contracts.SuccessJSONValid = successJSONValid
	r.report.Contracts.MetricsOK = r.report.Contracts.MetricsOK && !containsContractFailure(failures, "/metrics")
	r.report.Contracts.Failures = failures
	r.summary.Contracts = errorEnvelopeValid && successJSONValid && r.report.Contracts.MetricsOK

	if !r.summary.Contracts {
		r.addFailure("contracts", "fallaron validaciones de contrato JSON/errores/metrics", "contract validation", strings.Join(failures, "\n"))
	}
}

func containsContractFailure(items []string, needle string) bool {
	for _, item := range items {
		if strings.Contains(item, needle) {
			return true
		}
	}
	return false
}

func stringifySteps(steps ...RequestResult) string {
	parts := make([]string, 0, len(steps))
	for _, step := range steps {
		parts = append(parts, fmt.Sprintf("%s status=%s%s body=%s", step.Name, intString(step.StatusCode), errorSuffix(step.Error), step.Body))
	}
	return strings.Join(parts, "\n")
}

func errorSuffix(err string) string {
	if strings.TrimSpace(err) == "" {
		return ""
	}
	return " error=" + err
}

func (r *Runner) addFailure(section, message, command, evidence string) {
	r.failures = append(r.failures, Failure{
		Section:  section,
		Message:  message,
		Command:  command,
		Evidence: evidence,
	})
}

func (r *Runner) writeReport() error {
	r.report.AllPassed = len(r.failures) == 0 &&
		r.summary.ComposeUp &&
		r.summary.HealthReady &&
		r.summary.AuthRBAC &&
		r.summary.Courses &&
		r.summary.Content &&
		r.summary.Enrollments &&
		r.summary.Payments &&
		r.summary.Contracts

	data, err := json.MarshalIndent(r.report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.reportsDir, "integration_flow.json"), data, 0o644)
}

func (r *Runner) printCheck(ok bool, label string) {
	if ok {
		fmt.Printf("OK   %s\n", label)
		return
	}
	fmt.Printf("FAIL %s\n", label)
}

func (r *Runner) PrintSummary() {
	r.printCheck(r.summary.ComposeUp, "compose up")
	r.printCheck(r.summary.HealthReady, "health/ready")
	r.printCheck(r.summary.AuthRBAC, "auth/rbac")
	r.printCheck(r.summary.Courses, "courses")
	r.printCheck(r.summary.Content, "content")
	r.printCheck(r.summary.Enrollments, "enrollments")
	r.printCheck(r.summary.Payments, "payments")
	r.printCheck(r.summary.Contracts, "contracts")

	if len(r.failures) == 0 && r.report.AllPassed {
		fmt.Println("TODO OK")
		return
	}

	fmt.Println("FAIL")
	for i, failure := range r.failures {
		fmt.Printf("%d. [%s] %s\n", i+1, failure.Section, failure.Message)
		if failure.Command != "" {
			fmt.Printf("   comando: %s\n", failure.Command)
		}
		if failure.Evidence != "" {
			fmt.Printf("   evidencia: %s\n", strings.ReplaceAll(strings.ReplaceAll(failure.Evidence, "\r", " "), "\n", " | "))
		}
	}
}
