package payment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaystackClient_InitializeTransaction_Success(t *testing.T) {
	// Create a test server that returns a successful response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/transaction/initialize", r.URL.Path)

		// Verify headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-secret-key", r.Header.Get("Authorization"))

		// Verify request body
		var reqBody paystackRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		assert.Equal(t, "PAY_TEST_REF", reqBody.Reference)
		assert.Equal(t, int64(100000), reqBody.Amount) // 1000.00 in minor units
		assert.Equal(t, "NGN", reqBody.Currency)
		assert.Equal(t, "customer@example.com", reqBody.Email)
		assert.Equal(t, "https://example.com/callback", reqBody.CallbackURL)

		// Return successful response
		response := paystackResponse{
			Status: true,
			Data: struct {
				AuthorizationURL string `json:"authorization_url"`
				AccessCode       string `json:"access_code"`
				Reference        string `json:"reference"`
			}{
				AuthorizationURL: "https://paystack.com/pay/abc123",
				AccessCode:       "ACC_123456",
				Reference:        "PAY_TEST_REF",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with test configuration
	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
		BaseURL:   server.URL,
	})

	// Call InitializeTransaction
	ctx := context.Background()
	providerRef, authURL, err := client.InitializeTransaction(
		ctx,
		"PAY_TEST_REF",
		100000,
		"NGN",
		"customer@example.com",
		"https://example.com/callback",
	)

	// Verify results
	require.NoError(t, err)
	assert.Equal(t, "ACC_123456", providerRef)
	require.NotNil(t, authURL)
	assert.Equal(t, "https://paystack.com/pay/abc123", authURL.String())
}

func TestPaystackClient_InitializeTransaction_PaystackError(t *testing.T) {
	// Create a test server that returns an error response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := paystackResponse{
			Status:  false,
			Message: "Invalid API key",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
		BaseURL:   server.URL,
	})

	ctx := context.Background()
	_, _, err := client.InitializeTransaction(
		ctx,
		"PAY_TEST_REF",
		100000,
		"NGN",
		"customer@example.com",
		"https://example.com/callback",
	)

	// Verify error is returned
	require.Error(t, err)
	assert.Contains(t, err.Error(), "paystack API error")
	assert.Contains(t, err.Error(), "status=401")
	// Verify secret key is NOT in the error message
	assert.NotContains(t, err.Error(), "test-secret-key")
}

func TestPaystackClient_InitializeTransaction_MalformedJSON(t *testing.T) {
	// Create a test server that returns malformed JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
		BaseURL:   server.URL,
	})

	ctx := context.Background()
	_, _, err := client.InitializeTransaction(
		ctx,
		"PAY_TEST_REF",
		100000,
		"NGN",
		"customer@example.com",
		"https://example.com/callback",
	)

	// Verify error is returned
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse response")
	// Verify secret key is NOT in the error message
	assert.NotContains(t, err.Error(), "test-secret-key")
}

func TestPaystackClient_InitializeTransaction_MissingAuthorizationURL(t *testing.T) {
	// Create a test server that returns a response without authorization URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := paystackResponse{
			Status: true,
			Data: struct {
				AuthorizationURL string `json:"authorization_url"`
				AccessCode       string `json:"access_code"`
				Reference        string `json:"reference"`
			}{
				AuthorizationURL: "", // Missing
				AccessCode:       "ACC_123456",
				Reference:        "PAY_TEST_REF",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
		BaseURL:   server.URL,
	})

	ctx := context.Background()
	_, _, err := client.InitializeTransaction(
		ctx,
		"PAY_TEST_REF",
		100000,
		"NGN",
		"customer@example.com",
		"https://example.com/callback",
	)

	// Verify error is returned
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing or mismatches transaction data")
}

func TestPaystackClient_InitializeTransaction_NonSuccessStatus(t *testing.T) {
	// Create a test server that returns a non-success status in the response body
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := paystackResponse{
			Status:  false,
			Message: "Invalid key test-secret-key; insufficient funds",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
		BaseURL:   server.URL,
	})

	ctx := context.Background()
	_, _, err := client.InitializeTransaction(
		ctx,
		"PAY_TEST_REF",
		100000,
		"NGN",
		"customer@example.com",
		"https://example.com/callback",
	)

	// Verify error is returned
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-success status")
	assert.Contains(t, err.Error(), "Invalid key [REDACTED]; insufficient funds")
	assert.NotContains(t, err.Error(), "test-secret-key")
}

func TestPaystackClient_InitializeTransaction_RejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("x", maxPaystackResponseBytes+1)))
	}))
	defer server.Close()

	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
		BaseURL:   server.URL,
	})

	_, _, err := client.InitializeTransaction(context.Background(), "PAY_TEST_REF", 100000, "NGN", "customer@example.com", "https://example.com/callback")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "response body exceeds limit")
	assert.NotContains(t, err.Error(), strings.Repeat("x", 64))
}

