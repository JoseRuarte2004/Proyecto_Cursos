package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (v *Verifier) runEnrollmentsAndPayments() bool {
	enrollmentReport := StepReport{
		GeneratedAt: nowRFC3339(),
		Steps:       map[string]RequestResult{},
		Notes:       map[string]any{},
		AllPassed:   true,
	}
	paymentsReport := StepReport{
		GeneratedAt: nowRFC3339(),
		Steps:       map[string]RequestResult{},
		Notes:       map[string]any{},
		AllPassed:   true,
	}
	webhookReport := StepReport{
		GeneratedAt: nowRFC3339(),
		Steps:       map[string]RequestResult{},
		Notes:       map[string]any{},
		AllPassed:   true,
	}

	availabilityBefore := v.httpRequest(
		"availability_before",
		"GET",
		v.base.Enrollments+"/courses/"+v.state.CourseID+"/availability",
		nil,
		"",
		"200 with capacity=1 and available=1",
		func(result RequestResult) bool {
			payload := parseJSONMap(result.rawBody)
			return result.StatusCode == 200 && result.Error == "" &&
				int(floatFromMap(payload, "capacity")) == 1 &&
				int(floatFromMap(payload, "available")) == 1
		},
	)
	enrollmentReport.Steps["availability_before"] = availabilityBefore

	reserveStudent1 := v.httpRequest(
		"reserve_student_1",
		"POST",
		v.base.Enrollments+"/enrollments/reserve",
		map[string]string{
			"Authorization": "Bearer " + v.state.StudentToken,
			"Content-Type":  "application/json",
		},
		fmt.Sprintf(`{"courseId":"%s"}`, v.state.CourseID),
		"201",
		func(result RequestResult) bool { return result.StatusCode == 201 && result.Error == "" },
	)
	enrollmentReport.Steps["reserve_student_1"] = reserveStudent1

	duplicateReserve := v.httpRequest(
		"reserve_student_1_duplicate",
		"POST",
		v.base.Enrollments+"/enrollments/reserve",
		map[string]string{
			"Authorization": "Bearer " + v.state.StudentToken,
			"Content-Type":  "application/json",
		},
		fmt.Sprintf(`{"courseId":"%s"}`, v.state.CourseID),
		"409",
		func(result RequestResult) bool { return result.StatusCode == 409 && result.Error == "" },
	)
	enrollmentReport.Steps["reserve_student_1_duplicate"] = duplicateReserve

	v.state.Student2Email = fmt.Sprintf("verify-student2-%d@example.com", timestampNonce())
	student2Password := "student2234"
	registerStudent2 := v.httpRequest(
		"register_student_2",
		"POST",
		v.base.Users+"/auth/register",
		map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"name":"Verify Student 2","email":"%s","password":"%s","phone":"333333","dni":"20000003","address":"QA 456"}`, v.state.Student2Email, student2Password),
		"201",
		func(result RequestResult) bool { return result.StatusCode == 201 && result.Error == "" },
	)
	enrollmentReport.Steps["register_student_2"] = registerStudent2
	v.state.Student2ID = stringFromMap(parseJSONMap(registerStudent2.rawBody), "id")

	verifyLinkStudent2, verifyEvidenceStudent2 := v.pollEmailLink("users-api", v.state.Student2Email, "Verify your email", 30*time.Second, 2*time.Second)
	enrollmentReport.Notes["student2VerifyLinkFound"] = verifyLinkStudent2 != ""
	if verifyLinkStudent2 == "" {
		enrollmentReport.AllPassed = false
		v.addFailure(
			"enrollments cupo",
			"no pude encontrar en logs el link de verificacion del email del segundo student",
			"docker logs users-api",
			verifyEvidenceStudent2,
			fileLink(v.root, "services/users-api/internal/app/mailer.go"),
		)
	} else {
		verifyStudent2 := v.httpRequest(
			"verify_student_2_email",
			"GET",
			verifyLinkStudent2,
			nil,
			"",
			"200",
			func(result RequestResult) bool {
				return result.StatusCode == 200 && result.Error == "" && stringFromMap(parseJSONMap(result.rawBody), "status") == "verified"
			},
		)
		enrollmentReport.Steps["verify_student_2_email"] = verifyStudent2
	}

	loginStudent2 := v.httpRequest(
		"login_student_2",
		"POST",
		v.base.Users+"/auth/login",
		map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"email":"%s","password":"%s"}`, v.state.Student2Email, student2Password),
		"200",
		func(result RequestResult) bool { return result.StatusCode == 200 && result.Error == "" },
	)
	enrollmentReport.Steps["login_student_2"] = loginStudent2
	v.state.Student2Token = stringFromMap(parseJSONMap(loginStudent2.rawBody), "token")
	v.writeTokensReport()

	createOrder := v.httpRequest(
		"create_order",
		"POST",
		v.base.Payments+"/orders",
		map[string]string{
			"Authorization":   "Bearer " + v.state.StudentToken,
			"Content-Type":    "application/json",
			"Idempotency-Key": "verify-order-" + fmt.Sprint(timestampNonce()),
		},
		fmt.Sprintf(`{"courseId":"%s","provider":"mercadopago"}`, v.state.CourseID),
		"201",
		func(result RequestResult) bool {
			payload := parseJSONMap(result.rawBody)
			return result.StatusCode == 201 && result.Error == "" &&
				stringFromMap(payload, "orderId") != "" &&
				stringFromMap(payload, "checkoutUrl") != ""
		},
	)
	paymentsReport.Steps["create_order"] = createOrder
	v.state.OrderID = stringFromMap(parseJSONMap(createOrder.rawBody), "orderId")

	webhookFirst := v.httpRequest(
		"webhook_paid_first",
		"POST",
		v.base.Payments+"/webhooks/mercadopago",
		map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"orderId":"%s","providerPaymentId":"pay_%d","status":"paid"}`, v.state.OrderID, timestampNonce()),
		"200 and published=true",
		func(result RequestResult) bool {
			payload := parseJSONMap(result.rawBody)
			return result.StatusCode == 200 && result.Error == "" && boolFromMap(payload, "published")
		},
	)
	paymentsReport.Steps["webhook_paid_first"] = webhookFirst
	webhookReport.Steps["webhook_paid_first"] = webhookFirst

	webhookSecond := v.httpRequest(
		"webhook_paid_second",
		"POST",
		v.base.Payments+"/webhooks/mercadopago",
		map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"orderId":"%s","providerPaymentId":"repeat_%d","status":"paid"}`, v.state.OrderID, timestampNonce()),
		"200 and published=false",
		func(result RequestResult) bool {
			payload := parseJSONMap(result.rawBody)
			return result.StatusCode == 200 && result.Error == "" && !boolFromMap(payload, "published")
		},
	)
	paymentsReport.Steps["webhook_paid_second"] = webhookSecond
	webhookReport.Steps["webhook_paid_second"] = webhookSecond

	orderQuery := fmt.Sprintf(`SELECT id, user_id, course_id, status, COALESCE(provider_payment_id,''), idempotency_key FROM orders WHERE id = '%s';`, v.state.OrderID)
	orderPoll := v.poll(45*time.Second, 2*time.Second, func() PollResult {
		output, err := v.queryPostgres(orderQuery)
		return PollResult{OK: err == nil && strings.Contains(output, "|paid|"), Evidence: output}
	})
	_ = osWrite(filepath.Join(v.reportsDir, "db_orders.txt"), orderPoll.Evidence)
	if !orderPoll.OK {
		paymentsReport.AllPassed = false
		v.addFailure(
			"payments + webhook idempotencia",
			"la orden no quedo en paid en la base",
			orderQuery,
			orderPoll.Evidence,
			fileLink(v.root, "services/payments-api/internal/repository/postgres.go"),
		)
	}

	enrollmentQuery := fmt.Sprintf(`SELECT id, user_id, course_id, status, created_at FROM enrollments WHERE user_id = '%s' AND course_id = '%s' ORDER BY created_at DESC;`, v.state.StudentID, v.state.CourseID)
	enrollmentPoll := v.poll(45*time.Second, 2*time.Second, func() PollResult {
		output, err := v.queryPostgres(enrollmentQuery)
		return PollResult{OK: err == nil && strings.Contains(output, "|active|"), Evidence: output}
	})
	_ = osWrite(filepath.Join(v.reportsDir, "db_enrollments.txt"), enrollmentPoll.Evidence)
	if !enrollmentPoll.OK {
		paymentsReport.AllPassed = false
		v.addFailure(
			"rabbit consume + enrollment active",
			"el enrollment no paso a active despues del webhook paid",
			enrollmentQuery,
			enrollmentPoll.Evidence,
			fileLink(v.root, "services/enrollments-api/internal/app/payment_paid_consumer.go"),
		)
	}

	meAfterPaid := v.httpRequest(
		"student_me_enrollments_after_paid",
		"GET",
		v.base.Enrollments+"/me/enrollments",
		map[string]string{"Authorization": "Bearer " + v.state.StudentToken},
		"",
		"200 and course active",
		func(result RequestResult) bool {
			return result.StatusCode == 200 && result.Error == "" && findEnrollmentStatus(result.rawBody, v.state.CourseID) == "active"
		},
	)
	paymentsReport.Steps["student_me_enrollments_after_paid"] = meAfterPaid

	studentLessonsAfterPaid := v.httpRequest(
		"student_lessons_after_paid",
		"GET",
		v.base.Content+"/courses/"+v.state.CourseID+"/lessons",
		map[string]string{"Authorization": "Bearer " + v.state.StudentToken},
		"",
		"200",
		func(result RequestResult) bool { return result.StatusCode == 200 && result.Error == "" },
	)
	paymentsReport.Steps["student_lessons_after_paid"] = studentLessonsAfterPaid

	availabilityAfterPaid := v.httpRequest(
		"availability_after_paid",
		"GET",
		v.base.Enrollments+"/courses/"+v.state.CourseID+"/availability",
		nil,
		"",
		"200 with available=0",
		func(result RequestResult) bool {
			payload := parseJSONMap(result.rawBody)
			return result.StatusCode == 200 && result.Error == "" &&
				int(floatFromMap(payload, "activeCount")) == 1 &&
				int(floatFromMap(payload, "available")) == 0
		},
	)
	enrollmentReport.Steps["availability_after_paid"] = availabilityAfterPaid

	reserveStudent2AfterPaid := v.httpRequest(
		"reserve_student_2_after_paid",
		"POST",
		v.base.Enrollments+"/enrollments/reserve",
		map[string]string{
			"Authorization": "Bearer " + v.state.Student2Token,
			"Content-Type":  "application/json",
		},
		fmt.Sprintf(`{"courseId":"%s"}`, v.state.CourseID),
		"409",
		func(result RequestResult) bool { return result.StatusCode == 409 && result.Error == "" },
	)
	enrollmentReport.Steps["reserve_student_2_after_paid"] = reserveStudent2AfterPaid

	consumerLogs, logsErr := v.dockerLogs("enrollments-api", 300)
	consumerFound := logsErr == nil && strings.Contains(consumerLogs, `"msg":"payment event processed"`) && strings.Contains(consumerLogs, v.state.OrderID)
	paymentsReport.Notes["consumerLogFound"] = consumerFound
	if consumerFound {
		paymentsReport.Notes["consumerLogSnippet"] = firstMatchingLine(consumerLogs, `"msg":"payment event processed"`)
	} else {
		paymentsReport.AllPassed = false
		v.addFailure(
			"rabbit consume + enrollment active",
			"no encontre evidencia del consumer payment.paid en logs de enrollments-api",
			"docker logs enrollments-api",
			consumerLogs,
			fileLink(v.root, "services/enrollments-api/internal/app/payment_paid_consumer.go"),
		)
	}

	enrollmentReport.Notes["courseId"] = v.state.CourseID
	enrollmentReport.Notes["student1Id"] = v.state.StudentID
	enrollmentReport.Notes["student2Id"] = v.state.Student2ID
	enrollmentReport.Notes["capacityValidationStrategy"] = "second student reserve checked after payment confirmation because availability counts ACTIVE seats"
	paymentsReport.Notes["orderId"] = v.state.OrderID
	webhookReport.Notes["orderId"] = v.state.OrderID

	for _, step := range enrollmentReport.Steps {
		if !step.OK {
			enrollmentReport.AllPassed = false
		}
	}
	for _, step := range paymentsReport.Steps {
		if !step.OK {
			paymentsReport.AllPassed = false
		}
	}
	for _, step := range webhookReport.Steps {
		if !step.OK {
			webhookReport.AllPassed = false
		}
	}

	v.writeJSONReport("enrollments_flow.json", enrollmentReport)
	v.writeJSONReport("payments_flow.json", paymentsReport)
	v.writeJSONReport("webhook_idempotency.json", webhookReport)

	v.summary.EnrollmentsCapacity = enrollmentReport.AllPassed
	v.summary.PaymentsWebhook = paymentsReport.Steps["create_order"].OK && webhookReport.AllPassed && strings.Contains(orderPoll.Evidence, "|paid|")
	v.summary.RabbitEnrollment = enrollmentPoll.OK && meAfterPaid.OK && studentLessonsAfterPaid.OK && consumerFound

	if !enrollmentReport.AllPassed {
		v.addFailure(
			"enrollments cupo",
			"fallo la reserva o el control de cupo",
			"GET /availability | POST /enrollments/reserve",
			prettyJSON(enrollmentReport),
			fileLink(v.root, "services/enrollments-api/internal/app/router.go"),
		)
	}
	if !v.summary.PaymentsWebhook {
		v.addFailure(
			"payments + webhook idempotencia",
			"fallo la creacion de order o la idempotencia del webhook",
			"POST /orders | POST /webhooks/mercadopago",
			prettyJSON(map[string]any{
				"payments": paymentsReport,
				"webhook":  webhookReport,
				"dbOrder":  orderPoll.Evidence,
			}),
			fileLink(v.root, "services/payments-api/internal/app/router.go"),
		)
	}
	if !v.summary.RabbitEnrollment {
		v.addFailure(
			"rabbit consume + enrollment active",
			"el evento payment.paid no quedo reflejado completamente en enrollment active + acceso a lessons",
			"payment.paid -> enrollments consumer",
			prettyJSON(map[string]any{
				"payments":     paymentsReport,
				"enrollmentDB": enrollmentPoll.Evidence,
				"consumerLogs": firstMatchingLine(consumerLogs, `"msg":"payment event processed"`),
			}),
			fileLink(v.root, "services/enrollments-api/internal/app/payment_paid_consumer.go"),
		)
	}

	return enrollmentReport.AllPassed && v.summary.PaymentsWebhook && v.summary.RabbitEnrollment
}

