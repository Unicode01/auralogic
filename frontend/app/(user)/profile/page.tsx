'use client'

import { useAuth } from '@/hooks/use-auth'
import { useQuery } from '@tanstack/react-query'
import { getPublicConfig } from '@/lib/api'
import { Card, CardHeader, CardTitle, CardContent, CardDescription } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import {
  User,
  Mail,
  Shield,
  Calendar,
  Settings,
  Package,
  LogOut,
  Globe,
  ChevronRight,
  Key,
  ShieldCheck,
  MessageSquare,
  BookOpen,
  Megaphone,
  Sun,
  Moon,
  Monitor
} from 'lucide-react'
import Link from 'next/link'
import { formatDate } from '@/lib/utils'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { useIsMobile } from '@/hooks/use-mobile'
import { useTheme, Theme } from '@/contexts/theme-context'

// 角色颜色
const roleColors: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
  user: 'secondary',
  admin: 'default',
  super_admin: 'destructive',
}

export default function ProfilePage() {
  const { user } = useAuth()
  const { locale, setLocale } = useLocale()
  const { isMobile } = useIsMobile()
  const { theme, setTheme } = useTheme()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.profile)

  const { data: publicConfigData } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
    staleTime: 5 * 60 * 1000,
  })
  const ticketEnabled = publicConfigData?.data?.ticket?.enabled ?? true

  // 角色翻译
  const roleLabels: Record<string, string> = {
    user: t.profile.roleUser,
    admin: t.profile.roleAdmin,
    super_admin: t.profile.roleSuperAdmin,
  }

  const handleLogout = () => {
    if (typeof window !== 'undefined') {
      localStorage.removeItem('auth_token')
      window.location.href = '/login'
    }
  }

  const toggleLanguage = () => {
    setLocale(locale === 'zh' ? 'en' : 'zh')
  }

  const cycleTheme = () => {
    const themes: Theme[] = ['light', 'dark', 'system']
    const currentIndex = themes.indexOf(theme)
    const nextIndex = (currentIndex + 1) % themes.length
    setTheme(themes[nextIndex])
  }

  const getThemeLabel = () => {
    switch (theme) {
      case 'light': return t.theme?.light || '浅色'
      case 'dark': return t.theme?.dark || '深色'
      case 'system': return t.theme?.system || '跟随系统'
      default: return theme
    }
  }

  const getThemeIcon = () => {
    switch (theme) {
      case 'light': return <Sun className="h-5 w-5 text-muted-foreground" />
      case 'dark': return <Moon className="h-5 w-5 text-muted-foreground" />
      case 'system': return <Monitor className="h-5 w-5 text-muted-foreground" />
      default: return <Sun className="h-5 w-5 text-muted-foreground" />
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl md:text-3xl font-bold">{t.profile.profileCenter}</h1>
        {!isMobile && (
          <Button asChild variant="outline">
            <Link href="/profile/settings">
              <Settings className="mr-2 h-4 w-4" />
              {t.common.edit}
            </Link>
          </Button>
        )}
      </div>

      {/* 基本信息卡片 */}
      <Card>
        <CardHeader>
          <CardTitle>{t.profile.basicInfo}</CardTitle>
          <CardDescription>{t.profile.accountBasicInfo}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-start gap-3">
            <User className="h-5 w-5 text-muted-foreground mt-0.5" />
            <div className="flex-1">
              <dt className="text-sm text-muted-foreground mb-1">{t.profile.name}</dt>
              <dd className="font-medium">{user?.name || t.profile.notSet}</dd>
            </div>
          </div>

          <div className="flex items-start gap-3">
            <Mail className="h-5 w-5 text-muted-foreground mt-0.5" />
            <div className="flex-1">
              <dt className="text-sm text-muted-foreground mb-1">{t.profile.email}</dt>
              <dd className="font-medium">{user?.email}</dd>
            </div>
          </div>

          <div className="flex items-start gap-3">
            <Shield className="h-5 w-5 text-muted-foreground mt-0.5" />
            <div className="flex-1">
              <dt className="text-sm text-muted-foreground mb-1">{t.profile.role}</dt>
              <dd className="flex items-center gap-2">
                <Badge variant={roleColors[user?.role] || 'secondary'}>
                  {roleLabels[user?.role] || user?.role}
                </Badge>
              </dd>
            </div>
          </div>

          <div className="flex items-start gap-3">
            <Calendar className="h-5 w-5 text-muted-foreground mt-0.5" />
            <div className="flex-1">
              <dt className="text-sm text-muted-foreground mb-1">{t.profile.registerTime}</dt>
              <dd className="font-medium">
                {user?.createdAt || user?.created_at
                  ? formatDate(user.createdAt || user.created_at)
                  : t.profile.unknown}
              </dd>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 设置选项 - 移动端显示更多选项 */}
      <Card>
        <CardHeader>
          <CardTitle>{isMobile ? t.profile.quickActions : t.profile.quickActions}</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <div className="divide-y">
            {/* 我的订单 */}
            <Link
              href="/orders"
              className="flex items-center justify-between p-4 hover:bg-accent transition-colors"
            >
              <div className="flex items-center gap-3">
                <Package className="h-5 w-5 text-muted-foreground" />
                <span>{t.sidebar.myOrders}</span>
              </div>
              <ChevronRight className="h-5 w-5 text-muted-foreground" />
            </Link>

            {/* 序列号验证 */}
            <Link
              href="/serial-verify"
              className="flex items-center justify-between p-4 hover:bg-accent transition-colors"
            >
              <div className="flex items-center gap-3">
                <ShieldCheck className="h-5 w-5 text-muted-foreground" />
                <span>{t.sidebar.serialVerify}</span>
              </div>
              <ChevronRight className="h-5 w-5 text-muted-foreground" />
            </Link>

            {/* 客服中心 */}
            {ticketEnabled && (
            <Link
              href="/tickets"
              className="flex items-center justify-between p-4 hover:bg-accent transition-colors"
            >
              <div className="flex items-center gap-3">
                <MessageSquare className="h-5 w-5 text-muted-foreground" />
                <span>{t.sidebar.supportCenter || '客服中心'}</span>
              </div>
              <ChevronRight className="h-5 w-5 text-muted-foreground" />
            </Link>
            )}

            {/* 知识库 */}
            <Link
              href="/knowledge"
              className="flex items-center justify-between p-4 hover:bg-accent transition-colors"
            >
              <div className="flex items-center gap-3">
                <BookOpen className="h-5 w-5 text-muted-foreground" />
                <span>{t.sidebar.knowledgeBase || '知识库'}</span>
              </div>
              <ChevronRight className="h-5 w-5 text-muted-foreground" />
            </Link>

            {/* 公告 */}
            <Link
              href="/announcements"
              className="flex items-center justify-between p-4 hover:bg-accent transition-colors"
            >
              <div className="flex items-center gap-3">
                <Megaphone className="h-5 w-5 text-muted-foreground" />
                <span>{t.sidebar.announcements || '公告'}</span>
              </div>
              <ChevronRight className="h-5 w-5 text-muted-foreground" />
            </Link>

            {/* 账户设置 */}
            <Link
              href="/profile/settings"
              className="flex items-center justify-between p-4 hover:bg-accent transition-colors"
            >
              <div className="flex items-center gap-3">
                <Settings className="h-5 w-5 text-muted-foreground" />
                <span>{t.profile.accountSettings}</span>
              </div>
              <ChevronRight className="h-5 w-5 text-muted-foreground" />
            </Link>

            {/* 语言切换 */}
            <button
              onClick={toggleLanguage}
              className="flex items-center justify-between p-4 hover:bg-accent transition-colors w-full text-left"
            >
              <div className="flex items-center gap-3">
                <Globe className="h-5 w-5 text-muted-foreground" />
                <span>{t.sidebar.language}</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-sm text-muted-foreground">
                  {locale === 'zh' ? '中文' : 'English'}
                </span>
                <ChevronRight className="h-5 w-5 text-muted-foreground" />
              </div>
            </button>

            {/* 主题切换 */}
            <button
              onClick={cycleTheme}
              className="flex items-center justify-between p-4 hover:bg-accent transition-colors w-full text-left"
            >
              <div className="flex items-center gap-3">
                {getThemeIcon()}
                <span>{t.sidebar?.theme || '主题'}</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-sm text-muted-foreground">
                  {getThemeLabel()}
                </span>
                <ChevronRight className="h-5 w-5 text-muted-foreground" />
              </div>
            </button>

            {/* 管理后台入口 - 仅管理员可见 */}
            {(user?.role === 'admin' || user?.role === 'super_admin') && (
              <Link
                href="/admin/dashboard"
                className="flex items-center justify-between p-4 hover:bg-accent transition-colors"
              >
                <div className="flex items-center gap-3">
                  <Shield className="h-5 w-5 text-muted-foreground" />
                  <span>{t.sidebar.adminPanel}</span>
                </div>
                <ChevronRight className="h-5 w-5 text-muted-foreground" />
              </Link>
            )}
          </div>
        </CardContent>
      </Card>

      {/* 退出登录 */}
      <Card className="border-destructive/20">
        <CardContent className="p-0">
          <button
            onClick={handleLogout}
            className="flex items-center justify-between p-4 hover:bg-destructive/5 transition-colors w-full text-left text-destructive"
          >
            <div className="flex items-center gap-3">
              <LogOut className="h-5 w-5" />
              <span>{t.auth.logout}</span>
            </div>
          </button>
        </CardContent>
      </Card>

    </div>
  )
}
