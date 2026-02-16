import { Skeleton } from '@/components/ui/page-loading'

export default function Loading() {
  return (
    <div className="space-y-6">
      {/* 返回按钮和标题 */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Skeleton className="h-9 w-24" />
          <Skeleton className="h-9 w-32" />
        </div>
        <Skeleton className="h-10 w-32" />
      </div>

      {/* 订单详情卡片 */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* 左侧订单信息 */}
        <div className="lg:col-span-2 space-y-6">
          {/* 订单状态卡片 */}
          <div className="rounded-lg border bg-card p-6 space-y-4">
            <div className="flex items-center justify-between">
              <Skeleton className="h-6 w-24" />
              <Skeleton className="h-6 w-20" />
            </div>
            <Skeleton className="h-px w-full" />
            <div className="space-y-3">
              <div className="flex justify-between">
                <Skeleton className="h-4 w-20" />
                <Skeleton className="h-4 w-40" />
              </div>
              <div className="flex justify-between">
                <Skeleton className="h-4 w-20" />
                <Skeleton className="h-4 w-32" />
              </div>
            </div>
          </div>

          {/* 商品列表卡片 */}
          <div className="rounded-lg border bg-card p-6 space-y-4">
            <Skeleton className="h-6 w-24" />
            <Skeleton className="h-px w-full" />
            {Array.from({ length: 2 }).map((_, i) => (
              <div key={i} className="flex gap-4 py-4">
                <Skeleton className="h-20 w-20 rounded" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-5 w-48" />
                  <Skeleton className="h-4 w-32" />
                  <Skeleton className="h-4 w-24" />
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* 右侧信息 */}
        <div className="space-y-6">
          {/* 收货信息 */}
          <div className="rounded-lg border bg-card p-6 space-y-4">
            <Skeleton className="h-6 w-24" />
            <Skeleton className="h-px w-full" />
            <div className="space-y-3">
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
            </div>
          </div>

          {/* 价格信息 */}
          <div className="rounded-lg border bg-card p-6 space-y-4">
            <Skeleton className="h-6 w-24" />
            <Skeleton className="h-px w-full" />
            <div className="space-y-3">
              <div className="flex justify-between">
                <Skeleton className="h-4 w-16" />
                <Skeleton className="h-4 w-20" />
              </div>
              <div className="flex justify-between">
                <Skeleton className="h-4 w-16" />
                <Skeleton className="h-4 w-20" />
              </div>
              <Skeleton className="h-px w-full" />
              <div className="flex justify-between">
                <Skeleton className="h-5 w-16" />
                <Skeleton className="h-5 w-24" />
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
