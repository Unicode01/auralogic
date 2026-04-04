'use client'
/* eslint-disable @next/next/no-img-element */

import { useCallback, useMemo, useState, type ReactNode } from 'react'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { OrderStatusBadge } from './order-status-badge'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Package,
  MapPin,
  Truck,
  Shield,
  Clock3,
  AlertTriangle,
  CircleSlash,
  MessageSquare,
  ShieldCheck,
  Key,
  Copy,
  Eye,
  EyeOff,
  Headphones,
} from 'lucide-react'
import { formatDate, formatCurrency } from '@/lib/utils'
import type { Order } from '@/types/order'
import type { VirtualProductStock } from '@/types/product'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { PluginSlotBatchBoundary } from '@/lib/plugin-slot-batch'
import toast from 'react-hot-toast'

interface ProductSerial {
  id: number
  serial_number: string
  product_code: string
  sequence_number: number
  anti_counterfeit_code: string
  view_count: number
  created_at: string
  product?: {
    name: string
    sku: string
  }
}

interface OrderDetailProps {
  order: Order
  serials?: ProductSerial[]
  virtualStocks?: VirtualProductStock[]
  isVirtualOnly?: boolean
  paymentCard?: ReactNode
  shippingForm?: ReactNode
  shippingFormURL?: string
  shippingFormToken?: string
  shippingFormExpiresAt?: string
  showVirtualStockRemark?: boolean
  showOperationalMeta?: boolean
  pluginSlotNamespace?: string
  pluginSlotContext?: Record<string, any>
  pluginSlotPath?: string
}

