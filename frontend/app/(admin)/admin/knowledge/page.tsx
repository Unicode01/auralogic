'use client'

import { useEffect, useMemo, useState } from 'react'
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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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
import { Textarea } from '@/components/ui/textarea'
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
  FileText,
  FolderTree,
  Loader2,
  Pencil,
  Plus,
  Save,
  Search,
  Trash2,
  X,
} from 'lucide-react'

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

function flattenCategories(
  cats: KnowledgeCategory[],
  depth = 0,
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

export default function AdminKnowledgePage() {
  const queryClient = useQueryClient()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminKnowledge)

  // Category dialog state
  const [categoryDialogOpen, setCategoryDialogOpen] = useState(false)
  const [editingCategory, setEditingCategory] =
    useState<KnowledgeCategory | null>(null)
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
  const [collapsedCategoryIds, setCollapsedCategoryIds] = useState<
    Set<number>
  >(new Set())
  const [articleForm, setArticleForm] = useState<ArticleFormData>({
    title: '',
    category_id: undefined,
    sort_order: 0,
    content: '',
  })

  const { data: categoriesData, isLoading: categoriesLoading } = useQuery({
    queryKey: ['adminKnowledgeCategories'],
    queryFn: getAdminKnowledgeCategories,
  })

  const categories: KnowledgeCategory[] = categoriesData?.data || []
  const flatCategories = useMemo(
    () => flattenCategories(categories),
    [categories],
  )
  const topLevelCategories = useMemo(
    () => categories.filter((c) => !c.parent_id),
    [categories],
  )

  const getCategoryTotalArticleCount = (category: KnowledgeCategory): number => {
    if (typeof category.total_article_count === 'number') {
      return category.total_article_count
    }
    const own = category.article_count || 0
    const children =
      category.children?.reduce(
        (sum, child) => sum + getCategoryTotalArticleCount(child),
        0,
      ) || 0
    return own + children
  }

  const { data: articlesData, isLoading: articlesLoading } = useQuery({
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
  const totalArticles = articlesData?.data?.total || 0
  const totalPages = Math.ceil(totalArticles / limit) || 1

  const { data: selectedArticleData, isLoading: selectedArticleLoading } =
    useQuery({
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
    setArticleForm({
      title: a.title || '',
      category_id: a.category_id || undefined,
      sort_order: a.sort_order ?? 0,
      content: a.content || '',
    })
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
    onError: (error: Error) => {
      toast.error(`${t.knowledge.createFailed}: ${error.message}`)
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
    onError: (error: Error) => {
      toast.error(`${t.knowledge.updateFailed}: ${error.message}`)
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
    onError: (error: Error) => {
      toast.error(`${t.knowledge.deleteFailed}: ${error.message}`)
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
        setArticleForm({
          title: '',
          category_id: undefined,
          sort_order: 0,
          content: '',
        })
      }
      setDeleteArticleId(null)
    },
    onError: (error: Error) => {
      toast.error(`${t.knowledge.deleteFailed}: ${error.message}`)
    },
  })

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
    },
    onError: (error: Error) => {
      toast.error(`${t.knowledge.createFailed}: ${error.message}`)
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
      if (selectedArticleId) {
        queryClient.invalidateQueries({
          queryKey: ['adminKnowledgeArticle', selectedArticleId],
        })
      }
    },
    onError: (error: Error) => {
      toast.error(`${t.knowledge.updateFailed}: ${error.message}`)
    },
  })

  const isCategoryMutating =
    createCategoryMutation.isPending || updateCategoryMutation.isPending
  const isArticleSaving =
    createArticleMutation.isPending || updateArticleMutation.isPending
  const isEditorLoading = editorMode === 'edit' && selectedArticleLoading

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
      toast.error(`${t.knowledge.categoryName} is required`)
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
    setEditorMode('create')
    setSelectedArticleId(null)
    setArticleForm({
      title: '',
      category_id: categoryId ? Number(categoryId) : undefined,
      sort_order: 0,
      content: '',
    })
  }

  const handleSelectArticle = (id: number) => {
    setEditorMode('edit')
    setSelectedArticleId(id)
  }

  const handleCloseEditor = () => {
    setEditorMode('empty')
    setSelectedArticleId(null)
    setArticleForm({ title: '', category_id: undefined, sort_order: 0, content: '' })
  }

  const handleSaveArticle = () => {
    if (!articleForm.title.trim()) {
      toast.error(`${t.knowledge.articleTitle} is required`)
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
          className={`group flex items-center justify-between py-2.5 px-3 hover:bg-muted/50 transition-colors rounded-md ${
            depth > 0 ? 'border-l-2 border-muted ml-6' : ''
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
          <div className="flex items-center gap-2 min-w-0">
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
            <span className="font-medium truncate">{category.name}</span>
            <Badge
              variant="secondary"
              className="text-xs shrink-0"
              title={`${t.knowledge.articles}: ${totalCount}`}
            >
              {totalCount}
            </Badge>
            <Badge variant="outline" className="text-xs shrink-0">
              {t.knowledge.sortOrder}: {category.sort_order}
            </Badge>
          </div>
          <div className="flex items-center gap-2 shrink-0">
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
          ? category.children?.map((child) =>
              renderCategoryNode(child, depth + 1),
            )
          : null}
      </div>
    )
  }

  return (
    <div className="min-h-[calc(100dvh-4rem)] flex flex-col gap-4">
      <div className="flex items-start justify-between gap-4 shrink-0">
        <h1 className="text-lg md:text-xl font-bold">
          {t.admin.knowledgeManagement}
        </h1>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[420px_1fr] gap-6 flex-1 min-h-0">
        <Card className="overflow-hidden flex flex-col min-w-0 min-h-0">
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between gap-3">
              <CardTitle className="text-base">
                {t.knowledge.categoryManagement}
              </CardTitle>
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
              <div className="flex-1" />
              <Badge variant="outline" className="text-xs">
                {t.knowledge.categories}: {categories.length}
              </Badge>
            </div>
          </CardHeader>
          <CardContent className="p-0 flex-1 min-h-0">
            <ScrollArea className="h-full">
              <div className="p-3">
                {categoriesLoading ? (
                  <div className="flex items-center justify-center py-10">
                    <Loader2 className="h-5 w-5 animate-spin" />
                  </div>
                ) : categories.length === 0 ? (
                  <div className="text-center py-10 text-muted-foreground text-sm">
                    {t.admin.noData}
                  </div>
                ) : (
                  <div className="space-y-1">
                    {categories.map((category) => renderCategoryNode(category))}
                  </div>
                )}

                <Separator className="my-4" />

                <div className="flex items-center justify-between gap-3 mb-3">
                  <div className="flex items-center gap-2">
                    <FileText className="h-4 w-4 text-muted-foreground" />
                    <span className="text-sm font-semibold">
                      {t.knowledge.articleManagement}
                    </span>
                  </div>
                  <Button size="sm" onClick={handleStartCreateArticle}>
                    <Plus className="mr-1.5 h-4 w-4" />
                    {t.knowledge.addArticle}
                  </Button>
                </div>

                <div className="relative mb-3">
                  <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
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

                {articlesLoading ? (
                  <div className="flex items-center justify-center py-10">
                    <Loader2 className="h-5 w-5 animate-spin" />
                  </div>
                ) : articles.length === 0 ? (
                  <div className="text-center py-10 text-muted-foreground text-sm">
                    {t.knowledge.noArticles}
                  </div>
                ) : (
                  <div className="space-y-1">
                    {articles.map((article) => (
                      <div
                        key={article.id}
                        className={`group flex items-center justify-between gap-3 rounded-md px-3 py-2.5 hover:bg-muted/50 transition-colors ${
                          editorMode === 'edit' &&
                          selectedArticleId === article.id
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
                        <div className="flex-1 min-w-0">
                          <div className="font-medium truncate">
                            {article.title}
                          </div>
                          <div className="flex items-center gap-2 mt-1 text-xs text-muted-foreground">
                            {article.category ? (
                              <Badge
                                variant="secondary"
                                className="text-[11px]"
                              >
                                {article.category.name}
                              </Badge>
                            ) : (
                              <Badge variant="outline" className="text-[11px]">
                                {t.knowledge.uncategorized}
                              </Badge>
                            )}
                            <span>
                              {new Date(
                                article.created_at,
                              ).toLocaleDateString()}
                            </span>
                          </div>
                        </div>
                        <div className="flex items-center gap-1 shrink-0">
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
                        </div>
                      </div>
                    ))}
                  </div>
                )}

                {totalPages > 1 && (
                  <div className="flex items-center justify-between gap-2 mt-4">
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
        <Card className="overflow-hidden flex flex-col min-w-0 min-h-0">
          {editorMode === 'empty' ? (
            <CardContent className="p-0 flex-1">
              <div className="flex flex-col items-center justify-center text-center px-4 py-10 h-full">
                <div className="h-12 w-12 rounded-xl bg-muted flex items-center justify-center mb-4">
                  <FileText className="h-6 w-6 text-muted-foreground" />
                </div>
                <div className="text-base font-semibold">
                  {t.knowledge.articles}
                </div>
                <div className="text-sm text-muted-foreground mt-1">
                  {t.knowledge.articleDetail}
                </div>
                <div className="flex items-center gap-2 mt-6">
                  <Button onClick={handleStartCreateArticle}>
                    <Plus className="mr-1.5 h-4 w-4" />
                    {t.knowledge.addArticle}
                  </Button>
                </div>
              </div>
            </CardContent>
          ) : (
            <>
              <CardHeader className="pb-3 min-w-0">
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <CardTitle className="text-base">
                      {editorMode === 'create'
                        ? t.knowledge.addArticle
                        : t.knowledge.editArticle}
                    </CardTitle>
                    <div className="text-xs text-muted-foreground mt-1">
                      {editorMode === 'edit' && selectedArticleId
                        ? `#${selectedArticleId}`
                        : t.knowledge.articleDetail}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={handleCloseEditor}
                    >
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
              <CardContent className="p-0 flex-1 min-h-0 min-w-0">
                {isEditorLoading ? (
                  <div className="h-full flex items-center justify-center">
                    <Loader2 className="h-8 w-8 animate-spin" />
                  </div>
                ) : (
                  <div className="p-4 flex flex-col gap-4 h-full min-h-0 min-w-0">

                  <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 min-w-0">
                    <div className="space-y-2">
                      <Label htmlFor="articleTitle">
                        {t.knowledge.articleTitle}{' '}
                        <span className="text-red-500">*</span>
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
                              category_id:
                                value === 'none' ? undefined : Number(value),
                            })
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="none">
                              {t.knowledge.uncategorized}
                            </SelectItem>
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
                        <Label htmlFor="articleSortOrder">
                          {t.knowledge.sortOrder}
                        </Label>
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

                    <div className="flex flex-col gap-2 flex-1 min-h-0 min-w-0">
                      <Tabs
                        defaultValue="edit"
                        className="flex flex-col flex-1 min-h-0 w-full min-w-0"
                      >
                        <div className="flex items-center justify-between gap-3 mb-2">
                          <Label>{t.knowledge.articleContent}</Label>
                          <TabsList className="shrink-0">
                            <TabsTrigger value="edit">
                              {t.knowledge.editTab}
                            </TabsTrigger>
                            <TabsTrigger value="preview">
                              {t.knowledge.previewTab}
                            </TabsTrigger>
                          </TabsList>
                        </div>

                        <div className="flex-1 min-h-0 min-w-0">
                          <TabsContent
                            value="edit"
                            className="mt-0 h-full min-h-0 min-w-0"
                          >
                            <Textarea
                              value={articleForm.content}
                              onChange={(e) =>
                                setArticleForm({
                                  ...articleForm,
                                  content: e.target.value,
                                })
                              }
                              className="h-full min-h-0 w-full font-mono resize-none"
                              placeholder={t.knowledge.articleContent}
                            />
                          </TabsContent>

                          <TabsContent
                            value="preview"
                            className="mt-0 h-full min-h-0 min-w-0"
                          >
                            <div className="h-full min-h-0 w-full overflow-auto border rounded-md p-4 bg-background">
                              {articleForm.content ? (
                                <MarkdownMessage
                                  content={articleForm.content}
                                  allowHtml
                                  className="prose dark:prose-invert max-w-none text-base [&_*]:text-foreground"
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
                onChange={(e) =>
                  setCategoryForm({ ...categoryForm, name: e.target.value })
                }
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
              {isCategoryMutating ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              ) : null}
              {editingCategory ? t.common.save : t.common.create}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={deleteCategoryId !== null}
        onOpenChange={() => setDeleteCategoryId(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t.admin.confirmDelete}</AlertDialogTitle>
            <AlertDialogDescription>
              {t.knowledge.confirmDeleteCategory}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() =>
                deleteCategoryId &&
                deleteCategoryMutation.mutate(deleteCategoryId)
              }
              className="bg-red-600 hover:bg-red-700"
            >
              {t.common.delete}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={deleteArticleId !== null}
        onOpenChange={() => setDeleteArticleId(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t.admin.confirmDelete}</AlertDialogTitle>
            <AlertDialogDescription>
              {t.knowledge.confirmDeleteArticle}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() =>
                deleteArticleId &&
                deleteArticleMutation.mutate(deleteArticleId)
              }
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
