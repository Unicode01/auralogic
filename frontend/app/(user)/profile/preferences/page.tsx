'use client'

import { useCallback, useEffect, useMemo, useState } from 'react'
import Link from 'next/link'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ArrowLeft,
  Bell,
  Globe,
  Mail,
  MessageSquare,
  Monitor,
  Moon,
  Sun,
  type LucideIcon,
} from 'lucide-react'
import { useAuth } from '@/hooks/use-auth'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { useToast } from '@/hooks/use-toast'
import { useTheme, type Theme } from '@/contexts/theme-context'
import { getPublicConfig, updateUserPreferences } from '@/lib/api'
import { getTranslations } from '@/lib/i18n'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { PluginSlotBatchBoundary } from '@/lib/plugin-slot-batch'

type NotificationPrefs = {
  email_notify_order: boolean
  email_notify_ticket: boolean
  email_notify_marketing: boolean
  sms_notify_marketing: boolean
}

type ThemeOption = {
  value: Theme
  label: string
  description: string
  icon: LucideIcon
}

type LanguageOption = {
  value: 'zh' | 'en'
  label: string
}

const defaultNotificationPrefs: NotificationPrefs = {
  email_notify_order: true,
  email_notify_ticket: true,
  email_notify_marketing: false,
  sms_notify_marketing: false,
}

