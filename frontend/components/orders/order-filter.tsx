'use client'

import { useState, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getUsers, getOrderCountries } from '@/lib/api'
import { useDebounce } from '@/hooks/use-debounce'
import { Card, CardContent } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import { Search, X, User } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'

// 国家名称映射（用于显示）
const COUNTRY_NAMES: Record<string, { zh: string; en: string }> = {
  "CN": { zh: "中国", en: "China" },
  "HK": { zh: "中国香港", en: "Hong Kong" },
  "MO": { zh: "中国澳门", en: "Macau" },
  "TW": { zh: "中国台湾", en: "Taiwan" },
  "US": { zh: "美国", en: "United States" },
  "CA": { zh: "加拿大", en: "Canada" },
  "GB": { zh: "英国", en: "United Kingdom" },
  "JP": { zh: "日本", en: "Japan" },
  "KR": { zh: "韩国", en: "South Korea" },
  "SG": { zh: "新加坡", en: "Singapore" },
  "AU": { zh: "澳大利亚", en: "Australia" },
  "DE": { zh: "德国", en: "Germany" },
  "FR": { zh: "法国", en: "France" },
  "IT": { zh: "意大利", en: "Italy" },
  "ES": { zh: "西班牙", en: "Spain" },
  "NL": { zh: "荷兰", en: "Netherlands" },
  "BE": { zh: "比利时", en: "Belgium" },
  "SE": { zh: "瑞典", en: "Sweden" },
  "NO": { zh: "挪威", en: "Norway" },
  "DK": { zh: "丹麦", en: "Denmark" },
  "FI": { zh: "芬兰", en: "Finland" },
  "PL": { zh: "波兰", en: "Poland" },
  "CZ": { zh: "捷克", en: "Czech Republic" },
  "AT": { zh: "奥地利", en: "Austria" },
  "CH": { zh: "瑞士", en: "Switzerland" },
  "PT": { zh: "葡萄牙", en: "Portugal" },
  "GR": { zh: "希腊", en: "Greece" },
  "IE": { zh: "爱尔兰", en: "Ireland" },
  "NZ": { zh: "新西兰", en: "New Zealand" },
  "MY": { zh: "马来西亚", en: "Malaysia" },
  "TH": { zh: "泰国", en: "Thailand" },
  "VN": { zh: "越南", en: "Vietnam" },
  "PH": { zh: "菲律宾", en: "Philippines" },
  "ID": { zh: "印度尼西亚", en: "Indonesia" },
  "IN": { zh: "印度", en: "India" },
  "AE": { zh: "阿联酋", en: "UAE" },
  "SA": { zh: "沙特阿拉伯", en: "Saudi Arabia" },
  "IL": { zh: "以色列", en: "Israel" },
  "TR": { zh: "土耳其", en: "Turkey" },
  "BR": { zh: "巴西", en: "Brazil" },
  "MX": { zh: "墨西哥", en: "Mexico" },
  "AR": { zh: "阿根廷", en: "Argentina" },
  "ZA": { zh: "南非", en: "South Africa" },
  "EG": { zh: "埃及", en: "Egypt" },
  "RU": { zh: "俄罗斯", en: "Russia" },
  "UA": { zh: "乌克兰", en: "Ukraine" },
}

interface OrderFilterProps {
  status?: string
  search?: string
  userId?: number
  country?: string
  productSearch?: string
  onStatusChange: (status: string | undefined) => void
  onSearchChange?: (search: string) => void
  onUserChange?: (userId: number | undefined) => void
  onCountryChange?: (country: string | undefined) => void
  onProductSearchChange?: (productSearch: string) => void
  useSmartCountryFilter?: boolean // 是否使用智能国家筛选（仅管理员）
}

