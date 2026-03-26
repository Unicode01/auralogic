'use client'

import Link from 'next/link'
import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import {
  type AdminPluginMarketSource,
  type PaymentMethodMarketArtifact,
  type PaymentMethodMarketCatalogItem,
  getPaymentMethodMarketArtifact,
  getPaymentMethodMarketCatalog,
  getPaymentMethodMarketSources,
} from '@/lib/api'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { getTranslations } from '@/lib/i18n'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { resolveManifestLocalizedString } from '@/lib/package-manifest-schema'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ArrowLeft, ExternalLink, Loader2, Package, RefreshCw, Search } from 'lucide-react'

type PaymentMarketFilters = {
  source_id: string
  channel: string
  q: string
}

function resolvePaymentMarketTitle(
  item: Pick<PaymentMethodMarketCatalogItem, 'title' | 'name'> | Pick<PaymentMethodMarketArtifact, 'title' | 'name'>,
  locale: string
): string {
  return resolveManifestLocalizedString(item.title, locale) || String(item.name || '').trim()
}

function resolvePaymentMarketDescription(
  item:
    | Pick<PaymentMethodMarketCatalogItem, 'description' | 'summary'>
    | Pick<PaymentMethodMarketArtifact, 'description' | 'summary'>,
  locale: string,
  fallback: string
): string {
  return (
    resolveManifestLocalizedString(item.description, locale) ||
    resolveManifestLocalizedString(item.summary, locale) ||
    fallback
  )
}

function buildDefaultFilters(
  searchParams: ReturnType<typeof useSearchParams>
): PaymentMarketFilters {
  return {
    source_id: String(searchParams?.get('source_id') || '').trim(),
    channel: String(searchParams?.get('channel') || '').trim(),
    q: String(searchParams?.get('q') || searchParams?.get('search') || '').trim(),
  }
}

function buildPaymentMarketImportHref(
  source: AdminPluginMarketSource,
  name: string,
  version: string
): string {
  const params = new URLSearchParams()
  params.set('market_import', '1')
  params.set('market_kind', 'payment_package')
  params.set('market_name', name)
  params.set('market_version', version)
  params.set('market_source_id', source.source_id)
  params.set('market_source_base_url', source.base_url)
  if (source.name) {
    params.set('market_source_name', source.name)
  }
  if (source.public_key) {
    params.set('market_source_public_key', source.public_key)
  }
  if (source.default_channel) {
    params.set('market_source_channel', source.default_channel)
  }
  if (source.allowed_kinds?.length) {
    params.set('market_source_allowed_kinds', source.allowed_kinds.join(','))
  }
  return `/admin/payment-methods?${params.toString()}`
}

