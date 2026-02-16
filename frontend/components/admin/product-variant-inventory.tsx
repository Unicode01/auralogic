'use client'

import { useState, useEffect, useMemo, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
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
import { Package, Sparkles, AlertCircle } from 'lucide-react'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'

export interface VariantInventoryBinding {
    attributes: Record<string, string>
    inventory_id: number | null
    priority: number
}

interface ProductVariantInventoryProps {
    attributes: Array<{ name: string; values: string[]; mode: 'user_select' | 'blind_box' }>
    variantMode: 'user_select' | 'blind_box'
    bindings: VariantInventoryBinding[]
    inventories: any[]
    onVariantModeChange: (mode: 'user_select' | 'blind_box') => void
    onBindingsChange: (bindings: VariantInventoryBinding[]) => void
}

export function ProductVariantInventory({
    attributes,
    variantMode,
    bindings,
    inventories,
    onVariantModeChange,
    onBindingsChange,
}: ProductVariantInventoryProps) {
    const [selectKey, setSelectKey] = useState(0)
    const { locale } = useLocale()
    const t = getTranslations(locale)

    const normalizeAttributes = useCallback((attrs: Record<string, string>) => {
        const sorted = Object.keys(attrs).sort().reduce((obj: Record<string, string>, key) => {
            obj[key] = attrs[key]
            return obj
        }, {})
        return sorted
    }, [])

    const variants = useMemo(() => {
        if (attributes.length === 0) {
            return [{}]
        }

        const validAttrs = attributes.filter(attr => attr.name && attr.values.length > 0)
        if (validAttrs.length === 0) {
            return [{}]
        }

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
        return combinations.map((combo) => {
            const comboObj = Object.assign({}, ...combo) as Record<string, string>
            const sorted = Object.keys(comboObj).sort().reduce((obj: Record<string, string>, key) => {
                obj[key] = comboObj[key]
                return obj
            }, {})
            return sorted
        })
    }, [attributes])

    useEffect(() => {
        const uniqueVariants = Array.from(
            new Map(variants.map(v => [JSON.stringify(v), v])).values()
        )

        const existingBindings = new Map(
            bindings.map(b => [JSON.stringify(normalizeAttributes(b.attributes)), b])
        )

        const newBindings: VariantInventoryBinding[] = uniqueVariants.map(attrs => {
            const key = JSON.stringify(attrs)
            const existing = existingBindings.get(key)
            return existing || {
                attributes: attrs,
                inventory_id: null,
                priority: 1,
            }
        })

        const sortAttrs = (attrs: Record<string, string>) => {
            const keys = Object.keys(attrs).sort()
            return keys.map(k => `${k}:${attrs[k]}`).join(',')
        }
        const currentAttrsKey = bindings.map(b => sortAttrs(b.attributes)).sort().join(';')
        const newAttrsKey = newBindings.map(b => sortAttrs(b.attributes)).sort().join(';')

        if (currentAttrsKey !== newAttrsKey || bindings.length === 0) {
            onBindingsChange(newBindings)
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [variants, normalizeAttributes])

    useEffect(() => {
        setSelectKey(prev => prev + 1)
    }, [variantMode])

    const updateBinding = useCallback((
        index: number,
        field: keyof VariantInventoryBinding,
        value: any
    ) => {
        const updated = [...bindings]
        if (field === 'inventory_id') {
            updated[index].inventory_id = value ? parseInt(value) : null
        } else if (field === 'priority') {
            updated[index].priority = value
        }
        onBindingsChange(updated)
    }, [bindings, onBindingsChange])

    const getInventoryInfo = useCallback((inventoryId: number | null) => {
        if (!inventoryId) return null
        return inventories.find((inv: any) => inv.id === inventoryId)
    }, [inventories])

    const attrToString = useCallback((attrs: Record<string, string>) => {
        if (Object.keys(attrs).length === 0) return t.admin.defaultVariant
        return Object.entries(attrs)
            .map(([k, v]) => `${k}:${v}`)
            .join(', ')
    }, [t])

    const totalWeight = useMemo(() => {
        return bindings.reduce((sum, b) => {
            if (b.inventory_id) {
                const isBlindBox = Object.keys(b.attributes).some(key => {
                    const attr = attributes.find(a => a.name === key)
                    return attr?.mode === 'blind_box'
                })
                if (isBlindBox) {
                    return sum + (b.priority || 1)
                }
            }
            return sum
        }, 0)
    }, [bindings, attributes])

    const allConfigured = useMemo(() => {
        return bindings.every(b => b.inventory_id !== null)
    }, [bindings])

    const hasBindings = bindings.length > 0
    const isDefaultProduct = bindings.length === 1 && Object.keys(bindings[0]?.attributes || {}).length === 0

    return (
        <Card>
            <CardHeader>
                <CardTitle className="flex items-center justify-between">
                    <span className="flex items-center gap-2">
                        <Package className="h-5 w-5" />
                        {t.admin.variantInventoryConfig}
                    </span>
                    <Badge variant={variantMode === 'blind_box' ? 'default' : 'secondary'}>
                        {variantMode === 'blind_box' ? (
                            <>
                                <Sparkles className="h-3 w-3 mr-1" />
                                {t.admin.blindBoxMode}
                            </>
                        ) : (
                            <>
                                <Package className="h-3 w-3 mr-1" />
                                {t.admin.userSelectMode}
                            </>
                        )}
                    </Badge>
                </CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
                {hasBindings && (
                    <>
                        {!allConfigured && (
                            <div className="flex items-center gap-2 p-4 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg text-yellow-800 dark:text-yellow-300">
                                <AlertCircle className="h-4 w-4 flex-shrink-0" />
                                <p className="text-sm">
                                    {isDefaultProduct ? t.admin.configDefaultProduct : t.admin.configAllVariants}
                                </p>
                            </div>
                        )}

                        <div className="space-y-3">
                            <Label>
                                {isDefaultProduct
                                    ? t.admin.noSpecInventory
                                    : t.admin.variantInventoryCount.replace('{count}', String(bindings.length))}
                            </Label>
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead>{t.admin.variantCombo}</TableHead>
                                        <TableHead>{t.admin.variantType}</TableHead>
                                        <TableHead>{t.admin.inventoryConfig}</TableHead>
                                        <TableHead>{t.admin.inventoryInfo}</TableHead>
                                        <TableHead>{t.admin.weight}</TableHead>
                                        <TableHead>{t.admin.probability}</TableHead>
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    {bindings.map((binding, index) => {
                                        const isBlindBox = Object.keys(binding.attributes).some(key => {
                                            const attr = attributes.find(a => a.name === key)
                                            return attr?.mode === 'blind_box'
                                        })

                                        const inventory = getInventoryInfo(binding.inventory_id)
                                        const probability =
                                            isBlindBox && totalWeight > 0
                                                ? ((binding.priority / totalWeight) * 100).toFixed(1)
                                                : '0'

                                        return (
                                            <TableRow key={index}>
                                                <TableCell>
                                                    <div className="flex flex-wrap gap-1">
                                                        {Object.entries(binding.attributes).map(([key, value]) => (
                                                            <Badge key={key} variant="outline" className="text-xs">
                                                                {String(key)}:{String(value)}
                                                            </Badge>
                                                        ))}
                                                        {Object.keys(binding.attributes).length === 0 && (
                                                            <span className="text-sm text-muted-foreground">{t.admin.defaultVariant}</span>
                                                        )}
                                                    </div>
                                                </TableCell>
                                                <TableCell>
                                                    {isBlindBox ? (
                                                        <Badge variant="default" className="text-xs">
                                                            <Sparkles className="h-3 w-3 mr-1" />
                                                            {t.admin.blindBoxBadge}
                                                        </Badge>
                                                    ) : (
                                                        <Badge variant="secondary" className="text-xs">
                                                            <Package className="h-3 w-3 mr-1" />
                                                            {t.admin.userSelectBadge}
                                                        </Badge>
                                                    )}
                                                </TableCell>
                                                <TableCell>
                                                    <Select
                                                        value={binding.inventory_id?.toString() || ''}
                                                        onValueChange={(value) =>
                                                            updateBinding(index, 'inventory_id', value)
                                                        }
                                                    >
                                                        <SelectTrigger className="w-[300px]">
                                                            <SelectValue placeholder={t.admin.selectInventory} />
                                                        </SelectTrigger>
                                                        <SelectContent>
                                                            {inventories.map((inv: any) => (
                                                                <SelectItem key={inv.id} value={inv.id.toString()}>
                                                                    {String(inv.name || '')} ({t.admin.remaining}: {Number(inv.stock || 0) - Number(inv.sold_quantity || 0) - Number(inv.reserved_quantity || 0)})
                                                                </SelectItem>
                                                            ))}
                                                        </SelectContent>
                                                    </Select>
                                                </TableCell>
                                                <TableCell>
                                                    {inventory ? (
                                                        <div className="text-sm space-y-1">
                                                            <div className="font-medium">{String(inventory.name || '')}</div>
                                                            {inventory.sku && (
                                                                <div className="text-muted-foreground">SKU: {String(inventory.sku)}</div>
                                                            )}
                                                            <div className="flex flex-wrap gap-1">
                                                                {Object.entries(inventory.attributes || {}).map(
                                                                    ([key, value]) => (
                                                                        <Badge key={key} variant="secondary" className="text-xs">
                                                                            {String(key)}: {String(value)}
                                                                        </Badge>
                                                                    )
                                                                )}
                                                            </div>
                                                            <div>
                                                                {t.admin.remaining}: {Number(inventory.stock || 0) - Number(inventory.sold_quantity || 0) - Number(inventory.reserved_quantity || 0)}
                                                            </div>
                                                        </div>
                                                    ) : (
                                                        <span className="text-sm text-muted-foreground">{t.admin.notConfigured}</span>
                                                    )}
                                                </TableCell>
                                                <TableCell>
                                                    {isBlindBox ? (
                                                        <Input
                                                            type="number"
                                                            min="1"
                                                            value={binding.priority}
                                                            onChange={(e) =>
                                                                updateBinding(
                                                                    index,
                                                                    'priority',
                                                                    parseInt(e.target.value) || 1
                                                                )
                                                            }
                                                            className="w-20"
                                                            disabled={!binding.inventory_id}
                                                        />
                                                    ) : (
                                                        <span className="text-sm text-muted-foreground">-</span>
                                                    )}
                                                </TableCell>
                                                <TableCell>
                                                    {isBlindBox && binding.inventory_id ? (
                                                        <Badge variant="outline">{probability}%</Badge>
                                                    ) : (
                                                        <span className="text-sm text-muted-foreground">-</span>
                                                    )}
                                                </TableCell>
                                            </TableRow>
                                        )
                                    })}
                                </TableBody>
                            </Table>
                        </div>

                        {allConfigured && bindings.some(b => {
                            return Object.keys(b.attributes).some(key => {
                                const attr = attributes.find(a => a.name === key)
                                return attr?.mode === 'blind_box'
                            })
                        }) && (
                                <div className="p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg">
                                    <h4 className="font-medium text-blue-900 dark:text-blue-300 mb-2 flex items-center gap-2">
                                        <Sparkles className="h-4 w-4" />
                                        {t.admin.blindBoxRules}
                                    </h4>
                                    <div className="text-sm text-blue-800 dark:text-blue-400 space-y-1">
                                        {bindings.filter(binding => {
                                            return Object.keys(binding.attributes).some(key => {
                                                const attr = attributes.find(a => a.name === key)
                                                return attr?.mode === 'blind_box'
                                            })
                                        }).map((binding, index) => {
                                            const inventory = getInventoryInfo(binding.inventory_id)
                                            const probability =
                                                totalWeight > 0
                                                    ? ((binding.priority / totalWeight) * 100).toFixed(1)
                                                    : '0.0'
                                            return (
                                                <div key={index}>
                                                    • {attrToString(binding.attributes)}: {probability}% {t.admin.probability}
                                                    {inventory && ` (${String(inventory.name || '')})`}
                                                </div>
                                            )
                                        })}
                                    </div>
                                </div>
                            )}
                    </>
                )}
            </CardContent>
        </Card>
    )
}
