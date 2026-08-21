package payment

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

const mockSignatureHeader = "X-Webhook-Signature"

// MockProvider simulates a dynamic QRIS gateway for development. Payments do
// not actually settle; use the simulate endpoint (or a signed webhook call)
// to mark an order paid.
type MockProvider struct {
	webhookSecret string
}

func NewMockProvider(webhookSecret string) *MockProvider {
	return &MockProvider{webhookSecret: webhookSecret}
}

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) CreatePayment(_ context.Context, req CreatePaymentRequest) (*CreatePaymentResult, error) {
	qris := fmt.Sprintf("MOCKQRIS|%s|%.2f|IDR|%s", req.ExternalID, req.Amount, uuid.NewString())
	return &CreatePaymentResult{
		QRISString:   qris,
		QRISImageURL: "",
	}, nil
}

func (m *MockProvider) VerifyWebhookSignature(payload []byte, headers http.Header) error {
	secret := headers.Get(mockSignatureHeader)
	if m.webhookSecret == "" || secret == "" {
		return errors.New("missing webhook signature")
	}
	if subtle.ConstantTimeCompare([]byte(secret), []byte(m.webhookSecret)) != 1 {
		return errors.New("invalid webhook signature")
	}
	return nil
}
