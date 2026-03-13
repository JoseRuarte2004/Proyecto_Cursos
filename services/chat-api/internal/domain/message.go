package domain

import "time"

type Attachment struct {
	ID          string    `json:"id"`
	MessageID   string    `json:"messageId,omitempty"`
	RoomID      string    `json:"roomId,omitempty"`
	Kind        string    `json:"kind"`
	FileName    string    `json:"fileName"`
	ContentType string    `json:"contentType"`
	SizeBytes   int64     `json:"sizeBytes"`
	CreatedAt   time.Time `json:"createdAt"`
	Data        []byte    `json:"-"`
}

type Message struct {
	ID          string       `json:"id"`
	RoomID      string       `json:"roomId"`
	SenderID    string       `json:"senderId"`
	SenderRole  string       `json:"senderRole"`
	Content     string       `json:"content"`
	Attachments []Attachment `json:"attachments,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
}