export function OrderFilter({
  status,
  search,
  userId,
  country,
  productSearch,
  onStatusChange,
  onSearchChange,
  onUserChange,
  onCountryChange,
  onProductSearchChange,
  useSmartCountryFilter = false,
}: OrderFilterProps) {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const [userSearch, setUserSearch] = useState('')
  const [selectedUser, setSelectedUser] = useState<any>(null)
  const [showUserList, setShowUserList] = useState(false)
  const debouncedUserSearch = useDebounce(userSearch, 300)

  // 获取有订单的国家列表（仅管理员）
  const { data: countriesData } = useQuery({
    queryKey: ['orderCountries'],
    queryFn: getOrderCountries,
    enabled: useSmartCountryFilter && !!onCountryChange,
  })

  const { data: usersData } = useQuery({
    queryKey: ['users-for-filter', debouncedUserSearch],
    queryFn: () => getUsers({ page: 1, limit: 20, search: debouncedUserSearch }),
    enabled: !!onUserChange && showUserList, // 只在需要用户筛选且显示列表时加载
  })

  // 从URL参数初始化时，加载选中的用户信息
  useEffect(() => {
    if (userId && !selectedUser) {
      getUsers({ page: 1, limit: 1, search: userId.toString() }).then((res: any) => {
        const user = res?.data?.items?.find((u: any) => u.id === userId)
        if (user) {
          setSelectedUser(user)
          setUserSearch(user.name || user.email)
        }
      })
    }
  }, [userId, selectedUser])

  const handleSelectUser = (user: any) => {
    setSelectedUser(user)
    setUserSearch(user.name || user.email)
    setShowUserList(false)
    onUserChange?.(user.id)
  }

  const handleClearUser = () => {
    setSelectedUser(null)
    setUserSearch('')
    onUserChange?.(undefined)
  }

  return (
    <Card>
      <CardContent className="pt-6">
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {/* 订单搜索 */}
          <div>
            <label className="text-sm font-medium mb-2 block">{t.order.searchOrder}</label>
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder={t.order.orderNoPlaceholder}
                value={search}
                onChange={(e) => onSearchChange?.(e.target.value)}
                className="pl-10"
              />
            </div>
          </div>

          {/* 商品搜索 */}
          {onProductSearchChange && (
            <div>
              <label className="text-sm font-medium mb-2 block">{t.order.searchProduct}</label>
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder={t.order.skuPlaceholder}
                  value={productSearch}
                  onChange={(e) => onProductSearchChange(e.target.value)}
                  className="pl-10"
                />
              </div>
            </div>
          )}

          {/* 用户筛选 */}
          {onUserChange && (
            <div className="relative">
              <label className="text-sm font-medium mb-2 block">{t.order.filterUser}</label>
              <div className="relative">
                <User className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder={t.order.searchUser}
                  value={userSearch}
                  onChange={(e) => {
                    setUserSearch(e.target.value)
                    setShowUserList(true)
                  }}
                  onFocus={() => setShowUserList(true)}
                  className="pl-10 pr-8"
                />
                {selectedUser && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={handleClearUser}
                    className="absolute right-1 top-1/2 -translate-y-1/2 h-7 w-7 p-0"
                  >
                    <X className="h-4 w-4" />
                  </Button>
                )}
              </div>

              {/* 用户列表下拉 */}
              {showUserList && !selectedUser && userSearch && (
                <div className="absolute top-full left-0 right-0 mt-1 bg-background border rounded-md shadow-lg z-50 max-h-60 overflow-y-auto">
                  {usersData && usersData?.data?.items?.length > 0 ? (
                    usersData.data.items.map((user: any) => (
                      <div
                        key={user.id}
                        className="px-3 py-2 hover:bg-muted cursor-pointer"
                        onClick={() => handleSelectUser(user)}
                      >
                        <div className="flex items-center justify-between">
                          <div className="flex flex-col">
                            <span className="text-sm font-medium">{user.name || user.email}</span>
                            {user.name && (
                              <span className="text-xs text-muted-foreground">{user.email}</span>
                            )}
                          </div>
                          <Badge variant="outline" className="text-xs">
                            ID: {user.id}
                          </Badge>
                        </div>
                      </div>
                    ))
                  ) : (
                    <div className="px-3 py-2 text-sm text-muted-foreground text-center">
                      {t.order.noUserFound}
                    </div>
                  )}
                </div>
              )}

              {/* 点击外部关闭下拉框 */}
              {showUserList && !selectedUser && (
                <div
                  className="fixed inset-0 z-40"
                  onClick={() => setShowUserList(false)}
                />
              )}
            </div>
          )}

          {/* 国家筛选 */}
          {onCountryChange && (
            <div>
              <label className="text-sm font-medium mb-2 block">{t.order.shippingCountry}</label>
              <Select value={country} onValueChange={onCountryChange}>
                <SelectTrigger>
                  <SelectValue placeholder={t.order.allCountries} />
                </SelectTrigger>
                <SelectContent className="max-h-[300px]">
                  <SelectItem value="all">{t.order.allCountries}</SelectItem>
                  {useSmartCountryFilter && countriesData?.data?.countries ? (
                    // 管理员：只显示有订单的国家
                    countriesData.data.countries.map((countryCode: string) => (
                      <SelectItem key={countryCode} value={countryCode}>
                        {COUNTRY_NAMES[countryCode]
                          ? (locale === 'zh' ? COUNTRY_NAMES[countryCode].zh : COUNTRY_NAMES[countryCode].en)
                          : countryCode}
                      </SelectItem>
                    ))
                  ) : (
                    // 用户端：显示所有常用国家（向后兼容）
                    Object.entries(COUNTRY_NAMES).map(([code, names]) => (
                      <SelectItem key={code} value={code}>
                        {locale === 'zh' ? names.zh : names.en}
                      </SelectItem>
                    ))
                  )}
                </SelectContent>
              </Select>
            </div>
          )}

          {/* 状态筛选 */}
          <div>
            <label className="text-sm font-medium mb-2 block">{t.order.orderStatus}</label>
            <Select value={status} onValueChange={onStatusChange}>
              <SelectTrigger>
                <SelectValue placeholder={t.order.allStatus} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t.order.status.all}</SelectItem>
                <SelectItem value="pending_payment">{t.order.status.pending_payment}</SelectItem>
                <SelectItem value="draft">{t.order.status.draft}</SelectItem>
                <SelectItem value="pending">{t.order.status.pending}</SelectItem>
                <SelectItem value="need_resubmit">{t.order.status.need_resubmit}</SelectItem>
                <SelectItem value="shipped">{t.order.status.shipped}</SelectItem>
                <SelectItem value="completed">{t.order.status.completed}</SelectItem>
                <SelectItem value="cancelled">{t.order.status.cancelled}</SelectItem>
                <SelectItem value="refunded">{t.order.status.refunded}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

