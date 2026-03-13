package main

import (
	"fmt"
)

func (v *Verifier) runCourses() bool {
	report := StepReport{
		GeneratedAt: nowRFC3339(),
		Steps:       map[string]RequestResult{},
		Notes:       map[string]any{},
		AllPassed:   true,
	}

	createCourse := v.httpRequest(
		"create_course_draft",
		"POST",
		v.base.Courses+"/courses",
		map[string]string{
			"Authorization": "Bearer " + v.state.AdminToken,
			"Content-Type":  "application/json",
		},
		`{"title":"Curso QA Verify","description":"Curso para flujo automatizado","category":"backend","imageUrl":"https://example.com/course.png","price":100.50,"currency":"USD","capacity":1,"status":"draft"}`,
		"201",
		func(result RequestResult) bool { return result.StatusCode == 201 && result.Error == "" },
	)
	report.Steps["create_course_draft"] = createCourse
	v.state.CourseID = stringFromMap(parseJSONMap(createCourse.rawBody), "id")

	publishCourse := v.httpRequest(
		"publish_course",
		"PATCH",
		v.base.Courses+"/courses/"+v.state.CourseID,
		map[string]string{
			"Authorization": "Bearer " + v.state.AdminToken,
			"Content-Type":  "application/json",
		},
		`{"status":"published"}`,
		"200",
		func(result RequestResult) bool { return result.StatusCode == 200 && result.Error == "" },
	)
	report.Steps["publish_course"] = publishCourse

	v.state.TeacherEmail = fmt.Sprintf("verify-teacher-%d@example.com", timestampNonce())
	teacherPassword := "teacher1234"
	registerTeacher := v.httpRequest(
		"register_teacher",
		"POST",
		v.base.Users+"/auth/register",
		map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"name":"Verify Teacher","email":"%s","password":"%s","phone":"222222","dni":"20000002","address":"QA Teacher 123"}`, v.state.TeacherEmail, teacherPassword),
		"201",
		func(result RequestResult) bool { return result.StatusCode == 201 && result.Error == "" },
	)
	report.Steps["register_teacher"] = registerTeacher
	v.state.TeacherID = stringFromMap(parseJSONMap(registerTeacher.rawBody), "id")

	promoteTeacher := v.httpRequest(
		"promote_teacher",
		"PATCH",
		v.base.Users+"/admin/users/"+v.state.TeacherID+"/role",
		map[string]string{
			"Authorization": "Bearer " + v.state.AdminToken,
			"Content-Type":  "application/json",
		},
		`{"role":"teacher"}`,
		"200",
		func(result RequestResult) bool { return result.StatusCode == 200 && result.Error == "" },
	)
	report.Steps["promote_teacher"] = promoteTeacher

	loginTeacher := v.httpRequest(
		"login_teacher",
		"POST",
		v.base.Users+"/auth/login",
		map[string]string{"Content-Type": "application/json"},
		fmt.Sprintf(`{"email":"%s","password":"%s"}`, v.state.TeacherEmail, teacherPassword),
		"200",
		func(result RequestResult) bool { return result.StatusCode == 200 && result.Error == "" },
	)
	report.Steps["login_teacher"] = loginTeacher
	v.state.TeacherToken = stringFromMap(parseJSONMap(loginTeacher.rawBody), "token")

	assignTeacher := v.httpRequest(
		"assign_teacher",
		"POST",
		v.base.Courses+"/courses/"+v.state.CourseID+"/teachers",
		map[string]string{
			"Authorization": "Bearer " + v.state.AdminToken,
			"Content-Type":  "application/json",
		},
		fmt.Sprintf(`{"teacherId":"%s"}`, v.state.TeacherID),
		"201",
		func(result RequestResult) bool { return result.StatusCode == 201 && result.Error == "" },
	)
	report.Steps["assign_teacher"] = assignTeacher

	teacherCourses := v.httpRequest(
		"teacher_me_courses",
		"GET",
		v.base.Courses+"/teacher/me/courses",
		map[string]string{"Authorization": "Bearer " + v.state.TeacherToken},
		"",
		"200 and includes course",
		func(result RequestResult) bool {
			return result.StatusCode == 200 && result.Error == "" && containsCourseByID(result.rawBody, v.state.CourseID)
		},
	)
	report.Steps["teacher_me_courses"] = teacherCourses

	publicCourses := v.httpRequest(
		"public_courses",
		"GET",
		v.base.Courses+"/courses?limit=10&offset=0",
		nil,
		"",
		"200 and includes course",
		func(result RequestResult) bool {
			return result.StatusCode == 200 && result.Error == "" && containsCourseByID(result.rawBody, v.state.CourseID)
		},
	)
	report.Steps["public_courses"] = publicCourses

	report.Notes["courseId"] = v.state.CourseID
	report.Notes["teacherId"] = v.state.TeacherID
	report.Notes["teacherEmail"] = v.state.TeacherEmail
	v.writeTokensReport()

	for _, step := range report.Steps {
		if !step.OK {
			report.AllPassed = false
		}
	}

	v.writeJSONReport("courses_flow.json", report)
	v.summary.Courses = report.AllPassed
	if !report.AllPassed {
		v.addFailure(
			"courses",
			"fallo el flujo de creacion/publicacion/asignacion del curso",
			"POST /courses | PATCH /courses/:id | POST /courses/:id/teachers | GET /teacher/me/courses",
			prettyJSON(report),
			fileLink(v.root, "services/courses-api/internal/app/router.go"),
		)
	}

	return report.AllPassed
}

func (v *Verifier) runContentPermissions() bool {
	report := StepReport{
		GeneratedAt: nowRFC3339(),
		Steps:       map[string]RequestResult{},
		Notes:       map[string]any{},
		AllPassed:   true,
	}

	lesson1 := v.httpRequest(
		"create_lesson_1",
		"POST",
		v.base.Content+"/courses/"+v.state.CourseID+"/lessons",
		map[string]string{
			"Authorization": "Bearer " + v.state.AdminToken,
			"Content-Type":  "application/json",
		},
		`{"title":"Lesson 1","description":"Intro","orderIndex":1,"videoUrl":"https://example.com/video-1"}`,
		"201",
		func(result RequestResult) bool { return result.StatusCode == 201 && result.Error == "" },
	)
	report.Steps["create_lesson_1"] = lesson1

	lesson2 := v.httpRequest(
		"create_lesson_2",
		"POST",
		v.base.Content+"/courses/"+v.state.CourseID+"/lessons",
		map[string]string{
			"Authorization": "Bearer " + v.state.AdminToken,
			"Content-Type":  "application/json",
		},
		`{"title":"Lesson 2","description":"Advanced","orderIndex":2,"videoUrl":"https://example.com/video-2"}`,
		"201",
		func(result RequestResult) bool { return result.StatusCode == 201 && result.Error == "" },
	)
	report.Steps["create_lesson_2"] = lesson2

	teacherRead := v.httpRequest(
		"teacher_read_lessons",
		"GET",
		v.base.Content+"/courses/"+v.state.CourseID+"/lessons",
		map[string]string{"Authorization": "Bearer " + v.state.TeacherToken},
		"",
		"200",
		func(result RequestResult) bool { return result.StatusCode == 200 && result.Error == "" },
	)
	report.Steps["teacher_read_lessons"] = teacherRead

	studentBeforeEnrollment := v.httpRequest(
		"student_read_lessons_before_enrollment",
		"GET",
		v.base.Content+"/courses/"+v.state.CourseID+"/lessons",
		map[string]string{"Authorization": "Bearer " + v.state.StudentToken},
		"",
		"not 200",
		func(result RequestResult) bool { return result.StatusCode != 200 && result.Error == "" },
	)
	report.Steps["student_read_lessons_before_enrollment"] = studentBeforeEnrollment

	report.Notes["courseId"] = v.state.CourseID
	for _, step := range report.Steps {
		if !step.OK {
			report.AllPassed = false
		}
	}

	v.writeJSONReport("content_permissions.json", report)
	v.summary.ContentPermissions = report.AllPassed
	if !report.AllPassed {
		v.addFailure(
			"content permisos",
			"fallaron los permisos de lessons para teacher o student",
			"POST /courses/:courseId/lessons | GET /courses/:courseId/lessons",
			prettyJSON(report),
			fileLink(v.root, "services/course-content-api/internal/app/router.go"),
		)
	}

	return report.AllPassed
}