export default function PaymentMethodsMarketPage() {
  const searchParams = useSearchParams()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(`${t.admin.pmImportFromMarket} - ${t.pageTitle.adminPaymentMethods}`)

  const initialFilters = useMemo(() => buildDefaultFilters(searchParams), [searchParams])
  const requestedArtifactName = String(searchParams?.get('name') || '').trim()
  const requestedVersion = String(searchParams?.get('version') || '').trim()
  const [filters, setFilters] = useState<PaymentMarketFilters>(initialFilters)
  const [appliedFilters, setAppliedFilters] = useState<PaymentMarketFilters>(initialFilters)
  const [selectedArtifactName, setSelectedArtifactName] = useState(requestedArtifactName)

  const sourcesQuery = useQuery({
    queryKey: ['paymentMethodMarketSources'],
    queryFn: () => getPaymentMethodMarketSources(),
    staleTime: 60_000,
  })

  const sources = useMemo(
    () => ((sourcesQuery.data?.data?.items || []) as AdminPluginMarketSource[]),
    [sourcesQuery.data?.data?.items]
  )
  const selectedSource = useMemo(
    () => sources.find((item) => item.source_id === appliedFilters.source_id) || null,
    [appliedFilters.source_id, sources]
  )

  useEffect(() => {
    if (sources.length === 0) {
      return
    }
    const fallbackSource =
      sources.find((item) => item.source_id === initialFilters.source_id) || sources[0]
    const nextChannel = initialFilters.channel || fallbackSource.default_channel || ''
    setFilters((prev) =>
      prev.source_id
        ? prev
        : {
            source_id: fallbackSource.source_id,
            channel: nextChannel,
            q: prev.q,
          }
    )
    setAppliedFilters((prev) =>
      prev.source_id
        ? prev
        : {
            source_id: fallbackSource.source_id,
            channel: nextChannel,
            q: prev.q,
          }
    )
  }, [initialFilters.channel, initialFilters.source_id, sources])

  const catalogQuery = useQuery({
    queryKey: ['paymentMethodMarketCatalog', appliedFilters],
    queryFn: () =>
      getPaymentMethodMarketCatalog({
        source_id: appliedFilters.source_id,
        channel: appliedFilters.channel || undefined,
        q: appliedFilters.q || undefined,
        limit: 24,
      }),
    enabled: !!appliedFilters.source_id,
    staleTime: 30_000,
  })

  const catalogItems = useMemo(
    () => ((catalogQuery.data?.data?.items || []) as PaymentMethodMarketCatalogItem[]),
    [catalogQuery.data?.data?.items]
  )

  useEffect(() => {
    if (catalogItems.length === 0) {
      setSelectedArtifactName('')
      return
    }
    const requestedMatch = requestedArtifactName
      ? catalogItems.find((item) => item.name === requestedArtifactName)
      : null
    const selectedMatch = selectedArtifactName
      ? catalogItems.find((item) => item.name === selectedArtifactName)
      : null
    if (selectedMatch) {
      return
    }
    setSelectedArtifactName(requestedMatch?.name || catalogItems[0].name)
  }, [catalogItems, requestedArtifactName, selectedArtifactName])

  const artifactQuery = useQuery({
    queryKey: ['paymentMethodMarketArtifact', appliedFilters.source_id, selectedArtifactName],
    queryFn: () =>
      getPaymentMethodMarketArtifact(selectedArtifactName, {
        source_id: appliedFilters.source_id,
      }),
    enabled: !!appliedFilters.source_id && !!selectedArtifactName,
    staleTime: 30_000,
  })

  const artifact = (artifactQuery.data?.data || null) as PaymentMethodMarketArtifact | null
  const artifactVersions = artifact?.versions || []
  const preferredVersion =
    requestedArtifactName && selectedArtifactName === requestedArtifactName ? requestedVersion : ''
  const artifactLatestVersion =
    preferredVersion ||
    String(artifact?.latest_version || '').trim() ||
    String(artifactVersions[0]?.version || '').trim()

  const sourceError = sourcesQuery.error
    ? resolveApiErrorMessage(sourcesQuery.error, t, t.admin.operationFailed)
    : ''
  const catalogError = catalogQuery.error
    ? resolveApiErrorMessage(catalogQuery.error, t, t.admin.operationFailed)
    : ''
  const artifactError = artifactQuery.error
    ? resolveApiErrorMessage(artifactQuery.error, t, t.admin.operationFailed)
    : ''
  const adminPaymentMethodsMarketPluginContext = {
    view: 'admin_payment_methods_market',
    filters: {
      source_id: appliedFilters.source_id || undefined,
      channel: appliedFilters.channel || undefined,
      q: appliedFilters.q || undefined,
    },
    selected_artifact: artifact
      ? {
          name: artifact.name,
          latest_version: artifact.latest_version,
          version_count: artifactVersions.length,
        }
      : selectedArtifactName
        ? {
            name: selectedArtifactName,
            latest_version: artifactLatestVersion || undefined,
            version_count: artifactVersions.length,
          }
        : undefined,
    summary: {
      source_count: sources.length,
      catalog_count: catalogItems.length,
      artifact_loaded: Boolean(artifact),
    },
  }

  const handleApplyFilters = () => {
    setAppliedFilters({
      source_id: filters.source_id,
      channel: filters.channel,
      q: filters.q.trim(),
    })
  }

  return (
    <div className="space-y-6">
      <PluginSlot
        slot="admin.payment_methods.market.top"
        context={adminPaymentMethodsMarketPluginContext}
      />
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 className="text-3xl font-bold">{t.admin.pmImportFromMarket}</h1>
          <p className="mt-1 text-muted-foreground">{t.admin.pmMarketBrowserDesc}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" asChild>
            <Link href="/admin/payment-methods">
              <ArrowLeft className="mr-2 h-4 w-4" />
              {t.common.back}
            </Link>
          </Button>
          <Button
            variant="outline"
            onClick={() => {
              void sourcesQuery.refetch()
              void catalogQuery.refetch()
              if (selectedArtifactName) {
                void artifactQuery.refetch()
              }
            }}
          >
            <RefreshCw className="mr-2 h-4 w-4" />
            {t.common.refresh}
          </Button>
        </div>
      </div>

      <Card>
        <PluginSlot
          slot="admin.payment_methods.market.filters"
          context={adminPaymentMethodsMarketPluginContext}
        />
        <CardHeader>
          <CardTitle>{t.admin.pmMarketBrowserFilters}</CardTitle>
          <CardDescription>{t.admin.pmMarketBrowserFiltersDesc}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {sourcesQuery.isLoading ? (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              {t.common.loading}
            </div>
          ) : sources.length === 0 ? (
            <div className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">
              <p className="font-medium text-foreground">{t.admin.pmMarketNoSources}</p>
              <p className="mt-2">{sourceError || t.admin.pmMarketNoSourcesDesc}</p>
            </div>
          ) : (
            <>
              <div className="grid gap-4 lg:grid-cols-[1.2fr_0.8fr_1.2fr_auto] lg:items-end">
                <div className="space-y-2">
                  <label className="text-sm font-medium">{t.admin.pmMarketSource}</label>
                  <Select
                    value={filters.source_id}
                    onValueChange={(value) => {
                      const nextSource = sources.find((item) => item.source_id === value) || null
                      const nextChannel = nextSource?.default_channel || ''
                      setFilters({
                        source_id: value,
                        channel: nextChannel,
                        q: filters.q,
                      })
                      setAppliedFilters({
                        source_id: value,
                        channel: nextChannel,
                        q: filters.q.trim(),
                      })
                      setSelectedArtifactName('')
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {sources.map((source) => (
                        <SelectItem key={source.source_id} value={source.source_id}>
                          {source.name || source.source_id}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">
                    {locale === 'zh' ? '渠道' : 'Channel'}
                  </label>
                  <Select
                    value={filters.channel || 'all'}
                    onValueChange={(value) =>
                      setFilters((prev) => ({
                        ...prev,
                        channel: value === 'all' ? '' : value,
                      }))
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">{t.common.all}</SelectItem>
                      <SelectItem value="stable">stable</SelectItem>
                      <SelectItem value="beta">beta</SelectItem>
                      <SelectItem value="alpha">alpha</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">{t.common.search}</label>
                  <Input
                    value={filters.q}
                    onChange={(e) =>
                      setFilters((prev) => ({
                        ...prev,
                        q: e.target.value,
                      }))
                    }
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        handleApplyFilters()
                      }
                    }}
                    placeholder={t.admin.pmMarketSearchPlaceholder}
                  />
                </div>
                <Button onClick={handleApplyFilters} disabled={!filters.source_id}>
                  <Search className="mr-2 h-4 w-4" />
                  {t.common.search}
                </Button>
              </div>

            </>
          )}
        </CardContent>
      </Card>

      <div className="grid gap-6 xl:grid-cols-[1.2fr_0.8fr]">
        <Card>
          <CardHeader>
            <CardTitle>{t.admin.pmMarketCatalogTitle}</CardTitle>
            <CardDescription>{t.admin.pmMarketCatalogDesc}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {catalogQuery.isLoading ? (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                {t.common.loading}
              </div>
            ) : catalogError ? (
              <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
                {catalogError}
              </div>
            ) : catalogItems.length === 0 ? (
              <div className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">
                {t.admin.pmMarketCatalogEmpty}
              </div>
            ) : (
              catalogItems.map((item) => {
                const isSelected = item.name === selectedArtifactName
                const itemTitle = resolvePaymentMarketTitle(item, locale)
                const itemDescription = resolvePaymentMarketDescription(item, locale, t.common.noData)
                return (
                  <button
                    key={item.name}
                    type="button"
                    onClick={() => setSelectedArtifactName(item.name)}
                    className={`w-full rounded-lg border p-4 text-left transition-colors ${
                      isSelected
                        ? 'border-primary bg-primary/5'
                        : 'border-border bg-background hover:bg-muted/30'
                    }`}
                  >
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div className="space-y-1">
                        <p className="font-medium">{itemTitle}</p>
                        <p className="text-xs text-muted-foreground">
                          {[item.name, item.latest_version ? `v${item.latest_version}` : null]
                            .filter(Boolean)
                            .join(' · ')}
                        </p>
                      </div>
                    </div>
                    <p className="mt-3 text-sm text-muted-foreground">
                      {itemDescription}
                    </p>
                  </button>
                )
              })
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t.admin.pmMarketArtifactTitle}</CardTitle>
            <CardDescription>{t.admin.pmMarketArtifactDesc}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {artifactQuery.isLoading ? (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Loader2 className="h-4 w-4 animate-spin" />
                {t.common.loading}
              </div>
            ) : artifactError ? (
              <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
                {artifactError}
              </div>
            ) : !artifact ? (
              <div className="rounded-lg border border-dashed p-6 text-sm text-muted-foreground">
                {t.admin.pmMarketArtifactEmpty}
              </div>
            ) : (
              <>
                {(() => {
                  const artifactTitle = resolvePaymentMarketTitle(artifact, locale)
                  const artifactDescription = resolvePaymentMarketDescription(
                    artifact,
                    locale,
                    t.common.noData
                  )
                  return (
                    <div className="space-y-2">
                      <div className="flex flex-wrap items-start justify-between gap-3">
                        <div>
                          <p className="text-lg font-semibold">{artifactTitle}</p>
                          <p className="text-xs text-muted-foreground">{artifact.name}</p>
                        </div>
                        {selectedSource && artifactLatestVersion ? (
                          <Button asChild>
                            <Link
                              href={buildPaymentMarketImportHref(
                                selectedSource,
                                artifact.name,
                                artifactLatestVersion
                              )}
                            >
                              <ExternalLink className="mr-2 h-4 w-4" />
                              {t.admin.pmMarketImportLatest}
                            </Link>
                          </Button>
                        ) : null}
                      </div>
                      <p className="text-sm text-muted-foreground">{artifactDescription}</p>
                    </div>
                  )
                })()}

                <div className="grid gap-3 md:grid-cols-2">
                  <div className="rounded-lg border bg-muted/20 px-3 py-2">
                    <p className="text-[11px] uppercase tracking-wide text-muted-foreground">
                      {t.admin.pmPackageVersion}
                    </p>
                    <p className="mt-1 text-sm font-medium">{artifact.latest_version || '-'}</p>
                  </div>
                  <div className="rounded-lg border bg-muted/20 px-3 py-2">
                    <p className="text-[11px] uppercase tracking-wide text-muted-foreground">
                      {locale === 'zh' ? '版本数' : 'Versions'}
                    </p>
                    <p className="mt-1 text-sm font-medium">{artifactVersions.length}</p>
                  </div>
                </div>

                <div className="space-y-3">
                  <div className="flex items-center gap-2">
                    <Package className="h-4 w-4 text-muted-foreground" />
                    <p className="text-sm font-medium">{t.admin.pmMarketVersionsTitle}</p>
                  </div>
                  {artifactVersions.length === 0 ? (
                    selectedSource && artifactLatestVersion ? (
                      <div className="rounded-lg border p-3">
                        <div className="flex flex-wrap items-center justify-between gap-3">
                          <div>
                            <p className="font-medium">{artifactLatestVersion}</p>
                            <p className="text-xs text-muted-foreground">
                              {t.admin.pmMarketVersionsFallback}
                            </p>
                          </div>
                          <Button variant="outline" asChild>
                            <Link
                              href={buildPaymentMarketImportHref(
                                selectedSource,
                                artifact.name,
                                artifactLatestVersion
                              )}
                            >
                              <ExternalLink className="mr-2 h-4 w-4" />
                              {t.admin.pmImportFromMarket}
                            </Link>
                          </Button>
                        </div>
                      </div>
                    ) : (
                      <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
                        {t.admin.pmMarketVersionsEmpty}
                      </div>
                    )
                  ) : (
                    artifactVersions.map((version) => {
                      const versionValue = String(version.version || '').trim()
                      const isRequested =
                        preferredVersion !== '' && preferredVersion === versionValue
                      return (
                        <div
                          key={`${versionValue}-${version.channel || 'default'}`}
                          className={`rounded-lg border p-3 ${
                            isRequested ? 'border-primary bg-primary/5' : 'bg-background'
                          }`}
                        >
                          <div className="flex flex-wrap items-center justify-between gap-3">
                            <div className="space-y-1">
                              <p className="font-medium">{versionValue || '-'}</p>
                              <p className="text-xs text-muted-foreground">
                                {[
                                  version.channel || null,
                                  version.published_at || t.common.noData,
                                  isRequested ? t.admin.pmMarketSelectedVersion : null,
                                ]
                                  .filter(Boolean)
                                  .join(' · ')}
                              </p>
                            </div>
                            {selectedSource ? (
                              <Button variant="outline" asChild>
                                <Link
                                  href={buildPaymentMarketImportHref(
                                    selectedSource,
                                    artifact.name,
                                    versionValue
                                  )}
                                >
                                  <ExternalLink className="mr-2 h-4 w-4" />
                                  {t.admin.pmImportFromMarket}
                                </Link>
                              </Button>
                            ) : null}
                          </div>
                        </div>
                      )
                    })
                  )}
                </div>
              </>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
