'use client'

import { useState, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getInventories } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Package, Sparkles } from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'

interface ProductAttribute {
  name: string
  values: string[]
}

interface VariantConfig {
  attributes: Record<string, string> // Combination, e.g. { color: blue, size: M }
  inventory_id: number | null
  is_random: boolean
  priority: number
}

interface ProductInventoryConfigProps {
  attributes: ProductAttribute[]
  inventoryMode: 'fixed' | 'random'
  onModeChange: (mode: 'fixed' | 'random') => void
  variantConfigs: VariantConfig[]
  onVariantConfigsChange: (configs: VariantConfig[]) => void
}

export function ProductInventoryConfig({
  attributes,
  inventoryMode,
  onModeChange,
  variantConfigs,
  onVariantConfigsChange,
}: ProductInventoryConfigProps) {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const [localMode, setLocalMode] = useState(inventoryMode)
  const [selectKey, setSelectKey] = useState(0)

  // 当外部inventoryMode改变时，同步本地状态并强制重新渲染Select
  useEffect(() => {
    setLocalMode(inventoryMode)
    setSelectKey(prev => prev + 1)
  }, [inventoryMode])

  const handleModeChange = (newMode: 'fixed' | 'random') => {
    setLocalMode(newMode)
    onModeChange(newMode)
  }
  // 获取所有库存配置
  const { data: inventoriesData } = useQuery({
    queryKey: ['inventories', 1, 200],
    queryFn: () => getInventories({ page: 1, limit: 200 }),
  })

  const inventories = inventoriesData?.data?.items || []

  // Helper: 属性转字符串
  const attrToString = (attrs: Record<string, string>) => {
    if (Object.keys(attrs).length === 0) return t.admin.defaultVariant
    return Object.entries(attrs)
      .map(([k, v]) => `${k}:${v}`)
      .join(', ')
  }

  // Helper: 获取库存信息
  const getInventoryInfo = (inventoryId: number | null) => {
    if (!inventoryId) return null
    return inventories.find((inv: any) => inv.id === inventoryId)
  }

  // 生成所有规格组合（笛卡尔积）
  const generateVariants = () => {
    if (attributes.length === 0) {
      return [{}] // 无规格商品
    }

    const validAttrs = attributes.filter(attr => attr.name && attr.values.length > 0)
    if (validAttrs.length === 0) {
      return [{}]
    }

    // 笛卡尔积
    const cartesian = (arr: any[][]): any[][] => {
      return arr.reduce(
        (a, b) => a.flatMap((x) => b.map((y) => [...x, y])),
        [[]]
      )
    }

    const attrArrays = validAttrs.map((attr) =>
      attr.values.map((value) => ({ [attr.name]: value }))
    )

    const combinations = cartesian(attrArrays)
    return combinations.map((combo) => Object.assign({}, ...combo))
  }

  const variants = generateVariants()

  // 当规格属性变化时，更新变体配置
  useEffect(() => {
    const newVariants = generateVariants()
    const existingConfigs = new Map(
      variantConfigs.map(c => [JSON.stringify(c.attributes), c])
    )

    const newConfigs: VariantConfig[] = newVariants.map(attrs => {
      const key = JSON.stringify(attrs)
      const existing = existingConfigs.get(key)
      return existing || {
        attributes: attrs,
        inventory_id: null,
        is_random: false,
        priority: 1,
      }
    })

    onVariantConfigsChange(newConfigs)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [JSON.stringify(attributes)])

  const updateVariantConfig = (
    index: number,
    field: 'inventory_id' | 'is_random' | 'priority',
    value: any
  ) => {
    const updated = [...variantConfigs]
    if (field === 'inventory_id') {
      updated[index].inventory_id = value ? parseInt(value) : null
    } else if (field === 'is_random') {
      updated[index].is_random = value
    } else if (field === 'priority') {
      updated[index].priority = value
    }
    onVariantConfigsChange(updated)
  }

  const totalWeight = variantConfigs
    .filter(v => v.is_random && v.inventory_id)
    .reduce((sum, v) => sum + v.priority, 0)

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t.admin.inventoryConfig}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        {/* 库存模式选择 */}
        <div className="space-y-2">
          <Label>{t.admin.inventoryModeLabel}</Label>
          <Select key={selectKey} value={localMode} onValueChange={handleModeChange}>
            <SelectTrigger className="w-64">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="fixed">
                <div className="flex items-center gap-2">
                  <Package className="h-4 w-4" />
                  {t.admin.inventoryModeFixedWithDesc}
                </div>
              </SelectItem>
              <SelectItem value="random">
                <div className="flex items-center gap-2">
                  <Sparkles className="h-4 w-4" />
                  {t.admin.inventoryModeRandomWithDesc}
                </div>
              </SelectItem>
            </SelectContent>
          </Select>
          <p className="text-sm text-muted-foreground">
            {localMode === 'random'
              ? t.admin.inventoryModeRandomHint
              : t.admin.inventoryModeFixedHint}
          </p>
        </div>

        {/* 规格组合配置表 */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <Label>{t.admin.variantInventoryConfig}</Label>
            <span className="text-sm text-muted-foreground">
              {t.admin.variantInventoryCount.replace('{count}', String(variants.length))}
            </span>
          </div>

          {variants.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground border-2 border-dashed rounded-lg">
              {t.admin.noSpecs}
            </div>
          ) : (
            <div className="border rounded-lg overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-[200px]">{t.admin.variantCombo}</TableHead>
                    <TableHead className="w-[300px]">{t.admin.inventoryConfig}</TableHead>
                    <TableHead className="w-[120px]">{t.admin.remainingStockCol}</TableHead>
                    {localMode === 'random' && (
                      <>
                        <TableHead className="w-[100px]">{t.admin.randomParticipationCol}</TableHead>
                        <TableHead className="w-[100px]">{t.admin.weight}</TableHead>
                        <TableHead className="w-[100px]">{t.admin.probability}</TableHead>
                      </>
                    )}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {variantConfigs.map((config, index) => {
                    const inventory = getInventoryInfo(config.inventory_id)
                    const probability =
                      localMode === 'random' && totalWeight > 0 && config.is_random
                        ? ((config.priority / totalWeight) * 100).toFixed(1)
                        : '0'

                    return (
                      <TableRow key={index}>
                        <TableCell>
                          <div className="flex flex-wrap gap-1">
                            {Object.entries(config.attributes).map(([key, value]) => (
                              <Badge key={key} variant="outline" className="text-xs">
                                {key}:{value}
                              </Badge>
                            ))}
                            {Object.keys(config.attributes).length === 0 && (
                              <span className="text-sm text-muted-foreground">{t.admin.defaultVariant}</span>
                            )}
                          </div>
                        </TableCell>
                        <TableCell>
                          <Select
                            value={config.inventory_id?.toString() || ''}
                            onValueChange={(value) =>
                              updateVariantConfig(index, 'inventory_id', value)
                            }
                          >
                            <SelectTrigger className="w-full">
                              <SelectValue placeholder={t.admin.selectInventory} />
                            </SelectTrigger>
                            <SelectContent>
                              {inventories.map((inv: any) => (
                                <SelectItem key={inv.id} value={inv.id.toString()}>
                                  {inv.name} ({t.admin.inventoryRemainingInline.replace('{count}', String(inv.stock - inv.sold_quantity - inv.reserved_quantity))})
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                          {inventory && (
                            <div className="mt-1 text-xs text-muted-foreground">
                              {Object.entries(inventory.attributes || {}).map(
                                ([k, v]) => `${k}:${v}`
                              ).join(', ') || t.common.noData}
                            </div>
                          )}
                        </TableCell>
                        <TableCell>
                          {inventory ? (
                            <Badge
                              variant={
                                inventory.stock - inventory.sold_quantity - inventory.reserved_quantity > 0
                                  ? 'default'
                                  : 'destructive'
                              }
                            >
                              {inventory.stock - inventory.sold_quantity - inventory.reserved_quantity}
                            </Badge>
                          ) : (
                            <span className="text-muted-foreground">-</span>
                          )}
                        </TableCell>
                        {localMode === 'random' && (
                          <>
                            <TableCell>
                              <Switch
                                checked={config.is_random}
                                disabled={!config.inventory_id}
                                onCheckedChange={(checked) =>
                                  updateVariantConfig(index, 'is_random', checked)
                                }
                              />
                            </TableCell>
                            <TableCell>
                              <Input
                                type="number"
                                min="1"
                                className="w-20"
                                value={config.priority}
                                disabled={!config.is_random}
                                onChange={(e) =>
                                  updateVariantConfig(
                                    index,
                                    'priority',
                                    parseInt(e.target.value) || 1
                                  )
                                }
                              />
                            </TableCell>
                            <TableCell>
                              {config.is_random && config.inventory_id ? (
                                <span className="text-sm font-medium">{probability}%</span>
                              ) : (
                                <span className="text-muted-foreground">{t.common.noData}</span>
                              )}
                            </TableCell>
                          </>
                        )}
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </div>
          )}

          {variants.length > 0 && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground p-3 bg-muted rounded-lg">
              <span>💡 {t.common.info}:</span>
              <span>
                {t.admin.variantConfigAutoGeneratedTip.replace('{count}', String(variants.length))}
              </span>
            </div>
          )}
        </div>

        {/* 盲盒模式概率说明 */}
        {localMode === 'random' && variantConfigs.filter(v => v.is_random && v.inventory_id).length > 0 && (
          <div className="p-4 bg-blue-50 dark:bg-blue-950 border border-blue-200 dark:border-blue-800 rounded-lg">
            <h4 className="font-medium text-blue-900 dark:text-blue-200 mb-2 flex items-center gap-2">
              <Sparkles className="h-4 w-4" />
              {t.admin.blindBoxRules}
            </h4>
            <div className="text-sm text-blue-800 dark:text-blue-200 space-y-1">
              {variantConfigs
                .filter(v => v.is_random && v.inventory_id)
                .map((config, idx) => {
                  const inventory = getInventoryInfo(config.inventory_id)
                  const probability =
                    totalWeight > 0
                      ? ((config.priority / totalWeight) * 100).toFixed(1)
                      : '0.0'
                  return (
                    <div key={idx}>
                      • {attrToString(config.attributes)}: {probability}%
                      ({t.admin.inventoryNameCol}: {inventory?.name || t.common.noData})
                    </div>
                  )
                })}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
