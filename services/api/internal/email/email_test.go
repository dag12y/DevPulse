package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResendSenderPostsMessage(t *testing.T) {
	var gotAuth, gotPayload resendPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/emails" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer re_test_key" {
			t.Errorf("authorization = %q", auth)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	sender := NewResend("re_test_key", "DevPulse <onboarding@resend.dev>").(*resendSender)
	sender.endpoint = server.URL + "/emails"
	sender.client = server.Client()

	err := sender.Send(context.Background(), Message{
		To:      "ada@example.com",
		Subject: "Verify your email",
		Text:    "Open https://example.com/verify",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotPayload.From != "DevPulse <onboarding@resend.dev>" {
		t.Errorf("from = %q, want default", gotPayload.From)
	}
	if gotPayload.To != "ada@example.com" || gotPayload.Subject != "Verify your email" {
		t.Errorf("payload = %+v", gotPayload)
	}
	if !strings.Contains(gotPayload.Text, "https://example.com/verify") {
		t.Errorf("text = %q", gotPayload.Text)
	}
	_ = gotAuth
}

func TestResendSenderHonorsMessageFrom(t *testing.T) {
	var gotFrom string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload resendPayload
		_ = json.NewDecoder(r.Body).Decode(&payload)
		gotFrom = payload.From
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewResend("key", "Default <default@example.com>").(*resendSender)
	sender.endpoint = server.URL + "/emails"
	sender.client = server.Client()

	if err := sender.Send(context.Background(), Message{To: "a@example.com", Subject: "s", Text: "t", From: "Custom <custom@example.com>"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotFrom != "Custom <custom@example.com>" {
		t.Errorf("from = %q, want message override", gotFrom)
	}
}

func TestResendSenderSurfacesProviderErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	sender := NewResend("bad-key", "DevPulse <x@example.com>").(*resendSender)
	sender.endpoint = server.URL + "/emails"
	sender.client = server.Client()

	if err := sender.Send(context.Background(), Message{To: "a@example.com", Subject: "s", Text: "t"}); err == nil {
		t.Fatal("expected an error for a non-2xx provider response")
	}
}

func TestLogSenderNeverFails(t *testing.T) {
	if err := NewLog().Send(context.Background(), Message{
		To: "dev@example.com", Subject: "link", Text: "https://example.com",
	}); err != nil {
		t.Fatalf("log sender: %v", err)
	}
}
