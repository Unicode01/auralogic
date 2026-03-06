export interface User {
  id: number
  uuid: string
  email: string
  name: string
  avatar?: string
  role: 'user' | 'admin' | 'super_admin'
  permissions?: string[]
  isActive: boolean
  is_active?: boolean
  total_spent_minor?: number
  total_order_count?: number
  createdAt: string
  created_at?: string
}

export interface LoginResponse {
  token: string
  token_type: string
  expires_in: number
  user: User
}

