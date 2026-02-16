import { Skeleton } from '@/components/ui/page-loading'

export default function Loading() {
  return (
    <div className="min-h-screen flex items-center justify-center">
      <div className="w-full max-w-md p-8 space-y-6">
        {/* Logo */}
        <div className="text-center space-y-2">
          <Skeleton className="h-12 w-12 mx-auto rounded-lg" />
          <Skeleton className="h-8 w-32 mx-auto" />
        </div>

        {/* 表单 */}
        <div className="space-y-4">
          <div className="space-y-2">
            <Skeleton className="h-4 w-12" />
            <Skeleton className="h-10 w-full" />
          </div>
          <div className="space-y-2">
            <Skeleton className="h-4 w-12" />
            <Skeleton className="h-10 w-full" />
          </div>
          <Skeleton className="h-10 w-full" />
        </div>

        {/* 其他登录方式 */}
        <div className="space-y-3">
          <Skeleton className="h-4 w-32 mx-auto" />
          <div className="flex justify-center gap-3">
            <Skeleton className="h-10 w-10 rounded-full" />
            <Skeleton className="h-10 w-10 rounded-full" />
          </div>
        </div>
      </div>
    </div>
  )
}
