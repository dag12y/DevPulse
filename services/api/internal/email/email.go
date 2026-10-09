// Package email delivers outbound auth mail: verification links and
// password resets. Delivery is behind a Sender interface so tests and
// development run without a provider account.
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Message is a single outbound email. HTML is optional; Text is
// required because every link must survive plain-text clients.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
	From    string
}

// Sender delivers one message. Implementations must be safe for
// concurrent use and should bound their own network timeout.
type Sender interface {
	Send(ctx context.Context, message Message) error
}

// NewLog returns a Sender that writes messages to the process log.
// Development uses it when no provider is configured: the verification
// or reset link still reaches the developer, in `docker logs` or the
// API terminal, without an account on a third-party service.
func NewLog() Sender { return logSender{} }

type logSender struct{}

func (logSender) Send(_ context.Context, message Message) error {
	slog.Info("email (dev log sender)",
		"to", message.To,
		"subject", message.Subject,
		"body", message.Text)
	return nil
}

const (
	resendEndpoint = "https://api.resend.com/emails"
	// resendTimeout bounds provider calls so a slow provider cannot
	// stall a register/reset request thread indefinitely.
	resendTimeout = 10 * time.Second
)

// NewResend returns a Sender backed by the Resend HTTP API. apiKey is
// the Resend secret; from is the default From header (e.g.
// "DevPulse <onboarding@resend.dev>").
func NewResend(apiKey, from string) Sender {
	return &resendSender{
		apiKey:   apiKey,
		from:     from,
		endpoint: resendEndpoint,
		client:   &http.Client{Timeout: resendTimeout},
	}
}

type resendSender struct {
	apiKey   string
	from     string
	endpoint string
	client   *http.Client
}

type resendPayload struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
	HTML    string `json:"html,omitempty"`
}

func (sender *resendSender) Send(ctx context.Context, message Message) error {
	from := message.From
	if from == "" {
		from = sender.from
	}
	payload := resendPayload{
		From:    from,
		To:      message.To,
		Subject: message.Subject,
		Text:    message.Text,
		HTML:    message.HTML,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode resend payload: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, sender.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build resend request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+sender.apiKey)

	response, err := sender.client.Do(request)
	if err != nil {
		return fmt.Errorf("send email via resend: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("resend returned status %d", response.StatusCode)
	}
	return nil
}
