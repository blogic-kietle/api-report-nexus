// Package licenseapi talks to the BLogic License API, which owns mail delivery.
package licenseapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"api-report-nexus/internal/domain/email"
)

type Sender struct {
	BaseURL string
	Client  *http.Client
}

// New returns a sender with its own timeout so a stalled License API cannot hold requests open.
func New(baseURL string, timeout time.Duration) *Sender {
	return &Sender{BaseURL: strings.TrimSuffix(baseURL, "/"), Client: &http.Client{Timeout: timeout}}
}

func (s *Sender) Send(ctx context.Context, msg email.Message) error {
	route, payload := "/utils/send-email-receipt", any(msg)
	if len(msg.Attachments) > 0 {
		route, payload = "/utils/common-email", commonEmail(msg)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL+route, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.Client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		// Keep the upstream detail: the Node service logged it too.
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("license api: %s: %s", resp.Status, bytes.TrimSpace(detail))
	}
	return nil
}

// commonEmail is the body of /utils/common-email, the endpoint BLogic View uses for several attachments.
func commonEmail(msg email.Message) any {
	type file struct {
		Name       string `json:"name"`
		Content    string `json:"content"`
		EncodeType string `json:"encodeType"`
	}
	files := make([]file, len(msg.Attachments))
	for i, a := range msg.Attachments {
		files[i] = file{a.Name, base64.StdEncoding.EncodeToString(a.Content), "BASE_64"}
	}
	return struct {
		ToEmails    []string `json:"toEmails"`
		Subject     string   `json:"subject"`
		Body        string   `json:"body"`
		Attachments []file   `json:"attachments"`
	}{msg.ToEmails, msg.Subject, msg.Body, files}
}