export function OrderDetail({
  order,
  serials,
  virtualStocks,
  isVirtualOnly = false,
  paymentCard,
  shippingForm,
  shippingFormURL,
  shippingFormToken,
  shippingFormExpiresAt,
  showVirtualStockRemark = false,
  showOperationalMeta = false,
  pluginSlotNamespace,
  pluginSlotContext,
  pluginSlotPath,
}: OrderDetailProps) {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const isDraft = order.status === 'draft'
  const isNeedResubmit = order.status === 'need_resubmit'
  const [showContent, setShowContent] = useState<Record<number, boolean>>({})
  const orderItems = Array.isArray(order.items) ? order.items : []
  const serialGenerationStatus = String(
    order.serialGenerationStatus || order.serial_generation_status || ''
  ).trim()
  const serialGenerationError = String(
    order.serialGenerationError || order.serial_generation_error || ''
  ).trim()
  const receiverAddressText = [
    order.receiverProvince || order.receiver_province,
    order.receiverCity || order.receiver_city,
    order.receiverDistrict || order.receiver_district,
    order.receiverAddress || order.receiver_address,
  ]
    .filter(Boolean)
    .join(' ')
    .concat(
      order.receiverPostcode || order.receiver_postcode
        ? ` (${order.receiverPostcode || order.receiver_postcode})`
        : ''
    )
  const formToken = String(shippingFormToken || order.formToken || order.form_token || '').trim()
  const formExpiresAt = String(
    shippingFormExpiresAt || order.formExpiresAt || order.form_expires_at || ''
  ).trim()
  const externalUserId = String(order.externalUserId || order.external_user_id || '').trim()
  const externalOrderId = String(order.externalOrderId || order.external_order_id || '').trim()
  const source = String(order.source || '').trim()
  const sourcePlatform = String(
    order.sourcePlatform || order.source_platform || order.platform || ''
  ).trim()
  const adminRemark = String(order.adminRemark || order.admin_remark || '').trim()
  const buildSectionPluginContext = useCallback(
    (section: string, extra?: Record<string, any>) => ({
      ...(pluginSlotContext || {}),
      section,
      ...(extra || {}),
    }),
    [pluginSlotContext]
  )
  const renderSectionPluginSlot = (
    slotSuffix: string,
    section: string,
    extra?: Record<string, any>,
    display: 'stack' | 'inline' = 'stack'
  ) =>
    pluginSlotNamespace ? (
      <PluginSlot
        slot={`${pluginSlotNamespace}.${slotSuffix}`}
        path={pluginSlotPath}
        context={buildSectionPluginContext(section, extra)}
        display={display}
      />
    ) : null

  const toggleContentVisibility = (id: number) => {
    setShowContent((prev) => ({ ...prev, [id]: !prev[id] }))
  }

  const copyToClipboard = (content: string) => {
    const normalized = String(content || '').trim()
    if (!normalized) return
    void navigator.clipboard.writeText(normalized)
    toast.success(t.order.copiedToClipboard)
  }

  const serialGenerationMeta = useMemo(() => {
    switch (serialGenerationStatus) {
      case 'queued':
        return {
          title: t.admin.serialGenerationQueued,
          description: t.admin.serialGenerationQueuedDesc,
          badgeClassName:
            'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-500/40 dark:bg-sky-950/30 dark:text-sky-200',
          icon: Clock3,
        }
      case 'processing':
        return {
          title: t.admin.serialGenerationProcessing,
          description: t.admin.serialGenerationProcessingDesc,
          badgeClassName:
            'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-500/40 dark:bg-amber-950/30 dark:text-amber-200',
          icon: Clock3,
        }
      case 'failed':
        return {
          title: t.admin.serialGenerationFailed,
          description: t.admin.serialGenerationFailedDesc,
          badgeClassName:
            'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-500/40 dark:bg-rose-950/30 dark:text-rose-200',
          icon: AlertTriangle,
        }
      case 'cancelled':
        return {
          title: t.admin.serialGenerationCancelled,
          description: t.admin.serialGenerationCancelledDesc,
          badgeClassName:
            'border-slate-200 bg-slate-50 text-slate-700 dark:border-slate-500/40 dark:bg-slate-950/30 dark:text-slate-200',
          icon: CircleSlash,
        }
      default:
        return null
    }
  }, [serialGenerationStatus, t.admin])
  const showSerialGenerationState =
    Boolean(serialGenerationMeta) && (!serials || serials.length === 0)
  const SerialGenerationIcon = serialGenerationMeta?.icon

  const renderCopyButton = (value: string, label: string) => {
    const normalized = String(value || '').trim()
    const buttonLabel = `${t.common.copy} ${label}`
    return (
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="h-7 w-7 p-0"
        onClick={() => copyToClipboard(normalized)}
        aria-label={buttonLabel}
        title={buttonLabel}
        disabled={!normalized}
      >
        <Copy className="h-3.5 w-3.5" />
      </Button>
    )
  }

  const slotScope =
    String(pluginSlotPath || '')
      .trim()
      .startsWith('/admin') ||
    String(pluginSlotNamespace || '')
      .trim()
      .toLowerCase()
      .startsWith('admin.')
      ? 'admin'
      : 'public'
  const orderDetailBatchItems = useMemo(() => {
    if (!pluginSlotNamespace) {
      return []
    }

    const items = [
      {
        slot: `${pluginSlotNamespace}.info_actions`,
        path: pluginSlotPath,
        hostContext: buildSectionPluginContext('info'),
      },
      {
        slot: `${pluginSlotNamespace}.info.after`,
        path: pluginSlotPath,
        hostContext: buildSectionPluginContext('info', {
          privacy_protected: Boolean(order.privacyProtected || order.privacy_protected),
          shared_to_support: Boolean(order.sharedToSupport || order.shared_to_support),
          has_form_submitted_at: Boolean(order.formSubmittedAt || order.form_submitted_at),
          has_shipped_at: Boolean(order.shippedAt || order.shipped_at),
        }),
      },
      {
        slot: `${pluginSlotNamespace}.product_actions`,
        path: pluginSlotPath,
        hostContext: buildSectionPluginContext('products', {
          item_count: orderItems.length,
        }),
      },
      {
        slot: `${pluginSlotNamespace}.products.after`,
        path: pluginSlotPath,
        hostContext: buildSectionPluginContext('products', {
          item_count: orderItems.length,
        }),
      },
    ]

    if (virtualStocks && virtualStocks.length > 0) {
      items.push(
        {
          slot: `${pluginSlotNamespace}.virtual_stock_actions`,
          path: pluginSlotPath,
          hostContext: buildSectionPluginContext('virtual_stocks', {
            virtual_stock_count: virtualStocks.length,
          }),
        },
        {
          slot: `${pluginSlotNamespace}.virtual_stocks.after`,
          path: pluginSlotPath,
          hostContext: buildSectionPluginContext('virtual_stocks', {
            virtual_stock_count: virtualStocks.length,
            show_remark: showVirtualStockRemark,
          }),
        }
      )
    }

    if (!isVirtualOnly && order.status !== 'pending_payment') {
      items.push({
        slot: `${pluginSlotNamespace}.shipping_actions`,
        path: pluginSlotPath,
        hostContext: buildSectionPluginContext('shipping', {
          has_tracking: Boolean(order.trackingNo || order.tracking_no),
          has_receiver: Boolean(order.receiverName || order.receiver_name),
        }),
      })

      if (order.receiverName || order.receiver_name || order.trackingNo || order.tracking_no) {
        if (order.receiverName || order.receiver_name) {
          items.push({
            slot: `${pluginSlotNamespace}.shipping.receiver.after`,
            path: pluginSlotPath,
            hostContext: buildSectionPluginContext('shipping', {
              shipping_section: 'receiver',
              has_receiver: true,
            }),
          })
        }
        if (order.trackingNo || order.tracking_no) {
          items.push({
            slot: `${pluginSlotNamespace}.shipping.tracking.after`,
            path: pluginSlotPath,
            hostContext: buildSectionPluginContext('shipping', {
              shipping_section: 'tracking',
              has_tracking: true,
            }),
          })
        }
      } else if (shippingForm) {
        items.push({
          slot: `${pluginSlotNamespace}.shipping.form.after`,
          path: pluginSlotPath,
          hostContext: buildSectionPluginContext('shipping', {
            shipping_section: 'form',
            has_tracking: false,
            has_receiver: false,
          }),
        })
      } else {
        items.push({
          slot: `${pluginSlotNamespace}.shipping.empty`,
          path: pluginSlotPath,
          hostContext: buildSectionPluginContext('shipping', {
            shipping_section: 'empty',
            has_tracking: false,
            has_receiver: false,
            is_draft: isDraft,
            need_resubmit: isNeedResubmit,
          }),
        })
      }
    }

    if (order.remark) {
      items.push({
        slot: `${pluginSlotNamespace}.remark.after`,
        path: pluginSlotPath,
        hostContext: buildSectionPluginContext('remark', {
          has_remark: true,
        }),
      })
    }

    if (serials && serials.length > 0) {
      items.push(
        {
          slot: `${pluginSlotNamespace}.serials_actions`,
          path: pluginSlotPath,
          hostContext: buildSectionPluginContext('serials', {
            serial_count: serials.length,
          }),
        },
        {
          slot: `${pluginSlotNamespace}.serials.after`,
          path: pluginSlotPath,
          hostContext: buildSectionPluginContext('serials', {
            serial_count: serials.length,
          }),
        }
      )
    }

    return items
  }, [
    buildSectionPluginContext,
    isDraft,
    isNeedResubmit,
    isVirtualOnly,
    order,
    orderItems.length,
    pluginSlotNamespace,
    pluginSlotPath,
    serials,
    shippingForm,
    showVirtualStockRemark,
    virtualStocks,
  ])

  const content = (
    <div className="space-y-6">
      {/* 订单基本信息 */}
      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <CardTitle className="flex min-w-0 flex-wrap items-center gap-2">
              <span>{t.order.orderInfo}</span>
              <OrderStatusBadge status={order.status} />
            </CardTitle>
            <div className="flex flex-wrap items-center gap-2">
              {(order.privacyProtected || order.privacy_protected) && (
                <Badge variant="outline" className="flex items-center gap-1">
                  <Shield className="h-3 w-3" />
                  {t.order.privacyProtected}
                </Badge>
              )}
              {(order.sharedToSupport || order.shared_to_support) && (
                <Badge variant="secondary" className="flex items-center gap-1">
                  <Headphones className="h-3 w-3" />
                  {t.order.sharedToSupport}
                </Badge>
              )}
              {pluginSlotNamespace ? (
                <PluginSlot
                  slot={`${pluginSlotNamespace}.info_actions`}
                  path={pluginSlotPath}
                  context={buildSectionPluginContext('info')}
                  display="inline"
                />
              ) : null}
            </div>
          </div>
        </CardHeader>

        <CardContent>
          <dl className="grid grid-cols-1 gap-4 text-sm md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            <div>
              <dt className="text-muted-foreground">{t.order.orderNo}</dt>
              <dd className="flex flex-wrap items-center gap-2 break-all font-mono font-medium">
                <span>{order.orderNo || order.order_no}</span>
                {renderCopyButton(String(order.orderNo || order.order_no || ''), t.order.orderNo)}
              </dd>
            </div>
            {(order.externalUserName || order.external_user_name) && (
              <div>
                <dt className="text-muted-foreground">{t.order.platformUser}</dt>
                <dd className="font-medium">
                  {order.externalUserName || order.external_user_name}
                </dd>
              </div>
            )}
            {(order.userEmail || order.user_email) && (
              <div>
                <dt className="text-muted-foreground">{t.order.accountEmail}</dt>
                <dd className="flex flex-wrap items-center gap-2 break-all font-medium">
                  <span>{order.userEmail || order.user_email}</span>
                  {renderCopyButton(
                    String(order.userEmail || order.user_email || ''),
                    t.order.accountEmail
                  )}
                </dd>
              </div>
            )}
            <div>
              <dt className="text-muted-foreground">{t.order.createdAt}</dt>
              <dd>{formatDate(order.createdAt || order.created_at || '')}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">{t.order.orderAmount}</dt>
              <dd className="font-semibold text-foreground">
                {formatCurrency(order.total_amount_minor ?? 0, order.currency)}
              </dd>
            </div>
            {showOperationalMeta && source && (
              <div>
                <dt className="text-muted-foreground">{t.order.orderSource}</dt>
                <dd className="font-medium">{source}</dd>
              </div>
            )}
            {showOperationalMeta && sourcePlatform && (
              <div>
                <dt className="text-muted-foreground">{t.order.sourcePlatform}</dt>
                <dd className="font-medium">{sourcePlatform}</dd>
              </div>
            )}
            {showOperationalMeta && externalUserId && (
              <div>
                <dt className="text-muted-foreground">{t.order.externalUserId}</dt>
                <dd className="flex flex-wrap items-center gap-2 break-all font-mono font-medium">
                  <span>{externalUserId}</span>
                  {renderCopyButton(externalUserId, t.order.externalUserId)}
                </dd>
              </div>
            )}
            {showOperationalMeta && externalOrderId && (
              <div>
                <dt className="text-muted-foreground">{t.order.externalOrderId}</dt>
                <dd className="flex flex-wrap items-center gap-2 break-all font-mono font-medium">
                  <span>{externalOrderId}</span>
                  {renderCopyButton(externalOrderId, t.order.externalOrderId)}
                </dd>
              </div>
            )}
            {showOperationalMeta && shippingFormURL && (
              <div>
                <dt className="text-muted-foreground">{t.order.shippingFormLink}</dt>
                <dd className="flex flex-wrap items-center gap-2 break-all font-medium">
                  <a
                    href={shippingFormURL}
                    target="_blank"
                    rel="noreferrer"
                    className="min-w-0 flex-1 break-all text-primary underline-offset-4 hover:underline"
                  >
                    {shippingFormURL}
                  </a>
                  {renderCopyButton(shippingFormURL, t.order.shippingFormLink)}
                </dd>
              </div>
            )}
            {showOperationalMeta && formToken && (
              <div>
                <dt className="text-muted-foreground">{t.order.shippingFormToken}</dt>
                <dd className="flex flex-wrap items-center gap-2 break-all font-mono font-medium">
                  <span>{formToken}</span>
                  {renderCopyButton(formToken, t.order.shippingFormToken)}
                </dd>
              </div>
            )}
            {showOperationalMeta && formExpiresAt && (
              <div>
                <dt className="text-muted-foreground">{t.order.formExpiresAt}</dt>
                <dd>{formatDate(formExpiresAt)}</dd>
              </div>
            )}
            {(order.formSubmittedAt || order.form_submitted_at) && (
              <div>
                <dt className="text-muted-foreground">{t.order.formSubmittedAt}</dt>
                <dd>{formatDate(order.formSubmittedAt || order.form_submitted_at || '')}</dd>
              </div>
            )}
            {showOperationalMeta && (order.updatedAt || order.updated_at) && (
              <div>
                <dt className="text-muted-foreground">{t.order.updatedAt}</dt>
                <dd>{formatDate(order.updatedAt || order.updated_at || '')}</dd>
              </div>
            )}
            {(order.shippedAt || order.shipped_at) && (
              <div>
                <dt className="text-muted-foreground">{t.order.shippedAt}</dt>
                <dd>{formatDate(order.shippedAt || order.shipped_at || '')}</dd>
              </div>
            )}
          </dl>
          {renderSectionPluginSlot('info.after', 'info', {
            privacy_protected: Boolean(order.privacyProtected || order.privacy_protected),
            shared_to_support: Boolean(order.sharedToSupport || order.shared_to_support),
            has_form_submitted_at: Boolean(order.formSubmittedAt || order.form_submitted_at),
            has_shipped_at: Boolean(order.shippedAt || order.shipped_at),
          })}
        </CardContent>
      </Card>

      {/* 商品信息与收货物流信息/虚拟产品内容 - 在宽屏上并排显示 */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* 商品信息 */}
        <Card className="lg:col-span-1">
          <CardHeader>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex min-w-0 items-center gap-2">
                <CardTitle className="flex min-w-0 items-center gap-2">
                  <Package className="h-5 w-5 shrink-0" />
                  <span className="truncate">{t.order.productInfo}</span>
                </CardTitle>
                <Badge variant="outline" className="shrink-0">
                  {orderItems.length}
                </Badge>
              </div>
              {pluginSlotNamespace ? (
                <PluginSlot
                  slot={`${pluginSlotNamespace}.product_actions`}
                  path={pluginSlotPath}
                  context={buildSectionPluginContext('products', {
                    item_count: orderItems.length,
                  })}
                  display="inline"
                />
              ) : null}
            </div>
          </CardHeader>

          <CardContent>
            <div className="space-y-4">
              {orderItems.map((item, index) => (
                <div key={index}>
                  {index > 0 && <Separator className="my-4" />}

                  <div className="flex gap-4">
                    {/* 商品图片 */}
                    <div className="h-20 w-20 flex-shrink-0 overflow-hidden rounded bg-muted">
                      {item.imageUrl || item.image_url ? (
                        <img
                          src={item.imageUrl || item.image_url}
                          alt={item.name}
                          className="h-full w-full object-cover"
                          onError={(e) => {
                            e.currentTarget.style.display = 'none'
                            e.currentTarget.parentElement
                              ?.querySelector('.img-fallback')
                              ?.classList.remove('hidden')
                          }}
                        />
                      ) : null}
                      <div
                        className={`img-fallback flex h-full w-full items-center justify-center ${item.imageUrl || item.image_url ? 'hidden' : ''}`}
                      >
                        <Package className="h-10 w-10 text-muted-foreground" />
                      </div>
                    </div>

                    {/* 商品信息 */}
                    <div className="min-w-0 flex-1">
                      <h4 className="truncate font-medium">{item.name}</h4>
                      <p className="truncate text-sm text-muted-foreground">SKU: {item.sku}</p>

                      {/* 商品属性 */}
                      {item.attributes && Object.keys(item.attributes).length > 0 && (
                        <div className="mt-1 flex flex-wrap gap-2">
                          {Object.entries(item.attributes).map(([key, value]) => (
                            <Badge key={key} variant="secondary" className="text-xs">
                              {key}: {value as string}
                            </Badge>
                          ))}
                        </div>
                      )}
                    </div>

                    {/* 数量 */}
                    <div className="flex-shrink-0 text-right">
                      <p className="font-medium">x{item.quantity}</p>
                    </div>
                  </div>
                </div>
              ))}
            </div>
            {renderSectionPluginSlot('products.after', 'products', {
              item_count: orderItems.length,
            })}
          </CardContent>
        </Card>

        {/* 付款方式卡片（待付款时显示在商品信息右侧） */}
        {paymentCard && <div className="lg:col-span-1">{paymentCard}</div>}

        {/* 虚拟产品卡密 - 与商品信息并排显示 */}
        {virtualStocks && virtualStocks.length > 0 && (
          <Card className="lg:col-span-1">
            <CardHeader>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex min-w-0 items-center gap-2">
                  <CardTitle className="flex min-w-0 items-center gap-2">
                    <Key className="h-5 w-5 shrink-0" />
                    <span className="truncate">{t.order.virtualProductContent}</span>
                  </CardTitle>
                  <Badge variant="outline" className="shrink-0">
                    {virtualStocks.length}
                  </Badge>
                </div>
                {pluginSlotNamespace ? (
                  <PluginSlot
                    slot={`${pluginSlotNamespace}.virtual_stock_actions`}
                    path={pluginSlotPath}
                    context={buildSectionPluginContext('virtual_stocks', {
                      virtual_stock_count: virtualStocks.length,
                    })}
                    display="inline"
                  />
                ) : null}
              </div>
            </CardHeader>

            <CardContent>
              <div className="space-y-3">
                <div className="rounded-lg border border-green-200 bg-green-50 p-3 text-sm text-green-800 dark:border-green-800 dark:bg-green-950 dark:text-green-200">
                  <p className="mb-1 font-medium">{t.order.virtualProductDelivered}</p>
                  <p>{t.order.virtualProductKeepSafe}</p>
                </div>

                {virtualStocks.map((stock) => (
                  <div key={stock.id} className="space-y-2 rounded-lg border p-3 md:p-4">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="flex min-w-0 flex-wrap items-center gap-1">
                        <code className="break-all rounded bg-muted px-2 py-1 font-mono text-sm md:px-3 md:py-2 md:text-lg">
                          {showContent[stock.id] ? stock.content : '************'}
                        </code>
                        <div className="flex shrink-0 items-center">
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            className="h-7 w-7 p-0 md:h-8 md:w-8"
                            onClick={() => toggleContentVisibility(stock.id)}
                            aria-label={`${showContent[stock.id] ? t.common.collapse : t.common.expand} ${t.order.virtualProductContent}`}
                            title={`${showContent[stock.id] ? t.common.collapse : t.common.expand} ${t.order.virtualProductContent}`}
                          >
                            {showContent[stock.id] ? (
                              <EyeOff className="h-4 w-4" />
                            ) : (
                              <Eye className="h-4 w-4" />
                            )}
                          </Button>
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            className="h-7 w-7 p-0 md:h-8 md:w-8"
                            onClick={() => copyToClipboard(stock.content)}
                            aria-label={`${t.common.copy} ${t.order.virtualProductContent}`}
                            title={`${t.common.copy} ${t.order.virtualProductContent}`}
                          >
                            <Copy className="h-4 w-4" />
                          </Button>
                        </div>
                      </div>
                    </div>
                    {showVirtualStockRemark && stock.remark && (
                      <p className="text-sm text-muted-foreground">{stock.remark}</p>
                    )}
                    {stock.delivered_at && (
                      <div className="text-xs text-muted-foreground">
                        {t.order.deliveryTime}: {formatDate(stock.delivered_at)}
                      </div>
                    )}
                  </div>
                ))}

                <div className="mt-2 text-xs text-muted-foreground">
                  {t.order.totalItemsCount.replace('{count}', String(virtualStocks.length))}
                </div>
              </div>
              {renderSectionPluginSlot('virtual_stocks.after', 'virtual_stocks', {
                virtual_stock_count: virtualStocks.length,
                show_remark: showVirtualStockRemark,
              })}
            </CardContent>
          </Card>
        )}

        {/* 收货与物流信息 - 虚拟商品订单和待付款订单不显示 */}
        {!isVirtualOnly &&
          order.status !== 'pending_payment' &&
          (order.receiverName || order.receiver_name || order.trackingNo || order.tracking_no ? (
            <Card className="lg:col-span-1">
              <CardHeader>
                <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                  <CardTitle className="flex items-center gap-2">
                    <Truck className="h-5 w-5" />
                    {t.order.shippingInfo} & {t.order.trackingInfo}
                  </CardTitle>
                  {pluginSlotNamespace ? (
                    <PluginSlot
                      slot={`${pluginSlotNamespace}.shipping_actions`}
                      path={pluginSlotPath}
                      context={buildSectionPluginContext('shipping', {
                        has_tracking: Boolean(order.trackingNo || order.tracking_no),
                        has_receiver: Boolean(order.receiverName || order.receiver_name),
                      })}
                      display="inline"
                    />
                  ) : null}
                </div>
              </CardHeader>

              <CardContent className="space-y-6">
                {/* 收货信息部分 */}
                {(order.receiverName || order.receiver_name) && (
                  <div>
                    <h4 className="mb-3 flex items-center gap-2 text-sm font-semibold">
                      <MapPin className="h-4 w-4" />
                      {t.order.shippingInfo}
                    </h4>
                    <dl className="space-y-3 pl-6 text-sm">
                      <div className="flex flex-col sm:flex-row">
                        <dt className="flex-shrink-0 text-muted-foreground sm:w-28">
                          {t.order.receiverName}
                        </dt>
                        <dd className="font-medium">{order.receiverName || order.receiver_name}</dd>
                      </div>
                      <div className="flex flex-col sm:flex-row">
                        <dt className="flex-shrink-0 text-muted-foreground sm:w-28">
                          {t.order.receiverPhone}
                        </dt>
                        <dd className="flex flex-wrap items-center gap-2 break-all font-medium">
                          <span>{order.receiverPhone || order.receiver_phone}</span>
                          {renderCopyButton(
                            String(order.receiverPhone || order.receiver_phone || ''),
                            t.order.receiverPhone
                          )}
                        </dd>
                      </div>
                      <div className="flex flex-col sm:flex-row">
                        <dt className="flex-shrink-0 text-muted-foreground sm:w-28">
                          {t.order.receiverEmail}
                        </dt>
                        <dd className="flex flex-wrap items-center gap-2 break-all font-medium">
                          <span>{order.receiverEmail || order.receiver_email}</span>
                          {renderCopyButton(
                            String(order.receiverEmail || order.receiver_email || ''),
                            t.order.receiverEmail
                          )}
                        </dd>
                      </div>
                      <div className="flex flex-col sm:flex-row">
                        <dt className="flex-shrink-0 text-muted-foreground sm:w-28">
                          {t.order.receiverAddress}
                        </dt>
                        <dd className="flex flex-wrap items-start gap-2 break-words font-medium">
                          <span className="min-w-0 flex-1">{receiverAddressText}</span>
                          {renderCopyButton(receiverAddressText, t.order.receiverAddress)}
                        </dd>
                      </div>
                    </dl>
                    {renderSectionPluginSlot('shipping.receiver.after', 'shipping', {
                      shipping_section: 'receiver',
                      has_receiver: true,
                    })}
                  </div>
                )}

                {/* 分隔线 - 只在两者都存在时显示 */}
                {(order.receiverName || order.receiver_name) &&
                  (order.trackingNo || order.tracking_no) && <Separator />}

                {/* 物流信息部分 */}
                {(order.trackingNo || order.tracking_no) && (
                  <div>
                    <h4 className="mb-3 flex items-center gap-2 text-sm font-semibold">
                      <Truck className="h-4 w-4" />
                      {t.order.trackingInfo}
                    </h4>
                    <dl className="space-y-3 pl-6 text-sm">
                      <div className="flex flex-col sm:flex-row">
                        <dt className="flex-shrink-0 text-muted-foreground sm:w-28">
                          {t.order.trackingNo}
                        </dt>
                        <dd className="flex flex-wrap items-center gap-2 break-all font-mono font-medium">
                          <span>{order.trackingNo || order.tracking_no}</span>
                          {renderCopyButton(
                            String(order.trackingNo || order.tracking_no || ''),
                            t.order.trackingNo
                          )}
                        </dd>
                      </div>
                      {(order.shippedAt || order.shipped_at) && (
                        <div className="flex flex-col sm:flex-row">
                          <dt className="flex-shrink-0 text-muted-foreground sm:w-28">
                            {t.order.shippedAt}
                          </dt>
                          <dd>{formatDate(order.shippedAt || order.shipped_at || '')}</dd>
                        </div>
                      )}
                    </dl>
                    {renderSectionPluginSlot('shipping.tracking.after', 'shipping', {
                      shipping_section: 'tracking',
                      has_tracking: true,
                    })}
                  </div>
                )}
              </CardContent>
            </Card>
          ) : shippingForm ? (
            <Card className="lg:col-span-1">
              <CardHeader>
                <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                  <CardTitle className="flex items-center gap-2">
                    <MapPin className="h-5 w-5" />
                    {t.order.shippingInfo}
                  </CardTitle>
                  {pluginSlotNamespace ? (
                    <PluginSlot
                      slot={`${pluginSlotNamespace}.shipping_actions`}
                      path={pluginSlotPath}
                      context={buildSectionPluginContext('shipping', {
                        has_tracking: false,
                        has_receiver: false,
                      })}
                      display="inline"
                    />
                  ) : null}
                </div>
              </CardHeader>
              <CardContent>
                {shippingForm}
                {renderSectionPluginSlot('shipping.form.after', 'shipping', {
                  shipping_section: 'form',
                  has_tracking: false,
                  has_receiver: false,
                })}
              </CardContent>
            </Card>
          ) : (
            <Card className="border-dashed lg:col-span-1">
              <CardHeader>
                <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                  <CardTitle className="flex items-center gap-2">
                    <Truck className="h-5 w-5" />
                    {t.order.shippingInfo}
                  </CardTitle>
                  {pluginSlotNamespace ? (
                    <PluginSlot
                      slot={`${pluginSlotNamespace}.shipping_actions`}
                      path={pluginSlotPath}
                      context={buildSectionPluginContext('shipping', {
                        has_tracking: false,
                        has_receiver: false,
                      })}
                      display="inline"
                    />
                  ) : null}
                </div>
              </CardHeader>
              <CardContent>
                <div className="py-8 text-center text-muted-foreground">
                  <MapPin className="mx-auto mb-3 h-12 w-12 opacity-50" />
                  <p className="mb-1 font-medium">{t.order.shippingNotFilled}</p>
                  <p className="text-sm">
                    {isDraft
                      ? t.order.shippingNotFilledDesc
                      : isNeedResubmit
                        ? t.order.shippingResubmitDesc
                        : t.order.noShippingInfo}
                  </p>
                </div>
                {renderSectionPluginSlot('shipping.empty', 'shipping', {
                  shipping_section: 'empty',
                  has_tracking: false,
                  has_receiver: false,
                  is_draft: isDraft,
                  need_resubmit: isNeedResubmit,
                })}
              </CardContent>
            </Card>
          ))}
      </div>

      {/* 用户备注 */}
      {order.remark && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <MessageSquare className="h-5 w-5" />
              {t.order.userRemark}
            </CardTitle>
          </CardHeader>

          <CardContent>
            <div className="rounded-md bg-muted/50 p-4">
              <p className="whitespace-pre-wrap text-sm">{order.remark}</p>
            </div>
            {renderSectionPluginSlot('remark.after', 'remark', {
              has_remark: true,
            })}
          </CardContent>
        </Card>
      )}

      {showOperationalMeta && adminRemark && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <MessageSquare className="h-5 w-5" />
              {t.order.adminRemark}
            </CardTitle>
          </CardHeader>

          <CardContent>
            <div className="rounded-md bg-muted/50 p-4">
              <p className="whitespace-pre-wrap text-sm">{adminRemark}</p>
            </div>
          </CardContent>
        </Card>
      )}

      {showSerialGenerationState && serialGenerationMeta ? (
        <Card>
          <CardHeader>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex min-w-0 items-center gap-2">
                <CardTitle className="flex min-w-0 items-center gap-2">
                  <ShieldCheck className="h-5 w-5 shrink-0" />
                  <span className="truncate">{t.admin.antiCounterfeitSerial}</span>
                </CardTitle>
                <Badge
                  variant="outline"
                  className={`shrink-0 ${serialGenerationMeta.badgeClassName}`}
                >
                  {serialGenerationMeta.title}
                </Badge>
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <div className="rounded-lg border border-dashed border-border/70 bg-muted/30 p-4">
              <div className="flex items-start gap-3">
                {SerialGenerationIcon ? (
                  <SerialGenerationIcon className="mt-0.5 h-5 w-5 text-muted-foreground" />
                ) : null}
                <div className="space-y-1">
                  <p className="text-sm font-medium">{serialGenerationMeta.title}</p>
                  <p className="text-sm text-muted-foreground">
                    {serialGenerationMeta.description}
                  </p>
                  {serialGenerationError ? (
                    <p className="text-xs text-destructive">
                      {t.admin.serialGenerationErrorLabel}: {serialGenerationError}
                    </p>
                  ) : null}
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      ) : null}

      {/* 产品序列号（管理员端） */}
      {serials && serials.length > 0 && (
        <Card>
          <CardHeader>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex min-w-0 items-center gap-2">
                <CardTitle className="flex min-w-0 items-center gap-2">
                  <ShieldCheck className="h-5 w-5 shrink-0" />
                  <span className="truncate">{t.admin.antiCounterfeitSerial}</span>
                </CardTitle>
                <Badge variant="outline" className="shrink-0">
                  {serials.length}
                </Badge>
              </div>
              {pluginSlotNamespace ? (
                <PluginSlot
                  slot={`${pluginSlotNamespace}.serials_actions`}
                  path={pluginSlotPath}
                  context={buildSectionPluginContext('serials', {
                    serial_count: serials.length,
                  })}
                  display="inline"
                />
              ) : null}
            </div>
          </CardHeader>

          <CardContent>
            <div className="space-y-3">
              <div className="rounded-lg border border-blue-200 bg-blue-50 p-3 text-sm text-blue-800 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-200">
                <p className="mb-1 font-medium">💡 {t.admin.shippingTip}</p>
                <p>{t.admin.shippingTipContent}</p>
              </div>

              {serials.map((serial, index) => (
                <div key={serial.id} className="space-y-2 rounded-lg border p-4">
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="mb-1 text-xs text-muted-foreground">
                        {serial.product?.name || t.admin.productFallback} (SKU:{' '}
                        {serial.product?.sku})
                      </div>
                      <div className="font-mono text-xl font-bold">{serial.serial_number}</div>
                    </div>
                    <Badge variant="outline" className="text-xs">
                      {t.admin.itemIndex.replace('{index}', String(serial.sequence_number))}
                    </Badge>
                  </div>
                  <div className="flex gap-2 text-xs text-muted-foreground">
                    <span>
                      {t.admin.productCodeLabel2}:{' '}
                      <span className="font-mono font-semibold">{serial.product_code}</span>
                    </span>
                    <span>•</span>
                    <span>
                      {t.admin.antiCounterfeitCodeLabel}:{' '}
                      <span className="font-mono font-semibold">
                        {serial.anti_counterfeit_code}
                      </span>
                    </span>
                    <span>•</span>
                    <span>
                      {t.admin.viewCountLabel}: {serial.view_count}
                    </span>
                  </div>
                </div>
              ))}

              <div className="mt-2 text-xs text-muted-foreground">
                {t.admin.serialSummary.replace('{count}', String(serials.length))}
              </div>
            </div>
            {renderSectionPluginSlot('serials.after', 'serials', {
              serial_count: serials.length,
            })}
          </CardContent>
        </Card>
      )}
    </div>
  )

  if (!pluginSlotNamespace || orderDetailBatchItems.length === 0) {
    return content
  }

  return (
    <PluginSlotBatchBoundary
      scope={slotScope}
      path={pluginSlotPath || (slotScope === 'admin' ? '/admin' : '/')}
      items={orderDetailBatchItems}
    >
      {content}
    </PluginSlotBatchBoundary>
  )
}
