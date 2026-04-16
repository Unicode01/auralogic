'use client'

import { Suspense, useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import * as z from 'zod'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from '@/components/ui/form'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { Loader2, Lock, ArrowLeft, CheckCircle2, Eye, EyeOff } from 'lucide-react'
import Link from 'next/link'
import { useRouter, useSearchParams } from 'next/navigation'
import { resetPassword } from '@/lib/api'
import { resolveAuthApiErrorMessage } from '@/lib/api-error'
import toast from 'react-hot-toast'
import { AuthBrandingPanel, AuthMobileBrand } from '@/components/auth-branding-panel'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { PageLoading } from '@/components/ui/page-loading'

export default function ResetPasswordPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center bg-background">
          <PageLoading />
        </div>
      }
    >
      <ResetPasswordContent />
    </Suspense>
  )
}

function ResetPasswordContent() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.resetPassword)
  const router = useRouter()
  const searchParams = useSearchParams()
  const token = searchParams.get('token') || ''
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [resetCompleted, setResetCompleted] = useState(false)
  const [redirectCountdown, setRedirectCountdown] = useState(3)
  const [showPassword, setShowPassword] = useState(false)
  const [showConfirmPassword, setShowConfirmPassword] = useState(false)
  const hasToken = Boolean(token)
  const authResetPasswordPluginContext = {
    view: 'auth_reset_password',
    state: {
      branch: resetCompleted ? 'success' : hasToken ? 'form' : 'invalid_token',
      has_token: hasToken,
      reset_completed: resetCompleted,
      redirect_countdown: resetCompleted ? redirectCountdown : undefined,
      is_submitting: isSubmitting,
    },
  }
  useEffect(() => {
    if (!resetCompleted) return
    if (redirectCountdown <= 0) {
      router.replace('/login')
      return
    }
    const timer = window.setTimeout(() => {
      setRedirectCountdown((current) => current - 1)
    }, 1000)
    return () => window.clearTimeout(timer)
  }, [redirectCountdown, resetCompleted, router])

  const schema = z
    .object({
      password: z.string().min(8, (t.auth.passwordMinLength as string).replace('{n}', '8')),
      confirm_password: z.string(),
    })
    .refine((data) => data.password === data.confirm_password, {
      message: t.auth.passwordMismatch,
      path: ['confirm_password'],
    })

  const form = useForm({
    resolver: zodResolver(schema),
    defaultValues: { password: '', confirm_password: '' },
  })

  async function onSubmit(values: { password: string; confirm_password: string }) {
    if (!token) {
      toast.error(t.auth.resetTokenInvalid)
      return
    }
    setIsSubmitting(true)
    try {
      await resetPassword({ token, new_password: values.password })
      toast.success(t.auth.passwordResetSuccess)
      setResetCompleted(true)
      setRedirectCountdown(3)
    } catch (e: any) {
      toast.error(resolveAuthApiErrorMessage(e, t, t.auth.requestFailed))
    } finally {
      setIsSubmitting(false)
    }
  }

  if (resetCompleted) {
    return (
      <div className="flex min-h-screen">
        <AuthBrandingPanel />
        <div className="flex flex-1 items-center justify-center bg-background p-6 sm:p-12">
          <div className="w-full max-w-sm space-y-4">
            <PluginSlot
              slot="auth.reset_password.top"
              context={authResetPasswordPluginContext}
            />
            <Card>
              <CardHeader className="text-center">
                <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-green-500/10">
                  <CheckCircle2 className="h-8 w-8 text-green-600 dark:text-green-400" />
                </div>
                <CardTitle>{t.auth.passwordResetSuccess}</CardTitle>
                <CardDescription>{t.auth.passwordResetSuccessDesc}</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="rounded-xl border bg-muted/20 p-3 text-center text-sm text-muted-foreground">
                  {(t.auth.redirectingToLoginIn as string).replace(
                    '{n}',
                    String(redirectCountdown)
                  )}
                </div>
                <PluginSlot
                  slot="auth.reset_password.success.after"
                  context={{ ...authResetPasswordPluginContext, section: 'success' }}
                />
                <Button className="w-full" onClick={() => router.replace('/login')}>
                  {t.auth.backToLogin}
                </Button>
              </CardContent>
            </Card>
            <PluginSlot
              slot="auth.reset_password.bottom"
              context={authResetPasswordPluginContext}
            />
            <PluginSlot
              slot="auth.reset_password.footer.before"
              context={{ ...authResetPasswordPluginContext, section: 'footer' }}
            />
          </div>
        </div>
      </div>
    )
  }

  if (!token) {
    return (
      <div className="flex min-h-screen">
        <AuthBrandingPanel />
        <div className="flex flex-1 items-center justify-center bg-background p-6 sm:p-12">
          <div className="w-full max-w-sm space-y-4">
            <PluginSlot
              slot="auth.reset_password.top"
              context={authResetPasswordPluginContext}
            />
            <Card>
              <CardHeader className="text-center">
                <CardTitle>{t.auth.resetTokenInvalid}</CardTitle>
                <CardDescription>{t.auth.verifyMissingLinkDesc}</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <PluginSlot
                  slot="auth.reset_password.invalid_token.after"
                  context={{ ...authResetPasswordPluginContext, section: 'invalid_token' }}
                />
                <Button asChild variant="outline" className="w-full">
                  <Link href="/forgot-password">{t.auth.requestNewResetLink}</Link>
                </Button>
                <Button asChild className="w-full">
                  <Link href="/login">{t.auth.backToLogin}</Link>
                </Button>
              </CardContent>
            </Card>
            <PluginSlot
              slot="auth.reset_password.bottom"
              context={authResetPasswordPluginContext}
            />
            <PluginSlot
              slot="auth.reset_password.footer.before"
              context={{ ...authResetPasswordPluginContext, section: 'footer' }}
            />
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen">
      <AuthBrandingPanel />
      <div className="flex flex-1 items-center justify-center bg-background p-6 sm:p-12">
        <div className="w-full max-w-sm space-y-6 sm:space-y-8">
          <AuthMobileBrand />

          <PluginSlot slot="auth.reset_password.top" context={authResetPasswordPluginContext} />

          <div className="space-y-2">
            <h2 className="text-2xl font-semibold tracking-tight text-foreground">
              {t.auth.resetPasswordTitle}
            </h2>
            <p className="text-sm text-muted-foreground">{t.auth.resetPasswordDesc}</p>
          </div>
          <p className="text-sm text-muted-foreground">{t.auth.resetPasswordNextStep}</p>

          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-5">
              <PluginSlot
                slot="auth.reset_password.form.top"
                context={{ ...authResetPasswordPluginContext, section: 'form' }}
              />
              <FormField
                control={form.control}
                name="password"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <FormLabel className="text-sm font-medium">{t.auth.newPassword}</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <Lock className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                        <Input
                          type={showPassword ? 'text' : 'password'}
                          placeholder={t.auth.passwordPlaceholder}
                          className="h-11 pl-10 pr-10"
                          {...field}
                        />
                        <button
                          type="button"
                          onClick={() => setShowPassword((value) => !value)}
                          className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
                          aria-label={showPassword ? t.auth.hidePassword : t.auth.showPassword}
                          title={showPassword ? t.auth.hidePassword : t.auth.showPassword}
                        >
                          {showPassword ? (
                            <EyeOff className="h-4 w-4" />
                          ) : (
                            <Eye className="h-4 w-4" />
                          )}
                        </button>
                      </div>
                    </FormControl>
                    <p className="text-xs text-muted-foreground">{t.profile.passwordRequirement}</p>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="confirm_password"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <FormLabel className="text-sm font-medium">
                      {t.auth.confirmNewPassword}
                    </FormLabel>
                    <FormControl>
                      <div className="relative">
                        <Lock className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                        <Input
                          type={showConfirmPassword ? 'text' : 'password'}
                          placeholder={t.auth.confirmPasswordPlaceholder}
                          className="h-11 pl-10 pr-10"
                          {...field}
                        />
                        <button
                          type="button"
                          onClick={() => setShowConfirmPassword((value) => !value)}
                          className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
                          aria-label={
                            showConfirmPassword ? t.auth.hidePassword : t.auth.showPassword
                          }
                          title={showConfirmPassword ? t.auth.hidePassword : t.auth.showPassword}
                        >
                          {showConfirmPassword ? (
                            <EyeOff className="h-4 w-4" />
                          ) : (
                            <Eye className="h-4 w-4" />
                          )}
                        </button>
                      </div>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <PluginSlot
                slot="auth.reset_password.submit.before"
                context={{ ...authResetPasswordPluginContext, section: 'form' }}
              />
              <Button
                type="submit"
                className="h-11 w-full text-sm font-medium"
                disabled={isSubmitting}
              >
                {isSubmitting ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    {t.auth.resetPassword}
                  </>
                ) : (
                  t.auth.resetPassword
                )}
              </Button>
              <PluginSlot
                slot="auth.reset_password.form.after"
                context={{ ...authResetPasswordPluginContext, section: 'form' }}
              />
            </form>
          </Form>

          <PluginSlot
            slot="auth.reset_password.bottom"
            context={authResetPasswordPluginContext}
          />

          <PluginSlot
            slot="auth.reset_password.footer.before"
            context={{ ...authResetPasswordPluginContext, section: 'footer' }}
          />
          <p className="text-center text-xs text-muted-foreground">
            <Link
              href="/login"
              className="inline-flex items-center gap-1 text-primary hover:underline"
            >
              <ArrowLeft className="h-3 w-3" />
              {t.auth.backToLogin}
            </Link>
          </p>
        </div>
      </div>
    </div>
  )
}
