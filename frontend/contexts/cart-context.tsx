'use client'

import { createContext, useContext, useState, useCallback, useEffect, ReactNode } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getCart, getCartCount, addToCart, updateCartItemQuantity, removeFromCart, clearCart, CartItem } from '@/lib/api'
import { useAuth } from '@/hooks/use-auth'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import toast from 'react-hot-toast'

interface CartContextType {
  items: CartItem[]
  totalPrice: number
  totalQuantity: number
  itemCount: number
  isLoading: boolean
  addItem: (productId: number, quantity: number, attributes?: Record<string, string>) => Promise<void>
  updateQuantity: (itemId: number, quantity: number) => Promise<void>
  removeItem: (itemId: number) => Promise<void>
  removeItems: (itemIds: number[]) => Promise<void>
  clear: () => Promise<void>
  refetch: () => void
}

const CartContext = createContext<CartContextType | undefined>(undefined)

export function CartProvider({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const queryClient = useQueryClient()

  // 获取购物车数据
  const { data: cartData, isLoading, refetch } = useQuery({
    queryKey: ['cart'],
    queryFn: getCart,
    enabled: isAuthenticated,
    staleTime: 1000 * 60, // 1分钟
  })

  const items = cartData?.data?.items || []
  const totalPrice = cartData?.data?.total_price || 0
  const totalQuantity = cartData?.data?.total_quantity || 0
  const itemCount = cartData?.data?.item_count || 0

  // 添加商品到购物车
  const addItemMutation = useMutation({
    mutationFn: (data: { product_id: number; quantity: number; attributes?: Record<string, string> }) =>
      addToCart(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['cart'] })
      toast.success(t.cart.addedToCart)
    },
    onError: (error: Error) => {
      toast.error(error.message || t.cart.addFailed)
    },
  })

  // 更新数量
  const updateQuantityMutation = useMutation({
    mutationFn: ({ itemId, quantity }: { itemId: number; quantity: number }) =>
      updateCartItemQuantity(itemId, quantity),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['cart'] })
    },
    onError: (error: Error) => {
      toast.error(error.message || t.cart.updateFailed)
    },
  })

  // 移除商品
  const removeItemMutation = useMutation({
    mutationFn: (itemId: number) => removeFromCart(itemId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['cart'] })
      toast.success(t.cart.removedFromCart)
    },
    onError: (error: Error) => {
      toast.error(error.message || t.cart.removeFailed)
    },
  })

  // 清空购物车
  const clearCartMutation = useMutation({
    mutationFn: clearCart,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['cart'] })
      toast.success(t.cart.cartCleared)
    },
    onError: (error: Error) => {
      toast.error(error.message || t.cart.clearFailed)
    },
  })

  const addItem = useCallback(async (productId: number, quantity: number, attributes?: Record<string, string>) => {
    if (quantity < 1 || quantity > 9999) return
    await addItemMutation.mutateAsync({ product_id: productId, quantity, attributes })
  }, [addItemMutation])

  const updateQuantityHandler = useCallback(async (itemId: number, quantity: number) => {
    if (quantity < 1 || quantity > 9999) return
    await updateQuantityMutation.mutateAsync({ itemId, quantity })
  }, [updateQuantityMutation])

  const removeItem = useCallback(async (itemId: number) => {
    await removeItemMutation.mutateAsync(itemId)
  }, [removeItemMutation])

  // 批量移除商品（不显示单独的toast）
  const removeItems = useCallback(async (itemIds: number[]) => {
    try {
      await Promise.all(itemIds.map(id => removeFromCart(id)))
      queryClient.invalidateQueries({ queryKey: ['cart'] })
    } catch (error) {
      console.error('Failed to remove items:', error)
    }
  }, [queryClient])

  const clear = useCallback(async () => {
    await clearCartMutation.mutateAsync()
  }, [clearCartMutation])

  return (
    <CartContext.Provider
      value={{
        items,
        totalPrice,
        totalQuantity,
        itemCount,
        isLoading,
        addItem,
        updateQuantity: updateQuantityHandler,
        removeItem,
        removeItems,
        clear,
        refetch,
      }}
    >
      {children}
    </CartContext.Provider>
  )
}

export function useCart() {
  const context = useContext(CartContext)
  if (context === undefined) {
    throw new Error('useCart must be used within a CartProvider')
  }
  return context
}
