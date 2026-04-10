import { getAnnouncement, getKnowledgeArticle } from '@/lib/api'

export function getAnnouncementQueryOptions(announcementId: number) {
  return {
    queryKey: ['announcement', announcementId] as const,
    queryFn: () => getAnnouncement(announcementId),
  }
}

export function getKnowledgeArticleQueryOptions(articleId: number) {
  return {
    queryKey: ['knowledgeArticle', articleId] as const,
    queryFn: () => getKnowledgeArticle(articleId),
  }
}
