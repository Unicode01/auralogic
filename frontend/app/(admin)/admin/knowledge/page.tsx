'use client'

import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  KnowledgeArticle,
  KnowledgeCategory,
  createKnowledgeArticle,
  createKnowledgeCategory,
  deleteKnowledgeArticle,
  deleteKnowledgeCategory,
  getAdminKnowledgeArticle,
  getAdminKnowledgeArticles,
  getAdminKnowledgeCategories,
  updateKnowledgeArticle,
  updateKnowledgeCategory,
} from '@/lib/api'
import toast from 'react-hot-toast'
import { getTranslations } from '@/lib/i18n'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { resolveClientAPIProxyURL } from '@/lib/api-base-url'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { PluginExtensionList } from '@/components/plugins/plugin-extension-list'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { MarkdownEditor } from '@/components/ui/markdown-editor'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { MarkdownMessage } from '@/components/ui/markdown-message'
import {
  ChevronRight,
  Download,
  FileText,
  FolderTree,
  Loader2,
  Pencil,
  Plus,
  Save,
  Search,
  Trash2,
  Upload,
  X,
} from 'lucide-react'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { usePluginExtensionBatch } from '@/lib/plugin-extension-batch'
import { usePermission } from '@/hooks/use-permission'

interface CategoryFormData {
  name: string
  parent_id?: number
  sort_order: number
}

interface ArticleFormData {
  title: string
  category_id?: number
  sort_order: number
  content: string
}

type ArticleEditorMode = 'empty' | 'create' | 'edit'

const EMPTY_KNOWLEDGE_CATEGORIES: KnowledgeCategory[] = []

function flattenCategories(
  cats: KnowledgeCategory[],
  depth = 0
): { id: number; name: string; depth: number }[] {
  const result: { id: number; name: string; depth: number }[] = []
  for (const cat of cats) {
    result.push({ id: cat.id, name: cat.name, depth })
    if (cat.children?.length) {
      result.push(...flattenCategories(cat.children, depth + 1))
    }
  }
  return result
}

function createEmptyArticleForm(categoryId?: number): ArticleFormData {
  return {
    title: '',
    category_id: categoryId,
    sort_order: 0,
    content: '',
  }
}

function findCategoryById(categories: KnowledgeCategory[], id: number): KnowledgeCategory | null {
  for (const category of categories) {
    if (category.id === id) {
      return category
    }
    if (category.children?.length) {
      const matched = findCategoryById(category.children, id)
      if (matched) {
        return matched
      }
    }
  }
  return null
}

