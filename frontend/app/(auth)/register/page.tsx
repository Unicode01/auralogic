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
import { useAuth } from '@/hooks/use-auth'
import { createRegisterSchema, registerSchema } from '@/lib/validators'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { Loader2, Mail, Lock, ArrowRight, User } from 'lucide-react'
import Link from 'next/link'
import { useQuery } from '@tanstack/react-query'
import { getPublicConfig, getCaptcha } from '@/lib/api'
import { useRouter } from 'next/navigation'
import { useEffect, useState, useRef } from 'react'
import { useTheme } from '@/contexts/theme-context'

export default function RegisterPage() {
  const { register: registerUser, isRegistering } = useAuth()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.register)
  const router = useRouter()
  const [captchaToken, setCaptchaToken] = useState('')
  const [builtinCode, setBuiltinCode] = useState('')
  const { resolvedTheme } = useTheme()
  const captchaContainerRef = useRef<HTMLDivElement>(null)
  const widgetRendered = useRef(false)
  const widgetIdRef = useRef<any>(null)

  const { data: publicConfig } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
  })

  const allowRegistration = publicConfig?.data?.allow_registration
  const captchaConfig = publicConfig?.data?.captcha
  const needCaptcha = captchaConfig?.provider && captchaConfig.provider !== 'none' && captchaConfig.enable_for_register

  const { data: builtinCaptcha, refetch: refetchCaptcha } = useQuery({
    queryKey: ['captcha', 'register'],
    queryFn: getCaptcha,
    enabled: needCaptcha && captchaConfig?.provider === 'builtin',
  })

  // If registration is disabled, redirect to login
  useEffect(() => {
    if (publicConfig && !allowRegistration) {
      router.replace('/login')
    }
  }, [publicConfig, allowRegistration, router])

  // Load Turnstile/reCAPTCHA scripts
  useEffect(() => {
    if (!needCaptcha) return

    if (captchaConfig.provider === 'cloudflare' && !document.getElementById('cf-turnstile-script')) {
      const script = document.createElement('script')
      script.id = 'cf-turnstile-script'
      script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?onload=onTurnstileLoad'
      script.async = true
      ;(window as any).onTurnstileLoad = () => {
        if (captchaContainerRef.current && !widgetRendered.current) {
          widgetRendered.current = true
          widgetIdRef.current = (window as any).turnstile.render(captchaContainerRef.current, {
            sitekey: captchaConfig.site_key,
            theme: resolvedTheme === 'dark' ? 'dark' : 'light',
            callback: (token: string) => setCaptchaToken(token),
            'expired-callback': () => setCaptchaToken(''),
          })
        }
      }
      document.head.appendChild(script)
    } else if (captchaConfig.provider === 'google' && !document.getElementById('recaptcha-script')) {
      const script = document.createElement('script')
      script.id = 'recaptcha-script'
      script.src = 'https://www.google.com/recaptcha/api.js?onload=onRecaptchaLoad&render=explicit'
      script.async = true
      ;(window as any).onRecaptchaLoad = () => {
        if (captchaContainerRef.current && !widgetRendered.current) {
          widgetRendered.current = true
          widgetIdRef.current = (window as any).grecaptcha.render(captchaContainerRef.current, {
            sitekey: captchaConfig.site_key,
            theme: resolvedTheme === 'dark' ? 'dark' : 'light',
            callback: (token: string) => setCaptchaToken(token),
            'expired-callback': () => setCaptchaToken(''),
          })
        }
      }
      document.head.appendChild(script)
    }
  }, [needCaptcha, captchaConfig])

  // Render widget if script already loaded
  useEffect(() => {
    if (!needCaptcha || widgetRendered.current || !captchaContainerRef.current) return

    if (captchaConfig.provider === 'cloudflare' && (window as any).turnstile) {
      widgetRendered.current = true
      widgetIdRef.current = (window as any).turnstile.render(captchaContainerRef.current, {
        sitekey: captchaConfig.site_key,
        theme: resolvedTheme === 'dark' ? 'dark' : 'light',
        callback: (token: string) => setCaptchaToken(token),
        'expired-callback': () => setCaptchaToken(''),
      })
    } else if (captchaConfig.provider === 'google' && (window as any).grecaptcha?.render) {
      widgetRendered.current = true
      widgetIdRef.current = (window as any).grecaptcha.render(captchaContainerRef.current, {
        sitekey: captchaConfig.site_key,
        theme: resolvedTheme === 'dark' ? 'dark' : 'light',
        callback: (token: string) => setCaptchaToken(token),
        'expired-callback': () => setCaptchaToken(''),
      })
    }
  }, [needCaptcha, captchaConfig])

  const schema = createRegisterSchema({
    invalidEmail: t.auth.invalidEmail,
    passwordMin8: (t.auth.passwordMinLength as string).replace('{n}', '8'),
    nameMin2: (t.auth.nameMinLength as string).replace('{n}', '2'),
    passwordMismatch: t.auth.passwordMismatch,
  })

  const form = useForm({
    resolver: zodResolver(schema),
    defaultValues: {
      email: '',
      name: '',
      password: '',
      confirm_password: '',
    },
  })

  function resetCaptcha() {
    if (!needCaptcha) return
    if (captchaConfig.provider === 'builtin') {
      refetchCaptcha()
      setBuiltinCode('')
    } else if (captchaConfig.provider === 'cloudflare' && (window as any).turnstile) {
      ;(window as any).turnstile.reset(widgetIdRef.current)
      setCaptchaToken('')
    } else if (captchaConfig.provider === 'google' && (window as any).grecaptcha) {
      ;(window as any).grecaptcha.reset(widgetIdRef.current)
      setCaptchaToken('')
    }
  }

  function onSubmit(values: z.infer<typeof registerSchema>) {
    let token = captchaToken
    if (needCaptcha && captchaConfig.provider === 'builtin') {
      token = `${builtinCaptcha?.data?.captcha_id}:${builtinCode}`
    }
    registerUser({
      email: values.email,
      name: values.name,
      password: values.password,
      captcha_token: token || undefined,
    }, {
      onError: () => resetCaptcha(),
    })
  }

  if (publicConfig && !allowRegistration) {
    return null
  }

  return (
    <div className="min-h-screen flex">
      {/* Left branding panel - hidden on mobile */}
      <div className="hidden lg:flex lg:w-1/2 relative overflow-hidden bg-primary">
        {/* Background pattern */}
        <div className="absolute inset-0 opacity-10">
          <div className="absolute top-0 left-0 w-full h-full"
            style={{
              backgroundImage: `radial-gradient(circle at 25% 25%, rgba(255,255,255,0.2) 0%, transparent 50%),
                               radial-gradient(circle at 75% 75%, rgba(255,255,255,0.15) 0%, transparent 50%)`,
            }}
          />
          <div className="absolute -top-24 -left-24 w-96 h-96 rounded-full border border-white/20" />
          <div className="absolute -bottom-32 -right-32 w-[500px] h-[500px] rounded-full border border-white/10" />
          <div className="absolute top-1/2 left-1/4 w-64 h-64 rounded-full border border-white/10" />
        </div>

        {/* Content */}
        <div className="relative z-10 flex flex-col justify-between w-full p-12">
          <div>
            <h1 className="text-3xl font-bold text-primary-foreground tracking-tight">
              AuraLogic
            </h1>
          </div>

          <div className="space-y-6">
            <h2 className="text-4xl font-bold text-primary-foreground leading-tight">
              {locale === 'zh' ? '现代化电商\n管理平台' : 'Modern\nE-commerce\nPlatform'}
            </h2>
            <p className="text-primary-foreground/70 text-lg max-w-md leading-relaxed">
              {t.home.subtitle}
            </p>
          </div>

          <p className="text-primary-foreground/50 text-sm">
            {t.common.copyright}
          </p>
        </div>
      </div>

      {/* Right form panel */}
      <div className="flex-1 flex items-center justify-center p-6 sm:p-12 bg-background">
        <div className="w-full max-w-sm space-y-5 sm:space-y-8">
          {/* Mobile logo */}
          <div className="lg:hidden text-center">
            <h1 className="text-2xl font-bold text-foreground tracking-tight">
              AuraLogic
            </h1>
          </div>

          {/* Header */}
          <div className="space-y-2">
            <h2 className="text-2xl font-semibold tracking-tight text-foreground">
              {t.auth.welcomeRegister}
            </h2>
            <p className="text-sm text-muted-foreground">
              {t.auth.registerDescription}
            </p>
          </div>

          {/* Form */}
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4 sm:space-y-5">
              <FormField
                control={form.control}
                name="email"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <FormLabel className="text-sm font-medium">{t.auth.email}</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <Mail className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                        <Input
                          type="email"
                          placeholder={t.auth.emailPlaceholder}
                          className="pl-10 h-11"
                          {...field}
                        />
                      </div>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <FormLabel className="text-sm font-medium">{t.auth.name}</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <User className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                        <Input
                          type="text"
                          placeholder={t.auth.namePlaceholder}
                          className="pl-10 h-11"
                          {...field}
                        />
                      </div>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="password"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <FormLabel className="text-sm font-medium">{t.auth.password}</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                        <Input
                          type="password"
                          placeholder={t.auth.passwordPlaceholder}
                          className="pl-10 h-11"
                          {...field}
                        />
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
                    <FormLabel className="text-sm font-medium">{t.auth.confirmPassword}</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                        <Input
                          type="password"
                          placeholder={t.auth.confirmPasswordPlaceholder}
                          className="pl-10 h-11"
                          {...field}
                        />
                      </div>
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

              <Button
                type="submit"
                className="w-full h-11 text-sm font-medium"
                disabled={isRegistering}
              >
                {isRegistering ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    {t.auth.registering}
                  </>
                ) : (
                  <>
                    {t.auth.createAccount}
                    <ArrowRight className="ml-2 h-4 w-4" />
                  </>
                )}
              </Button>
            </form>
          </Form>

          {/* Footer */}
          <p className="text-center text-xs text-muted-foreground">
            {t.auth.hasAccount}{' '}
            <Link href="/login" className="text-primary hover:underline">
              {t.auth.login}
            </Link>
          </p>
        </div>
      </div>
    </div>
  )
}