func TestPaystackClient_InitializeTransaction_ServerError(t *testing.T) {
	// Create a test server that returns a 500 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
		BaseURL:   server.URL,
	})

	ctx := context.Background()
	_, _, err := client.InitializeTransaction(
		ctx,
		"PAY_TEST_REF",
		100000,
		"NGN",
		"customer@example.com",
		"https://example.com/callback",
	)

	// Verify error is returned
	require.Error(t, err)
	assert.Contains(t, err.Error(), "paystack API error")
	assert.Contains(t, err.Error(), "status=500")
	// Verify secret key is NOT in the error message
	assert.NotContains(t, err.Error(), "test-secret-key")
}

func TestPaystackClient_InitializeTransaction_ContextTimeout(t *testing.T) {
	// Create a test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   1 * time.Millisecond, // Very short timeout
		BaseURL:   server.URL,
	})

	ctx := context.Background()
	_, _, err := client.InitializeTransaction(
		ctx,
		"PAY_TEST_REF",
		100000,
		"NGN",
		"customer@example.com",
		"https://example.com/callback",
	)

	// Verify timeout error is returned
	require.Error(t, err)
	assert.Contains(t, err.Error(), "execute request")
}

func TestSanitizeErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		expected string
	}{
		{
			name:     "JSON with message",
			body:     []byte(`{"status": false, "message": "Invalid request"}`),
			expected: "Invalid request",
		},
		{
			name:     "JSON without message",
			body:     []byte(`{"status": false}`),
			expected: "unexpected response format",
		},
		{
			name:     "Not JSON",
			body:     []byte("not json"),
			expected: "unexpected response format",
		},
		{
			name:     "Empty body",
			body:     []byte(""),
			expected: "unexpected response format",
		},
		{
			name:     "JSON with secret key in body",
			body:     []byte(`{"secret": "sk_test_123", "message": "Error"}`),
			expected: "Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeErrorMessage(tt.body, "sk_test_123")
			assert.Equal(t, tt.expected, result)
			// Ensure no secret-like patterns are in the result
			assert.NotContains(t, result, "sk_test")
			assert.NotContains(t, result, "sk_live")
		})
	}
}

func TestPaystackClient_DefaultBaseURL(t *testing.T) {
	// Test that default base URL is used when BaseURL is empty
	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
		BaseURL:   "", // Empty - should use default
	})

	// We can't easily test the actual URL without making a request,
	// but we can verify the client was created successfully
	assert.NotNil(t, client)
	assert.NotNil(t, client.client)
}

func TestPaystackClient_ImplementsInterface(t *testing.T) {
	// Verify that PaystackClient implements TransactionInitializer
	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
	})

	// This should compile if the interface is properly implemented
	var _ TransactionInitializer = client
}

func TestPaystackClient_VerifyTransaction_Success(t *testing.T) {
	// Create a test server that returns a successful verification response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/transaction/verify/PAY_TEST_REF", r.URL.Path)

		// Verify headers
		assert.Equal(t, "Bearer test-secret-key", r.Header.Get("Authorization"))

		// Return successful verification response
		response := paystackVerifyResponse{
			Status: true,
			Data: struct {
				ID             int    `json:"id"`
				Domain         string `json:"domain"`
				Status         string `json:"status"`
				Reference      string `json:"reference"`
				Amount         int64  `json:"amount"`
				Currency       string `json:"currency"`
				GatewayResponse string `json:"gateway_response"`
				PaidAt         string `json:"paid_at"`
				CreatedAt      string `json:"created_at"`
				Channel        string `json:"channel"`
			}{
				ID:              12345,
				Domain:          "test",
				Status:          "success",
				Reference:       "PAY_TEST_REF",
				Amount:          100000,
				Currency:        "NGN",
				GatewayResponse: "Successful",
				PaidAt:          "2024-01-15T10:30:00.000Z",
				CreatedAt:       "2024-01-15T10:25:00.000Z",
				Channel:         "card",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with test configuration
	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
		BaseURL:   server.URL,
	})

	// Call VerifyTransaction
	ctx := context.Background()
	status, ref, amount, currency, err := client.VerifyTransaction(ctx, "PAY_TEST_REF")

	// Verify results
	require.NoError(t, err)
	assert.Equal(t, "success", status)
	assert.Equal(t, "PAY_TEST_REF", ref)
	assert.Equal(t, int64(100000), amount)
	assert.Equal(t, "NGN", currency)
}

