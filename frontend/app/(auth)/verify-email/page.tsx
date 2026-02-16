'use client'

import { Suspense, useEffect, useState } from 'react'
import { useSearchParams, useRouter } from 'next/navigation'
import { useMutation } from '@tanstack/react-query'
import { verifyEmail, resendVerification } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Loader2, Mail, CheckCircle2, XCircle, RefreshCw } from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { setToken, setUser } from '@/lib/auth'
import toast from 'react-hot-toast'

export default function VerifyEmailPage() {
  return (
    <Suspense fallback={
      <div className="min-h-screen flex items-center justify-center p-6 bg-background">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    }>
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

  const verifyMutation = useMutation({
    mutationFn: verifyEmail,
    onSuccess: (data: any) => {
      setStatus('success')
      if (data.data?.token) {
        setToken(data.data.token)
        setUser(data.data.user)
        setTimeout(() => router.push('/orders'), 2000)
      } else {
        setTimeout(() => router.push('/login'), 3000)
      }
    },
    onError: () => {
      setStatus('error')
    },
  })

  const resendMutation = useMutation({
    mutationFn: resendVerification,
    onSuccess: () => {
      toast.success(t.auth.verificationResent)
    },
    onError: () => {
      toast.error(t.auth.resendFailed)
    },
  })

  useEffect(() => {
    if (token && !pending) {
      setStatus('verifying')
      verifyMutation.mutate(token)
    } else if (pending) {
      setStatus('pending')
    }
  }, [token, pending])

  return (
    <div className="min-h-screen flex items-center justify-center p-6 bg-background">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <div className="mx-auto mb-4 w-16 h-16 rounded-full bg-primary/10 flex items-center justify-center">
            {status === 'verifying' && <Loader2 className="h-8 w-8 text-primary animate-spin" />}
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
            {status === 'error' && t.auth.verifyFailedDesc}
            {status === 'pending' && email && (
              (t.auth.verifyPendingDesc as string).replace('{email}', email)
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {status === 'error' && (
            <div className="space-y-2">
              <Button
                variant="outline"
                className="w-full"
                onClick={() => router.push('/login')}
              >
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
              <p className="text-sm text-muted-foreground text-center">
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
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
