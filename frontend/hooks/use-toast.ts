'use client'

import toast, { type Renderable } from 'react-hot-toast'

export function useToast() {
  return {
    success: (message: Renderable) => toast.success(message),
    error: (message: Renderable) => toast.error(message),
    loading: (message: Renderable) => toast.loading(message),
    promise: toast.promise,
    dismiss: toast.dismiss,
  }
}

