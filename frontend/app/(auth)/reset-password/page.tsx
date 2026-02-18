'use client'

import { useState, Suspense } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import * as z from 'zod'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Form, FormField, FormItem, FormLabel, FormControl, FormMessage,
} from '@/components/ui/form'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { Loader2, Lock, ArrowLeft } from 'lucide-react'
import Link from 'next/link'
import { useRouter, useSearchParams } from 'next/navigation'
import { resetPassword } from '@/lib/api'
import toast from 'react-hot-toast'
import { AuthBrandingPanel } from '@/components/auth-branding-panel'

export default function ResetPasswordPage() {
  return <Suspense><ResetPasswordContent /></Suspense>
}

function ResetPasswordContent() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.resetPassword)
  const router = useRouter()
  const searchParams = useSearchParams()
  const token = searchParams.get('token') || ''
  const [isSubmitting, setIsSubmitting] = useState(false)

  const schema = z.object({
    password: z.string().min(8, (t.auth.passwordMinLength as string).replace('{n}', '8')),
    confirm_password: z.string(),
  }).refine((data) => data.password === data.confirm_password, {
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
      router.push('/login')
    } catch (e: any) {
      const msg = e?.message || t.auth.requestFailed
      toast.error(msg === 'Reset token expired or invalid' ? t.auth.resetTokenExpired : msg)
    } finally {
      setIsSubmitting(false)
    }
  }

  if (!token) {
    return (
      <div className="min-h-screen flex">
        <AuthBrandingPanel />
        <div className="flex-1 flex items-center justify-center p-6 sm:p-12 bg-background">
          <div className="w-full max-w-sm space-y-6 text-center">
            <div className="lg:hidden">
              <h1 className="text-2xl font-bold text-foreground tracking-tight">AuraLogic</h1>
            </div>
            <p className="text-destructive">{t.auth.resetTokenInvalid}</p>
            <Link href="/login" className="text-primary hover:underline text-sm">
              {t.auth.backToLogin}
            </Link>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen flex">
      <AuthBrandingPanel />
      <div className="flex-1 flex items-center justify-center p-6 sm:p-12 bg-background">
        <div className="w-full max-w-sm space-y-6 sm:space-y-8">
          <div className="lg:hidden text-center">
            <h1 className="text-2xl font-bold text-foreground tracking-tight">AuraLogic</h1>
          </div>

          <div className="space-y-2">
            <h2 className="text-2xl font-semibold tracking-tight text-foreground">
              {t.auth.resetPasswordTitle}
            </h2>
            <p className="text-sm text-muted-foreground">
              {t.auth.resetPasswordDesc}
            </p>
          </div>

          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-5">
              <FormField
                control={form.control}
                name="password"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <FormLabel className="text-sm font-medium">{t.auth.newPassword}</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                        <Input type="password" placeholder={t.auth.passwordPlaceholder} className="pl-10 h-11" {...field} />
                      </div>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="confirm_password"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <FormLabel className="text-sm font-medium">{t.auth.confirmNewPassword}</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                        <Input type="password" placeholder={t.auth.confirmPasswordPlaceholder} className="pl-10 h-11" {...field} />
                      </div>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <Button type="submit" className="w-full h-11 text-sm font-medium" disabled={isSubmitting}>
                {isSubmitting ? (
                  <><Loader2 className="mr-2 h-4 w-4 animate-spin" />{t.auth.resetPassword}</>
                ) : t.auth.resetPassword}
              </Button>
            </form>
          </Form>

          <p className="text-center text-xs text-muted-foreground">
            <Link href="/login" className="text-primary hover:underline inline-flex items-center gap-1">
              <ArrowLeft className="h-3 w-3" />
              {t.auth.backToLogin}
            </Link>
          </p>
        </div>
      </div>
    </div>
  )
}
