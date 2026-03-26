'use client'

import { Suspense, useEffect, useRef, useState } from 'react'
import { useSearchParams, useRouter } from 'next/navigation'
import { useMutation } from '@tanstack/react-query'
import { verifyEmail, resendVerification } from '@/lib/api'
import { resolveAuthApiErrorMessage } from '@/lib/api-error'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Loader2, Mail, CheckCircle2, XCircle, RefreshCw } from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { setToken, setUser } from '@/lib/auth'
import toast from 'react-hot-toast'

export default function VerifyEmailPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center bg-background p-6">
          <Loader2 className="h-8 w-8 animate-spin text-primary" />
        </div>
      }
    >
      <VerifyEmailContent />
    </Suspense>
  )
}

function VerifyEmailContent() {
  const searchParams = useSearchParams()
  const router = useRouter()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.verifyEmail || 'Verify Email')

  const token = searchParams.get('token')
  const email = searchParams.get('email')
  const pending = searchParams.get('pending') === 'true'

  const [status, setStatus] = useState<'verifying' | 'success' | 'error' | 'pending'>('pending')
  const [errorMessage, setErrorMessage] = useState('')
  const [redirectTarget, setRedirectTarget] = useState<'orders' | 'login' | null>(null)
  const verifiedTokenRef = useRef<string | null>(null)
  const redirectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const verifyMutation = useMutation({
    mutationFn: verifyEmail,
    onSuccess: (data: any) => {
      setErrorMessage('')
      setStatus('success')
      const nextTarget = data.data?.token ? 'orders' : 'login'
      setRedirectTarget(nextTarget)
      if (data.data?.token) {
        setToken(data.data.token)
        setUser(data.data.user)
      } else {
        setRedirectTarget('login')
      }
      if (redirectTimerRef.current) {
        clearTimeout(redirectTimerRef.current)
      }
      redirectTimerRef.current = setTimeout(
        () => {
          router.push(nextTarget === 'orders' ? '/orders' : '/login')
        },
        nextTarget === 'orders' ? 2000 : 3000
      )
    },
    onError: (error) => {
      const message = resolveAuthApiErrorMessage(error, t, t.auth.verifyFailedDesc)
      setErrorMessage(message)
      setStatus('error')
      setRedirectTarget(null)
      toast.error(message)
    },
  })

  const resendMutation = useMutation({
    mutationFn: resendVerification,
    onSuccess: () => {
      toast.success(t.auth.verificationResent)
    },
    onError: (error) => {
      toast.error(resolveAuthApiErrorMessage(error, t, t.auth.resendFailed))
    },
  })

  useEffect(() => {
    if (pending) {
      setErrorMessage('')
      setRedirectTarget(null)
      setStatus('pending')
      return
    }

    if (!token) {
      setStatus('error')
      setRedirectTarget(null)
      setErrorMessage(t.auth.verifyMissingLinkDesc)
      return
    }

    if (verifiedTokenRef.current === token) {
      return
    }

    verifiedTokenRef.current = token
    if (token) {
      setErrorMessage('')
      setStatus('verifying')
      verifyMutation.mutate(token)
    }
  }, [pending, t.auth.verifyMissingLinkDesc, token, verifyMutation])

  useEffect(() => {
    return () => {
      if (redirectTimerRef.current) {
        clearTimeout(redirectTimerRef.current)
      }
    }
  }, [])

  const pendingDescription = email
    ? (t.auth.verifyPendingDesc as string).replace('{email}', email)
    : t.auth.verifyPendingGenericDesc
  const nextStepDescription =
    redirectTarget === 'orders' ? t.auth.autoRedirectOrders : t.auth.autoRedirectLogin
  const authVerifyEmailPluginContext = {
    view: 'auth_verify_email',
    verification: {
      status,
      pending,
      has_token: Boolean(token),
      email: email || undefined,
      redirect_target: redirectTarget || undefined,
    },
    state: {
      status,
      verifying: verifyMutation.isPending,
      resending: resendMutation.isPending,
      has_error: Boolean(errorMessage),
      can_continue: status === 'success',
      can_retry: status === 'error' && Boolean(token),
      can_resend: Boolean(email),
    },
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-6">
      <div className="w-full max-w-md space-y-4">
        <PluginSlot slot="auth.verify_email.top" context={authVerifyEmailPluginContext} />
        <Card className="w-full">
        <CardHeader className="text-center">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-primary/10">
            {status === 'verifying' && <Loader2 className="h-8 w-8 animate-spin text-primary" />}
            {status === 'success' && <CheckCircle2 className="h-8 w-8 text-green-500" />}
            {status === 'error' && <XCircle className="h-8 w-8 text-destructive" />}
            {status === 'pending' && <Mail className="h-8 w-8 text-primary" />}
          </div>
          <CardTitle>
            {status === 'verifying' && t.auth.verifying}
            {status === 'success' && t.auth.verifySuccess}
            {status === 'error' && t.auth.verifyFailed}
            {status === 'pending' && t.auth.verifyYourEmail}
          </CardTitle>
          <CardDescription>
            {status === 'verifying' && t.auth.verifyingDesc}
            {status === 'success' && t.auth.verifySuccessDesc}
            {status === 'error' && (errorMessage || t.auth.verifyFailedDesc)}
            {status === 'pending' && pendingDescription}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <PluginSlot
            slot="auth.verify_email.status.after"
            context={{ ...authVerifyEmailPluginContext, section: 'status' }}
          />
          {(email || (status === 'success' && redirectTarget)) && (
            <div className="grid gap-3 sm:grid-cols-2">
              {email && (
                <div className="rounded-xl border bg-muted/20 p-3">
                  <div className="text-xs text-muted-foreground">{t.auth.email}</div>
                  <div className="mt-1 break-all text-sm font-medium">
                    {(t.auth.sentTo as string).replace('{target}', email)}
                  </div>
                </div>
              )}
              {status === 'success' && redirectTarget && (
                <div className="rounded-xl border bg-muted/20 p-3">
                  <div className="text-xs text-muted-foreground">{t.auth.nextStep}</div>
                  <p className="mt-2 text-sm text-foreground">{nextStepDescription}</p>
                </div>
              )}
            </div>
          )}
          <PluginSlot
            slot="auth.verify_email.summary.after"
            context={{ ...authVerifyEmailPluginContext, section: 'summary' }}
          />
          {status === 'error' && errorMessage && errorMessage !== t.auth.verifyFailedDesc && (
            <p className="rounded-md border border-destructive/20 bg-destructive/5 px-3 py-2 text-sm text-destructive">
              {errorMessage}
            </p>
          )}
          <PluginSlot
            slot="auth.verify_email.actions.before"
            context={{ ...authVerifyEmailPluginContext, section: 'actions' }}
          />
          {status === 'success' && (
            <Button
              className="w-full"
              onClick={() => router.push(redirectTarget === 'orders' ? '/orders' : '/login')}
            >
              {redirectTarget === 'orders' ? t.auth.continueToOrders : t.auth.continueToLogin}
            </Button>
          )}
          {status === 'error' && (
            <div className="space-y-2">
              {token && (
                <Button
                  className="w-full"
                  onClick={() => {
                    setErrorMessage('')
                    setStatus('verifying')
                    verifyMutation.mutate(token)
                  }}
                  disabled={verifyMutation.isPending}
                >
                  {verifyMutation.isPending ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      {t.auth.verifying}
                    </>
                  ) : (
                    <>
                      <RefreshCw className="mr-2 h-4 w-4" />
                      {t.auth.retryVerification}
                    </>
                  )}
                </Button>
              )}
              <Button variant="outline" className="w-full" onClick={() => router.push('/login')}>
                {t.auth.backToLogin}
              </Button>
              {email && (
                <Button
                  variant="default"
                  className="w-full"
                  onClick={() => resendMutation.mutate(email)}
                  disabled={resendMutation.isPending}
                >
                  {resendMutation.isPending ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      {t.auth.sending}
                    </>
                  ) : (
                    <>
                      <RefreshCw className="mr-2 h-4 w-4" />
                      {t.auth.resendVerification}
                    </>
                  )}
                </Button>
              )}
            </div>
          )}
          {status === 'pending' && email && (
            <div className="space-y-2">
              <p className="text-center text-sm text-muted-foreground">
                {t.auth.didntReceiveEmail}
              </p>
              <Button
                variant="outline"
                className="w-full"
                onClick={() => resendMutation.mutate(email)}
                disabled={resendMutation.isPending}
              >
                {resendMutation.isPending ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    {t.auth.sending}
                  </>
                ) : (
                  <>
                    <RefreshCw className="mr-2 h-4 w-4" />
                    {t.auth.resend}
                  </>
                )}
              </Button>
              <Button variant="ghost" className="w-full" onClick={() => router.push('/login')}>
                {t.auth.backToLogin}
              </Button>
            </div>
          )}
          <PluginSlot
            slot="auth.verify_email.actions.after"
            context={{ ...authVerifyEmailPluginContext, section: 'actions' }}
          />
        </CardContent>
        </Card>
        <PluginSlot slot="auth.verify_email.bottom" context={authVerifyEmailPluginContext} />
        <PluginSlot
          slot="auth.verify_email.footer.before"
          context={{ ...authVerifyEmailPluginContext, section: 'footer' }}
        />
      </div>
    </div>
  )
}
