# AuraLogic Template Package Samples

This directory hosts official host-managed template and rule-pack samples for the market ecosystem.

Current sample packages:

- `template_packages/email-order-paid` - email template package targeting the `order_paid` event
- `template_packages/landing-home` - landing page template package targeting the `home` slug
- `template_packages/invoice-default` - invoice template package for the host invoice renderer
- `template_packages/auth-branding-default` - auth branding template package for the host login/register pages
- `template_packages/page-rules-checkout` - page rule pack package with a checkout enhancement example

Packaging:

1. Run `python ./template_packages/scripts/package-samples.py`
2. Generated ZIP files will be written into each sample directory
3. Those ZIP files can be uploaded through the host, or published to `market_registry`

To seed only template samples into a local market registry:

1. `cd market_registry`
2. `node ./scripts/publish-official-samples.mjs --include templates --rebuild`
