'use client'

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
import { shippingFormSchema } from '@/lib/validators'
import toast from 'react-hot-toast'
import { Globe, Package } from 'lucide-react'

// 全球国家电话区号映射表（完整版）
const phoneCodeMap: { [key: string]: string } = {
  // 亚洲
  'CN': '+86', 'HK': '+852', 'MO': '+853', 'TW': '+886',
  'JP': '+81', 'KR': '+82', 'SG': '+65', 'MY': '+60',
  'TH': '+66', 'VN': '+84', 'ID': '+62', 'PH': '+63',
  'IN': '+91', 'PK': '+92', 'BD': '+880', 'LK': '+94',
  'MM': '+95', 'KH': '+855', 'LA': '+856', 'BN': '+673',
  'MV': '+960', 'NP': '+977', 'BT': '+975', 'MN': '+976',
  'AF': '+93', 'IQ': '+964', 'IR': '+98', 'IL': '+972',
  'JO': '+962', 'KW': '+965', 'SA': '+966', 'AE': '+971',
  'QA': '+974', 'OM': '+968', 'YE': '+967', 'SY': '+963',
  'LB': '+961', 'PS': '+970', 'TR': '+90', 'KZ': '+7',
  'UZ': '+998', 'TM': '+993', 'KG': '+996', 'TJ': '+992',
  'AM': '+374', 'AZ': '+994', 'GE': '+995', 'TL': '+670',

  // 欧洲
  'GB': '+44', 'FR': '+33', 'DE': '+49', 'IT': '+39',
  'ES': '+34', 'PT': '+351', 'NL': '+31', 'BE': '+32',
  'CH': '+41', 'AT': '+43', 'SE': '+46', 'NO': '+47',
  'DK': '+45', 'FI': '+358', 'IS': '+354', 'IE': '+353',
  'PL': '+48', 'CZ': '+420', 'SK': '+421', 'HU': '+36',
  'RO': '+40', 'BG': '+359', 'GR': '+30', 'HR': '+385',
  'SI': '+386', 'RS': '+381', 'BA': '+387', 'ME': '+382',
  'MK': '+389', 'AL': '+355', 'UA': '+380', 'BY': '+375',
  'MD': '+373', 'RU': '+7', 'EE': '+372', 'LV': '+371',
  'LT': '+370', 'CY': '+357', 'MT': '+356', 'LU': '+352',
  'MC': '+377', 'AD': '+376', 'SM': '+378', 'VA': '+379',
  'LI': '+423',

  // 北美洲
  'US': '+1', 'CA': '+1', 'MX': '+52',
  'GT': '+502', 'BZ': '+501', 'SV': '+503', 'HN': '+504',
  'NI': '+505', 'CR': '+506', 'PA': '+507', 'CU': '+53',
  'JM': '+1', 'HT': '+509', 'DO': '+1', 'BS': '+1',
  'BB': '+1', 'TT': '+1', 'AG': '+1', 'DM': '+1',
  'GD': '+1', 'KN': '+1', 'LC': '+1', 'VC': '+1',

  // 南美洲
  'BR': '+55', 'AR': '+54', 'CL': '+56', 'CO': '+57',
  'PE': '+51', 'VE': '+58', 'EC': '+593', 'BO': '+591',
  'PY': '+595', 'UY': '+598', 'GY': '+592', 'SR': '+597',

  // 大洋洲
  'AU': '+61', 'NZ': '+64', 'FJ': '+679', 'PG': '+675',
  'SB': '+677', 'VU': '+678', 'NC': '+687', 'PF': '+689',
  'WS': '+685', 'TO': '+676', 'KI': '+686', 'FM': '+691',
  'MH': '+692', 'PW': '+680', 'NR': '+674', 'TV': '+688',

  // 非洲
  'EG': '+20', 'ZA': '+27', 'NG': '+234', 'KE': '+254',
  'ET': '+251', 'TZ': '+255', 'UG': '+256', 'DZ': '+213',
  'MA': '+212', 'TN': '+216', 'LY': '+218', 'SD': '+249',
  'SS': '+211', 'GH': '+233', 'CI': '+225', 'SN': '+221',
  'CM': '+237', 'AO': '+244', 'MZ': '+258', 'MG': '+261',
  'ZW': '+263', 'ZM': '+260', 'MW': '+265', 'BW': '+267',
  'NA': '+264', 'LS': '+266', 'SZ': '+268', 'MU': '+230',
  'SC': '+248', 'RW': '+250', 'BI': '+257', 'DJ': '+253',
  'ER': '+291', 'SO': '+252', 'GA': '+241', 'CG': '+242',
  'CD': '+243', 'CF': '+236', 'TD': '+235', 'NE': '+227',
  'ML': '+223', 'BF': '+226', 'SL': '+232', 'LR': '+231',
  'GM': '+220', 'GN': '+224', 'GW': '+245', 'MR': '+222',
  'BJ': '+229', 'TG': '+228', 'GQ': '+240', 'CV': '+238',
  'ST': '+239', 'KM': '+269',
}

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
}

