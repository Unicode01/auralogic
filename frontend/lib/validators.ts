import * as z from 'zod'

interface ValidatorMessages {
  invalidEmail?: string
  passwordMin6?: string
  passwordMin8?: string
  nameMin2?: string
  passwordMismatch?: string
}

// 登录表单验证
export function createLoginSchema(m?: ValidatorMessages) {
  return z.object({
    email: z.string().email(m?.invalidEmail || 'Invalid email format'),
    password: z.string().min(6, m?.passwordMin6 || 'Password must be at least 6 characters'),
  })
}
export const loginSchema = createLoginSchema()

// 注册表单验证
export function createRegisterSchema(m?: ValidatorMessages) {
  return z.object({
    email: z.string().email(m?.invalidEmail || 'Invalid email format'),
    name: z.string().min(2, m?.nameMin2 || 'Name must be at least 2 characters'),
    password: z.string().min(8, m?.passwordMin8 || 'Password must be at least 8 characters'),
    confirm_password: z.string(),
  }).refine((data) => data.password === data.confirm_password, {
    message: m?.passwordMismatch || 'Passwords do not match',
    path: ['confirm_password'],
  })
}
export const registerSchema = createRegisterSchema()

// 发货信息表单验证（基础版本 - 国内地址）
export const shippingFormSchema = z.object({
  receiver_name: z.string().min(2, '姓名至少2个字符'),
  phone_code: z.string().default('+86'),  // 手机区号，默认+86
  receiver_phone: z.string().min(8, '手机号至少8位'),
  receiver_email: z.string().email('邮箱格式错误'),
  receiver_country: z.string().min(2, '请选择国家/地区'),
  receiver_province: z.string().optional(),
  receiver_city: z.string().optional(),
  receiver_district: z.string().optional(),
  receiver_address: z.string().min(5, '详细地址至少5个字符'),
  receiver_postcode: z.string().optional(),
  privacy_protected: z.boolean().optional(),
  password: z.string().min(8, '密码至少8位').optional().or(z.literal('')),
  user_remark: z.string().max(500, '备注最多500个字符').optional().or(z.literal('')),
})

// 修改密码验证
export const changePasswordSchema = z.object({
  old_password: z.string().min(6, '密码至少6位'),
  new_password: z.string().min(8, '新密码至少8位'),
  confirm_password: z.string(),
}).refine((data) => data.new_password === data.confirm_password, {
  message: '两次输入的密码不一致',
  path: ['confirm_password'],
})

// 创建管理员验证
export const createAdminSchema = z.object({
  email: z.string().email('邮箱格式错误'),
  password: z.string().min(8, '密码至少8位'),
  name: z.string().min(2, '姓名至少2个字符'),
  permissions: z.array(z.string()).min(1, '至少选择一个权限'),
})

// 创建用户验证
export const createUserSchema = z.object({
  email: z.string().email('邮箱格式错误'),
  password: z.string().min(8, '密码至少8位'),
  name: z.string().min(2, '姓名至少2个字符').optional(),
  role: z.enum(['user', 'admin', 'super_admin']).default('user'),
})
