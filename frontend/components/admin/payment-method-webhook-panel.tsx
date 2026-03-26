'use client'

import { useEffect, useMemo, useState } from 'react'
import { Copy, Link2, ShieldCheck } from 'lucide-react'
import toast from 'react-hot-toast'

import type { PaymentMethodPackageWebhookManifest } from '@/lib/api'
import { getTranslations } from '@/lib/i18n'
import { useLocale } from '@/hooks/use-locale'
import { resolveManifestLocalizedString } from '@/lib/package-manifest-schema'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

type PaymentMethodWebhookPanelProps = {
  title?: string
  description?: string
  manifest?: string | null
  webhooks?: PaymentMethodPackageWebhookManifest[] | null
  paymentMethodId?: number | string | null
}

type ResolvedPaymentMethodWebhookManifest = Omit<
  PaymentMethodPackageWebhookManifest,
  'description' | 'action' | 'method' | 'auth_mode'
> & {
  key: string
  description?: string
  action: string
  method: string
  auth_mode: string
}

function parseStoredWebhookManifest(
  manifest?: string | null
): PaymentMethodPackageWebhookManifest[] {
  const trimmed = String(manifest || '').trim()
  if (!trimmed) return []
  try {
    const parsed = JSON.parse(trimmed)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return []
    }
    const manifestObject = parsed as Record<string, unknown>
    return Array.isArray(manifestObject.webhooks)
      ? (manifestObject.webhooks as PaymentMethodPackageWebhookManifest[])
      : []
  } catch {
    return []
  }
}