func TestPaystackClient_VerifyTransaction_Pending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := paystackVerifyResponse{
			Status: true,
			Data: struct {
				ID             int    `json:"id"`
				Domain         string `json:"domain"`
				Status         string `json:"status"`
				Reference      string `json:"reference"`
				Amount         int64  `json:"amount"`
				Currency       string `json:"currency"`
				GatewayResponse string `json:"gateway_response"`
				PaidAt         string `json:"paid_at"`
				CreatedAt      string `json:"created_at"`
				Channel        string `json:"channel"`
			}{
				Status:    "pending",
				Reference: "PAY_TEST_REF",
				Amount:    100000,
				Currency:  "NGN",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
		BaseURL:   server.URL,
	})

	ctx := context.Background()
	status, ref, amount, currency, err := client.VerifyTransaction(ctx, "PAY_TEST_REF")

	require.NoError(t, err)
	assert.Equal(t, "pending", status)
	assert.Equal(t, "PAY_TEST_REF", ref)
	assert.Equal(t, int64(100000), amount)
	assert.Equal(t, "NGN", currency)
}

func TestPaystackClient_VerifyTransaction_Failed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := paystackVerifyResponse{
			Status: true,
			Data: struct {
				ID             int    `json:"id"`
				Domain         string `json:"domain"`
				Status         string `json:"status"`
				Reference      string `json:"reference"`
				Amount         int64  `json:"amount"`
				Currency       string `json:"currency"`
				GatewayResponse string `json:"gateway_response"`
				PaidAt         string `json:"paid_at"`
				CreatedAt      string `json:"created_at"`
				Channel        string `json:"channel"`
			}{
				Status:         "failed",
				Reference:      "PAY_TEST_REF",
				Amount:         100000,
				Currency:       "NGN",
				GatewayResponse: "Insufficient funds",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
		BaseURL:   server.URL,
	})

	ctx := context.Background()
	status, ref, amount, currency, err := client.VerifyTransaction(ctx, "PAY_TEST_REF")

	require.NoError(t, err)
	assert.Equal(t, "failed", status)
	assert.Equal(t, "PAY_TEST_REF", ref)
	assert.Equal(t, int64(100000), amount)
	assert.Equal(t, "NGN", currency)
}

func TestPaystackClient_VerifyTransaction_PaystackError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := paystackVerifyResponse{
			Status:  false,
			Message: "Invalid API key",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
		BaseURL:   server.URL,
	})

	ctx := context.Background()
	_, _, _, _, err := client.VerifyTransaction(ctx, "PAY_TEST_REF")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "paystack API error")
	assert.Contains(t, err.Error(), "status=401")
	assert.NotContains(t, err.Error(), "test-secret-key")
}

func TestPaystackClient_VerifyTransaction_NonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := paystackVerifyResponse{
			Status:  false,
			Message: "Invalid key test-secret-key; transaction not found",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
		BaseURL:   server.URL,
	})

	ctx := context.Background()
	_, _, _, _, err := client.VerifyTransaction(ctx, "PAY_TEST_REF")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-success status")
	assert.Contains(t, err.Error(), "Invalid key [REDACTED]; transaction not found")
	assert.NotContains(t, err.Error(), "test-secret-key")
}

func TestPaystackClient_VerifyTransaction_MissingData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := paystackVerifyResponse{
			Status: true,
			Data: struct {
				ID             int    `json:"id"`
				Domain         string `json:"domain"`
				Status         string `json:"status"`
				Reference      string `json:"reference"`
				Amount         int64  `json:"amount"`
				Currency       string `json:"currency"`
				GatewayResponse string `json:"gateway_response"`
				PaidAt         string `json:"paid_at"`
				CreatedAt      string `json:"created_at"`
				Channel        string `json:"channel"`
			}{
				Status:   "", // Missing
				Reference: "PAY_TEST_REF",
				Amount:   100000,
				Currency: "NGN",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
		BaseURL:   server.URL,
	})

	ctx := context.Background()
	_, _, _, _, err := client.VerifyTransaction(ctx, "PAY_TEST_REF")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing transaction data")
}

func TestPaystackClient_VerifyTransaction_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
		BaseURL:   server.URL,
	})

	ctx := context.Background()
	_, _, _, _, err := client.VerifyTransaction(ctx, "PAY_TEST_REF")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse response")
	assert.NotContains(t, err.Error(), "test-secret-key")
}

func TestPaystackClient_VerifyTransaction_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
		BaseURL:   server.URL,
	})

	ctx := context.Background()
	_, _, _, _, err := client.VerifyTransaction(ctx, "PAY_TEST_REF")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "paystack API error")
	assert.Contains(t, err.Error(), "status=500")
	assert.NotContains(t, err.Error(), "test-secret-key")
}

func TestPaystackClient_VerifyTransaction_ContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   1 * time.Millisecond,
		BaseURL:   server.URL,
	})

	ctx := context.Background()
	_, _, _, _, err := client.VerifyTransaction(ctx, "PAY_TEST_REF")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "execute request")
}

func TestPaystackClient_ImplementsTransactionVerifier(t *testing.T) {
	client := NewPaystackClient(PaystackConfig{
		SecretKey: "test-secret-key",
		Timeout:   5 * time.Second,
	})

	var _ TransactionVerifier = client
}
