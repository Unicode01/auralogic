'use client'

import { useState, useEffect } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { useQuery, useMutation } from '@tanstack/react-query'
import {
  getAdminKnowledgeCategories,
  getAdminKnowledgeArticle,
  updateKnowledgeArticle,
  KnowledgeCategory,
} from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { MarkdownEditor } from '@/components/ui/markdown-editor'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import toast from 'react-hot-toast'
import { ArrowLeft, BookOpen, Loader2, Save } from 'lucide-react'
import Link from 'next/link'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { MarkdownMessage } from '@/components/ui/markdown-message'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { PluginSlot } from '@/components/plugins/plugin-slot'

interface ArticleForm {
  title: string
  category_id?: number
  sort_order: number
  content: string
}

export default function EditKnowledgeArticlePage() {
  const params = useParams()
  const router = useRouter()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminKnowledgeArticleEdit)
  const articleId = Number(params.id)
  const formatKnowledgeError = (error: unknown, fallback: string) => {
    const detail = resolveApiErrorMessage(error, t, fallback)
    return detail === fallback ? fallback : `${fallback}: ${detail}`
  }

  const [form, setForm] = useState<ArticleForm>({
    title: '',
    category_id: undefined,
    sort_order: 0,
    content: '',
  })

  // Fetch article
  const {
    data: articleData,
    isLoading,
    isError: articleLoadFailed,
    refetch: refetchArticle,
  } = useQuery({
    queryKey: ['adminKnowledgeArticle', articleId],
    queryFn: () => getAdminKnowledgeArticle(articleId),
    enabled: !!articleId,
    refetchOnMount: 'always',
    staleTime: 0,
  })

  // Fetch categories for the select
  const { data: categoriesData } = useQuery({
    queryKey: ['adminKnowledgeCategories'],
    queryFn: getAdminKnowledgeCategories,
  })

  const categories: KnowledgeCategory[] = categoriesData?.data || []

  // Flatten categories for select options
  const flattenCategories = (
    cats: KnowledgeCategory[],
    depth = 0
  ): { id: number; name: string; depth: number }[] => {
    const result: { id: number; name: string; depth: number }[] = []
    for (const cat of cats) {
      result.push({ id: cat.id, name: cat.name, depth })
      if (cat.children?.length) {
        result.push(...flattenCategories(cat.children, depth + 1))
      }
    }
    return result
  }

  const flatCategories = flattenCategories(categories)
  const selectedCategoryLabel =
    flatCategories.find((cat) => cat.id === form.category_id)?.name || t.knowledge.uncategorized
  const initialArticle = articleData?.data
  const isDirty = Boolean(
    initialArticle &&
    (form.title !== (initialArticle.title || '') ||
      form.category_id !== (initialArticle.category_id || undefined) ||
      form.sort_order !== (initialArticle.sort_order ?? 0) ||
      form.content !== (initialArticle.content || ''))
  )
  const contentCharCount = form.content.trim().length
  const contentLineCount = form.content ? form.content.replace(/\r\n/g, '\n').split('\n').length : 0
  const adminKnowledgeArticleDetailPluginContext = {
    view: 'admin_knowledge_article_detail',
    resource_id: articleId,
    article: initialArticle
      ? {
          id: articleId,
          title: initialArticle.title,
          category_id: initialArticle.category_id,
          sort_order: initialArticle.sort_order,
          created_at: initialArticle.created_at,
          updated_at: initialArticle.updated_at,
        }
      : {
          id: Number.isFinite(articleId) ? articleId : undefined,
        },
    form: {
      title: form.title || undefined,
      category_id: form.category_id,
      sort_order: form.sort_order,
      content_length: contentCharCount,
      content_line_count: contentLineCount,
      dirty: isDirty,
    },
    state: {
      load_failed: articleLoadFailed && !initialArticle,
      not_found: !isLoading && !articleLoadFailed && !initialArticle,
    },
  }

  const formatKnowledgeTime = (value?: string) => {
    if (!value) return '-'
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return value
    return date.toLocaleString(locale === 'zh' ? 'zh-CN' : 'en-US', { hour12: false })
  }

  // Populate form when article data loads
  useEffect(() => {
    if (articleData?.data) {
      const article = articleData.data
      setForm({
        title: article.title || '',
        category_id: article.category_id || undefined,
        sort_order: article.sort_order ?? 0,
        content: article.content || '',
      })
    }
  }, [articleData])

  const saveMutation = useMutation({
    mutationFn: (data: ArticleForm) =>
      updateKnowledgeArticle(articleId, {
        title: data.title,
        content: data.content,
        category_id: data.category_id,
        sort_order: data.sort_order,
      }),
    onSuccess: () => {
      toast.success(t.knowledge.articleUpdated)
      router.push('/admin/knowledge')
    },
    onError: (error: unknown) => {
      toast.error(formatKnowledgeError(error, t.knowledge.updateFailed))
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()

    if (!form.title.trim()) {
      toast.error(t.knowledge.articleTitleRequired)
      return
    }

    saveMutation.mutate(form)
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-center">
          <Loader2 className="mx-auto mb-4 h-8 w-8 animate-spin" />
          <p className="text-muted-foreground">{t.common.loading}</p>
        </div>
      </div>
    )
  }

  if (articleLoadFailed && !initialArticle) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <BookOpen className="mb-4 h-12 w-12 text-muted-foreground" />
        <p className="text-base font-medium">{t.knowledge.detailLoadFailed}</p>
        <p className="mb-4 mt-2 max-w-md text-sm text-muted-foreground">
          {t.knowledge.detailLoadFailedDesc}
        </p>
        <div className="flex flex-wrap justify-center gap-2">
          <Button variant="outline" onClick={() => refetchArticle()}>
            {t.common.refresh}
          </Button>
          <Button variant="ghost" asChild>
            <Link href="/admin/knowledge">{t.common.back}</Link>
          </Button>
        </div>
        <PluginSlot
          slot="admin.knowledge_article_detail.load_failed"
          context={{ ...adminKnowledgeArticleDetailPluginContext, section: 'detail_state' }}
        />
      </div>
    )
  }

  if (!initialArticle) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <BookOpen className="mb-4 h-12 w-12 text-muted-foreground" />
        <p className="text-base font-medium">{t.knowledge.articleNotFound}</p>
        <p className="mb-4 mt-2 max-w-md text-sm text-muted-foreground">
          {t.knowledge.articleNotFoundDesc}
        </p>
        <Button variant="outline" asChild>
          <Link href="/admin/knowledge">
            <ArrowLeft className="mr-2 h-4 w-4" />
            {t.common.back}
          </Link>
        </Button>
        <PluginSlot
          slot="admin.knowledge_article_detail.not_found"
          context={{ ...adminKnowledgeArticleDetailPluginContext, section: 'detail_state' }}
        />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <PluginSlot
        slot="admin.knowledge_article_detail.top"
        context={adminKnowledgeArticleDetailPluginContext}
      />
      <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
        <div className="flex items-center gap-4">
          <Button variant="outline" size="sm" asChild>
            <Link href="/admin/knowledge">
              <ArrowLeft className="h-4 w-4 md:mr-1.5" />
              <span className="hidden md:inline">{t.common.back}</span>
            </Link>
          </Button>
          <h1 className="text-lg font-bold md:text-xl">{t.knowledge.editArticle}</h1>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button type="button" variant="outline" asChild>
            <Link href="/admin/knowledge">{t.common.cancel}</Link>
          </Button>
          <Button
            type="submit"
            form="knowledge-article-edit-form"
            disabled={saveMutation.isPending}
          >
            <Save className="mr-2 h-4 w-4" />
            {saveMutation.isPending ? t.admin.saving : t.common.save}
          </Button>
        </div>
      </div>

      <form id="knowledge-article-edit-form" onSubmit={handleSubmit} className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>{t.knowledge.editArticle}</CardTitle>
            <div className="mt-2 flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
              <span>#{articleId}</span>
              {isDirty ? <span>{t.knowledge.articleUnsavedChanges}</span> : null}
              <span>{t.knowledge.selectCategory}: {selectedCategoryLabel}</span>
              <span>{t.knowledge.sortOrder}: {form.sort_order}</span>
              <span>
                {t.knowledge.articleContentChars.replace('{count}', String(contentCharCount))}
              </span>
              <span>
                {t.knowledge.articleContentLines.replace('{count}', String(contentLineCount))}
              </span>
            </div>
            <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
              <span>
                {t.knowledge.articleCreatedAt}: {formatKnowledgeTime(initialArticle?.created_at)}
              </span>
              <span>
                {t.knowledge.articleUpdatedAt}: {formatKnowledgeTime(initialArticle?.updated_at)}
              </span>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <PluginSlot
              slot="admin.knowledge_article_detail.form.top"
              context={{ ...adminKnowledgeArticleDetailPluginContext, section: 'form' }}
            />
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="title">
                  {t.knowledge.articleTitle} <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="title"
                  value={form.title}
                  onChange={(e) => setForm({ ...form, title: e.target.value })}
                  placeholder={t.knowledge.articleTitlePlaceholder}
                  required
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label>{t.knowledge.selectCategory}</Label>
                  <Select
                    value={form.category_id?.toString() || 'none'}
                    onValueChange={(value) =>
                      setForm({
                        ...form,
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
                  <Label htmlFor="sort_order">{t.knowledge.sortOrder}</Label>
                  <Input
                    id="sort_order"
                    type="number"
                    value={form.sort_order}
                    onChange={(e) =>
                      setForm({ ...form, sort_order: parseInt(e.target.value) || 0 })
                    }
                  />
                </div>
              </div>
            </div>

            <div className="space-y-2">
              <Label>{t.knowledge.articleContent}</Label>
              <Tabs defaultValue="edit">
                <TabsList>
                  <TabsTrigger value="edit">{t.knowledge.editTab}</TabsTrigger>
                  <TabsTrigger value="preview">{t.knowledge.previewTab}</TabsTrigger>
                </TabsList>
                <TabsContent value="edit">
                  <MarkdownEditor
                    value={form.content}
                    onChange={(v) => setForm({ ...form, content: v })}
                    height="400px"
                    placeholder={t.knowledge.articleContent}
                  />
                </TabsContent>
                <TabsContent value="preview">
                  <div className="min-h-[400px] rounded-md border p-4">
                    {form.content ? (
                      <MarkdownMessage content={form.content} allowHtml className="markdown-body" />
                    ) : (
                      <p className="text-muted-foreground">{t.knowledge.noPreviewContent}</p>
                    )}
                  </div>
                </TabsContent>
              </Tabs>
            </div>
          </CardContent>
        </Card>

        <div className="space-y-3">
          <PluginSlot
            slot="admin.knowledge_article_detail.submit.before"
            context={{ ...adminKnowledgeArticleDetailPluginContext, section: 'submit' }}
          />
          <div className="flex justify-end gap-4">
            <Button type="button" variant="outline" asChild>
              <Link href="/admin/knowledge">{t.common.cancel}</Link>
            </Button>
            <Button type="submit" disabled={saveMutation.isPending}>
              <Save className="mr-2 h-4 w-4" />
              {saveMutation.isPending ? t.admin.saving : t.common.save}
            </Button>
          </div>
        </div>
      </form>
    </div>
  )
}