export default function PreferencesPage() {
  const { user } = useAuth()
  const { locale, setLocale } = useLocale()
  const { theme, setTheme } = useTheme()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.profilePreferences)
  const isGuest = !user

  const toast = useToast()
  const queryClient = useQueryClient()
  const [notificationPrefs, setNotificationPrefs] =
    useState<NotificationPrefs>(defaultNotificationPrefs)

  const { data: publicConfig } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
  })

  const smtpEnabled = Boolean(publicConfig?.data?.smtp_enabled)
  const smsEnabled = Boolean(publicConfig?.data?.sms_enabled)
  const hasServiceConfig = typeof publicConfig !== 'undefined'
  const emailServiceStatus = hasServiceConfig
    ? smtpEnabled
      ? t.profile.serviceStatusEnabled
      : t.profile.serviceStatusDisabled
    : t.profile.unknown
  const smsServiceStatus = hasServiceConfig
    ? smsEnabled
      ? t.profile.serviceStatusEnabled
      : t.profile.serviceStatusDisabled
    : t.profile.unknown

  const languageOptions: LanguageOption[] = useMemo(
    () => [
      { value: 'zh', label: t.language.zh },
      { value: 'en', label: t.language.en },
    ],
    [t.language.zh, t.language.en]
  )

  const themeOptions: ThemeOption[] = useMemo(
    () => [
      {
        value: 'light',
        label: t.theme.light,
        description: t.profile.themeLightDesc,
        icon: Sun,
      },
      {
        value: 'dark',
        label: t.theme.dark,
        description: t.profile.themeDarkDesc,
        icon: Moon,
      },
      {
        value: 'system',
        label: t.theme.system,
        description: t.profile.themeSystemDesc,
        icon: Monitor,
      },
    ],
    [
      t.theme.light,
      t.theme.dark,
      t.theme.system,
      t.profile.themeLightDesc,
      t.profile.themeDarkDesc,
      t.profile.themeSystemDesc,
    ]
  )

  const currentLanguageLabel =
    languageOptions.find((option) => option.value === locale)?.label || locale
  const currentThemeLabel = themeOptions.find((option) => option.value === theme)?.label || theme
  const persistedNotificationPrefs = useMemo<NotificationPrefs>(
    () => ({
      email_notify_order: user?.email_notify_order ?? true,
      email_notify_ticket: user?.email_notify_ticket ?? true,
      email_notify_marketing: user?.email_notify_marketing ?? false,
      sms_notify_marketing: user?.sms_notify_marketing ?? false,
    }),
    [
      user?.email_notify_marketing,
      user?.email_notify_order,
      user?.email_notify_ticket,
      user?.sms_notify_marketing,
    ]
  )
  const notificationPrefsDirty =
    !isGuest &&
    (notificationPrefs.email_notify_order !== persistedNotificationPrefs.email_notify_order ||
      notificationPrefs.email_notify_ticket !== persistedNotificationPrefs.email_notify_ticket ||
      notificationPrefs.email_notify_marketing !==
        persistedNotificationPrefs.email_notify_marketing ||
      notificationPrefs.sms_notify_marketing !== persistedNotificationPrefs.sms_notify_marketing)

  useEffect(() => {
    if (!user) return
    const nextPrefs: NotificationPrefs = {
      email_notify_order: user.email_notify_order ?? true,
      email_notify_ticket: user.email_notify_ticket ?? true,
      email_notify_marketing: user.email_notify_marketing ?? false,
      sms_notify_marketing: user.sms_notify_marketing ?? false,
    }
    setNotificationPrefs((prev) => {
      if (
        prev.email_notify_order === nextPrefs.email_notify_order &&
        prev.email_notify_ticket === nextPrefs.email_notify_ticket &&
        prev.email_notify_marketing === nextPrefs.email_notify_marketing &&
        prev.sms_notify_marketing === nextPrefs.sms_notify_marketing
      ) {
        return prev
      }
      return nextPrefs
    })
  }, [
    user,
    user?.id,
    user?.email_notify_order,
    user?.email_notify_ticket,
    user?.email_notify_marketing,
    user?.sms_notify_marketing,
  ])

  const saveNotificationPrefsMutation = useMutation({
    mutationFn: (payload: NotificationPrefs) => updateUserPreferences(payload),
    onSuccess: () => {
      toast.success(t.profile.notificationSaveSuccess)
      queryClient.invalidateQueries({ queryKey: ['currentUser'] })
    },
    onError: (error: any) => {
      toast.error(resolveApiErrorMessage(error, t, t.profile.notificationSaveFailed))
    },
  })

  const handleSaveNotificationPrefs = useCallback(() => {
    saveNotificationPrefsMutation.mutate(notificationPrefs)
  }, [saveNotificationPrefsMutation, notificationPrefs])

  const backHref = isGuest ? '/products' : '/profile'
  const userProfilePreferencesPluginContext = {
    view: 'user_profile_preferences',
    summary: {
      is_guest: isGuest,
      locale,
      theme,
      smtp_enabled: smtpEnabled,
      sms_enabled: smsEnabled,
    },
    notifications: notificationPrefs,
    state: {
      is_guest: isGuest,
      notification_saving: saveNotificationPrefsMutation.isPending,
      notification_dirty: notificationPrefsDirty,
      smtp_available: smtpEnabled,
      sms_available: smsEnabled,
    },
  }
  const preferenceBatchItems = useMemo(
    () => [
      {
        slot: 'user.profile.preferences.top',
        hostContext: userProfilePreferencesPluginContext,
      },
      {
        slot: 'user.profile.preferences.display.after',
        hostContext: { ...userProfilePreferencesPluginContext, section: 'display' },
      },
      {
        slot: 'user.profile.preferences.notifications.top',
        hostContext: { ...userProfilePreferencesPluginContext, section: 'notifications' },
      },
      {
        slot: 'user.profile.preferences.notifications.save.before',
        hostContext: { ...userProfilePreferencesPluginContext, section: 'notifications' },
      },
    ],
    [userProfilePreferencesPluginContext]
  )

  return (
    <PluginSlotBatchBoundary
      scope="public"
      path="/profile/preferences"
      items={preferenceBatchItems}
    >
      <div className="space-y-6">
        <PluginSlot
          slot="user.profile.preferences.top"
          context={userProfilePreferencesPluginContext}
        />
        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div className="space-y-2">
            <div className="flex items-center gap-4">
              <Button asChild variant="outline" size="icon" className="md:hidden">
                <Link href={backHref}>
                  <ArrowLeft className="h-5 w-5" />
                  <span className="sr-only">
                    {isGuest ? t.sidebar.productCenter : t.profile.profileCenter}
                  </span>
                </Link>
              </Button>
              <h1 className="text-2xl font-bold md:text-3xl">{t.sidebar.preferences}</h1>
            </div>
            {isGuest ? (
              <p className="text-sm text-muted-foreground">{t.profile.notificationLoginHint}</p>
            ) : null}
          </div>
          {isGuest && (
            <Button asChild variant="outline" size="sm">
              <Link href="/login">{t.auth.loginToContinue}</Link>
            </Button>
          )}
        </div>

        <div className={cn('grid gap-6', !isGuest && 'xl:grid-cols-[1fr_1.4fr]')}>
          <Card className="h-fit">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Globe className="h-5 w-5" />
                {t.profile.displayPreferences}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <p className="text-sm font-medium">{t.profile.languagePreference}</p>
                  <span className="text-sm text-muted-foreground">{currentLanguageLabel}</span>
                </div>
                <p className="text-xs text-muted-foreground">{t.profile.languagePreferenceDesc}</p>
                <div className="grid grid-cols-2 gap-2">
                  {languageOptions.map((option) => (
                    <Button
                      key={option.value}
                      variant={locale === option.value ? 'default' : 'outline'}
                      onClick={() => setLocale(option.value)}
                      className="h-10"
                      aria-pressed={locale === option.value}
                    >
                      {option.label}
                    </Button>
                  ))}
                </div>
              </div>

              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <p className="text-sm font-medium">{t.profile.themePreference}</p>
                  <span className="text-sm text-muted-foreground">{currentThemeLabel}</span>
                </div>
                <p className="text-xs text-muted-foreground">{t.profile.themePreferenceDesc}</p>
                <div className="space-y-2">
                  {themeOptions.map((option) => {
                    const Icon = option.icon
                    return (
                      <button
                        key={option.value}
                        type="button"
                        onClick={() => setTheme(option.value)}
                        aria-pressed={theme === option.value}
                        className={cn(
                          'w-full rounded-lg border p-3 text-left transition-colors',
                          theme === option.value ? 'border-primary bg-primary/5' : 'hover:bg-accent'
                        )}
                      >
                        <div className="flex items-start gap-3">
                          <Icon className="mt-0.5 h-4 w-4 text-muted-foreground" />
                          <div className="space-y-0.5">
                            <div className="text-sm font-medium">{option.label}</div>
                            <div className="text-xs text-muted-foreground">
                              {option.description}
                            </div>
                          </div>
                        </div>
                      </button>
                    )
                  })}
                </div>
              </div>
              <PluginSlot
                slot="user.profile.preferences.display.after"
                context={{ ...userProfilePreferencesPluginContext, section: 'display' }}
              />
            </CardContent>
          </Card>

          {!isGuest && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Bell className="h-5 w-5" />
                  {t.profile.notificationSettings}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <PluginSlot
                  slot="user.profile.preferences.notifications.top"
                  context={{ ...userProfilePreferencesPluginContext, section: 'notifications' }}
                />
                <div className="space-y-3 rounded-xl border p-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Mail className="h-4 w-4 text-muted-foreground" />
                      <p className="text-sm font-medium">{t.profile.email}</p>
                    </div>
                    <span className="text-sm text-muted-foreground">{emailServiceStatus}</span>
                  </div>
                  {hasServiceConfig && !smtpEnabled && (
                    <p className="text-xs text-muted-foreground">
                      {t.profile.emailServiceUnavailableHint}
                    </p>
                  )}
                  <NotificationSwitchRow
                    label={t.profile.emailOrderNotifications}
                    checked={notificationPrefs.email_notify_order}
                    onCheckedChange={(checked) =>
                      setNotificationPrefs((prev) => ({ ...prev, email_notify_order: checked }))
                    }
                    disabled={!smtpEnabled}
                    enabledLabel={t.common.yes}
                    disabledLabel={t.common.no}
                  />
                  <NotificationSwitchRow
                    label={t.profile.emailTicketNotifications}
                    checked={notificationPrefs.email_notify_ticket}
                    onCheckedChange={(checked) =>
                      setNotificationPrefs((prev) => ({ ...prev, email_notify_ticket: checked }))
                    }
                    disabled={!smtpEnabled}
                    enabledLabel={t.common.yes}
                    disabledLabel={t.common.no}
                  />
                  <NotificationSwitchRow
                    label={t.profile.emailMarketingNotifications}
                    checked={notificationPrefs.email_notify_marketing}
                    onCheckedChange={(checked) =>
                      setNotificationPrefs((prev) => ({ ...prev, email_notify_marketing: checked }))
                    }
                    disabled={!smtpEnabled}
                    enabledLabel={t.common.yes}
                    disabledLabel={t.common.no}
                  />
                </div>

                <div className="space-y-3 rounded-xl border p-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <MessageSquare className="h-4 w-4 text-muted-foreground" />
                      <p className="text-sm font-medium">SMS</p>
                    </div>
                    <span className="text-sm text-muted-foreground">{smsServiceStatus}</span>
                  </div>
                  {hasServiceConfig && !smsEnabled && (
                    <p className="text-xs text-muted-foreground">
                      {t.profile.smsServiceUnavailableHint}
                    </p>
                  )}
                  <NotificationSwitchRow
                    label={t.profile.smsMarketingNotifications}
                    checked={notificationPrefs.sms_notify_marketing}
                    onCheckedChange={(checked) =>
                      setNotificationPrefs((prev) => ({ ...prev, sms_notify_marketing: checked }))
                    }
                    disabled={!smsEnabled}
                    enabledLabel={t.common.yes}
                    disabledLabel={t.common.no}
                  />
                </div>

                <PluginSlot
                  slot="user.profile.preferences.notifications.save.before"
                  context={{ ...userProfilePreferencesPluginContext, section: 'notifications' }}
                />
                <div className="flex justify-end pt-1">
                  <Button
                    onClick={handleSaveNotificationPrefs}
                    disabled={saveNotificationPrefsMutation.isPending}
                  >
                    {saveNotificationPrefsMutation.isPending
                      ? t.profile.notificationSaving
                      : t.profile.notificationSave}
                  </Button>
                </div>
              </CardContent>
            </Card>
          )}
        </div>
        <PluginSlot
          slot="user.profile.preferences.bottom"
          context={userProfilePreferencesPluginContext}
        />
      </div>
    </PluginSlotBatchBoundary>
  )
}

interface NotificationSwitchRowProps {
  label: string
  checked: boolean
  onCheckedChange: (checked: boolean) => void
  disabled: boolean
  enabledLabel: string
  disabledLabel: string
}

function NotificationSwitchRow({
  label,
  checked,
  onCheckedChange,
  disabled,
  enabledLabel,
  disabledLabel,
}: NotificationSwitchRowProps) {
  return (
    <div className="flex items-center justify-between rounded-lg border px-3 py-2.5">
      <div className="space-y-0.5">
        <p className="text-sm">{label}</p>
        <p className="text-xs text-muted-foreground">{checked ? enabledLabel : disabledLabel}</p>
      </div>
      <Switch
        checked={checked}
        onCheckedChange={onCheckedChange}
        disabled={disabled}
        aria-label={label}
      />
    </div>
  )
}
