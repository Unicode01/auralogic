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
  createdAt: string
  created_at?: string
}

export interface LoginResponse {
  token: string
  token_type: string
  expires_in: number
  user: User
}