func (v *Verifier) runLogChecks() bool {
	for i := 0; i < 15; i++ {
		_ = v.httpRequest("users_health_noise", "GET", v.base.Users+"/health", nil, "", "200", func(result RequestResult) bool { return result.StatusCode == 200 })
		_ = v.httpRequest("payments_health_noise", "GET", v.base.Payments+"/health", nil, "", "200", func(result RequestResult) bool { return result.StatusCode == 200 })
	}

	report := LogReport{
		GeneratedAt: nowRFC3339(),
		Counts:      map[string]int{},
		Lines:       map[string][]string{},
		AllPassed:   true,
	}

	for _, service := range []string{"users-api", "payments-api"} {
		output, err := v.dockerLogs(service, 500)
		if err != nil {
			report.AllPassed = false
			v.addFailure("logs JSON", "no pude leer logs de "+service, "docker logs "+service, output, fileLink(v.root, "internal/platform/logger/logger.go"))
			continue
		}

		valid := make([]string, 0, 15)
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var payload map[string]any
			if err := json.Unmarshal([]byte(line), &payload); err != nil {
				continue
			}
			if stringFromMap(payload, "requestId") == "" {
				continue
			}
			valid = append(valid, line)
		}

		if len(valid) < 15 {
			report.AllPassed = false
			v.addFailure("logs JSON", "no encontre al menos 15 lineas JSON con requestId en "+service, "docker logs --tail 500 "+service, strings.Join(valid, "\n"), fileLink(v.root, "internal/platform/logger/logger.go"))
			report.Counts[service] = len(valid)
			report.Lines[service] = valid
			continue
		}

		report.Counts[service] = len(valid)
		report.Lines[service] = valid[len(valid)-15:]
	}

	combined := []string{}
	for _, service := range []string{"users-api", "payments-api"} {
		if len(report.Lines[service]) == 0 {
			continue
		}
		combined = append(combined, "# "+service)
		combined = append(combined, report.Lines[service]...)
	}
	_ = osWrite(filepath.Join(v.reportsDir, "log_samples.txt"), strings.Join(combined, "\n")+"\n")

	v.summary.LogsJSON = report.AllPassed
	if !report.AllPassed {
		v.writeJSONReport("log_samples.json", report)
		return false
	}

	return true
}

func osWrite(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func firstMatchingLine(output, needle string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}