function normalizeWebhookMethod(method?: string): string {
  const normalized = String(method || '')
    .trim()
    .toUpperCase()
  if (!normalized) return 'POST'
  if (normalized === 'ANY' || normalized === '*') return '*'
  if (['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].includes(normalized)) {
    return normalized
  }
  return normalized
}

function normalizeWebhookAuthMode(authMode?: string): string {
  const normalized = String(authMode || '')
    .trim()
    .toLowerCase()
  if (!normalized) return 'none'
  if (['none', 'query', 'header', 'hmac_sha256'].includes(normalized)) {
    return normalized
  }
  return normalized
}

function normalizeWebhookManifest(
  webhook: PaymentMethodPackageWebhookManifest,
  locale?: string
): ResolvedPaymentMethodWebhookManifest | null {
  const key = String(webhook?.key || '').trim()
  if (!key) return null

  const authMode = normalizeWebhookAuthMode(webhook.auth_mode)
  const header = String(webhook.header || '').trim()
  const queryParam = String(webhook.query_param || '').trim()
  const signatureHeader = String(webhook.signature_header || '').trim()

  return {
    ...webhook,
    key,
    description: resolveManifestLocalizedString(webhook.description, locale) || undefined,
    action: String(webhook.action || '').trim() || `webhook.${key}`,
    method: normalizeWebhookMethod(webhook.method),
    auth_mode: authMode,
    secret_key: String(webhook.secret_key || '').trim() || undefined,
    header: authMode === 'header' ? header || 'X-Plugin-Webhook-Token' : header || undefined,
    query_param: authMode === 'query' ? queryParam || 'token' : queryParam || undefined,
    signature_header:
      authMode === 'hmac_sha256'
        ? signatureHeader || 'X-Plugin-Webhook-Signature'
        : signatureHeader || undefined,
  }
}

function resolvePaymentMethodID(paymentMethodId?: number | string | null): number | null {
  if (typeof paymentMethodId === 'number' && Number.isFinite(paymentMethodId) && paymentMethodId > 0) {
    return paymentMethodId
  }
  if (typeof paymentMethodId === 'string') {
    const trimmed = paymentMethodId.trim()
    if (/^\d+$/.test(trimmed)) {
      const parsed = Number.parseInt(trimmed, 10)
      if (Number.isFinite(parsed) && parsed > 0) {
        return parsed
      }
    }
  }
  return null
}

function buildPaymentWebhookPath(paymentMethodId: number | null, key: string): string {
  const normalizedKey = String(key || '').trim().replace(/^\/+|\/+$/g, '')
  const targetID = paymentMethodId !== null ? String(paymentMethodId) : '{id}'
  return `/api/payment-methods/${targetID}/webhooks/${normalizedKey}`
}

function formatWebhookAuthModeLabel(authMode: string, t: ReturnType<typeof getTranslations>): string {
  switch (authMode) {
    case 'none':
      return t.admin.pmWebhookAuthNone
    case 'query':
      return t.admin.pmWebhookAuthQuery
    case 'header':
      return t.admin.pmWebhookAuthHeader
    case 'hmac_sha256':
      return t.admin.pmWebhookAuthHMACSHA256
    default:
      return authMode
  }
}

export function PaymentMethodWebhookPanel(props: PaymentMethodWebhookPanelProps) {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const concretePaymentMethodID = useMemo(
    () => resolvePaymentMethodID(props.paymentMethodId),
    [props.paymentMethodId]
  )
  const [resolvedBaseURL, setResolvedBaseURL] = useState(() =>
    String(process.env.NEXT_PUBLIC_API_URL || '')
      .trim()
      .replace(/\/+$/g, '')
  )
  useEffect(() => {
    if (resolvedBaseURL || typeof window === 'undefined' || !window.location?.origin) {
      return
    }
    setResolvedBaseURL(window.location.origin.replace(/\/+$/g, ''))
  }, [resolvedBaseURL])
  const items = useMemo(() => {
    const source =
      props.webhooks !== undefined ? props.webhooks || [] : parseStoredWebhookManifest(props.manifest)
    return source
      .map((item) => normalizeWebhookManifest(item, locale))
      .filter((item): item is ResolvedPaymentMethodWebhookManifest => item !== null)
  }, [locale, props.manifest, props.webhooks])

  const handleCopy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value)
      toast.success(t.common.copiedToClipboard)
    } catch {
      toast.error(t.admin.operationFailed)
    }
  }

  return (
    <Card className="bg-muted/30 [&_code]:break-all [&_p]:break-words">
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-sm">
          <Link2 className="h-4 w-4" />
          {props.title || t.admin.pmWebhookPanelTitle}
        </CardTitle>
        <CardDescription>
          {props.description || t.admin.pmWebhookPanelDesc}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {items.length === 0 ? (
          <div className="rounded-md border border-dashed px-3 py-4 text-sm text-muted-foreground">
            {t.admin.pmWebhookNoneDeclared}
          </div>
        ) : (
          items.map((item) => {
            const callbackPath = buildPaymentWebhookPath(concretePaymentMethodID, item.key)
            const callbackValue =
              concretePaymentMethodID !== null && resolvedBaseURL
                ? `${resolvedBaseURL}${callbackPath}`
                : callbackPath
            const canCopy = concretePaymentMethodID !== null

            return (
              <div key={item.key} className="rounded-md border bg-background/80 p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{item.key}</span>
                      <Badge variant="outline">{item.method}</Badge>
                      <Badge variant={item.auth_mode === 'none' ? 'secondary' : 'default'}>
                        <ShieldCheck className="mr-1 h-3 w-3" />
                        {formatWebhookAuthModeLabel(item.auth_mode, t)}
                      </Badge>
                    </div>
                    {item.description ? (
                      <p className="text-sm text-muted-foreground">{item.description}</p>
                    ) : null}
                  </div>
                  {canCopy ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => void handleCopy(callbackValue)}
                    >
                      <Copy className="mr-2 h-4 w-4" />
                      {t.admin.pmWebhookCopyUrl}
                    </Button>
                  ) : null}
                </div>

                <div className="mt-3 grid gap-3 text-xs sm:grid-cols-2">
                  <div className="space-y-1">
                    <p className="text-muted-foreground">{t.admin.pmWebhookCallbackUrl}</p>
                    <code>{callbackValue}</code>
                  </div>
                  <div className="space-y-1">
                    <p className="text-muted-foreground">{t.admin.pmWebhookAction}</p>
                    <code>{item.action}</code>
                  </div>
                  <div className="space-y-1">
                    <p className="text-muted-foreground">{t.admin.pmWebhookMethod}</p>
                    <code>{item.method}</code>
                  </div>
                  <div className="space-y-1">
                    <p className="text-muted-foreground">{t.admin.pmWebhookAuthMode}</p>
                    <code>{formatWebhookAuthModeLabel(item.auth_mode, t)}</code>
                  </div>
                  {item.secret_key ? (
                    <div className="space-y-1">
                      <p className="text-muted-foreground">{t.admin.pmWebhookSecretSource}</p>
                      <code>{item.secret_key}</code>
                    </div>
                  ) : null}
                  {item.query_param ? (
                    <div className="space-y-1">
                      <p className="text-muted-foreground">{t.admin.pmWebhookQueryParam}</p>
                      <code>{item.query_param}</code>
                    </div>
                  ) : null}
                  {item.header ? (
                    <div className="space-y-1">
                      <p className="text-muted-foreground">{t.admin.pmWebhookHeader}</p>
                      <code>{item.header}</code>
                    </div>
                  ) : null}
                  {item.signature_header ? (
                    <div className="space-y-1">
                      <p className="text-muted-foreground">{t.admin.pmWebhookSignatureHeader}</p>
                      <code>{item.signature_header}</code>
                    </div>
                  ) : null}
                </div>
              </div>
            )
          })
        )}
      </CardContent>
    </Card>
  )
}
