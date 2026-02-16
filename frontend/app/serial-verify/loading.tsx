import { Skeleton } from '@/components/ui/page-loading'

export default function Loading() {
  return (
    <div className="container max-w-lg mx-auto py-8 space-y-6">
      {/* 标题 */}
      <div className="text-center space-y-2">
        <Skeleton className="h-8 w-48 mx-auto" />
        <Skeleton className="h-4 w-64 mx-auto" />
      </div>

      {/* 验证表单 */}
      <div className="rounded-lg border bg-card p-6 space-y-4">
        <div className="space-y-2">
          <Skeleton className="h-4 w-20" />
          <Skeleton className="h-10 w-full" />
        </div>
        <Skeleton className="h-10 w-full" />
      </div>
    </div>
  )
}
