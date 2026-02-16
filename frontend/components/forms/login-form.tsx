'use client'

import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import * as z from 'zod'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from '@/components/ui/form'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { useAuth } from '@/hooks/use-auth'
import { createLoginSchema, loginSchema } from '@/lib/validators'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { useQuery } from '@tanstack/react-query'
import { getPublicConfig, getCaptcha } from '@/lib/api'
import { useState, useEffect, useRef } from 'react'

declare global {
  interface Window {
    turnstile?: any
    grecaptcha?: any
    onTurnstileLoad?: () => void
    onRecaptchaLoad?: () => void
  }
}

export function LoginForm() {
  const { login, isLoggingIn } = useAuth()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const [captchaToken, setCaptchaToken] = useState('')
  const [builtinCode, setBuiltinCode] = useState('')
  const captchaContainerRef = useRef<HTMLDivElement>(null)
  const widgetRendered = useRef(false)

  const { data: publicConfig } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
  })

  const captchaConfig = publicConfig?.data?.captcha
  const needCaptcha = captchaConfig?.provider && captchaConfig.provider !== 'none' && captchaConfig.enable_for_login

  const { data: builtinCaptcha, refetch: refetchCaptcha } = useQuery({
    queryKey: ['captcha', 'login'],
    queryFn: getCaptcha,
    enabled: needCaptcha && captchaConfig?.provider === 'builtin',
  })

  // Load Turnstile/reCAPTCHA scripts
  useEffect(() => {
    if (!needCaptcha) return

    if (captchaConfig.provider === 'cloudflare' && !document.getElementById('cf-turnstile-script')) {
      const script = document.createElement('script')
      script.id = 'cf-turnstile-script'
      script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?onload=onTurnstileLoad'
      script.async = true
      window.onTurnstileLoad = () => {
        if (captchaContainerRef.current && !widgetRendered.current) {
          widgetRendered.current = true
          window.turnstile.render(captchaContainerRef.current, {
            sitekey: captchaConfig.site_key,
            callback: (token: string) => setCaptchaToken(token),
          })
        }
      }
      document.head.appendChild(script)
    } else if (captchaConfig.provider === 'google' && !document.getElementById('recaptcha-script')) {
      const script = document.createElement('script')
      script.id = 'recaptcha-script'
      script.src = 'https://www.google.com/recaptcha/api.js?onload=onRecaptchaLoad&render=explicit'
      script.async = true
      window.onRecaptchaLoad = () => {
        if (captchaContainerRef.current && !widgetRendered.current) {
          widgetRendered.current = true
          window.grecaptcha.render(captchaContainerRef.current, {
            sitekey: captchaConfig.site_key,
            callback: (token: string) => setCaptchaToken(token),
          })
        }
      }
      document.head.appendChild(script)
    }
  }, [needCaptcha, captchaConfig])

  // Render widget if script already loaded
  useEffect(() => {
    if (!needCaptcha || widgetRendered.current || !captchaContainerRef.current) return

    if (captchaConfig.provider === 'cloudflare' && window.turnstile) {
      widgetRendered.current = true
      window.turnstile.render(captchaContainerRef.current, {
        sitekey: captchaConfig.site_key,
        callback: (token: string) => setCaptchaToken(token),
      })
    } else if (captchaConfig.provider === 'google' && window.grecaptcha?.render) {
      widgetRendered.current = true
      window.grecaptcha.render(captchaContainerRef.current, {
        sitekey: captchaConfig.site_key,
        callback: (token: string) => setCaptchaToken(token),
      })
    }
  }, [needCaptcha, captchaConfig])

  const schema = createLoginSchema({
    invalidEmail: t.auth.invalidEmail,
    passwordMin6: (t.auth.passwordMinLength as string).replace('{n}', '6'),
  })

  const form = useForm({
    resolver: zodResolver(schema),
    defaultValues: {
      email: '',
      password: '',
    },
  })

  function onSubmit(values: z.infer<typeof loginSchema>) {
    let token = captchaToken
    if (needCaptcha && captchaConfig.provider === 'builtin') {
      token = `${builtinCaptcha?.data?.captcha_id}:${builtinCode}`
    }
    login({ ...values, captcha_token: token || undefined })
  }

  return (
    <Card className="w-full max-w-md">
      <CardHeader>
        <CardTitle className="text-2xl text-center">{t.auth.login}</CardTitle>
      </CardHeader>

      <CardContent>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
            <FormField
              control={form.control}
              name="email"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t.auth.email}</FormLabel>
                  <FormControl>
                    <Input
                      type="email"
                      placeholder={t.auth.emailPlaceholder}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="password"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t.auth.password}</FormLabel>
                  <FormControl>
                    <Input
                      type="password"
                      placeholder={t.auth.passwordPlaceholder}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Captcha */}
            {needCaptcha && (
              <div className="space-y-2">
                {(captchaConfig.provider === 'cloudflare' || captchaConfig.provider === 'google') && (
                  <div ref={captchaContainerRef} />
                )}
                {captchaConfig.provider === 'builtin' && builtinCaptcha?.data && (
                  <>
                    <label className="text-sm font-medium">{t.auth.captcha}</label>
                    <div className="flex items-center gap-2">
                      <Input
                        placeholder={t.auth.captchaPlaceholder}
                        value={builtinCode}
                        onChange={(e) => setBuiltinCode(e.target.value)}
                        maxLength={4}
                        className="h-11"
                      />
                      <img
                        src={builtinCaptcha.data.image}
                        alt="captcha"
                        className="border border-border rounded-md h-11 shrink-0 cursor-pointer dark:brightness-90"
                        onClick={() => { refetchCaptcha(); setBuiltinCode('') }}
                        title={t.auth.captchaRefresh}
                      />
                    </div>
                  </>
                )}
              </div>
            )}

            <Button type="submit" className="w-full" disabled={isLoggingIn}>
              {isLoggingIn
                ? t.auth.loggingIn
                : t.auth.login}
            </Button>
          </form>
        </Form>
      </CardContent>
    </Card>
  )
}
