package email

import "context"

type Attachment struct {
	Name    string
	Content []byte
}

type Message struct {
	ToEmails    []string `json:"toEmails"`
	Subject     string   `json:"subject"`
	Body        string   `json:"body"`
	FileName    string   `json:"fileName,omitempty"`
	FileContent string   `json:"fileContent,omitempty"`
	// Attachments selects the License API endpoint that takes several files; FileName and FileContent are then ignored.
	Attachments []Attachment `json:"-"`
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
}
