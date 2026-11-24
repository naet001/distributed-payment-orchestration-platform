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

**Result policy (defined; implementation still pending):**

- [x] Define behavior for pending, success, failed, and unknown Paystack results.

| Verified Paystack result | Payment action |
| --- | --- |
| `pending` | Keep the internal status `pending`; a later verification can report a final result. |
| `success` | Set `succeeded` only after verification confirms the payment reference, amount, and currency. |
| `failed` | Set `failed` only when verification confirms the transaction failed. An initialization or network error is not a failed payment. |
| Unknown status | Do not change the payment. Return a safe verification error for investigation or retry. Never infer `succeeded` or `failed`. |
| Verification/provider error | Do not change the payment. Return a safe error; allow a later verification attempt. |

A repeated result that matches the current terminal status is safe to accept. A conflicting result after `succeeded`, `failed`, or `cancelled` must not overwrite that terminal status; report the transition conflict for investigation. A cancellation is an internal action and is not inferred from an unknown provider status.

- [x] Map verified Paystack `success` to internal `succeeded`.
- [x] Map verified Paystack `failed` to internal `failed`.
- [x] Keep pending or unknown results from being marked successful.
- [x] Test known and unknown provider status values.

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
