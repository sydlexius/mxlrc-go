package auth

import (
	"context"
	"testing"
)

func TestNewWebhookServiceValidatesConfiguredKey(t *testing.T) {
	svc, err := NewWebhookService([]string{" mxlrc_configured "})
	if err != nil {
		t.Fatalf("NewWebhookService: %v", err)
	}
	if _, err := svc.ValidateKey(context.Background(), "mxlrc_configured", ScopeWebhook); err != nil {
		t.Fatalf("ValidateKey configured key: %v", err)
	}
}

func TestNewWebhookServiceRejectsMalformedKey(t *testing.T) {
	if _, err := NewWebhookService([]string{"secret"}); err == nil {
		t.Fatal("NewWebhookService malformed key returned nil error")
	}
}
