package payment

import (
	"context"
	"net/http"
)

type CreatePaymentRequest struct {
	ExternalID    string
	Amount        float64
	Currency      string
	CustomerName  string
	CustomerEmail string
}

type CreatePaymentResult struct {
	QRISString   string
	QRISImageURL string
}

// Provider is the payment gateway abstraction. Only the Mock provider ships
// today; wire a real gateway (Midtrans/Xendit/Tripay) behind the same
// interface by adding a provider and setting PAYMENT_PROVIDER accordingly.
type Provider interface {
	Name() string
	CreatePayment(ctx context.Context, req CreatePaymentRequest) (*CreatePaymentResult, error)
	// VerifyWebhookSignature authenticates a gateway callback. Return nil on ok.
	VerifyWebhookSignature(payload []byte, headers http.Header) error
}

func NewProvider(config map[string]string) Provider {
	if config["provider"] == "mock" {
		return NewMockProvider(config["webhook_secret"])
	}
	return NewMockProvider(config["webhook_secret"])
}
