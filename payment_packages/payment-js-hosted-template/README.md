# Payment JS Hosted Template

AuraLogic `payment_js` hosted checkout / redirect starter sample.

This sample complements `payment_packages/payment-js-template`:

- creates and caches a remote checkout session
- stores checkout metadata back into the host order
- polls the provider session status over `AuraLogic.http`
- declares an HMAC-protected webhook
- includes local manifest validation, smoke tests, and ZIP packaging

## Files

- `manifest.json` - hosted checkout payment package manifest
- `src/index.js` - runtime callbacks for hosted session creation, status polling, webhook handling, and refund
- `scripts/build.mjs` - copies the runtime entry into `dist/`
- `scripts/test.mjs` - smoke-tests the built package in a Node VM with mocked AuraLogic APIs
- `scripts/package.mjs` - creates `payment-js-hosted-template.zip`

## Commands

1. `make deps`
2. `npm run validate:manifest`
3. `npm run test`
4. `npm run package`

## Upload flow

1. Run `npm run package`
2. Upload `payment-js-hosted-template.zip` through `/admin/payment-methods`
3. Open the created payment method and review the package-governed config defaults
4. Point your upstream provider callback to `/api/payment-methods/{id}/webhooks/payment.notify`

## What this sample demonstrates

- `onGeneratePaymentCard(order, config)`
  - creates or reuses a checkout session against the remote provider API
  - writes hosted checkout metadata through `AuraLogic.order.updatePaymentData(...)`
  - renders a simple redirect / hosted checkout CTA for the user
- `onCheckPaymentStatus(order, config)`
  - reuses the stored checkout session and polls the provider for current status
  - persists a paid record once the provider returns a success state
- `onWebhook(hook, config)`
  - consumes provider callback payloads after host-side HMAC verification
  - finalizes the order directly or queues later polling when still pending
- `onRefund(order, config)`
  - shows how to call a refund endpoint for a previously created hosted session

## Notes

- The host currently expects `cache_ttl` for payment card caching.
- The HMAC webhook secret comes from `config.webhook_secret`; the host verifies the signature before `onWebhook(...)` runs.
- For real providers, swap the demo `/session`, `/session-status`, and `/refund` endpoints for your upstream API contract.
- For runtime API details, see `docs/PAYMENT_JS_API.md`.