function formatKnowledgeDateTime(value?: string) {
  if (!value) {
    return '--'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString()
}

function buildAdminKnowledgeArticleRowSummary(article: KnowledgeArticle) {
  return {
    id: article.id,
    title: article.title,
    category_id: article.category_id,
    category_name: article.category?.name,
    sort_order: article.sort_order,
    content_length: article.content?.length || 0,
    created_at: article.created_at,
    updated_at: article.updated_at,
  }
}

export default function AdminKnowledgePage() {
  const queryClient = useQueryClient()
  const { locale } = useLocale()
  const { hasPermission } = usePermission()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminKnowledge)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const canEditKnowledge = hasPermission('knowledge.edit')
  const formatKnowledgeError = (error: unknown, fallback: string) => {
    const detail = resolveApiErrorMessage(error, t, fallback)
    return detail === fallback ? fallback : `${fallback}: ${detail}`
  }
  const readFetchErrorMessage = async (response: Response, fallback: string) => {
    try {
      const contentType = response.headers.get('content-type') || ''
      if (contentType.includes('application/json')) {
        const payload = await response.json()
        return resolveApiErrorMessage(payload, t, fallback)
      }

      const text = (await response.text()).trim()
      if (text) {
        try {
          return resolveApiErrorMessage(JSON.parse(text), t, fallback)
        } catch {
          return text
        }
      }
    } catch {
      // ignore parse errors and fall back to the provided message
    }
    return fallback
  }

  // Category dialog state
  const [categoryDialogOpen, setCategoryDialogOpen] = useState(false)
  const [editingCategory, setEditingCategory] = useState<KnowledgeCategory | null>(null)
  const [categoryForm, setCategoryForm] = useState<CategoryFormData>({
    name: '',
    parent_id: undefined,
    sort_order: 0,
  })

  // Delete confirmation state
  const [deleteCategoryId, setDeleteCategoryId] = useState<number | null>(null)
  const [deleteArticleId, setDeleteArticleId] = useState<number | null>(null)

  // Articles filter state
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')
  const [categoryId, setCategoryId] = useState<string | undefined>()
  const limit = 20

  // Editor state
  const [editorMode, setEditorMode] = useState<ArticleEditorMode>('empty')
  const [selectedArticleId, setSelectedArticleId] = useState<number | null>(null)
  const [collapsedCategoryIds, setCollapsedCategoryIds] = useState<Set<number>>(new Set())
  const [articleForm, setArticleForm] = useState<ArticleFormData>(createEmptyArticleForm())
  const [articleBaseline, setArticleBaseline] = useState<ArticleFormData | null>(null)

  const {
    data: categoriesData,
    isLoading: categoriesLoading,
    isError: categoriesLoadFailed,
    refetch: refetchCategories,
  } = useQuery({
    queryKey: ['adminKnowledgeCategories'],
    queryFn: getAdminKnowledgeCategories,
  })

  const categories: KnowledgeCategory[] = categoriesData?.data ?? EMPTY_KNOWLEDGE_CATEGORIES
  const flatCategories = useMemo(() => flattenCategories(categories), [categories])
  const topLevelCategories = useMemo(() => categories.filter((c) => !c.parent_id), [categories])

  const getCategoryTotalArticleCount = (category: KnowledgeCategory): number => {
    if (typeof category.total_article_count === 'number') {
      return category.total_article_count
    }
    const own = category.article_count || 0
    const children =
      category.children?.reduce((sum, child) => sum + getCategoryTotalArticleCount(child), 0) || 0
    return own + children
  }

  const {
    data: articlesData,
    isLoading: articlesLoading,
    isError: articlesLoadFailed,
    refetch: refetchArticles,
  } = useQuery({
    queryKey: ['adminKnowledgeArticles', page, categoryId, search],
    queryFn: () =>
      getAdminKnowledgeArticles({
        page,
        limit,
        category_id: categoryId,
        search: search || undefined,
      }),
  })

  const articles: KnowledgeArticle[] = articlesData?.data?.items || []
  const totalArticles = Number(articlesData?.data?.pagination?.total || 0)
  const totalPages = Number(articlesData?.data?.pagination?.total_pages || 0) || 1

  const {
    data: selectedArticleData,
    isLoading: selectedArticleLoading,
    isError: selectedArticleLoadFailed,
    refetch: refetchSelectedArticle,
  } = useQuery({
    queryKey: ['adminKnowledgeArticle', selectedArticleId],
    queryFn: () => getAdminKnowledgeArticle(selectedArticleId as number),
    enabled: editorMode === 'edit' && !!selectedArticleId,
    refetchOnMount: 'always',
    staleTime: 0,
  })

  useEffect(() => {
    if (editorMode !== 'edit') return
    if (!selectedArticleData?.data) return

    const a = selectedArticleData.data as KnowledgeArticle
    const nextForm = {
      title: a.title || '',
      category_id: a.category_id || undefined,
      sort_order: a.sort_order ?? 0,
      content: a.content || '',
    }
    setArticleForm(nextForm)
    setArticleBaseline(nextForm)
  }, [editorMode, selectedArticleData])

  const createCategoryMutation = useMutation({
    mutationFn: (data: CategoryFormData) => createKnowledgeCategory(data),
    onSuccess: () => {
      toast.success(t.knowledge.categoryCreated)
      queryClient.invalidateQueries({ queryKey: ['adminKnowledgeCategories'] })
      setCategoryDialogOpen(false)
      setEditingCategory(null)
      setCategoryForm({ name: '', parent_id: undefined, sort_order: 0 })
    },
    onError: (error: unknown) => {
      toast.error(formatKnowledgeError(error, t.knowledge.createFailed))
    },
  })

  const updateCategoryMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: CategoryFormData }) =>
      updateKnowledgeCategory(id, data),
    onSuccess: () => {
      toast.success(t.knowledge.categoryUpdated)
      queryClient.invalidateQueries({ queryKey: ['adminKnowledgeCategories'] })
      setCategoryDialogOpen(false)
      setEditingCategory(null)
      setCategoryForm({ name: '', parent_id: undefined, sort_order: 0 })
    },
    onError: (error: unknown) => {
      toast.error(formatKnowledgeError(error, t.knowledge.updateFailed))
    },
  })

  const deleteCategoryMutation = useMutation({
    mutationFn: (id: number) => deleteKnowledgeCategory(id),
    onSuccess: (_res, id) => {
      toast.success(t.knowledge.categoryDeleted)
      queryClient.invalidateQueries({ queryKey: ['adminKnowledgeCategories'] })
      if (typeof id === 'number' && categoryId === id.toString()) {
        setCategoryId(undefined)
        setPage(1)
      }
      setDeleteCategoryId(null)
    },
    onError: (error: unknown) => {
      toast.error(formatKnowledgeError(error, t.knowledge.deleteFailed))
    },
  })

  const deleteArticleMutation = useMutation({
    mutationFn: (id: number) => deleteKnowledgeArticle(id),
    onSuccess: (_res, id) => {
      toast.success(t.knowledge.articleDeleted)
      queryClient.invalidateQueries({ queryKey: ['adminKnowledgeArticles'] })
      queryClient.invalidateQueries({ queryKey: ['adminKnowledgeCategories'] })
      if (typeof id === 'number' && id === selectedArticleId) {
        setEditorMode('empty')
        setSelectedArticleId(null)
        setArticleForm(createEmptyArticleForm())
        setArticleBaseline(null)
      }
      setDeleteArticleId(null)
    },
    onError: (error: unknown) => {
      toast.error(formatKnowledgeError(error, t.knowledge.deleteFailed))
    },
  })

  const importMutation = useMutation({
    mutationFn: async (file: File) => {
      const formData = new FormData()
      formData.append('file', file)
      formData.append('conflict_mode', 'upsert')

      const response = await fetch(resolveClientAPIProxyURL('/api/admin/knowledge/import'), {
        method: 'POST',
        body: formData,
      })

      if (!response.ok) {
        throw new Error(await readFetchErrorMessage(response, t.knowledge.importFailed))
      }

      return response.json()
    },
    onSuccess: (data) => {
      toast.dismiss()
      const result = data?.data
      toast.success(result?.message || t.knowledge.importSuccess, {
        duration: 4000,
      })
      queryClient.invalidateQueries({ queryKey: ['adminKnowledgeCategories'] })
      queryClient.invalidateQueries({ queryKey: ['adminKnowledgeArticles'] })
      queryClient.invalidateQueries({ queryKey: ['adminKnowledgeArticle'] })
      refetchCategories()
      refetchArticles()
      if (selectedArticleId) {
        refetchSelectedArticle()
      }
    },
    onError: (error: unknown) => {
      toast.dismiss()
      toast.error(formatKnowledgeError(error, t.knowledge.importFailed))
    },
  })

  const handleExportKnowledge = () => {
    fetch(resolveClientAPIProxyURL('/api/admin/knowledge/export'))
      .then(async (res) => {
        if (!res.ok) {
          throw new Error(await readFetchErrorMessage(res, t.admin.exportFailed))
        }
        return res.blob()
      })
      .then((blob) => {
        const blobUrl = window.URL.createObjectURL(blob)
        const anchor = document.createElement('a')
        anchor.href = blobUrl
        anchor.download = `knowledge_package_${new Date().toISOString().slice(0, 10)}.json`
        document.body.appendChild(anchor)
        anchor.click()
        document.body.removeChild(anchor)
        window.URL.revokeObjectURL(blobUrl)
        toast.success(t.knowledge.exportSuccess)
      })
      .catch((err: Error) => {
        toast.error(`${t.admin.exportFailed}: ${err.message}`)
      })
  }

  const handleImportKnowledgeClick = () => {
    fileInputRef.current?.click()
  }

  const handleKnowledgeFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) {
      return
    }

    if (!file.name.toLowerCase().endsWith('.json')) {
      toast.error(t.knowledge.jsonFileFormatError)
      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
      return
    }

    toast.loading(t.admin.importLoading)
    importMutation.mutate(file)

    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
  }

  const createArticleMutation = useMutation({
    mutationFn: (data: ArticleFormData) =>
      createKnowledgeArticle({
        title: data.title,
        content: data.content,
        category_id: data.category_id,
        sort_order: data.sort_order,
      }),
    onSuccess: (res: any) => {
      toast.success(t.knowledge.articleCreated)
      queryClient.invalidateQueries({ queryKey: ['adminKnowledgeArticles'] })
      queryClient.invalidateQueries({ queryKey: ['adminKnowledgeCategories'] })

      const maybeId = res?.data?.id
      if (typeof maybeId === 'number' && maybeId > 0) {
        setEditorMode('edit')
        setSelectedArticleId(maybeId)
        queryClient.invalidateQueries({
          queryKey: ['adminKnowledgeArticle', maybeId],
        })
        return
      }

      setEditorMode('empty')
      setSelectedArticleId(null)
      setArticleForm(createEmptyArticleForm())
      setArticleBaseline(null)
    },
    onError: (error: unknown) => {
      toast.error(formatKnowledgeError(error, t.knowledge.createFailed))
    },
  })

  const updateArticleMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: ArticleFormData }) =>
      updateKnowledgeArticle(id, {
        title: data.title,
        content: data.content,
        category_id: data.category_id,
        sort_order: data.sort_order,
      }),
    onSuccess: () => {
      toast.success(t.knowledge.articleUpdated)
      queryClient.invalidateQueries({ queryKey: ['adminKnowledgeArticles'] })
      queryClient.invalidateQueries({ queryKey: ['adminKnowledgeCategories'] })
      setArticleBaseline(articleForm)
      if (selectedArticleId) {
        queryClient.invalidateQueries({
          queryKey: ['adminKnowledgeArticle', selectedArticleId],
        })
      }
    },
    onError: (error: unknown) => {
      toast.error(formatKnowledgeError(error, t.knowledge.updateFailed))
    },
  })

  const isCategoryMutating = createCategoryMutation.isPending || updateCategoryMutation.isPending
  const isArticleSaving = createArticleMutation.isPending || updateArticleMutation.isPending
  const isEditorLoading = editorMode === 'edit' && selectedArticleLoading
  const selectedCategory = useMemo(
    () => (categoryId ? findCategoryById(categories, Number(categoryId)) : null),
    [categories, categoryId]
  )
  const selectedArticle = selectedArticleData?.data as KnowledgeArticle | undefined
  const editorNotFound =
    editorMode === 'edit' && !isEditorLoading && !selectedArticleLoadFailed && !selectedArticle
  const articleContentCharCount = articleForm.content.length
  const articleContentLineCount = articleForm.content
    ? articleForm.content.split(/\r?\n/).length
    : 0
  const visibleArticleStart = totalArticles === 0 ? 0 : (page - 1) * limit + 1
  const visibleArticleEnd = totalArticles === 0 ? 0 : Math.min(page * limit, totalArticles)
  const articleRangeSummary = totalArticles
    ? t.knowledge.articleRangeSummary
        .replace('{start}', String(visibleArticleStart))
        .replace('{end}', String(visibleArticleEnd))
        .replace('{total}', String(totalArticles))
    : t.knowledge.articleRangeSummary
        .replace('{start}', '0')
        .replace('{end}', '0')
        .replace('{total}', '0')
  const hasUnsavedArticleChanges =
    editorMode !== 'empty' && JSON.stringify(articleForm) !== JSON.stringify(articleBaseline)
  const adminKnowledgePluginContext = {
    view: 'admin_knowledge',
    filters: {
      page,
      search: search || undefined,
      category_id: categoryId ? Number(categoryId) : undefined,
    },
    pagination: {
      page,
      total: totalArticles,
      total_pages: totalPages,
      limit,
    },
    selection: {
      selected_category_id: categoryId ? Number(categoryId) : undefined,
      selected_article_id: selectedArticleId || undefined,
      editor_mode: editorMode,
      has_unsaved_changes: hasUnsavedArticleChanges,
    },
    summary: {
      category_count: categories.length,
      top_level_category_count: topLevelCategories.length,
      current_page_article_count: articles.length,
      active_filter_count: Number(Boolean(categoryId)) + Number(Boolean(search.trim())),
      article_content_char_count: articleContentCharCount,
      article_content_line_count: articleContentLineCount,
    },
    state: {
      categories_load_failed: categoriesLoadFailed && categories.length === 0,
      categories_empty: !categoriesLoading && !categoriesLoadFailed && categories.length === 0,
      articles_load_failed: articlesLoadFailed && articles.length === 0,
      articles_empty: !articlesLoading && !articlesLoadFailed && articles.length === 0,
      editor_empty: editorMode === 'empty',
      editor_loading: isEditorLoading,
      editor_load_failed: editorMode === 'edit' && selectedArticleLoadFailed && !selectedArticle,
      editor_not_found: editorNotFound,
    },
  }
  const adminKnowledgeRowActionItems = articles.map((article, index) => ({
    key: String(article.id),
    slot: 'admin.knowledge.row_actions',
    path: '/admin/knowledge',
    hostContext: {
      view: 'admin_knowledge_row',
      article: buildAdminKnowledgeArticleRowSummary(article),
      row: {
        index: index + 1,
        absolute_index: (page - 1) * limit + index + 1,
        selected: editorMode === 'edit' && selectedArticleId === article.id,
      },
      filters: adminKnowledgePluginContext.filters,
      pagination: adminKnowledgePluginContext.pagination,
      selection: adminKnowledgePluginContext.selection,
    },
  }))
  const adminKnowledgeRowActionExtensions = usePluginExtensionBatch({
    scope: 'admin',
    path: '/admin/knowledge',
    items: adminKnowledgeRowActionItems,
    enabled: articles.length > 0,
  })
  const deleteCategoryTarget = deleteCategoryId
    ? findCategoryById(categories, deleteCategoryId)
    : null
  const deleteArticleTarget = deleteArticleId
    ? articles.find((article) => article.id === deleteArticleId) ||
      (selectedArticleId === deleteArticleId ? selectedArticle : undefined)
    : undefined

  const openAddCategoryDialog = () => {
    setEditingCategory(null)
    setCategoryForm({ name: '', parent_id: undefined, sort_order: 0 })
    setCategoryDialogOpen(true)
  }

  const openEditCategoryDialog = (category: KnowledgeCategory) => {
    setEditingCategory(category)
    setCategoryForm({
      name: category.name,
      parent_id: category.parent_id || undefined,
      sort_order: category.sort_order,
    })
    setCategoryDialogOpen(true)
  }

  const closeCategoryDialog = () => {
    setCategoryDialogOpen(false)
    setEditingCategory(null)
    setCategoryForm({ name: '', parent_id: undefined, sort_order: 0 })
  }

  const handleCategorySubmit = () => {
    if (!categoryForm.name.trim()) {
      toast.error(t.knowledge.categoryNameRequired)
      return
    }

    if (editingCategory) {
      updateCategoryMutation.mutate({
        id: editingCategory.id,
        data: categoryForm,
      })
      return
    }

    createCategoryMutation.mutate(categoryForm)
  }

  const handleSelectAllCategories = () => {
    setCategoryId(undefined)
    setPage(1)
  }

  const handleSelectCategory = (id: number) => {
    setCategoryId(id.toString())
    setPage(1)
  }

  const handleStartCreateArticle = () => {
    const nextForm = createEmptyArticleForm(categoryId ? Number(categoryId) : undefined)
    setEditorMode('create')
    setSelectedArticleId(null)
    setArticleForm(nextForm)
    setArticleBaseline(nextForm)
  }

  const handleSelectArticle = (id: number) => {
    setEditorMode('edit')
    setSelectedArticleId(id)
  }

  const handleCloseEditor = () => {
    setEditorMode('empty')
    setSelectedArticleId(null)
    setArticleForm(createEmptyArticleForm())
    setArticleBaseline(null)
  }

  const handleSaveArticle = () => {
    if (!articleForm.title.trim()) {
      toast.error(t.knowledge.articleTitleRequired)
      return
    }

    if (editorMode === 'create') {
      createArticleMutation.mutate(articleForm)
      return
    }

    if (editorMode === 'edit' && selectedArticleId) {
      updateArticleMutation.mutate({ id: selectedArticleId, data: articleForm })
    }
  }

  const renderCategoryNode = (category: KnowledgeCategory, depth = 0) => {
    const hasChildren = !!category.children?.length
    const isCollapsed = collapsedCategoryIds.has(category.id)
    const totalCount = getCategoryTotalArticleCount(category)

    return (
      <div key={category.id}>
        <div
          className={`group flex items-center justify-between rounded-md px-3 py-2.5 transition-colors hover:bg-muted/50 ${
            depth > 0 ? 'ml-6 border-l-2 border-muted' : ''
          } ${categoryId === category.id.toString() ? 'bg-muted/60' : ''}`}
          role="button"
          tabIndex={0}
          onClick={() => handleSelectCategory(category.id)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              handleSelectCategory(category.id)
            }
          }}
        >
          <div className="flex min-w-0 items-center gap-2">
            {hasChildren ? (
              <Button
                size="sm"
                variant="ghost"
                className="h-7 w-7 p-0"
                onClick={(e) => {
                  e.stopPropagation()
                  setCollapsedCategoryIds((prev) => {
                    const next = new Set(prev)
                    if (next.has(category.id)) next.delete(category.id)
                    else next.add(category.id)
                    return next
                  })
                }}
                aria-label={isCollapsed ? t.common.expand : t.common.collapse}
              >
                <ChevronRight
                  className={`h-4 w-4 text-muted-foreground transition-transform ${
                    isCollapsed ? '' : 'rotate-90'
                  }`}
                />
              </Button>
            ) : (
              <span className="h-7 w-7" />
            )}
            <FolderTree className="h-4 w-4 text-muted-foreground" />
            <span className="truncate font-medium">{category.name}</span>
            <Badge
              variant="secondary"
              className="shrink-0 text-xs"
              title={`${t.knowledge.articles}: ${totalCount}`}
            >
              {totalCount}
            </Badge>
            <Badge variant="outline" className="shrink-0 text-xs">
              {t.knowledge.sortOrder}: {category.sort_order}
            </Badge>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <Button
              size="sm"
              variant="ghost"
              onClick={(e) => {
                e.stopPropagation()
                openEditCategoryDialog(category)
              }}
              aria-label={t.knowledge.editCategory}
            >
              <Pencil className="h-4 w-4" />
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="text-destructive hover:text-destructive"
              onClick={(e) => {
                e.stopPropagation()
                setDeleteCategoryId(category.id)
              }}
              aria-label={t.common.delete}
            >
              <Trash2 className="h-4 w-4" />
            </Button>
          </div>
        </div>
        {!isCollapsed
          ? category.children?.map((child) => renderCategoryNode(child, depth + 1))
          : null}
      </div>
    )
  }

  return (
    <div className="flex min-h-[calc(100dvh-4rem)] flex-col gap-4">
      <PluginSlot slot="admin.knowledge.top" context={adminKnowledgePluginContext} />
      <div className="flex shrink-0 flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div>
          <h1 className="text-lg font-bold md:text-xl">{t.admin.knowledgeManagement}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{articleRangeSummary}</p>
          <p className="mt-2 text-xs text-muted-foreground">{t.knowledge.importHint}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <input
            ref={fileInputRef}
            type="file"
            accept=".json,application/json"
            className="hidden"
            onChange={handleKnowledgeFileChange}
          />
          {canEditKnowledge ? (
            <Button
              variant="outline"
              onClick={handleImportKnowledgeClick}
              disabled={importMutation.isPending}
            >
              <Upload className="mr-1.5 h-4 w-4" />
              {t.knowledge.importKnowledge}
            </Button>
          ) : null}
          <Button variant="outline" onClick={handleExportKnowledge}>
            <Download className="mr-1.5 h-4 w-4" />
            {t.knowledge.exportKnowledge}
          </Button>
        </div>
      </div>

      <div className="grid min-h-0 flex-1 grid-cols-1 gap-6 xl:grid-cols-[420px_1fr]">
        <Card className="flex min-h-0 min-w-0 flex-col overflow-hidden">
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between gap-3">
              <CardTitle className="text-base">{t.knowledge.categoryManagement}</CardTitle>
              <Button size="sm" onClick={openAddCategoryDialog}>
                <Plus className="mr-1.5 h-4 w-4" />
                {t.knowledge.addCategory}
              </Button>
            </div>
            <div className="mt-3 flex items-center gap-2">
              <Button
                size="sm"
                variant={categoryId ? 'outline' : 'secondary'}
                onClick={handleSelectAllCategories}
              >
                {t.knowledge.allCategories}
              </Button>
            </div>
          </CardHeader>
          <CardContent className="min-h-0 flex-1 p-0">
            <ScrollArea className="h-full">
              <div className="p-3">
                {categoriesLoading ? (
                  <div className="flex items-center justify-center py-10">
                    <Loader2 className="h-5 w-5 animate-spin" />
                  </div>
                ) : categoriesLoadFailed && categories.length === 0 ? (
                  <div className="space-y-4 py-10 text-center">
                    <FolderTree className="mx-auto h-10 w-10 text-muted-foreground" />
                    <div className="space-y-1">
                      <p className="text-sm font-medium">{t.knowledge.loadFailed}</p>
                      <p className="text-xs text-muted-foreground">{t.knowledge.loadFailedDesc}</p>
                    </div>
                    <div className="flex justify-center">
                      <Button size="sm" variant="outline" onClick={() => refetchCategories()}>
                        {t.common.refresh}
                      </Button>
                    </div>
                    <PluginSlot
                      slot="admin.knowledge.categories.load_failed"
                      context={{ ...adminKnowledgePluginContext, section: 'categories_state' }}
                    />
                  </div>
                ) : categories.length === 0 ? (
                  <div className="py-10 text-center text-sm text-muted-foreground">
                    {t.admin.noData}
                    <PluginSlot
                      slot="admin.knowledge.categories.empty"
                      context={{ ...adminKnowledgePluginContext, section: 'categories_state' }}
                    />
                  </div>
                ) : (
                  <div className="space-y-1">
                    {categories.map((category) => renderCategoryNode(category))}
                  </div>
                )}

                <Separator className="my-4" />

                <div className="mb-3 flex items-center justify-between gap-3">
                  <div className="flex items-center gap-2">
                    <FileText className="h-4 w-4 text-muted-foreground" />
                    <span className="text-sm font-semibold">{t.knowledge.articleManagement}</span>
                  </div>
                  <Button size="sm" onClick={handleStartCreateArticle}>
                    <Plus className="mr-1.5 h-4 w-4" />
                    {t.knowledge.addArticle}
                  </Button>
                </div>

                <div className="relative mb-3">
                  <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    placeholder={t.knowledge.searchArticles}
                    value={search}
                    onChange={(e) => {
                      setSearch(e.target.value)
                      setPage(1)
                    }}
                    className="pl-9"
                  />
                </div>

                <div className="mb-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                  <span>{selectedCategory?.name || t.knowledge.allCategories}</span>
                  {search.trim() ? (
                    <span>
                      {t.common.search}: {search.trim()}
                    </span>
                  ) : (
                    <span>{t.knowledge.articleFilterHint}</span>
                  )}
                </div>

                {articlesLoading ? (
                  <div className="flex items-center justify-center py-10">
                    <Loader2 className="h-5 w-5 animate-spin" />
                  </div>
                ) : articlesLoadFailed && articles.length === 0 ? (
                  <div className="space-y-4 py-10 text-center">
                    <FileText className="mx-auto h-10 w-10 text-muted-foreground" />
                    <div className="space-y-1">
                      <p className="text-sm font-medium">{t.knowledge.loadFailed}</p>
                      <p className="text-xs text-muted-foreground">{t.knowledge.loadFailedDesc}</p>
                    </div>
                    <div className="flex flex-wrap justify-center gap-2">
                      <Button size="sm" variant="outline" onClick={() => refetchArticles()}>
                        {t.common.refresh}
                      </Button>
                      {(categoryId || search.trim()) && (
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => {
                            setCategoryId(undefined)
                            setSearch('')
                            setPage(1)
                          }}
                        >
                          {t.common.reset}
                        </Button>
                      )}
                    </div>
                    <PluginSlot
                      slot="admin.knowledge.articles.load_failed"
                      context={{ ...adminKnowledgePluginContext, section: 'articles_state' }}
                    />
                  </div>
                ) : articles.length === 0 ? (
                  <div className="py-10 text-center text-sm text-muted-foreground">
                    {t.knowledge.noArticles}
                    <PluginSlot
                      slot="admin.knowledge.articles.empty"
                      context={{ ...adminKnowledgePluginContext, section: 'articles_state' }}
                    />
                  </div>
                ) : (
                  <div className="space-y-1">
                    {articles.map((article) => (
                      <div
                        key={article.id}
                        className={`group flex items-center justify-between gap-3 rounded-md px-3 py-2.5 transition-colors hover:bg-muted/50 ${
                          editorMode === 'edit' && selectedArticleId === article.id
                            ? 'bg-muted/60'
                            : ''
                        }`}
                        role="button"
                        tabIndex={0}
                        onClick={() => handleSelectArticle(article.id)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ' ') {
                            handleSelectArticle(article.id)
                          }
                        }}
                      >
                        <div className="min-w-0 flex-1">
                          <div className="truncate font-medium">{article.title}</div>
                          <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                            {article.category ? (
                              <Badge variant="secondary" className="text-[11px]">
                                {article.category.name}
                              </Badge>
                            ) : (
                              <Badge variant="outline" className="text-[11px]">
                                {t.knowledge.uncategorized}
                              </Badge>
                            )}
                            <span>{new Date(article.created_at).toLocaleDateString()}</span>
                          </div>
                        </div>
                        <div className="flex shrink-0 flex-wrap items-center justify-end gap-1">
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={(e) => {
                              e.stopPropagation()
                              handleSelectArticle(article.id)
                            }}
                            aria-label={t.knowledge.editArticle}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            size="sm"
                            variant="ghost"
                            className="text-destructive hover:text-destructive"
                            onClick={(e) => {
                              e.stopPropagation()
                              setDeleteArticleId(article.id)
                            }}
                            aria-label={t.common.delete}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                          <PluginExtensionList
                            extensions={adminKnowledgeRowActionExtensions[String(article.id)] || []}
                            display="inline"
                          />
                        </div>
                      </div>
                    ))}
                  </div>
                )}

                {totalPages > 1 && (
                  <div className="mt-4 flex items-center justify-between gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={page <= 1}
                      onClick={() => setPage((p) => p - 1)}
                    >
                      {t.admin.prevPage}
                    </Button>
                    <span className="text-xs text-muted-foreground">
                      {t.admin.page
                        .replace('{current}', page.toString())
                        .replace('{total}', totalPages.toString())}
                    </span>
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={page >= totalPages}
                      onClick={() => setPage((p) => p + 1)}
                    >
                      {t.admin.nextPage}
                    </Button>
                  </div>
                )}
              </div>
            </ScrollArea>
          </CardContent>
        </Card>
        <Card className="flex min-h-0 min-w-0 flex-col overflow-hidden">
          {editorMode === 'empty' ? (
            <CardContent className="flex-1 p-0">
              <div className="flex h-full flex-col items-center justify-center px-4 py-10 text-center">
                <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-muted">
                  <FileText className="h-6 w-6 text-muted-foreground" />
                </div>
                <div className="text-base font-semibold">{t.knowledge.articles}</div>
                <div className="mt-1 text-sm text-muted-foreground">
                  {t.knowledge.articleDetail}
                </div>
                <div className="mt-6 flex flex-col items-center gap-3">
                  <PluginSlot
                    slot="admin.knowledge.editor.empty"
                    context={{ ...adminKnowledgePluginContext, section: 'editor_state' }}
                  />
                  <Button onClick={handleStartCreateArticle}>
                    <Plus className="mr-1.5 h-4 w-4" />
                    {t.knowledge.addArticle}
                  </Button>
                </div>
              </div>
            </CardContent>
          ) : (
            <>
              <CardHeader className="min-w-0 pb-3">
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-2">
                    <CardTitle className="text-base">
                      {editorMode === 'create' ? t.knowledge.addArticle : t.knowledge.editArticle}
                    </CardTitle>
                    <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                      {editorMode === 'edit' ? (
                        <>
                          {selectedArticleId ? <span>#{selectedArticleId}</span> : null}
                          <span>
                            {t.knowledge.articleCreatedAt}:{' '}
                            {formatKnowledgeDateTime(selectedArticle?.created_at)}
                          </span>
                          <span>
                            {t.knowledge.articleUpdatedAt}:{' '}
                            {formatKnowledgeDateTime(selectedArticle?.updated_at)}
                          </span>
                          {hasUnsavedArticleChanges ? (
                            <span>{t.knowledge.articleUnsavedChanges}</span>
                          ) : null}
                        </>
                      ) : (
                        <>
                          <span>
                            {selectedCategory
                              ? `${t.knowledge.selectCategory}: ${selectedCategory.name}`
                              : t.knowledge.addArticle}
                          </span>
                          {hasUnsavedArticleChanges ? (
                            <span>{t.knowledge.articleUnsavedChanges}</span>
                          ) : null}
                        </>
                      )}
                    </div>
                    <PluginSlot
                      slot="admin.knowledge.editor.meta.after"
                      context={{ ...adminKnowledgePluginContext, section: 'editor_meta' }}
                    />
                  </div>
                  <div className="flex items-center gap-2">
                    <Button type="button" variant="outline" size="sm" onClick={handleCloseEditor}>
                      <X className="mr-1.5 h-4 w-4" />
                      {t.common.cancel}
                    </Button>
                    {!isEditorLoading ? (
                      <Button
                        type="button"
                        size="sm"
                        disabled={isArticleSaving}
                        onClick={handleSaveArticle}
                      >
                        {isArticleSaving ? (
                          <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
                        ) : (
                          <Save className="mr-1.5 h-4 w-4" />
                        )}
                        {t.common.save}
                      </Button>
                    ) : null}
                  </div>
                </div>
              </CardHeader>
              <CardContent className="min-h-0 min-w-0 flex-1 p-0">
                {isEditorLoading ? (
                  <div className="flex h-full items-center justify-center">
                    <Loader2 className="h-8 w-8 animate-spin" />
                  </div>
                ) : selectedArticleLoadFailed && !selectedArticle ? (
                  <div className="flex h-full flex-col items-center justify-center gap-4 px-4 py-10 text-center">
                    <FileText className="h-10 w-10 text-muted-foreground" />
                    <div className="space-y-1">
                      <p className="text-sm font-medium">{t.knowledge.detailLoadFailed}</p>
                      <p className="text-xs text-muted-foreground">
                        {t.knowledge.detailLoadFailedDesc}
                      </p>
                    </div>
                    <div className="flex flex-wrap justify-center gap-2">
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={() => refetchSelectedArticle()}
                      >
                        {t.common.refresh}
                      </Button>
                      <Button type="button" size="sm" variant="ghost" onClick={handleCloseEditor}>
                        {t.common.cancel}
                      </Button>
                    </div>
                  </div>
                ) : editorNotFound ? (
                  <div className="flex h-full flex-col items-center justify-center gap-4 px-4 py-10 text-center">
                    <FileText className="h-10 w-10 text-muted-foreground" />
                    <div className="space-y-1">
                      <p className="text-sm font-medium">{t.knowledge.articleNotFound}</p>
                      <p className="text-xs text-muted-foreground">
                        {t.knowledge.articleNotFoundDesc}
                      </p>
                    </div>
                    <Button type="button" size="sm" variant="outline" onClick={handleCloseEditor}>
                      {t.common.back}
                    </Button>
                  </div>
                ) : (
                  <div className="flex h-full min-h-0 min-w-0 flex-col gap-4 p-4">
                    <div className="grid min-w-0 grid-cols-1 gap-4 lg:grid-cols-2">
                      <div className="space-y-2">
                        <Label htmlFor="articleTitle">
                          {t.knowledge.articleTitle} <span className="text-red-500">*</span>
                        </Label>
                        <Input
                          id="articleTitle"
                          value={articleForm.title}
                          onChange={(e) =>
                            setArticleForm({
                              ...articleForm,
                              title: e.target.value,
                            })
                          }
                          placeholder={t.knowledge.articleTitlePlaceholder}
                          required
                        />
                      </div>

                      <div className="grid grid-cols-2 gap-4">
                        <div className="space-y-2">
                          <Label>{t.knowledge.selectCategory}</Label>
                          <Select
                            value={articleForm.category_id?.toString() || 'none'}
                            onValueChange={(value) =>
                              setArticleForm({
                                ...articleForm,
                                category_id: value === 'none' ? undefined : Number(value),
                              })
                            }
                          >
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="none">{t.knowledge.uncategorized}</SelectItem>
                              {flatCategories.map((cat) => (
                                <SelectItem key={cat.id} value={cat.id.toString()}>
                                  {'  '.repeat(cat.depth)}
                                  {cat.name}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </div>

                        <div className="space-y-2">
                          <Label htmlFor="articleSortOrder">{t.knowledge.sortOrder}</Label>
                          <Input
                            id="articleSortOrder"
                            type="number"
                            value={articleForm.sort_order}
                            onChange={(e) =>
                              setArticleForm({
                                ...articleForm,
                                sort_order: parseInt(e.target.value) || 0,
                              })
                            }
                          />
                        </div>
                      </div>
                    </div>

                    <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-2">
                      <Tabs
                        defaultValue="edit"
                        className="flex min-h-0 w-full min-w-0 flex-1 flex-col"
                      >
                        <div className="mb-2 flex flex-wrap items-center justify-between gap-3">
                          <div className="space-y-1">
                            <Label>{t.knowledge.articleContent}</Label>
                            <div className="text-xs text-muted-foreground">
                              {t.knowledge.articleContentChars.replace(
                                '{count}',
                                String(articleContentCharCount)
                              )}
                              {' / '}
                              {t.knowledge.articleContentLines.replace(
                                '{count}',
                                String(articleContentLineCount)
                              )}
                            </div>
                          </div>
                          <TabsList className="shrink-0">
                            <TabsTrigger value="edit">{t.knowledge.editTab}</TabsTrigger>
                            <TabsTrigger value="preview">{t.knowledge.previewTab}</TabsTrigger>
                          </TabsList>
                        </div>

                        <div className="min-h-0 min-w-0 flex-1">
                          <TabsContent value="edit" className="mt-0 h-full min-h-0 min-w-0">
                            <MarkdownEditor
                              value={articleForm.content}
                              onChange={(v) =>
                                setArticleForm({
                                  ...articleForm,
                                  content: v,
                                })
                              }
                              fill
                              className="h-full min-h-0 w-full"
                              placeholder={t.knowledge.articleContent}
                            />
                          </TabsContent>

                          <TabsContent value="preview" className="mt-0 h-full min-h-0 min-w-0">
                            <div className="h-full min-h-0 w-full overflow-auto rounded-md border bg-background p-4">
                              {articleForm.content ? (
                                <MarkdownMessage
                                  content={articleForm.content}
                                  allowHtml
                                  className="markdown-body"
                                />
                              ) : (
                                <p className="text-muted-foreground">
                                  {t.knowledge.noPreviewContent}
                                </p>
                              )}
                            </div>
                          </TabsContent>
                        </div>
                      </Tabs>
                    </div>
                  </div>
                )}
              </CardContent>
            </>
          )}
        </Card>
      </div>

      <Dialog open={categoryDialogOpen} onOpenChange={setCategoryDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editingCategory ? t.knowledge.editCategory : t.knowledge.addCategory}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="categoryName">
                {t.knowledge.categoryName} <span className="text-red-500">*</span>
              </Label>
              <Input
                id="categoryName"
                value={categoryForm.name}
                onChange={(e) => setCategoryForm({ ...categoryForm, name: e.target.value })}
                placeholder={t.knowledge.categoryNamePlaceholder}
              />
            </div>
            <div className="space-y-2">
              <Label>{t.knowledge.parentCategory}</Label>
              <Select
                value={categoryForm.parent_id?.toString() || 'none'}
                onValueChange={(value) =>
                  setCategoryForm({
                    ...categoryForm,
                    parent_id: value === 'none' ? undefined : Number(value),
                  })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">{t.knowledge.noParent}</SelectItem>
                  {topLevelCategories
                    .filter((c) => c.id !== editingCategory?.id)
                    .map((cat) => (
                      <SelectItem key={cat.id} value={cat.id.toString()}>
                        {cat.name}
                      </SelectItem>
                    ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="sortOrder">{t.knowledge.sortOrder}</Label>
              <Input
                id="sortOrder"
                type="number"
                value={categoryForm.sort_order}
                onChange={(e) =>
                  setCategoryForm({
                    ...categoryForm,
                    sort_order: parseInt(e.target.value) || 0,
                  })
                }
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={closeCategoryDialog}>
              {t.common.cancel}
            </Button>
            <Button onClick={handleCategorySubmit} disabled={isCategoryMutating}>
              {isCategoryMutating ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
              {editingCategory ? t.common.save : t.common.create}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleteCategoryId !== null} onOpenChange={() => setDeleteCategoryId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t.admin.confirmDelete}</AlertDialogTitle>
            <AlertDialogDescription>
              <div className="space-y-3">
                <p>{t.knowledge.confirmDeleteCategory}</p>
                {deleteCategoryTarget ? (
                  <div className="rounded-md border border-dashed bg-muted/20 p-3 text-xs text-foreground">
                    <div className="font-medium">{deleteCategoryTarget.name}</div>
                    <div className="mt-1 text-muted-foreground">
                      {t.knowledge.articles}: {getCategoryTotalArticleCount(deleteCategoryTarget)}
                    </div>
                  </div>
                ) : null}
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteCategoryId && deleteCategoryMutation.mutate(deleteCategoryId)}
              className="bg-red-600 hover:bg-red-700"
            >
              {t.common.delete}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={deleteArticleId !== null} onOpenChange={() => setDeleteArticleId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t.admin.confirmDelete}</AlertDialogTitle>
            <AlertDialogDescription>
              <div className="space-y-3">
                <p>{t.knowledge.confirmDeleteArticle}</p>
                {deleteArticleTarget ? (
                  <div className="rounded-md border border-dashed bg-muted/20 p-3 text-xs text-foreground">
                    <div className="font-medium">{deleteArticleTarget.title}</div>
                    <div className="mt-1 text-muted-foreground">
                      {t.knowledge.selectCategory}:{' '}
                      {deleteArticleTarget.category?.name || t.knowledge.uncategorized}
                    </div>
                  </div>
                ) : null}
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => deleteArticleId && deleteArticleMutation.mutate(deleteArticleId)}
              className="bg-red-600 hover:bg-red-700"
            >
              {t.common.delete}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
