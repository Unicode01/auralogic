'use client'

import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getUsers, createUser, updateUser, deleteUser, getAdmins, createAdmin, updateAdmin, deleteAdmin, createAdminOrder, getVirtualInventories } from '@/lib/api'
import { DataTable } from '@/components/admin/data-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from '@/components/ui/form'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Checkbox } from '@/components/ui/checkbox'
import { Textarea } from '@/components/ui/textarea'
import { Search, UserPlus, Shield, Trash2, Edit, ShoppingBag, Plus, X } from 'lucide-react'
import { getRoleColor } from '@/lib/role-utils'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { createUserSchema, createAdminSchema } from '@/lib/validators'
import { useToast } from '@/hooks/use-toast'
import { PERMISSIONS, PERMISSIONS_BY_CATEGORY, CATEGORY_LABEL_KEYS } from '@/lib/constants'
import Link from 'next/link'
import * as z from 'zod'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'

type ViewMode = 'all' | 'users' | 'admins'

export default function AdminUsersPage() {
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [viewMode, setViewMode] = useState<ViewMode>('all')
  const [openUser, setOpenUser] = useState(false)
  const [openAdmin, setOpenAdmin] = useState(false)
  const [editingUser, setEditingUser] = useState<any>(null)
  const [openEdit, setOpenEdit] = useState(false)
  const [editingRole, setEditingRole] = useState<string>('user')
  const [editingIsActive, setEditingIsActive] = useState<'true' | 'false'>('true')
  const [editingPermissions, setEditingPermissions] = useState<string[]>([])
  const [openCreateOrder, setOpenCreateOrder] = useState(false)
  const [createOrderUser, setCreateOrderUser] = useState<any>(null)
  const [orderItems, setOrderItems] = useState<{ sku: string; name: string; quantity: number; unit_price: number; product_type: string; virtual_inventory_id?: number }[]>([{ sku: '', name: '', quantity: 1, unit_price: 0, product_type: 'physical' }])
  const queryClient = useQueryClient()
  const toast = useToast()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminUsers)

  const { data: usersData, isLoading: usersLoading } = useQuery({
    queryKey: ['adminUsers', page, search],
    queryFn: () => getUsers({ page, limit: 20, search }),
  })

  const { data: adminsData, isLoading: adminsLoading } = useQuery({
    queryKey: ['admins'],
    queryFn: () => getAdmins(),
  })

  const { data: virtualInventoriesData } = useQuery({
    queryKey: ['virtualInventories'],
    queryFn: () => getVirtualInventories({ limit: 100 }),
  })

  const userForm = useForm({
    resolver: zodResolver(createUserSchema),
    defaultValues: {
      email: '',
      password: '',
      name: '',
      role: 'user' as const,
    },
  })

  const adminForm = useForm<z.infer<typeof createAdminSchema>>({
    resolver: zodResolver(createAdminSchema),
    defaultValues: {
      email: '',
      password: '',
      name: '',
      permissions: [],
    },
  })

  const createUserMutation = useMutation({
    mutationFn: createUser,
    onSuccess: () => {
      toast.success(t.admin.userCreated)
      queryClient.invalidateQueries({ queryKey: ['adminUsers'] })
      setOpenUser(false)
      userForm.reset()
    },
    onError: (error: any) => {
      toast.error(error.message || t.admin.createFailed)
    },
  })

  const deleteUserMutation = useMutation({
    mutationFn: deleteUser,
    onSuccess: () => {
      toast.success(t.admin.userDeleted)
      queryClient.invalidateQueries({ queryKey: ['adminUsers'] })
    },
    onError: (error: any) => {
      toast.error(error.message || t.admin.deleteFailed)
    },
  })

  const createAdminMutation = useMutation({
    mutationFn: createAdmin,
    onSuccess: () => {
      toast.success(t.admin.adminCreated)
      queryClient.invalidateQueries({ queryKey: ['admins'] })
      queryClient.invalidateQueries({ queryKey: ['adminUsers'] })
      setOpenAdmin(false)
      adminForm.reset()
    },
    onError: (error: any) => {
      toast.error(error.message || t.admin.createFailed)
    },
  })

  const updateUserMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: any }) =>
      editingUser?.role === 'user' ? updateUser(id, data) : updateAdmin(id, data),
    onSuccess: () => {
      toast.success(t.admin.updateSuccess)
      queryClient.invalidateQueries({ queryKey: ['adminUsers'] })
      queryClient.invalidateQueries({ queryKey: ['admins'] })
      setOpenEdit(false)
      setEditingUser(null)
    },
    onError: (error: any) => {
      toast.error(error.message || t.admin.updateFailed)
    },
  })

  const deleteAdminMutation = useMutation({
    mutationFn: deleteAdmin,
    onSuccess: () => {
      toast.success(t.admin.adminDeleted)
      queryClient.invalidateQueries({ queryKey: ['admins'] })
      queryClient.invalidateQueries({ queryKey: ['adminUsers'] })
    },
    onError: (error: any) => {
      toast.error(error.message || t.admin.deleteFailed)
    },
  })

  const createOrderMutation = useMutation({
    mutationFn: createAdminOrder,
    onSuccess: () => {
      toast.success(t.admin.orderCreated)
      setOpenCreateOrder(false)
      setCreateOrderUser(null)
      setOrderItems([{ sku: '', name: '', quantity: 1, unit_price: 0, product_type: 'physical' }])
    },
    onError: (error: any) => {
      toast.error(error.message || t.admin.orderCreateFailed)
    },
  })

  function onSubmitUser(values: z.infer<typeof createUserSchema>) {
    createUserMutation.mutate(values)
  }

  function onSubmitAdmin(values: z.infer<typeof createAdminSchema>) {
    createAdminMutation.mutate(values)
  }

  function handleEdit(user: any) {
    setEditingUser(user)
    setEditingRole(user.role)
    setEditingIsActive((user.is_active || user.isActive) ? 'true' : 'false')
    setEditingPermissions(user.permissions || [])
    setOpenEdit(true)
  }

  function handleUpdate(data: any) {
    if (editingUser) {
      updateUserMutation.mutate({ id: editingUser.id, data })
    }
  }

  const allUsers = usersData?.data?.items || []
  const allAdmins = adminsData?.data?.items || []

  let displayData = []
  if (viewMode === 'all') {
    displayData = allUsers
  } else if (viewMode === 'users') {
    displayData = allUsers.filter((u: any) => u.role === 'user')
  } else {
    displayData = allUsers.filter((u: any) => u.role === 'admin' || u.role === 'super_admin')
  }

  const isLoading = usersLoading

  const columns = [
    {
      header: 'ID',
      accessorKey: 'id',
    },
    {
      header: t.admin.email,
      accessorKey: 'email',
    },
    {
      header: t.admin.name,
      accessorKey: 'name',
      cell: ({ row }: { row: { original: any } }) => row.original.name || '-',
    },
    {
      header: t.admin.role,
      cell: ({ row }: { row: { original: any } }) => (
        <Badge variant={getRoleColor(row.original.role)}>
          {{ user: t.admin.normalUser, admin: t.admin.admin, super_admin: t.admin.superAdminRole }[row.original.role as string] || row.original.role}
        </Badge>
      ),
    },
    {
      header: t.admin.permissions,
      cell: ({ row }: { row: { original: any } }) => {
        const isAdmin = row.original.role === 'admin' || row.original.role === 'super_admin'
        const permCount = row.original.permissions?.length || 0
        return isAdmin ? <span className="text-sm text-muted-foreground">{t.admin.permissionCount.replace('{count}', String(permCount))}</span> : '-'
      },
    },
    {
      header: t.admin.status,
      cell: ({ row }: { row: { original: any } }) => (
        <Badge variant={row.original.isActive || row.original.is_active ? 'active' : 'secondary'}>
          {row.original.isActive || row.original.is_active ? t.admin.active : t.admin.inactive}
        </Badge>
      ),
    },
    {
      header: t.admin.actions,
      cell: ({ row }: { row: { original: any } }) => (
        <div className="flex gap-2">
          <Button
            asChild
            size="sm"
            variant="secondary"
          >
            <Link href={`/admin/orders?user_id=${row.original.id}`}>
              {t.admin.viewOrders}
            </Link>
          </Button>
          <Button
            size="sm"
            variant="secondary"
            onClick={() => {
              setCreateOrderUser(row.original)
              setOrderItems([{ sku: '', name: '', quantity: 1, unit_price: 0, product_type: 'physical' }])
              setOpenCreateOrder(true)
            }}
          >
            <ShoppingBag className="h-4 w-4 mr-1" />
            {t.admin.createOrderForUser}
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => handleEdit(row.original)}
          >
            <Edit className="h-4 w-4 mr-1" />
            {t.common.edit}
          </Button>
          {row.original.role === 'user' && (
            <Button
              size="sm"
              variant="destructive"
              onClick={() => {
                if (confirm(t.admin.confirmDeleteUser.replace('{email}', row.original.email))) {
                  deleteUserMutation.mutate(row.original.id)
                }
              }}
              disabled={deleteUserMutation.isPending}
            >
              <Trash2 className="h-4 w-4" />
            </Button>
          )}
          {(row.original.role === 'admin' || row.original.role === 'super_admin') && (
            <Button
              size="sm"
              variant="destructive"
              onClick={() => {
                if (confirm(t.admin.confirmDeleteAdmin.replace('{email}', row.original.email))) {
                  deleteAdminMutation.mutate(row.original.id)
                }
              }}
              disabled={deleteAdminMutation.isPending}
            >
              <Trash2 className="h-4 w-4" />
            </Button>
          )}
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">{t.admin.userManagement}</h1>
        <div className="flex gap-2">
          <Dialog open={openUser} onOpenChange={setOpenUser}>
            <DialogTrigger asChild>
              <Button variant="outline">
                <UserPlus className="mr-2 h-4 w-4" />
                {t.admin.addUser}
              </Button>
            </DialogTrigger>
            <DialogContent className="max-w-md">
              <DialogHeader>
                <DialogTitle>{t.admin.createNewUser}</DialogTitle>
              </DialogHeader>
              <Form {...userForm}>
                <form onSubmit={userForm.handleSubmit(onSubmitUser)} className="space-y-4">
                  <FormField
                    control={userForm.control}
                    name="email"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t.admin.emailRequired}</FormLabel>
                        <FormControl>
                          <Input type="email" placeholder="user@example.com" {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={userForm.control}
                    name="name"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t.admin.name}</FormLabel>
                        <FormControl>
                          <Input placeholder={t.admin.userNamePlaceholder} {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={userForm.control}
                    name="password"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t.admin.passwordRequired}</FormLabel>
                        <FormControl>
                          <Input type="password" placeholder={t.admin.passwordHint} {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={userForm.control}
                    name="role"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t.admin.roleRequired}</FormLabel>
                        <Select onValueChange={field.onChange} defaultValue={field.value}>
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue placeholder={t.admin.selectRole} />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectItem value="user">{t.admin.normalUser}</SelectItem>
                            <SelectItem value="admin">{t.admin.admin}</SelectItem>
                            <SelectItem value="super_admin">{t.admin.superAdminRole}</SelectItem>
                          </SelectContent>
                        </Select>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <div className="flex gap-2">
                    <Button type="submit" disabled={createUserMutation.isPending}>
                      {createUserMutation.isPending ? t.admin.creating : t.admin.createUser}
                    </Button>
                    <Button type="button" variant="outline" onClick={() => setOpenUser(false)}>
                      {t.common.cancel}
                    </Button>
                  </div>
                </form>
              </Form>
            </DialogContent>
          </Dialog>

          <Dialog open={openAdmin} onOpenChange={setOpenAdmin}>
            <DialogTrigger asChild>
              <Button>
                <Shield className="mr-2 h-4 w-4" />
                {t.admin.addAdmin}
              </Button>
            </DialogTrigger>
            <DialogContent className="max-w-2xl">
              <DialogHeader>
                <DialogTitle>{t.admin.createNewAdmin}</DialogTitle>
              </DialogHeader>
              <Form {...adminForm}>
                <form onSubmit={adminForm.handleSubmit(onSubmitAdmin)} className="space-y-4">
                  <FormField
                    control={adminForm.control}
                    name="email"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t.admin.emailRequired}</FormLabel>
                        <FormControl>
                          <Input type="email" placeholder="admin@example.com" {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={adminForm.control}
                    name="name"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t.admin.nameRequired}</FormLabel>
                        <FormControl>
                          <Input placeholder={t.admin.adminNamePlaceholder} {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={adminForm.control}
                    name="password"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t.admin.passwordRequired}</FormLabel>
                        <FormControl>
                          <Input type="password" placeholder={t.admin.passwordHint} {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={adminForm.control}
                    name="permissions"
                    render={() => (
                      <FormItem className="flex flex-col min-h-0">
                        <FormLabel className="shrink-0">{t.admin.permissionsRequired} <span className="text-sm text-muted-foreground font-normal">{t.admin.permissionsHint}</span></FormLabel>
                        <div className="flex gap-2 mb-2">
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            onClick={() => adminForm.setValue('permissions', PERMISSIONS.map(p => p.value))}
                          >
                            {t.admin.permSelectAll}
                          </Button>
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            onClick={() => adminForm.setValue('permissions', [])}
                          >
                            {t.admin.permDeselectAll}
                          </Button>
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            onClick={() => {
                              const current = adminForm.getValues('permissions') || []
                              const allValues = PERMISSIONS.map(p => p.value)
                              adminForm.setValue('permissions', allValues.filter(v => !current.includes(v)))
                            }}
                          >
                            {t.admin.permInvertSelection}
                          </Button>
                        </div>
                        <div className="border rounded-md p-4 h-64 overflow-y-auto space-y-4">
                          {Object.entries(PERMISSIONS_BY_CATEGORY).map(([category, perms]) => (
                            <div key={category} className="space-y-2">
                              <div className="font-medium text-sm text-primary border-b pb-1 flex items-center justify-between">
                                <span>{t.admin[CATEGORY_LABEL_KEYS[category] as keyof typeof t.admin] || category}</span>
                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="sm"
                                  className="h-5 px-1.5 text-xs"
                                  onClick={() => {
                                    const current = adminForm.getValues('permissions') || []
                                    const categoryValues = perms.map(p => p.value)
                                    const allSelected = categoryValues.every(v => current.includes(v))
                                    if (allSelected) {
                                      adminForm.setValue('permissions', current.filter(v => !categoryValues.includes(v)))
                                    } else {
                                      adminForm.setValue('permissions', [...new Set([...current, ...categoryValues])])
                                    }
                                  }}
                                >
                                  {(adminForm.watch('permissions') || []).length > 0 && perms.map(p => p.value).every(v => (adminForm.watch('permissions') || []).includes(v)) ? t.admin.permDeselectAll : t.admin.permSelectAll}
                                </Button>
                              </div>
                              <div className="grid grid-cols-2 gap-2 pl-2">
                                {perms.map((permission) => (
                                  <FormField
                                    key={permission.value}
                                    control={adminForm.control}
                                    name="permissions"
                                    render={({ field }) => (
                                      <FormItem className="flex items-start space-x-2 space-y-0">
                                        <FormControl>
                                          <Checkbox
                                            checked={field.value?.includes(permission.value)}
                                            onCheckedChange={(checked) => {
                                              const currentValue = field.value || []
                                              const permValue = permission.value
                                              if (checked) {
                                                field.onChange([...currentValue, permValue])
                                              } else {
                                                field.onChange(currentValue.filter((v) => v !== permValue))
                                              }
                                            }}
                                          />
                                        </FormControl>
                                        <FormLabel className="text-sm font-normal cursor-pointer leading-tight">
                                          {t.admin[permission.labelKey as keyof typeof t.admin] || permission.value}
                                        </FormLabel>
                                      </FormItem>
                                    )}
                                  />
                                ))}
                              </div>
                            </div>
                          ))}
                        </div>
                        <p className="text-xs text-muted-foreground mt-2">
                          {t.admin.selectedPermissions.replace('{count}', String(adminForm.watch('permissions')?.length || 0))}
                        </p>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <div className="flex gap-2">
                    <Button type="submit" disabled={createAdminMutation.isPending}>
                      {createAdminMutation.isPending ? t.admin.creating : t.admin.createAdmin}
                    </Button>
                    <Button type="button" variant="outline" onClick={() => setOpenAdmin(false)}>
                      {t.common.cancel}
                    </Button>
                  </div>
                </form>
              </Form>
            </DialogContent>
          </Dialog>
        </div>
      </div>

      <div className="flex items-center gap-4">
        <div className="flex gap-2">
          <Button
            variant={viewMode === 'all' ? 'active' : 'outline'}
            size="sm"
            onClick={() => setViewMode('all')}
          >
            {t.admin.all}
          </Button>
          <Button
            variant={viewMode === 'users' ? 'active' : 'outline'}
            size="sm"
            onClick={() => setViewMode('users')}
          >
            {t.admin.users}
          </Button>
          <Button
            variant={viewMode === 'admins' ? 'active' : 'outline'}
            size="sm"
            onClick={() => setViewMode('admins')}
          >
            {t.admin.admins}
          </Button>
        </div>
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder={t.admin.searchEmailOrName}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-10"
          />
        </div>
      </div>

      <DataTable
        columns={columns}
        data={displayData}
        isLoading={isLoading}
        pagination={{
          page,
          total_pages: usersData?.data?.pagination?.total_pages || 1,
          onPageChange: setPage,
        }}
      />

      <Dialog
        open={openEdit}
        onOpenChange={(v) => {
          setOpenEdit(v)
          if (!v) {
            setEditingUser(null)
            setEditingRole('user')
            setEditingIsActive('true')
            setEditingPermissions([])
          }
        }}
      >
        <DialogContent className={editingUser?.role !== 'user' ? "max-w-2xl" : "max-w-md"}>
          <DialogHeader>
            <DialogTitle>{editingUser?.role === 'user' ? t.admin.editUser : t.admin.editAdmin}</DialogTitle>
          </DialogHeader>
          {editingUser && (
            <form
              onSubmit={(e) => {
                e.preventDefault()
                const formData = new FormData(e.currentTarget)
                const pwd = (formData.get('password') as string) || ''

                if (pwd && pwd.length < 8) {
                  toast.error((t.auth.passwordMinLength as string).replace('{n}', '8'))
                  return
                }

                const data: any = {
                  name: formData.get('name') as string,
                  role: formData.get('role') as string,
                  is_active: formData.get('is_active') === 'true',
                }
                if (pwd) data.password = pwd
                if (editingUser.role !== 'user') {
                  data.permissions = editingPermissions
                }
                handleUpdate(data)
              }}
              className="space-y-4"
            >
              <div>
                <label className="text-sm font-medium">{t.admin.email}</label>
                <Input value={editingUser.email} disabled className="mt-1.5" />
              </div>

              <div>
                <label className="text-sm font-medium">{t.admin.name}</label>
                <Input
                  name="name"
                  defaultValue={editingUser.name}
                  className="mt-1.5"
                  required
                />
              </div>

              <div>
                <label className="text-sm font-medium">{t.admin.role}</label>
                <input type="hidden" name="role" value={editingRole} />
                <Select value={editingRole} onValueChange={setEditingRole}>
                  <SelectTrigger className="mt-1.5">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="user">{t.admin.normalUser}</SelectItem>
                    <SelectItem value="admin">{t.admin.admin}</SelectItem>
                    <SelectItem value="super_admin">{t.admin.superAdminRole}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div>
                <label className="text-sm font-medium">{t.admin.status}</label>
                <input type="hidden" name="is_active" value={editingIsActive} />
                <Select value={editingIsActive} onValueChange={(v) => setEditingIsActive(v as 'true' | 'false')}>
                  <SelectTrigger className="mt-1.5">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="true">{t.admin.active}</SelectItem>
                    <SelectItem value="false">{t.admin.inactive}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div>
                <label className="text-sm font-medium">{t.profile.newPassword}</label>
                <Input
                  name="password"
                  type="password"
                  placeholder={t.admin.passwordPlaceholder}
                  className="mt-1.5"
                />
                <p className="text-xs text-muted-foreground mt-1">{t.profile.passwordRequirement}</p>
              </div>

              {editingUser.role !== 'user' && (
                <div className="flex flex-col min-h-0">
                  <label className="text-sm font-medium shrink-0">
                    {t.admin.permissions} <span className="text-sm text-muted-foreground font-normal">{t.admin.selectedPermissions.replace('{count}', String(editingPermissions.length))}</span>
                  </label>
                  <div className="flex gap-2 mt-1.5 mb-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => setEditingPermissions(PERMISSIONS.map(p => p.value))}
                    >
                      {t.admin.permSelectAll}
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => setEditingPermissions([])}
                    >
                      {t.admin.permDeselectAll}
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        const allValues = PERMISSIONS.map(p => p.value)
                        setEditingPermissions(allValues.filter(v => !editingPermissions.includes(v)))
                      }}
                    >
                      {t.admin.permInvertSelection}
                    </Button>
                  </div>
                  <div className="border rounded-md p-4 h-64 overflow-y-auto space-y-4">
                    {Object.entries(PERMISSIONS_BY_CATEGORY).map(([category, perms]) => (
                      <div key={category} className="space-y-2">
                        <div className="font-medium text-sm text-primary border-b pb-1 flex items-center justify-between">
                          <span>{t.admin[CATEGORY_LABEL_KEYS[category] as keyof typeof t.admin] || category}</span>
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            className="h-5 px-1.5 text-xs"
                            onClick={() => {
                              const categoryValues = perms.map(p => p.value)
                              const allSelected = categoryValues.every(v => editingPermissions.includes(v))
                              if (allSelected) {
                                setEditingPermissions(editingPermissions.filter(v => !categoryValues.includes(v)))
                              } else {
                                setEditingPermissions([...new Set([...editingPermissions, ...categoryValues])])
                              }
                            }}
                          >
                            {perms.map(p => p.value).every(v => editingPermissions.includes(v)) ? t.admin.permDeselectAll : t.admin.permSelectAll}
                          </Button>
                        </div>
                        <div className="grid grid-cols-2 gap-2 pl-2">
                          {perms.map((permission) => (
                            <div key={permission.value} className="flex items-start space-x-2">
                              <Checkbox
                                id={`edit-${permission.value}`}
                                checked={editingPermissions.includes(permission.value)}
                                onCheckedChange={(checked) => {
                                  if (checked) {
                                    setEditingPermissions([...editingPermissions, permission.value])
                                  } else {
                                    setEditingPermissions(editingPermissions.filter((v) => v !== permission.value))
                                  }
                                }}
                              />
                              <label
                                htmlFor={`edit-${permission.value}`}
                                className="text-sm font-normal cursor-pointer leading-tight"
                              >
                                {t.admin[permission.labelKey as keyof typeof t.admin] || permission.value}
                              </label>
                            </div>
                          ))}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              <div className="flex gap-2">
                <Button type="submit" disabled={updateUserMutation.isPending}>
                  {updateUserMutation.isPending ? t.admin.saving : t.common.save}
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => {
                    setOpenEdit(false)
                    setEditingUser(null)
                    setEditingRole('user')
                    setEditingIsActive('true')
                  }}
                >
                  {t.common.cancel}
                </Button>
              </div>
            </form>
          )}
        </DialogContent>
      </Dialog>

      <Dialog
        open={openCreateOrder}
        onOpenChange={(v) => {
          setOpenCreateOrder(v)
          if (!v) {
            setCreateOrderUser(null)
            setOrderItems([{ sku: '', name: '', quantity: 1, unit_price: 0, product_type: 'physical' }])
          }
        }}
      >
        <DialogContent className="max-w-3xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{t.admin.createOrderTitle} - {createOrderUser?.email}</DialogTitle>
          </DialogHeader>
          {createOrderUser && (
            <form
              onSubmit={(e) => {
                e.preventDefault()
                const formData = new FormData(e.currentTarget)
                const validItems = orderItems.filter(item => item.sku && item.name && item.quantity > 0)
                if (validItems.length === 0) {
                  toast.error(t.admin.addItem)
                  return
                }
                const missingInventory = validItems.find(item => item.product_type === 'virtual' && !item.virtual_inventory_id)
                if (missingInventory) {
                  toast.error(t.admin.pleaseSelectVirtualInventory)
                  return
                }
                const totalAmountStr = formData.get('total_amount') as string
                const data: any = {
                  user_id: createOrderUser.id,
                  items: validItems,
                  remark: formData.get('remark') as string,
                  admin_remark: formData.get('admin_remark') as string,
                }
                if (totalAmountStr) {
                  data.total_amount = parseFloat(totalAmountStr)
                }
                createOrderMutation.mutate(data)
              }}
              className="space-y-6"
            >
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-sm font-medium">{t.admin.orderItems} *</label>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => setOrderItems([...orderItems, { sku: '', name: '', quantity: 1, unit_price: 0, product_type: 'physical' }])}
                  >
                    <Plus className="h-4 w-4 mr-1" />
                    {t.admin.addItem}
                  </Button>
                </div>
                <div className="space-y-3">
                  {orderItems.map((item, index) => (
                    <div key={index} className={`rounded-md border p-3 space-y-2 ${item.product_type === 'virtual' ? 'border-blue-200 bg-blue-50/30 dark:border-blue-800 dark:bg-blue-950/20' : ''}`}>
                      <div className="flex gap-2 items-start">
                        <div className="flex-1">
                          <Input
                            placeholder={t.admin.sku}
                            value={item.sku}
                            onChange={(e) => {
                              const newItems = [...orderItems]
                              newItems[index].sku = e.target.value
                              setOrderItems(newItems)
                            }}
                          />
                        </div>
                        <div className="flex-[2]">
                          <Input
                            placeholder={t.admin.productName}
                            value={item.name}
                            onChange={(e) => {
                              const newItems = [...orderItems]
                              newItems[index].name = e.target.value
                              setOrderItems(newItems)
                            }}
                          />
                        </div>
                        <div className="w-28">
                          <Select
                            value={item.product_type}
                            onValueChange={(v) => {
                              const newItems = [...orderItems]
                              newItems[index].product_type = v
                              if (v !== 'virtual') {
                                newItems[index].virtual_inventory_id = undefined
                              }
                              setOrderItems(newItems)
                            }}
                          >
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="physical">{t.admin.physicalProduct}</SelectItem>
                              <SelectItem value="virtual">{t.admin.virtualProduct}</SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                        <div className="w-24">
                          <Input
                            type="number"
                            placeholder={t.admin.unitPrice}
                            value={item.unit_price || ''}
                            min={0}
                            step="0.01"
                            onChange={(e) => {
                              const newItems = [...orderItems]
                              newItems[index].unit_price = parseFloat(e.target.value) || 0
                              setOrderItems(newItems)
                            }}
                          />
                        </div>
                        <div className="w-20">
                          <Input
                            type="number"
                            placeholder={t.admin.quantity}
                            value={item.quantity}
                            min={1}
                            onChange={(e) => {
                              const newItems = [...orderItems]
                              newItems[index].quantity = parseInt(e.target.value) || 1
                              setOrderItems(newItems)
                            }}
                          />
                        </div>
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          className="shrink-0"
                          disabled={orderItems.length <= 1}
                          onClick={() => setOrderItems(orderItems.filter((_, i) => i !== index))}
                        >
                          <X className="h-4 w-4" />
                        </Button>
                      </div>
                      {item.product_type === 'virtual' && (
                        <div>
                          <label className="text-xs text-muted-foreground mb-1 block">{t.admin.selectVirtualLabel} *</label>
                          <Select
                            value={item.virtual_inventory_id ? String(item.virtual_inventory_id) : ''}
                            onValueChange={(v) => {
                              const newItems = [...orderItems]
                              newItems[index].virtual_inventory_id = v ? Number(v) : undefined
                              setOrderItems(newItems)
                            }}
                          >
                            <SelectTrigger className={`w-full ${!item.virtual_inventory_id ? 'border-destructive' : ''}`}>
                              <SelectValue placeholder={t.admin.selectVirtualPlaceholder} />
                            </SelectTrigger>
                            <SelectContent>
                              {(virtualInventoriesData?.data?.items || []).map((vi: any) => (
                                <SelectItem key={vi.id} value={String(vi.id)}>
                                  {vi.name} ({vi.sku}) - {t.admin.availableCol}: {vi.available ?? 0}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              </div>

              <p className="text-xs text-muted-foreground">{t.admin.shippingInfoHint}</p>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-sm font-medium">{t.admin.orderRemark}</label>
                  <Textarea name="remark" placeholder={t.admin.orderRemark} className="mt-1.5" rows={2} />
                </div>
                <div>
                  <label className="text-sm font-medium">{t.admin.adminRemarkLabel}</label>
                  <Textarea name="admin_remark" placeholder={t.admin.adminRemarkLabel} className="mt-1.5" rows={2} />
                </div>
              </div>

              <div>
                <label className="text-sm font-medium">{t.admin.totalAmountOverride}</label>
                <Input name="total_amount" type="number" step="0.01" min={0} placeholder="0.00" className="mt-1.5" />
              </div>

              <div className="flex gap-2">
                <Button type="submit" disabled={createOrderMutation.isPending}>
                  {createOrderMutation.isPending ? t.admin.creating : t.common.create}
                </Button>
                <Button type="button" variant="outline" onClick={() => setOpenCreateOrder(false)}>
                  {t.common.cancel}
                </Button>
              </div>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
