import { HydrationBoundary, dehydrate } from '@tanstack/react-query'
import KnowledgeArticleClient from './knowledge-article-client'
import { getKnowledgeArticleQueryOptions } from '@/lib/content-detail-queries'
import { getServerAuthToken, getServerKnowledgeArticle } from '@/lib/server-api'
import { createServerQueryClient } from '@/lib/server-query-client'

function isPositiveInteger(value: number): boolean {
  return Number.isFinite(value) && value > 0
}

export default async function KnowledgeArticlePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const articleId = Number(id)
  const queryClient = createServerQueryClient()

  if ((await getServerAuthToken()) && isPositiveInteger(articleId)) {
    try {
      await queryClient.prefetchQuery({
        ...getKnowledgeArticleQueryOptions(articleId),
        queryFn: () => getServerKnowledgeArticle(articleId),
      })
    } catch {
      // Preserve the existing client-side loading and error handling.
    }
  }

  return (
    <HydrationBoundary state={dehydrate(queryClient)}>
      <KnowledgeArticleClient articleId={articleId} />
    </HydrationBoundary>
  )
}
