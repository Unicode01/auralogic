'use client'
/* eslint-disable @next/next/no-img-element */

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import * as z from 'zod'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { submitShippingForm, getCountries } from '@/lib/api'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { shippingFormSchema } from '@/lib/validators'
import toast from 'react-hot-toast'
import { Globe, Package } from 'lucide-react'
import { phoneCodeMap } from '@/lib/phone-codes'
import { getTranslations } from '@/lib/i18n'
import { PluginSlot } from '@/components/plugins/plugin-slot'

interface ShippingFormProps {
  formToken: string
  orderInfo: {
    orderNo: string
    order_no?: string
    items: any[]
    userEmail?: string
    user_email?: string
    userName?: string
    user_name?: string
  }
  lang?: string
  onSuccess?: () => void
  hideOrderItems?: boolean
  hidePassword?: boolean
  pluginSlotNamespace?: string
  pluginSlotContext?: Record<string, any>
}

export function ShippingForm({
  formToken,
  orderInfo,
  lang = 'zh',
  onSuccess,
  hideOrderItems = false,
  hidePassword = false,
  pluginSlotNamespace,
  pluginSlotContext,
}: ShippingFormProps) {
  const router = useRouter()
  const [currentLang, setCurrentLang] = useState(lang)
  const isEnglish = currentLang === 'en'
  const activeLocale = currentLang === 'en' ? 'en' : 'zh'
  const translations = getTranslations(activeLocale)
  const t = translations.shippingForm
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [countries, setCountries] = useState<any[]>([])

  // 从 localStorage 读取上次填写的收货信息
  const savedShipping = (() => {
    if (typeof window === 'undefined') return null
    try {
      const raw = localStorage.getItem('shipping_form_cache')
      return raw ? JSON.parse(raw) : null
    } catch { return null }
  })()

  const normalizeCountryCode = (value?: string, countryList?: any[]) => {
    const raw = String(value || '').trim()
    if (!raw) return 'CN'

    const upper = raw.toUpperCase()
    if (!countryList || countryList.length === 0) {
      return upper
    }

    const exact = countryList.find((c: any) => String(c.code || '').toUpperCase() === upper)
    if (exact) {
      return String(exact.code).toUpperCase()
    }

    const lower = raw.toLowerCase()
    const byName = countryList.find((c: any) => {
      const nameZh = String(c.name_zh || '').trim()
      const nameEn = String(c.name_en || '').trim().toLowerCase()
      return nameZh === raw || nameEn === lower
    })
    if (byName) {
      return String(byName.code).toUpperCase()
    }

    return 'CN'
  }

  const [selectedCountry, setSelectedCountry] = useState(normalizeCountryCode(savedShipping?.receiver_country))

  // 从订单信息中获取固定邮箱
  const fixedEmail = orderInfo.userEmail || orderInfo.user_email || ''
  // 从订单信息中获取默认用户名（可修改）
  const defaultUserName = orderInfo.userName || orderInfo.user_name || ''

  // 同步语言变化
  useEffect(() => {
    setCurrentLang(lang)
  }, [lang])

  // 获取国家列表
  useEffect(() => {
    getCountries()
      .then((response: any) => {
        setCountries(response.data || [])
      })
      .catch(err => {
        const currentTranslations = getTranslations(activeLocale)
        toast.error(resolveApiErrorMessage(err, currentTranslations, currentTranslations.common.failed))
        console.error('Failed to load countries:', err)
      })
  }, [activeLocale])

  // 为区号选择器生成选项列表
  const phoneCodeOptions = countries.map((country) => {
    const phoneCode = phoneCodeMap[country.code] || `+${country.code}`
    return {
      countryCode: country.code,
      phoneCode: phoneCode,
      countryName: isEnglish ? country.name_en : country.name_zh,
      // 使用唯一的value格式: countryCode:phoneCode
      uniqueValue: `${country.code}:${phoneCode}`
    }
  })

  // 从唯一值中提取区号的辅助函数
  const extractPhoneCode = (uniqueValue: string) => {
    const parts = uniqueValue.split(':')
    return parts.length > 1 ? parts[1] : uniqueValue
  }

  const form = useForm({
    resolver: zodResolver(shippingFormSchema),
    defaultValues: {
      receiver_name: defaultUserName || savedShipping?.receiver_name || '',
      phone_code: savedShipping?.phone_code || '+86',
      receiver_phone: savedShipping?.receiver_phone || '',
      receiver_email: fixedEmail,
      receiver_country: normalizeCountryCode(savedShipping?.receiver_country),
      receiver_province: savedShipping?.receiver_province || '',
      receiver_city: savedShipping?.receiver_city || '',
      receiver_district: savedShipping?.receiver_district || '',
      receiver_address: savedShipping?.receiver_address || '',
      receiver_postcode: savedShipping?.receiver_postcode || '',
      privacy_protected: savedShipping?.privacy_protected ?? false,
      password: '',
      user_remark: '',
    },
  })

  // 国家列表加载后，自动校正旧缓存中的国家值（如小写 code / 国家名称），避免选择框空白
  useEffect(() => {
    if (!countries.length) return
    const current = form.getValues('receiver_country')
    const normalized = normalizeCountryCode(current, countries)
    if (current !== normalized) {
      form.setValue('receiver_country', normalized, { shouldDirty: false, shouldTouch: false })
    }
    if (selectedCountry !== normalized) {
      setSelectedCountry(normalized)
    }
  }, [countries, form, selectedCountry])

  // 判断是否是中国（需要填写省市区）
  const isChina = selectedCountry === 'CN'
  const shippingFormPluginContext = pluginSlotNamespace
    ? {
        ...(pluginSlotContext || {}),
        shipping_form: {
          token_present: Boolean(formToken),
          is_submitting: isSubmitting,
          hide_order_items: hideOrderItems,
          hide_password: hidePassword,
          has_fixed_email: Boolean(fixedEmail),
          selected_country: selectedCountry || undefined,
        },
        state: {
          ...((pluginSlotContext && typeof pluginSlotContext.state === 'object'
            ? pluginSlotContext.state
            : {}) as Record<string, unknown>),
          ready: true,
          submitting: isSubmitting,
          is_china: isChina,
          has_fixed_email: Boolean(fixedEmail),
          hide_order_items: hideOrderItems,
          hide_password: hidePassword,
        },
      }
    : null
  const renderPluginSlot = (suffix: string, section: string) =>
    pluginSlotNamespace && shippingFormPluginContext ? (
      <PluginSlot
        slot={`${pluginSlotNamespace}.${suffix}`}
        context={{ ...shippingFormPluginContext, section }}
      />
    ) : null

  async function onSubmit(values: z.infer<typeof shippingFormSchema>) {
    setIsSubmitting(true)

    try {
      const result = await submitShippingForm({
        form_token: formToken,
        ...values,
      })

      const message = result.data?.message || t.submitSuccess
      toast.success(message)

      // 保存收货信息到本地，下次填表时自动填入
      try {
        const { password, user_remark, receiver_email, ...addressFields } = values
        localStorage.setItem('shipping_form_cache', JSON.stringify(addressFields))
      } catch {}

      // 新用户提示
      if (result.data?.user?.is_new) {
        toast.success(t.accountCreated)
      }

      if (onSuccess) {
        onSuccess()
      } else {
        // 提交成功后跳转到订单详情页
        const orderNo = result.data?.order_no || orderInfo.order_no
        if (orderNo) {
          setTimeout(() => {
            router.push(`/orders/${orderNo}?refresh=true`)
          }, 1000)
        }
      }
    } catch (error: any) {
      toast.error(resolveApiErrorMessage(error, translations, t.submitFailed))
    } finally {
      setIsSubmitting(false)
    }
  }

  const formContent = (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
        {renderPluginSlot('form.top', 'form')}
        <FormField
          control={form.control}
          name="receiver_name"
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t.receiverName} *</FormLabel>
              <FormControl>
                <Input placeholder={t.receiverName} {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* 手机号（包含区号选择器） */}
        <div className="grid grid-cols-[140px_1fr] gap-2">
          <FormField
            control={form.control}
            name="phone_code"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t.phoneCodeLabel} *</FormLabel>
                <Select
                  onValueChange={(value) => {
                    const phoneCode = extractPhoneCode(value)
                    field.onChange(phoneCode)
                  }}
                  value={
                    phoneCodeOptions.find(opt =>
                      opt.phoneCode === field.value && opt.countryCode === selectedCountry
                    )?.uniqueValue ||
                    phoneCodeOptions.find(opt => opt.phoneCode === field.value)?.uniqueValue ||
                    field.value
                  }
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent className="max-h-[300px]">
                    {phoneCodeOptions.map((option) => (
                      <SelectItem key={option.countryCode} value={option.uniqueValue}>
                        {option.phoneCode} {option.countryName}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="receiver_phone"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t.phone} *</FormLabel>
                <FormControl>
                  <Input placeholder={t.phone} {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        <FormField
          control={form.control}
          name="receiver_email"
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t.email} *</FormLabel>
              <FormControl>
                <Input
                  type="email"
                  placeholder={t.emailDesc}
                  {...field}
                  disabled={!!fixedEmail}
                  className={fixedEmail ? 'bg-muted cursor-not-allowed' : ''}
                />
              </FormControl>
              {fixedEmail && (
                <p className="text-sm text-muted-foreground">
                  {t.emailLocked}
                </p>
              )}
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="receiver_country"
          render={({ field }) => (
            <FormItem>
              <FormLabel>
                <Globe className="inline h-4 w-4 mr-1" />
                {t.country} *
              </FormLabel>
              <Select
                onValueChange={(value) => {
                  const normalized = normalizeCountryCode(value, countries)
                  field.onChange(normalized)
                  setSelectedCountry(normalized)
                  const phoneCode = phoneCodeMap[normalized] || '+' + normalized
                  form.setValue('phone_code', phoneCode)
                }}
                value={normalizeCountryCode(field.value, countries)}
              >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue placeholder={t.selectCountryPlaceholder} />
                    </SelectTrigger>
                  </FormControl>
                <SelectContent className="max-h-[300px]">
                  {countries.map((country) => (
                    <SelectItem key={country.code} value={country.code}>
                      {isEnglish ? country.name_en : country.name_zh}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          )}
        />

        {isChina && (
          <div className="grid grid-cols-3 gap-4">
            <FormField
              control={form.control}
              name="receiver_province"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t.province} *</FormLabel>
                  <FormControl>
                    <Input placeholder={t.provincePlaceholderCn} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="receiver_city"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t.city} *</FormLabel>
                  <FormControl>
                    <Input placeholder={t.cityPlaceholderCn} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="receiver_district"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t.district} *</FormLabel>
                  <FormControl>
                    <Input placeholder={t.districtPlaceholderCn} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        )}

        {!isChina && (
          <div className="grid grid-cols-2 gap-4">
            <FormField
              control={form.control}
              name="receiver_city"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t.cityOptional}</FormLabel>
                  <FormControl>
                    <Input placeholder={t.cityPlaceholder} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="receiver_province"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t.provinceOptional}</FormLabel>
                  <FormControl>
                    <Input placeholder={t.provincePlaceholder} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        )}

        <div className="grid grid-cols-2 gap-4">
          <FormField
            control={form.control}
            name="receiver_address"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t.address} *</FormLabel>
                <FormControl>
                  <Input placeholder={t.addressPlaceholder} {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name="receiver_postcode"
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t.postcodeOptional}</FormLabel>
                <FormControl>
                  <Input placeholder={t.postcodePlaceholder} {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>
        {renderPluginSlot('fields.after', 'fields')}

        <FormField
          control={form.control}
          name="user_remark"
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t.remark}</FormLabel>
              <FormControl>
                <Textarea
                  placeholder={t.remarkPlaceholder}
                  className="min-h-[100px]"
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="privacy_protected"
          render={({ field }) => (
            <FormItem className="flex items-start space-x-2 space-y-0 rounded-md border p-4">
              <FormControl>
                <Checkbox
                  checked={field.value}
                  onCheckedChange={field.onChange}
                />
              </FormControl>
              <div className="space-y-1 leading-none">
                <FormLabel className="font-medium">{t.privacyProtection}</FormLabel>
                <p className="text-sm text-muted-foreground">
                  {t.privacyDesc}
                </p>
              </div>
            </FormItem>
          )}
        />

        {!hidePassword && (
        <FormField
          control={form.control}
          name="password"
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t.setPassword}</FormLabel>
              <FormControl>
                <Input
                  type="password"
                  placeholder={t.passwordDesc}
                  {...field}
                />
              </FormControl>
              <div className="space-y-1 mt-2">
                  <p className="text-sm text-muted-foreground">
                    {t.passwordHint1}
                  </p>
                  <p className="text-sm text-orange-600 font-medium">
                    {t.passwordHint2}
                  </p>
                  <p className="text-sm text-muted-foreground">
                    {t.passwordHint3}
                  </p>
                </div>
              <FormMessage />
            </FormItem>
          )}
        />
        )}

        {renderPluginSlot('submit.before', 'submit')}
        <Button type="submit" className="w-full" disabled={isSubmitting}>
          {isSubmitting ? t.submitting : t.submit}
        </Button>
        {renderPluginSlot('form.bottom', 'form')}
      </form>
    </Form>
  )

  if (hideOrderItems) {
    return formContent
  }

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>{t.orderItems}</CardTitle>
        </CardHeader>
        <CardContent>
          {orderInfo.items.map((item, index) => {
            const imageUrl = item.image_url || item.imageUrl
            return (
              <div key={index} className="flex items-center gap-4 py-2">
                {imageUrl ? (
                  <img
                    src={imageUrl}
                    alt={item.name}
                    className="w-16 h-16 object-cover rounded"
                  />
                ) : (
                  <div className="w-16 h-16 bg-muted flex items-center justify-center rounded">
                    <Package className="w-8 h-8 text-muted-foreground" />
                  </div>
                )}
                <div>
                  <p className="font-medium">{item.name}</p>
                  <p className="text-sm text-muted-foreground">
                    {t.quantity}: {item.quantity}
                  </p>
                </div>
              </div>
            )
          })}
        </CardContent>
      </Card>
      {renderPluginSlot('order_items.after', 'order_items')}

      <Card>
        <CardHeader>
          <CardTitle>{t.shippingInfo}</CardTitle>
          <CardDescription>{t.shippingDesc}</CardDescription>
        </CardHeader>
        <CardContent>
          {formContent}
        </CardContent>
      </Card>
    </div>
  )
}

