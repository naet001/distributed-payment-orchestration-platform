package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PaystackConfig holds configuration for the Paystack client.
const maxPaystackResponseBytes = 1 << 20

type PaystackConfig struct {
	SecretKey string
	Timeout   time.Duration
	BaseURL   string // Default: "https://api.paystack.co"
}

// PaystackClient implements TransactionInitializer for Paystack.
type PaystackClient struct {
	config PaystackConfig
	client *http.Client
}

// NewPaystackClient creates a new Paystack client.
func NewPaystackClient(config PaystackConfig) *PaystackClient {
	return &PaystackClient{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// paystackRequest represents the request body for Paystack transaction initialization.
type paystackRequest struct {
	Reference   string `json:"reference"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Email       string `json:"email"`
	CallbackURL string `json:"callback_url"`
}

// paystackResponse represents the response from Paystack transaction initialization.
type paystackResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message,omitempty"`
	Data    struct {
		AuthorizationURL string `json:"authorization_url"`
		AccessCode       string `json:"access_code"`
		Reference        string `json:"reference"`
	} `json:"data"`
}

func (c *PaystackClient) Provider() string {
	return "paystack"
}

// InitializeTransaction creates a transaction with Paystack and returns the provider's
// reference and authorization URL.
func (c *PaystackClient) InitializeTransaction(
	ctx context.Context,
	reference string,
	amount int64,
	currency string,
	email string,
	callbackURL string,
) (providerRef string, authURL *url.URL, err error) {
	// Build request body
	reqBody := paystackRequest{
		Reference:   reference,
		Amount:      amount,
		Currency:    currency,
		Email:       email,
		CallbackURL: callbackURL,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("marshal request: %w", err)
	}

	// Build request URL
	baseURL := c.config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.paystack.co"
	}
	reqURL := fmt.Sprintf("%s/transaction/initialize", baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", nil, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.SecretKey)

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read at most one byte beyond the limit to detect oversized responses.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPaystackResponseBytes+1))
	if err != nil {
		return "", nil, fmt.Errorf("read response body: %w", err)
	}
	if len(body) > maxPaystackResponseBytes {
		return "", nil, fmt.Errorf("paystack response body exceeds limit")
	}

	// Handle non-success responses
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// Do not include raw provider responses or credentials in errors.
		return "", nil, fmt.Errorf("paystack API error: status=%d, message=%s", resp.StatusCode, sanitizeErrorMessage(body, c.config.SecretKey))
	}

	// Parse response
	var paystackResp paystackResponse
	if err := json.Unmarshal(body, &paystackResp); err != nil {
		return "", nil, fmt.Errorf("parse response: %w", err)
	}

	// Validate response structure
	if !paystackResp.Status {
		return "", nil, fmt.Errorf("paystack returned non-success status: %s", sanitizeProviderMessage(paystackResp.Message, c.config.SecretKey))
	}

	if paystackResp.Data.AuthorizationURL == "" || paystackResp.Data.AccessCode == "" || paystackResp.Data.Reference != reference {
		return "", nil, fmt.Errorf("paystack response is missing or mismatches transaction data")
	}

	// Accept only absolute HTTPS authorization URLs.
	parsedURL, err := url.Parse(paystackResp.Data.AuthorizationURL)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return "", nil, fmt.Errorf("paystack response contains an invalid authorization URL")
	}

	// Use access_code as provider reference (Paystack's transaction identifier).
	return paystackResp.Data.AccessCode, parsedURL, nil
}

// sanitizeErrorMessage removes sensitive information from error messages.
// It ensures secret keys and authorization headers are never exposed.
func sanitizeProviderMessage(message, secretKey string) string {
	if secretKey == "" {
		return message
	}
	return strings.ReplaceAll(message, secretKey, "[REDACTED]")
}

func sanitizeErrorMessage(body []byte, secretKey string) string {
	// Extract only a provider message. Never return a raw response body.
	var resp paystackResponse
	if err := json.Unmarshal(body, &resp); err != nil || resp.Message == "" {
		return "unexpected response format"
	}

	return sanitizeProviderMessage(resp.Message, secretKey)
}
