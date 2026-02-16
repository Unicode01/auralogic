'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { useQuery, useMutation } from '@tanstack/react-query'
import {
  getAdminKnowledgeCategories,
  createKnowledgeArticle,
  KnowledgeCategory,
} from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tabs,
  TabsList,
  TabsTrigger,
  TabsContent,
} from '@/components/ui/tabs'
import toast from 'react-hot-toast'
import { ArrowLeft, Save, Loader2 } from 'lucide-react'
import Link from 'next/link'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { MarkdownMessage } from '@/components/ui/markdown-message'

interface ArticleForm {
  title: string
  category_id?: number
  sort_order: number
  content: string
}

export default function CreateKnowledgeArticlePage() {
  const router = useRouter()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.adminKnowledgeArticleNew)

  const [form, setForm] = useState<ArticleForm>({
    title: '',
    category_id: undefined,
    sort_order: 0,
    content: '',
  })

  // Fetch categories for the select
  const { data: categoriesData } = useQuery({
    queryKey: ['adminKnowledgeCategories'],
    queryFn: getAdminKnowledgeCategories,
  })

  const categories: KnowledgeCategory[] = categoriesData?.data || []

  // Flatten categories for select options
  const flattenCategories = (cats: KnowledgeCategory[], depth = 0): { id: number; name: string; depth: number }[] => {
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

  const saveMutation = useMutation({
    mutationFn: (data: ArticleForm) =>
      createKnowledgeArticle({
        title: data.title,
        content: data.content,
        category_id: data.category_id,
        sort_order: data.sort_order,
      }),
    onSuccess: () => {
      toast.success(t.knowledge.articleCreated)
      router.push('/admin/knowledge')
    },
    onError: (error: Error) => {
      toast.error(`${t.knowledge.createFailed}: ${error.message}`)
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()

    if (!form.title.trim()) {
      toast.error(`${t.knowledge.articleTitle} is required`)
      return
    }

    saveMutation.mutate(form)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="outline" size="sm" asChild>
          <Link href="/admin/knowledge">
            <ArrowLeft className="mr-1.5 h-4 w-4" />
            <span className="hidden md:inline">{t.common.back}</span>
          </Link>
        </Button>
        <h1 className="text-lg md:text-xl font-bold">
          {t.knowledge.addArticle}
        </h1>
      </div>

      <form onSubmit={handleSubmit} className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>{t.knowledge.addArticle}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
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
                          {'  '.repeat(cat.depth)}{cat.name}
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
                  <Textarea
                    value={form.content}
                    onChange={(e) => setForm({ ...form, content: e.target.value })}
                    className="min-h-[400px] font-mono"
                    placeholder={t.knowledge.articleContent}
                  />
                </TabsContent>
                <TabsContent value="preview">
                  <div className="min-h-[400px] border rounded-md p-4">
                    {form.content ? (
                      <MarkdownMessage
                        content={form.content}
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
              </Tabs>
            </div>
          </CardContent>
        </Card>

        <div className="flex justify-end gap-4">
          <Button type="button" variant="outline" asChild>
            <Link href="/admin/knowledge">{t.common.cancel}</Link>
          </Button>
          <Button type="submit" disabled={saveMutation.isPending}>
            <Save className="mr-2 h-4 w-4" />
            {saveMutation.isPending ? t.admin.creating : t.common.save}
          </Button>
        </div>
      </form>
    </div>
  )
}
