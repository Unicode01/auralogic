import { Skeleton } from '@/components/ui/page-loading'

export default function Loading() {
  return (
    <div className="space-y-6">
      {/* 返回按钮和标题 */}
      <div className="flex items-center gap-4">
        <Skeleton className="h-9 w-20" />
        <Skeleton className="h-8 w-32" />
      </div>

      {/* 表单卡片 */}
      <div className="rounded-lg border bg-card">
        <div className="p-6 space-y-1.5">
          <Skeleton className="h-6 w-32" />
          <Skeleton className="h-4 w-48" />
        </div>
        <div className="p-6 pt-0 space-y-6">
          {/* 昵称 */}
          <div className="space-y-2">
            <Skeleton className="h-4 w-12" />
            <Skeleton className="h-10 w-full" />
          </div>

          {/* 邮箱 */}
          <div className="space-y-2">
            <Skeleton className="h-4 w-12" />
            <Skeleton className="h-10 w-full" />
          </div>

          {/* 密码 */}
          <div className="space-y-2">
            <Skeleton className="h-4 w-16" />
            <Skeleton className="h-10 w-full" />
          </div>

          {/* 确认密码 */}
          <div className="space-y-2">
            <Skeleton className="h-4 w-20" />
            <Skeleton className="h-10 w-full" />
          </div>

          {/* 保存按钮 */}
          <Skeleton className="h-10 w-24" />
        </div>
      </div>
    </div>
  )
}
