package domain

import "time"

type Attachment struct {
	ID          string    `json:"id"`
	LessonID    string    `json:"lessonId,omitempty"`
	CourseID    string    `json:"courseId,omitempty"`
	Kind        string    `json:"kind"`
	FileName    string    `json:"fileName"`
	ContentType string    `json:"contentType"`
	SizeBytes   int64     `json:"sizeBytes"`
	CreatedAt   time.Time `json:"createdAt"`
	Data        []byte    `json:"-"`
}

type Lesson struct {
	ID          string       `json:"id"`
	CourseID    string       `json:"courseId"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	OrderIndex  int          `json:"orderIndex"`
	VideoURL    string       `json:"videoUrl"`
	Attachments []Attachment `json:"attachments,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}
