'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { Card, CardHeader, CardContent, CardFooter } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { OrderStatusBadge } from './order-status-badge'
import type { Order } from '@/types/order'
import { formatDate, formatCurrency } from '@/lib/utils'
import { Package, Truck, Shield, FileEdit, AlertCircle, Loader2, Key, Headphones } from 'lucide-react'
import { getOrRefreshFormToken } from '@/lib/api'
import toast from 'react-hot-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'

interface OrderCardProps {
  order: Order
}

export function OrderCard({ order }: OrderCardProps) {
  const router = useRouter()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const orderNo = order.orderNo || order.order_no || ''
  const isDraft = order.status === 'draft'
  const isNeedResubmit = order.status === 'need_resubmit'

  // 判断是否为纯虚拟商品订单
  const isVirtualOnly = order.items.every(item =>
    (item.product_type || item.productType) === 'virtual'
  )

  // 纯虚拟商品订单不需要填写表单
  const needsFilling = (isDraft || isNeedResubmit) && !isVirtualOnly
  const [isGettingToken, setIsGettingToken] = useState(false)

  const handleFillForm = async (e: React.MouseEvent) => {
    e.preventDefault()
    setIsGettingToken(true)

    try {
      const response = await getOrRefreshFormToken(orderNo)
      const formToken = response.data.form_token

      if (formToken) {
        // 跳转到填表页
        router.push(`/form/shipping?token=${formToken}`)
      } else {
        toast.error(t.order.getFormTokenFailed)
      }
    } catch (error: any) {
      toast.error(error.message || t.order.getFormTokenFailed)
    } finally {
      setIsGettingToken(false)
    }
  }

  return (
    <Card className={`hover:shadow-lg transition-all flex flex-col h-full ${needsFilling ? 'border-amber-500/30 bg-amber-500/10' : ''}`}>
      <CardHeader className="pb-3">
        <div className="space-y-2">
          <div className="flex items-center justify-between gap-2">
            <OrderStatusBadge status={order.status} />
            <div className="flex items-center gap-1">
              {needsFilling && (
                <Badge
                  variant="outline"
                  className="text-amber-600 border-amber-300 bg-amber-50 dark:text-amber-300 dark:border-amber-800 dark:bg-amber-950/40"
                >
                  <AlertCircle className="h-3 w-3" />
                </Badge>
              )}
              {(order.privacyProtected || order.privacy_protected) && (
                <Badge variant="outline" className="flex items-center gap-1">
                  <Shield className="h-3 w-3" />
                </Badge>
              )}
              {(order.sharedToSupport || order.shared_to_support) && (
                <Badge variant="secondary" className="flex items-center gap-1" title={t.order.sharedToSupport}>
                  <Headphones className="h-3 w-3" />
                </Badge>
              )}
            </div>
          </div>
          <div>
            <h3 className="font-semibold text-sm truncate">{orderNo}</h3>
            <p className="text-xs text-muted-foreground">
              {formatDate(order.createdAt || order.created_at || '')}
            </p>
          </div>
        </div>
      </CardHeader>

      <CardContent className="space-y-3 flex-1">
        {/* 商品列表 */}
        <div className="space-y-2">
          {order.items.slice(0, 1).map((item, index) => (
            <div key={index} className="flex items-start gap-2">
              {/* 商品图片 */}
              {(item.imageUrl || item.image_url) ? (
                <div className="w-16 h-16 rounded overflow-hidden bg-muted flex-shrink-0">
                  <img
                    src={item.imageUrl || item.image_url || ''}
                    alt={item.name}
                    className="w-full h-full object-cover"
                    onError={(e) => {
                      e.currentTarget.style.display = 'none'
                      e.currentTarget.parentElement?.querySelector('.img-fallback')?.classList.remove('hidden')
                    }}
                  />
                  <div className="img-fallback w-full h-full flex items-center justify-center hidden">
                    <Package className="h-8 w-8 text-muted-foreground" />
                  </div>
                </div>
              ) : (
                <div className="w-16 h-16 rounded bg-muted flex items-center justify-center flex-shrink-0">
                  <Package className="h-8 w-8 text-muted-foreground" />
                </div>
              )}

              {/* 商品信息 */}
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium line-clamp-2 mb-1">{item.name}</p>
                {item.attributes && Object.keys(item.attributes).length > 0 && (
                  <div className="flex gap-1 flex-wrap mb-1">
                    {Object.entries(item.attributes).slice(0, 2).map(([key, value]) => (
                      <Badge key={key} variant="secondary" className="text-xs">
                        {key}: {value as string}
                      </Badge>
                    ))}
                  </div>
                )}
                <p className="text-xs text-muted-foreground">x{item.quantity}</p>
              </div>
            </div>
          ))}
          {order.items.length > 1 && (
            <p className="text-xs text-muted-foreground text-center py-1">
              {t.order.totalItemsCount.replace('{count}', String(order.items.length))}
            </p>
          )}
        </div>

        {/* 订单金额 */}
        <div className="flex items-center justify-between pt-2 border-t">
          <span className="text-sm text-muted-foreground">{t.order.amountLabel}</span>
          <span className="text-sm font-semibold text-foreground">
            {formatCurrency(order.totalAmount || order.total_amount, order.currency)}
          </span>
        </div>

        {/* 物流信息 */}
        {(order.trackingNo || order.tracking_no) && (
          <div className="flex items-center gap-2 pt-2 border-t">
            <Truck className="h-3.5 w-3.5 text-muted-foreground flex-shrink-0" />
            <span className="text-xs font-mono truncate">{order.trackingNo || order.tracking_no}</span>
          </div>
        )}

        {/* 虚拟商品已发货提示 */}
        {isVirtualOnly && order.status === 'shipped' && (
          <div className="flex items-start gap-2 p-2 border-t bg-green-500/10 rounded mt-auto">
            <Key className="h-3.5 w-3.5 text-green-600 dark:text-green-400 mt-0.5 flex-shrink-0" />
            <p className="text-xs text-green-700 dark:text-green-300 leading-tight">
              {t.order.virtualProductShipped}
            </p>
          </div>
        )}

        {/* 草稿提示（仅实物商品） */}
        {needsFilling && (
          <div className="flex items-start gap-2 p-2 border-t bg-amber-500/10 rounded mt-auto">
            <FileEdit className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0" />
            <p className="text-xs text-amber-700 dark:text-amber-300 leading-tight">
              {isDraft
                ? t.order.fillShippingPrompt
                : t.order.needResubmitShort}
            </p>
          </div>
        )}
      </CardContent>

      <CardFooter className="gap-2 pt-3">
        {needsFilling ? (
          <>
            <Button
              size="sm"
              className="flex-1"
              onClick={handleFillForm}
              disabled={isGettingToken}
            >
              {isGettingToken ? (
                <>
                  <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" />
                  {t.order.gettingTokenShort}
                </>
              ) : (
                <>
                  <FileEdit className="mr-1 h-3.5 w-3.5" />
                  {t.order.fillFormShort}
                </>
              )}
            </Button>
            <Button asChild size="sm" variant="outline">
              <Link href={`/orders/${orderNo}`}>{t.common.detail}</Link>
            </Button>
          </>
        ) : (
          <Button asChild size="sm" variant="outline" className="w-full">
            <Link href={`/orders/${orderNo}`}>{t.order.viewOrder}</Link>
          </Button>
        )}
      </CardFooter>
    </Card>
  )
}

