'use client'

import { useParams } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import { getKnowledgeArticle } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/page-loading'
import { ArrowLeft, BookOpen } from 'lucide-react'
import Link from 'next/link'
import { format } from 'date-fns'
import { zhCN } from 'date-fns/locale'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { MarkdownMessage } from '@/components/ui/markdown-message'
import { PluginSlot } from '@/components/plugins/plugin-slot'

export default function KnowledgeArticlePage() {
  const params = useParams()
  const articleId = Number(params.id)
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.knowledgeArticle)

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['knowledgeArticle', articleId],
    queryFn: () => getKnowledgeArticle(articleId),
    enabled: !!articleId,
  })

  const article = data?.data
  const userKnowledgeDetailPluginContext = {
    view: 'user_knowledge_detail',
    article: article
      ? {
          id: article.id,
          title: article.title,
          category_id: article.category_id,
          category_name: article.category?.name || undefined,
          created_at: article.created_at,
          updated_at: article.updated_at,
        }
      : {
          id: Number.isFinite(articleId) ? articleId : undefined,
        },
    summary: {
      content_length: article?.content.length || 0,
      has_category: Boolean(article?.category?.name),
    },
    state: {
      load_failed: isError && !article,
      not_found: !isLoading && !isError && !article,
    },
  }

  if (isLoading) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <Skeleton className="h-9 w-24" />
          <Skeleton className="h-7 w-64" />
        </div>
        <Card>
          <CardContent className="space-y-3 p-4 md:p-6">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-5/6" />
            <Skeleton className="h-4 w-4/6" />
            <Skeleton className="h-4 w-full" />
          </CardContent>
        </Card>
      </div>
    )
  }

  if (isError && !article) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <BookOpen className="mb-4 h-12 w-12 text-muted-foreground" />
        <p className="text-base font-medium">{t.knowledge.detailLoadFailed}</p>
        <p className="mb-4 mt-2 max-w-md text-sm text-muted-foreground">
          {t.knowledge.detailLoadFailedDesc}
        </p>
        <div className="flex flex-wrap justify-center gap-2">
          <Button variant="outline" onClick={() => refetch()}>
            {t.common.refresh}
          </Button>
          <Button variant="ghost" asChild>
            <Link href="/knowledge">{t.knowledge.backToList}</Link>
          </Button>
        </div>
        <PluginSlot
          slot="user.knowledge_detail.load_failed"
          context={{ ...userKnowledgeDetailPluginContext, section: 'detail_state' }}
        />
      </div>
    )
  }

  if (!article) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-center">
        <BookOpen className="mb-4 h-12 w-12 text-muted-foreground" />
        <p className="text-base font-medium">{t.knowledge.articleNotFound}</p>
        <p className="mb-4 mt-2 text-sm text-muted-foreground">{t.knowledge.articleNotFoundDesc}</p>
        <Button variant="outline" asChild>
          <Link href="/knowledge">
            <ArrowLeft className="mr-2 h-4 w-4" />
            {t.knowledge.backToList}
          </Link>
        </Button>
        <PluginSlot
          slot="user.knowledge_detail.not_found"
          context={{ ...userKnowledgeDetailPluginContext, section: 'detail_state' }}
        />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <PluginSlot slot="user.knowledge_detail.top" context={userKnowledgeDetailPluginContext} />
      {/* Back button + title */}
      <div className="flex items-center gap-3">
        <Button asChild variant="outline" size="sm">
          <Link href="/knowledge">
            <ArrowLeft className="h-4 w-4 md:mr-1.5" />
            <span className="hidden md:inline">{t.knowledge.backToList}</span>
            <span className="sr-only md:hidden">{t.knowledge.backToList}</span>
          </Link>
        </Button>
        <h1 className="line-clamp-1 text-lg font-bold md:text-xl">{article.title}</h1>
      </div>

      {/* Article content */}
      <Card>
        <CardContent className="p-4 md:p-6">
          <div className="mb-4 flex flex-wrap items-center gap-2">
            <span className="text-sm text-muted-foreground">
              {article.category?.name || t.knowledge.uncategorized}
            </span>
            <span className="text-sm text-muted-foreground">
              {format(new Date(article.created_at), 'yyyy-MM-dd HH:mm', {
                locale: locale === 'zh' ? zhCN : undefined,
              })}
            </span>
          </div>
          <PluginSlot
            slot="user.knowledge_detail.meta.after"
            context={{ ...userKnowledgeDetailPluginContext, section: 'meta' }}
          />
          <PluginSlot
            slot="user.knowledge_detail.content.before"
            context={{ ...userKnowledgeDetailPluginContext, section: 'content' }}
          />
          <MarkdownMessage content={article.content} allowHtml className="markdown-body" />
          <PluginSlot
            slot="user.knowledge_detail.content.after"
            context={{ ...userKnowledgeDetailPluginContext, section: 'content' }}
          />
        </CardContent>
      </Card>
      <PluginSlot
        slot="user.knowledge_detail.bottom"
        context={userKnowledgeDetailPluginContext}
      />
    </div>
  )
}
