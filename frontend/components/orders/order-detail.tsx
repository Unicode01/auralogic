'use client'

import { useState } from 'react'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { OrderStatusBadge } from './order-status-badge'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Package, MapPin, Truck, Shield, MessageSquare, ShieldCheck, Key, Copy, Eye, EyeOff, Headphones } from 'lucide-react'
import { formatDate, formatCurrency } from '@/lib/utils'
import type { Order } from '@/types/order'
import type { VirtualProductStock } from '@/types/product'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import toast from 'react-hot-toast'
import { ReactNode } from 'react'

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
}

export function OrderDetail({ order, serials, virtualStocks, isVirtualOnly = false, paymentCard, shippingForm }: OrderDetailProps) {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const isDraft = order.status === 'draft'
  const isNeedResubmit = order.status === 'need_resubmit'
  const [showContent, setShowContent] = useState<Record<number, boolean>>({})

  const toggleContentVisibility = (id: number) => {
    setShowContent(prev => ({ ...prev, [id]: !prev[id] }))
  }

  const copyToClipboard = (content: string) => {
    navigator.clipboard.writeText(content)
  }

  return (
    <div className="space-y-6">
      {/* 订单基本信息 */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>{t.order.orderInfo}</CardTitle>
            <div className="flex items-center gap-2">
              <OrderStatusBadge status={order.status} />
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
            </div>
          </div>
        </CardHeader>

        <CardContent>
          <dl className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 text-sm">
            <div>
              <dt className="text-muted-foreground">{t.order.orderNo}</dt>
              <dd className="font-mono font-medium">
                {order.orderNo || order.order_no}
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
                <dd className="font-medium">
                  {order.userEmail || order.user_email}
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
                {formatCurrency(order.totalAmount || order.total_amount, order.currency)}
              </dd>
            </div>
            {(order.formSubmittedAt || order.form_submitted_at) && (
              <div>
                <dt className="text-muted-foreground">{t.order.formSubmittedAt}</dt>
                <dd>
                  {formatDate(order.formSubmittedAt || order.form_submitted_at || '')}
                </dd>
              </div>
            )}
            {(order.shippedAt || order.shipped_at) && (
              <div>
                <dt className="text-muted-foreground">{t.order.shippedAt}</dt>
                <dd>{formatDate(order.shippedAt || order.shipped_at || '')}</dd>
              </div>
            )}
          </dl>
        </CardContent>
      </Card>

      {/* 商品信息与收货物流信息/虚拟产品内容 - 在宽屏上并排显示 */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* 商品信息 */}
        <Card className="lg:col-span-1">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Package className="h-5 w-5" />
              {t.order.productInfo}
            </CardTitle>
          </CardHeader>

          <CardContent>
            <div className="space-y-4">
              {order.items.map((item, index) => (
                <div key={index}>
                  {index > 0 && <Separator className="my-4" />}

                  <div className="flex gap-4">
                    {/* 商品图片 */}
                    <div className="w-20 h-20 rounded overflow-hidden bg-muted flex-shrink-0">
                      {item.imageUrl || item.image_url ? (
                        <img
                          src={item.imageUrl || item.image_url}
                          alt={item.name}
                          className="w-full h-full object-cover"
                          onError={(e) => {
                            e.currentTarget.style.display = 'none'
                            e.currentTarget.parentElement?.querySelector('.img-fallback')?.classList.remove('hidden')
                          }}
                        />
                      ) : null}
                      <div className={`img-fallback w-full h-full flex items-center justify-center ${(item.imageUrl || item.image_url) ? 'hidden' : ''}`}>
                        <Package className="w-10 h-10 text-muted-foreground" />
                      </div>
                    </div>

                    {/* 商品信息 */}
                    <div className="flex-1 min-w-0">
                      <h4 className="font-medium truncate">{item.name}</h4>
                      <p className="text-sm text-muted-foreground truncate">SKU: {item.sku}</p>

                      {/* 商品属性 */}
                      {item.attributes && Object.keys(item.attributes).length > 0 && (
                        <div className="flex flex-wrap gap-2 mt-1">
                          {Object.entries(item.attributes).map(([key, value]) => (
                            <Badge key={key} variant="secondary" className="text-xs">
                              {key}: {value as string}
                            </Badge>
                          ))}
                        </div>
                      )}
                    </div>

                    {/* 数量 */}
                    <div className="text-right flex-shrink-0">
                      <p className="font-medium">x{item.quantity}</p>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* 付款方式卡片（待付款时显示在商品信息右侧） */}
        {paymentCard && (
          <div className="lg:col-span-1">
            {paymentCard}
          </div>
        )}

        {/* 虚拟产品卡密 - 与商品信息并排显示 */}
        {virtualStocks && virtualStocks.length > 0 && (
          <Card className="lg:col-span-1">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Key className="h-5 w-5" />
                {t.order.virtualProductContent}
              </CardTitle>
            </CardHeader>

            <CardContent>
              <div className="space-y-3">
                <div className="bg-green-50 dark:bg-green-950 border border-green-200 dark:border-green-800 rounded-lg p-3 text-sm text-green-800 dark:text-green-200">
                  <p className="font-medium mb-1">{t.order.virtualProductDelivered}</p>
                  <p>{t.order.virtualProductKeepSafe}</p>
                </div>

                {virtualStocks.map((stock) => (
                  <div key={stock.id} className="border rounded-lg p-3 md:p-4 space-y-2">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="flex items-center gap-1 flex-wrap min-w-0">
                        <code className="bg-muted px-2 py-1 md:px-3 md:py-2 rounded font-mono text-sm md:text-lg break-all">
                          {showContent[stock.id] ? stock.content : '••••••••••••'}
                        </code>
                        <div className="flex items-center shrink-0">
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-7 w-7 md:h-8 md:w-8 p-0"
                            onClick={() => toggleContentVisibility(stock.id)}
                          >
                            {showContent[stock.id] ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-7 w-7 md:h-8 md:w-8 p-0"
                            onClick={() => {
                              copyToClipboard(stock.content)
                              toast.success(t.order.copiedToClipboard)
                            }}
                          >
                            <Copy className="w-4 h-4" />
                          </Button>
                        </div>
                      </div>
                    </div>
                    {stock.remark && (
                      <p className="text-sm text-muted-foreground">{stock.remark}</p>
                    )}
                    {stock.delivered_at && (
                      <div className="text-xs text-muted-foreground">
                        {t.order.deliveryTime}: {formatDate(stock.delivered_at)}
                      </div>
                    )}
                  </div>
                ))}

                <div className="text-xs text-muted-foreground mt-2">
                  {t.order.totalItemsCount.replace('{count}', String(virtualStocks.length))}
                </div>
              </div>
            </CardContent>
          </Card>
        )}

        {/* 收货与物流信息 - 虚拟商品订单和待付款订单不显示 */}
        {!isVirtualOnly && order.status !== 'pending_payment' && (
          (order.receiverName || order.receiver_name || order.trackingNo || order.tracking_no) ? (
          <Card className="lg:col-span-1">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Truck className="h-5 w-5" />
                {t.order.shippingInfo} & {t.order.trackingInfo}
              </CardTitle>
            </CardHeader>

            <CardContent className="space-y-6">
              {/* 收货信息部分 */}
              {(order.receiverName || order.receiver_name) && (
                <div>
                  <h4 className="font-semibold text-sm mb-3 flex items-center gap-2">
                    <MapPin className="h-4 w-4" />
                    {t.order.shippingInfo}
                  </h4>
                  <dl className="space-y-3 text-sm pl-6">
                    <div className="flex flex-col sm:flex-row">
                      <dt className="sm:w-28 text-muted-foreground flex-shrink-0">{t.order.receiverName}</dt>
                      <dd className="font-medium">
                        {order.receiverName || order.receiver_name}
                      </dd>
                    </div>
                    <div className="flex flex-col sm:flex-row">
                      <dt className="sm:w-28 text-muted-foreground flex-shrink-0">{t.order.receiverPhone}</dt>
                      <dd className="font-medium">
                        {order.receiverPhone || order.receiver_phone}
                      </dd>
                    </div>
                    <div className="flex flex-col sm:flex-row">
                      <dt className="sm:w-28 text-muted-foreground flex-shrink-0">{t.order.receiverEmail}</dt>
                      <dd className="font-medium break-words">
                        {order.receiverEmail || order.receiver_email}
                      </dd>
                    </div>
                    <div className="flex flex-col sm:flex-row">
                      <dt className="sm:w-28 text-muted-foreground flex-shrink-0">{t.order.receiverAddress}</dt>
                      <dd className="font-medium break-words">
                        {order.receiverProvince || order.receiver_province}{' '}
                        {order.receiverCity || order.receiver_city}{' '}
                        {order.receiverDistrict || order.receiver_district}{' '}
                        {order.receiverAddress || order.receiver_address}
                        {(order.receiverPostcode || order.receiver_postcode) &&
                          ` (${order.receiverPostcode || order.receiver_postcode})`}
                      </dd>
                    </div>
                  </dl>
                </div>
              )}

              {/* 分隔线 - 只在两者都存在时显示 */}
              {(order.receiverName || order.receiver_name) && (order.trackingNo || order.tracking_no) && (
                <Separator />
              )}

              {/* 物流信息部分 */}
              {(order.trackingNo || order.tracking_no) && (
                <div>
                  <h4 className="font-semibold text-sm mb-3 flex items-center gap-2">
                    <Truck className="h-4 w-4" />
                    {t.order.trackingInfo}
                  </h4>
                  <dl className="space-y-3 text-sm pl-6">
                    <div className="flex flex-col sm:flex-row">
                      <dt className="sm:w-28 text-muted-foreground flex-shrink-0">{t.order.trackingNo}</dt>
                      <dd className="font-mono font-medium break-all">
                        {order.trackingNo || order.tracking_no}
                      </dd>
                    </div>
                    {(order.shippedAt || order.shipped_at) && (
                      <div className="flex flex-col sm:flex-row">
                        <dt className="sm:w-28 text-muted-foreground flex-shrink-0">{t.order.shippedAt}</dt>
                        <dd>{formatDate(order.shippedAt || order.shipped_at || '')}</dd>
                      </div>
                    )}
                  </dl>
                </div>
              )}
            </CardContent>
          </Card>
        ) : shippingForm ? (
          <Card className="lg:col-span-1">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <MapPin className="h-5 w-5" />
                {t.order.shippingInfo}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {shippingForm}
            </CardContent>
          </Card>
        ) : (
          <Card className="border-dashed lg:col-span-1">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Truck className="h-5 w-5" />
                {t.order.shippingInfo}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-center py-8 text-muted-foreground">
                <MapPin className="w-12 h-12 mx-auto mb-3 opacity-50" />
                <p className="mb-1 font-medium">{t.order.shippingNotFilled}</p>
                <p className="text-sm">
                  {isDraft
                    ? t.order.shippingNotFilledDesc
                    : isNeedResubmit
                      ? t.order.shippingResubmitDesc
                      : t.order.noShippingInfo}
                </p>
              </div>
            </CardContent>
          </Card>
        )
        )}
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
            <div className="bg-muted/50 p-4 rounded-md">
              <p className="text-sm whitespace-pre-wrap">{order.remark}</p>
            </div>
          </CardContent>
        </Card>
      )}

      {/* 产品序列号（管理员端） */}
      {serials && serials.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <ShieldCheck className="h-5 w-5" />
              {t.admin.antiCounterfeitSerial}
            </CardTitle>
          </CardHeader>

          <CardContent>
            <div className="space-y-3">
              <div className="bg-blue-50 dark:bg-blue-950 border border-blue-200 dark:border-blue-800 rounded-lg p-3 text-sm text-blue-800 dark:text-blue-200">
                <p className="font-medium mb-1">💡 {t.admin.shippingTip}</p>
                <p>{t.admin.shippingTipContent}</p>
              </div>

              {serials.map((serial, index) => (
                <div key={serial.id} className="border rounded-lg p-4 space-y-2">
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-xs text-muted-foreground mb-1">
                        {serial.product?.name || t.admin.productFallback} (SKU: {serial.product?.sku})
                      </div>
                      <div className="font-mono font-bold text-xl">
                        {serial.serial_number}
                      </div>
                    </div>
                    <Badge variant="outline" className="text-xs">
                      {t.admin.itemIndex.replace('{index}', String(serial.sequence_number))}
                    </Badge>
                  </div>
                  <div className="flex gap-2 text-xs text-muted-foreground">
                    <span>{t.admin.productCodeLabel2}: <span className="font-mono font-semibold">{serial.product_code}</span></span>
                    <span>•</span>
                    <span>{t.admin.antiCounterfeitCodeLabel}: <span className="font-mono font-semibold">{serial.anti_counterfeit_code}</span></span>
                    <span>•</span>
                    <span>{t.admin.viewCountLabel}: {serial.view_count}</span>
                  </div>
                </div>
              ))}

              <div className="text-xs text-muted-foreground mt-2">
                {t.admin.serialSummary.replace('{count}', String(serials.length))}
              </div>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
