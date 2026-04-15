'use client'

import toast, { type Renderable } from 'react-hot-toast'

type ToastAPI = {
  success: (message: Renderable) => string
  error: (message: Renderable) => string
  loading: (message: Renderable) => string
  promise: typeof toast.promise
  dismiss: typeof toast.dismiss
}

// Keep a stable reference so effect/callback deps do not churn on every render.
const toastAPI: ToastAPI = {
  success: (message: Renderable) => toast.success(message),
  error: (message: Renderable) => toast.error(message),
  loading: (message: Renderable) => toast.loading(message),
  promise: toast.promise,
  dismiss: toast.dismiss,
}

export function useToast() {
  return toastAPI
}

