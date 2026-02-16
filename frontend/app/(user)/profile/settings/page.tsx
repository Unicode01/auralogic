'use client'

import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useAuth } from '@/hooks/use-auth'
import { Card, CardHeader, CardTitle, CardContent, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
  FormDescription,
} from '@/components/ui/form'
import { Separator } from '@/components/ui/separator'
import { changePasswordSchema } from '@/lib/validators'
import { changePassword } from '@/lib/api'
import { useToast } from '@/hooks/use-toast'
import { Key, User, ArrowLeft } from 'lucide-react'
import * as z from 'zod'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import Link from 'next/link'

export default function SettingsPage() {
  const { user } = useAuth()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.accountSettings)
  const toast = useToast()
  const [isChangingPassword, setIsChangingPassword] = useState(false)

  const passwordForm = useForm({
    resolver: zodResolver(changePasswordSchema),
    defaultValues: {
      old_password: '',
      new_password: '',
      confirm_password: '',
    },
  })

  async function onPasswordSubmit(values: z.infer<typeof changePasswordSchema>) {
    setIsChangingPassword(true)
    try {
      await changePassword(values.old_password, values.new_password)
      toast.success(t.profile.passwordChangeSuccess)
      passwordForm.reset()
    } catch (error: any) {
      toast.error(error.message || t.profile.passwordChangeFailed)
    } finally {
      setIsChangingPassword(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button asChild variant="ghost" size="icon" className="md:hidden">
          <Link href="/profile">
            <ArrowLeft className="h-5 w-5" />
          </Link>
        </Button>
        <h1 className="text-2xl md:text-3xl font-bold">{t.profile.accountSettings}</h1>
      </div>

      {/* 账户信息 */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <User className="h-5 w-5" />
            {t.profile.accountInfo}
          </CardTitle>
          <CardDescription>{t.profile.accountInfoReadonly}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <label className="text-sm font-medium">{t.profile.email}</label>
            <Input value={user?.email} disabled className="mt-2" />
          </div>
          <div>
            <label className="text-sm font-medium">{t.profile.name}</label>
            <Input value={user?.name || ''} disabled className="mt-2" />
          </div>
        </CardContent>
      </Card>

      <Separator />

      {/* 修改密码 */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Key className="h-5 w-5" />
            {t.profile.changePassword}
          </CardTitle>
          <CardDescription>
            {locale === 'zh' ? '修改您的登录密码，建议使用强密码' : 'Change your login password, recommend using a strong password'}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Form {...passwordForm}>
            <form
              onSubmit={passwordForm.handleSubmit(onPasswordSubmit)}
              className="space-y-4"
            >
              <FormField
                control={passwordForm.control}
                name="old_password"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t.profile.currentPassword}</FormLabel>
                    <FormControl>
                      <Input 
                        type="password" 
                        placeholder={t.profile.currentPasswordPlaceholder}
                        {...field} 
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={passwordForm.control}
                name="new_password"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t.profile.newPassword}</FormLabel>
                    <FormControl>
                      <Input 
                        type="password" 
                        placeholder={t.profile.newPasswordPlaceholder}
                        {...field} 
                      />
                    </FormControl>
                    <FormDescription>{t.profile.passwordRequirement}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={passwordForm.control}
                name="confirm_password"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t.profile.confirmPassword}</FormLabel>
                    <FormControl>
                      <Input 
                        type="password" 
                        placeholder={t.profile.confirmNewPasswordPlaceholder}
                        {...field} 
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <Button type="submit" disabled={isChangingPassword}>
                {isChangingPassword ? t.profile.changing : t.profile.changePassword}
              </Button>
            </form>
          </Form>
        </CardContent>
      </Card>

      {/* 账户安全提示 */}
      <Card className="border-yellow-500/30 bg-yellow-500/10">
        <CardHeader>
          <CardTitle className="text-base">{t.profile.securityTips}</CardTitle>
        </CardHeader>
        <CardContent className="text-sm space-y-2">
          <p>{t.profile.securityTip1}</p>
          <p>{t.profile.securityTip2}</p>
          <p>{t.profile.securityTip3}</p>
          <p>{t.profile.securityTip4}</p>
        </CardContent>
      </Card>
    </div>
  )
}

