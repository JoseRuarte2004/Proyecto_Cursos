package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (v *Verifier) runChatRealtime() bool {
	report := StepReport{
		GeneratedAt: nowRFC3339(),
		Steps:       map[string]RequestResult{},
		Notes:       map[string]any{},
		AllPassed:   true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	teacherClient, teacherHello, wsURL, err := dialCourseChat(ctx, v.base.Nginx, v.state.CourseID, v.state.TeacherToken)
	if err != nil {
		report.AllPassed = false
		report.Notes["teacherConnectError"] = err.Error()
		v.addFailure(
			"chat realtime + history",
			"no se pudo conectar websocket del teacher",
			"GET "+sanitizeWSURL(wsURL),
			err.Error(),
			fileLink(v.root, "services/chat-api/internal/app/websocket.go"),
		)
		v.writeJSONReport("chat_flow.json", report)
		return false
	}
	defer teacherClient.Close()

	studentClient, studentHello, studentWSURL, err := dialCourseChat(ctx, v.base.Nginx, v.state.CourseID, v.state.StudentToken)
	if err != nil {
		report.AllPassed = false
		report.Notes["studentConnectError"] = err.Error()
		v.addFailure(
			"chat realtime + history",
			"no se pudo conectar websocket del student",
			"GET "+sanitizeWSURL(studentWSURL),
			err.Error(),
			fileLink(v.root, "services/chat-api/internal/app/websocket.go"),
		)
		v.writeJSONReport("chat_flow.json", report)
		return false
	}
	defer studentClient.Close()

	report.Notes["teacherHello"] = teacherHello
	report.Notes["studentHello"] = studentHello

	content := fmt.Sprintf("Mensaje QA chat %d", timestampNonce())
	if err := teacherClient.sendMessage(content); err != nil {
		report.AllPassed = false
		report.Notes["sendError"] = err.Error()
		v.addFailure(
			"chat realtime + history",
			"el teacher no pudo enviar mensaje websocket",
			"ws send message",
			err.Error(),
			fileLink(v.root, "services/chat-api/internal/app/websocket.go"),
		)
		v.writeJSONReport("chat_flow.json", report)
		return false
	}

	studentMessage, err := studentClient.readJSON(15 * time.Second)
	if err != nil {
		report.AllPassed = false
		report.Notes["studentReceiveError"] = err.Error()
		v.addFailure(
			"chat realtime + history",
			"el student no recibio el mensaje websocket",
			"ws receive message",
			err.Error(),
			fileLink(v.root, "services/chat-api/internal/app/websocket.go"),
		)
		v.writeJSONReport("chat_flow.json", report)
		return false
	}

	teacherEcho, err := teacherClient.readJSON(15 * time.Second)
	if err != nil {
		report.Notes["teacherEchoError"] = err.Error()
	} else {
		report.Notes["teacherEcho"] = teacherEcho
	}

	v.state.ChatMsgID = stringFromMap(studentMessage, "id")
	report.Notes["studentReceived"] = studentMessage
	if stringFromMap(studentMessage, "type") != "message" ||
		stringFromMap(studentMessage, "content") != content ||
		stringFromMap(studentMessage, "senderId") != v.state.TeacherID ||
		stringFromMap(studentMessage, "courseId") != v.state.CourseID {
		report.AllPassed = false
		v.addFailure(
			"chat realtime + history",
			"el payload recibido por websocket no coincide con el mensaje enviado",
			"ws send/receive",
			prettyJSON(studentMessage),
			fileLink(v.root, "services/chat-api/internal/app/websocket.go"),
		)
	}

	history := v.httpRequest(
		"chat_history",
		"GET",
		v.base.Nginx+"/api/chat/courses/"+v.state.CourseID+"/messages?limit=20",
		map[string]string{"Authorization": "Bearer " + v.state.StudentToken},
		"",
		"200 and includes chat message",
		func(result RequestResult) bool {
			return result.StatusCode == 200 && result.Error == "" && containsMessage(result.rawBody, v.state.ChatMsgID, content)
		},
	)
	report.Steps["chat_history"] = history

	restMessage := v.httpRequest(
		"chat_post_message",
		"POST",
		v.base.Nginx+"/api/chat/courses/"+v.state.CourseID+"/messages",
		map[string]string{
			"Authorization": "Bearer " + v.state.StudentToken,
			"Content-Type":  "application/json",
		},
		fmt.Sprintf(`{"content":"%s"}`, "Mensaje REST chat"),
		"201",
		func(result RequestResult) bool { return result.StatusCode == 201 && result.Error == "" },
	)
	report.Steps["chat_post_message"] = restMessage

	dbQuery := fmt.Sprintf(`SELECT id, course_id, sender_id, sender_role, content FROM chat_messages WHERE course_id = '%s' ORDER BY created_at DESC LIMIT 5;`, v.state.CourseID)
	dbOutput, dbErr := v.queryPostgres(dbQuery)
	_ = os.WriteFile(filepath.Join(v.reportsDir, "db_chat_messages.txt"), []byte(dbOutput+"\n"), 0o644)
	if dbErr != nil || !strings.Contains(dbOutput, content) {
		report.AllPassed = false
		v.addFailure(
			"chat realtime + history",
			"no encontre el mensaje de chat persistido en Postgres",
			dbQuery,
			dbOutput,
			fileLink(v.root, "services/chat-api/internal/repository/postgres.go"),
		)
	}

	for _, step := range report.Steps {
		if !step.OK {
			report.AllPassed = false
		}
	}

	v.writeJSONReport("chat_flow.json", report)
	v.summary.ChatRealtime = report.AllPassed
	if !report.AllPassed {
		v.addFailure(
			"chat realtime + history",
			"fallo el flujo websocket o el historial REST del chat",
			"WS /api/chat/ws/courses/:courseId | GET /api/chat/courses/:courseId/messages",
			prettyJSON(report),
			fileLink(v.root, "services/chat-api/internal/app/router.go"),
		)
	}

	return report.AllPassed
}

func sanitizeWSURL(value string) string {
	if value == "" {
		return value
	}
	parsed := strings.Split(value, "?")
	return parsed[0] + "?token=REDACTED"
}

func containsMessage(raw, messageID, content string) bool {
	items := parseJSONArray(raw)
	for _, item := range items {
		if stringFromMap(item, "id") == messageID || stringFromMap(item, "content") == content {
			return true
		}
	}
	return false
}