// 翻译文本
const translations = {
  zh: {
    orderItems: '订单商品',
    quantity: '数量',
    shippingInfo: '填写发货信息',
    shippingDesc: '请填写准确的收货地址，以便快速发货',
    receiverName: '收货人姓名',
    phone: '手机号',
    email: '邮箱',
    emailDesc: '用于创建账号和接收通知',
    emailLocked: '邮箱已由第三方平台指定，不可修改',
    country: '国家/地区',
    province: '省份',
    city: '城市',
    district: '区县',
    cityOptional: '城市（可选）',
    provinceOptional: '州/省（可选）',
    address: '详细地址',
    addressPlaceholder: '街道、门牌号等',
    postcode: '邮政编码',
    postcodeOptional: '邮政编码（可选）',
    privacyProtection: '隐私保护',
    privacyDesc: '开启后，除发货管理员外，其他管理员无法查看完整收货信息',
    setPassword: '设置密码（可选）',
    passwordDesc: '不设置密码时，系统将自动生成并发送到您的邮箱',
    remark: '备注（可选）',
    remarkPlaceholder: '请输入备注信息',
    submit: '提交',
    submitting: '提交中...',
  },
  en: {
    orderItems: 'Order Items',
    quantity: 'Quantity',
    shippingInfo: 'Shipping Information',
    shippingDesc: 'Please provide accurate shipping address for fast delivery',
    receiverName: 'Receiver Name',
    phone: 'Phone Number',
    email: 'Email',
    emailDesc: 'For account creation and notifications',
    emailLocked: 'Email is fixed by the platform and cannot be changed',
    country: 'Country/Region',
    province: 'Province',
    city: 'City',
    district: 'District',
    cityOptional: 'City (Optional)',
    provinceOptional: 'State/Province (Optional)',
    address: 'Detailed Address',
    addressPlaceholder: 'Street, building number, etc.',
    postcode: 'Postal Code',
    postcodeOptional: 'Postal Code (Optional)',
    privacyProtection: 'Privacy Protection',
    privacyDesc: 'When enabled, only shipping managers can view complete shipping information',
    setPassword: 'Set Password (Optional)',
    passwordDesc: 'If not set, system will generate and send to your email',
    remark: 'Remark (Optional)',
    remarkPlaceholder: 'Enter remark information',
    submit: 'Submit',
    submitting: 'Submitting...',
  }
}

export function ShippingForm({ formToken, orderInfo, lang = 'zh', onSuccess, hideOrderItems = false, hidePassword = false }: ShippingFormProps) {
  const router = useRouter()
  const [currentLang, setCurrentLang] = useState(lang)
  const isEnglish = currentLang === 'en'
  const t = translations[currentLang as 'zh' | 'en']
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

  const [selectedCountry, setSelectedCountry] = useState(savedShipping?.receiver_country || 'CN')

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
        console.error('获取国家列表失败:', err)
      })
  }, [])

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
      receiver_country: savedShipping?.receiver_country || 'CN',
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

  // 判断是否是中国（需要填写省市区）
  const isChina = selectedCountry === 'CN'

  async function onSubmit(values: z.infer<typeof shippingFormSchema>) {
    setIsSubmitting(true)

    try {
      const result = await submitShippingForm({
        form_token: formToken,
        ...values,
      })

      const message = result.data?.message || '提交成功'
      toast.success(message)

      // 保存收货信息到本地，下次填表时自动填入
      try {
        const { password, user_remark, receiver_email, ...addressFields } = values
        localStorage.setItem('shipping_form_cache', JSON.stringify(addressFields))
      } catch {}

      // 新用户提示
      if (result.data?.user?.is_new) {
        toast.success('账号已自动创建，请查收邮件获取密码')
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
      toast.error(error.message || '提交失败')
    } finally {
      setIsSubmitting(false)
    }
  }

  const formContent = (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
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
                <FormLabel>{isEnglish ? 'Code' : '区号'} *</FormLabel>
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
                  field.onChange(value)
                  setSelectedCountry(value)
                  const phoneCode = phoneCodeMap[value] || '+' + value
                  form.setValue('phone_code', phoneCode)
                }}
                value={field.value}
              >
                <FormControl>
                  <SelectTrigger>
                    <SelectValue placeholder={isEnglish ? "Select Country/Region" : "请选择国家/地区"} />
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
                    <Input placeholder={isEnglish ? "Guangdong" : "广东省"} {...field} />
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
                    <Input placeholder={isEnglish ? "Shenzhen" : "深圳市"} {...field} />
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
                    <Input placeholder={isEnglish ? "Nanshan" : "南山区"} {...field} />
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
                    <Input placeholder="City" {...field} />
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
                    <Input placeholder="State/Province" {...field} />
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
                  <Input placeholder={isEnglish ? "ZIP/Postal Code" : "邮政编码"} {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

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
              {!isEnglish && (
                <div className="space-y-1 mt-2">
                  <p className="text-sm text-muted-foreground">
                    • 如果您是<span className="font-medium">首次使用</span>该邮箱，此密码将用于登录
                  </p>
                  <p className="text-sm text-orange-600 font-medium">
                    ⚠️ 如果该邮箱已注册，此处设置的密码将<span className="font-semibold">无效</span>，请使用原密码登录
                  </p>
                  <p className="text-sm text-muted-foreground">
                    • 留空则自动生成强密码并发送到邮箱
                  </p>
                </div>
              )}
              <FormMessage />
            </FormItem>
          )}
        />
        )}

        <Button type="submit" className="w-full" disabled={isSubmitting}>
          {isSubmitting ? t.submitting : t.submit}
        </Button>
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
                    {t.quantity}：{item.quantity}
                  </p>
                </div>
              </div>
            )
          })}
        </CardContent>
      </Card>

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

