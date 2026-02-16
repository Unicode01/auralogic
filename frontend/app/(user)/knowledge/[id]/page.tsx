'use client'

import { useParams } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import { getKnowledgeArticle } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ArrowLeft, BookOpen } from 'lucide-react'
import Link from 'next/link'
import { format } from 'date-fns'
import { zhCN } from 'date-fns/locale'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { MarkdownMessage } from '@/components/ui/markdown-message'

export default function KnowledgeArticlePage() {
  const params = useParams()
  const articleId = Number(params.id)
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.knowledgeArticle)

  const { data, isLoading } = useQuery({
    queryKey: ['knowledgeArticle', articleId],
    queryFn: () => getKnowledgeArticle(articleId),
    enabled: !!articleId,
  })

  const article = data?.data

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="animate-pulse space-y-4">
          <div className="h-8 bg-muted rounded w-1/3" />
          <div className="h-5 bg-muted rounded w-1/4" />
          <div className="space-y-2 pt-4">
            <div className="h-4 bg-muted rounded w-full" />
            <div className="h-4 bg-muted rounded w-full" />
            <div className="h-4 bg-muted rounded w-5/6" />
            <div className="h-4 bg-muted rounded w-4/6" />
          </div>
        </div>
      </div>
    )
  }

  if (!article) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <BookOpen className="h-12 w-12 text-muted-foreground mb-4" />
        <p className="text-muted-foreground mb-4">{t.knowledge.articleNotFound}</p>
        <Button variant="outline" asChild>
          <Link href="/knowledge">
            <ArrowLeft className="h-4 w-4 mr-2" />
            {t.knowledge.backToList}
          </Link>
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Back button + title */}
      <div className="flex items-center gap-3">
        <Button asChild variant="outline" size="sm">
          <Link href="/knowledge">
            <ArrowLeft className="mr-1.5 h-4 w-4" />
            <span className="hidden md:inline">{t.knowledge.backToList}</span>
          </Link>
        </Button>
        <h1 className="text-lg md:text-xl font-bold line-clamp-1">{article.title}</h1>
      </div>

      {/* Article meta */}
      <div>
        <div className="flex items-center gap-3 mt-2">
          {article.category && (
            <Badge variant="secondary">{article.category.name}</Badge>
          )}
          <span className="text-sm text-muted-foreground">
            {format(
              new Date(article.created_at),
              'yyyy-MM-dd HH:mm',
              { locale: locale === 'zh' ? zhCN : undefined }
            )}
          </span>
        </div>
      </div>

      {/* Article content */}
      <Card>
        <CardContent className="p-4 md:p-6">
          <MarkdownMessage
            content={article.content}
            allowHtml
            className="prose dark:prose-invert max-w-none text-base [&_*]:text-foreground"
          />
        </CardContent>
      </Card>
    </div>
  )
}
