# AuraLogic Payment Package Samples

This directory hosts official sample packages for the `payment_js` runtime.

Current entry points:

- `payment_packages/payment-js-template` - manual-review style starter package with config schema, secure header-auth webhook, polling callback, refund callback, smoke tests, and ZIP packaging.
- `payment_packages/payment-js-hosted-template` - hosted/redirect checkout sample with remote session creation, stored checkout context, status polling, HMAC webhook declaration, smoke tests, and ZIP packaging.
- `payment_packages/shared/validate-payment-package-manifest.mjs` - reusable local validator that mirrors the host-side payment package manifest rules closely enough for authoring and CI.
- `payment_packages/package.json` - top-level runner for validating, testing, or packaging every official payment sample in one command.

Recommended workflow:

1. Read `docs/PAYMENT_JS_API.md` for the host runtime contract.
2. Choose a sample that matches the integration style you need:
   - `payment-js-template` for webhook-first/manual confirmation flows
   - `payment-js-hosted-template` for hosted checkout / redirect style providers
3. Run `npm install` and `npm run package` inside that sample directory.
4. Or run `npm run test` / `npm run package` in `payment_packages/` to sweep all official samples.
5. Upload the generated ZIP through `/admin/payment-methods`.

If you want these official `payment_js` samples to appear in a local market registry, run:

1. `cd market_registry`
2. `node ./scripts/publish-official-samples.mjs --include payments --rebuild`
