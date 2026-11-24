# Stage 3 — Payment API and Paystack

**Status:** In progress  
**Goal:** Build and verify the payment flow from payment creation through Paystack verification.

## 3.1 Define payment lifecycle and statuses — Done

- [x] Record statuses: `pending`, `succeeded`, `failed`, and `cancelled`.
- [x] Allow `pending` to remain pending or move to `succeeded`, `failed`, or `cancelled`.
- [x] Treat `succeeded`, `failed`, and `cancelled` as terminal for one payment attempt; retries require a new attempt.
- [x] Enforce transitions through the payment service and a status-guarded repository update.
- [x] Test valid, repeated, invalid, and stale transitions.

**Complete when:** Lifecycle rules are documented and enforced, with behavior tests for allowed and rejected transitions.

## 3.2 Implement payment creation endpoint — Done

- [x] Validate amount, currency, and provider.
- [x] Generate a payment reference and save a pending payment.
- [x] Return the created payment.
- [x] Test valid and invalid requests and persistence behavior.

## 3.3 Implement Paystack transaction initialization — Not started

- [ ] Define a provider interface for transaction initialization.
- [ ] Add Paystack configuration, including a secret key and request timeout.
- [ ] Send the payment reference, amount, currency, and callback URL to Paystack.
- [ ] Handle provider errors and malformed responses without leaking secrets.
- [ ] Test the provider client with a controlled HTTP server.

## 3.4 Generate and return hosted checkout URL — Not started

- [ ] Store the provider transaction reference and authorization URL.
- [ ] Return the authorization URL only after successful initialization.
- [ ] Test the API response and persisted provider fields.

## 3.5 Implement payment verification — Not started

- [ ] Add a route to verify a payment by its reference.
- [ ] Load the payment and query Paystack for its transaction status.
- [ ] Persist the verified result through the service and repository.
- [ ] Define and test behavior for unknown references and provider failures.

## 3.6 Map Paystack responses to internal payment statuses — Not started

- [ ] Map validated Paystack success to internal `succeeded`.
- [ ] Map provider failure to the agreed internal status.
- [ ] Keep pending or unknown results from being marked successful.
- [ ] Test known and unknown provider status values.

## 3.7 Handle provider errors and timeouts — Not started

- [ ] Apply bounded request timeouts to Paystack calls.
- [ ] Distinguish timeout, network, authentication, malformed-response, and provider errors.
- [ ] Return safe API errors without exposing credentials or raw provider responses.
- [ ] Test timeout and representative provider failures.

## 3.8 Write unit and integration tests — In progress

- [x] Test payment creation and input validation.
- [x] Test payment persistence through the existing optional PostgreSQL integration test.
- [x] Test lifecycle rules and guarded status updates.
- [ ] Test Paystack initialization, checkout URL, verification, and status mapping.
- [ ] Test timeout and provider error behavior.
- [ ] Run PostgreSQL integration tests in a configured test environment.

**Complete when:** Unit and integration tests cover the full create → checkout → verify flow, including failure paths.

## Suggested implementation order

1. Define lifecycle transitions and failure behavior (3.1).
2. Add the Paystack client and configuration (3.3, 3.7).
3. Initialize transactions and return/store the checkout URL (3.4).
4. Add verification and status mapping (3.5, 3.6).
5. Complete end-to-end and PostgreSQL test coverage (3.8).

## Current scope notes

- The API currently creates a local pending payment; it does not call Paystack.
- Checkout URL generation and payment verification are not implemented.
- Confirm callback URL, supported currencies, and desired provider-failure behavior before finalizing Paystack integration.
