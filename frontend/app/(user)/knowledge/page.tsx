'use client'

import { useState, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getKnowledgeCategoryTree, getKnowledgeArticles, KnowledgeCategory } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Search, ChevronLeft, ChevronRight, ChevronDown, BookOpen, Folder, FolderOpen, FileText } from 'lucide-react'
import Link from 'next/link'
import { format } from 'date-fns'
import { zhCN } from 'date-fns/locale'
import { cn } from '@/lib/utils'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'

export default function KnowledgePage() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.knowledge)

  const [page, setPage] = useState(1)
  const [categoryId, setCategoryId] = useState<string | undefined>()
  const [search, setSearch] = useState('')
  const [searchInput, setSearchInput] = useState('')
  const [expandedCategories, setExpandedCategories] = useState<Set<number>>(new Set())
  const limit = 10

  // Debounce search
  useEffect(() => {
    const timer = setTimeout(() => {
      setSearch(searchInput)
      setPage(1)
    }, 300)
    return () => clearTimeout(timer)
  }, [searchInput])

  // Fetch category tree
  const { data: categoryData } = useQuery({
    queryKey: ['knowledgeCategoryTree'],
    queryFn: getKnowledgeCategoryTree,
  })

  // Fetch articles
  const { data: articlesData, isLoading } = useQuery({
    queryKey: ['knowledgeArticles', page, categoryId, search],
    queryFn: () => getKnowledgeArticles({
      page,
      limit,
      category_id: categoryId,
      search: search || undefined,
    }),
  })

  const categories: KnowledgeCategory[] = categoryData?.data || []
  const articles = articlesData?.data?.items || []
  const total = articlesData?.data?.total || 0
  const totalPages = Math.ceil(total / limit)

  const handleCategoryClick = (id?: string) => {
    setCategoryId(id)
    setPage(1)
  }

  const toggleExpand = (id: number) => {
    setExpandedCategories(prev => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const handleSearch = () => {
    setSearch(searchInput)
    setPage(1)
  }

  // Render category tree item (desktop sidebar)
  const renderCategoryItem = (cat: KnowledgeCategory, depth = 0) => {
    const hasChildren = cat.children && cat.children.length > 0
    const isExpanded = expandedCategories.has(cat.id)
    const isActive = categoryId === cat.id.toString()

    return (
      <div key={cat.id}>
        <div
          className={cn(
            'flex items-center gap-1.5 px-3 py-2 rounded-md cursor-pointer text-sm transition-colors',
            isActive ? 'bg-primary text-primary-foreground' : 'hover:bg-accent',
          )}
          style={{ paddingLeft: `${12 + depth * 16}px` }}
          onClick={() => handleCategoryClick(cat.id.toString())}
        >
          {hasChildren ? (
            <button
              className="p-0.5 shrink-0 -ml-0.5"
              onClick={(e) => {
                e.stopPropagation()
                toggleExpand(cat.id)
              }}
            >
              {isExpanded
                ? <ChevronDown className="h-3.5 w-3.5" />
                : <ChevronRight className="h-3.5 w-3.5" />
              }
            </button>
          ) : (
            <span className="w-[18px] shrink-0" />
          )}
          {hasChildren ? (
            isExpanded
              ? <FolderOpen className="h-4 w-4 shrink-0 text-amber-500" />
              : <Folder className="h-4 w-4 shrink-0 text-amber-500" />
          ) : (
            <FileText className="h-4 w-4 shrink-0 opacity-50" />
          )}
          <span className="truncate">{cat.name}</span>
        </div>
        {hasChildren && isExpanded && (
          <div>
            {cat.children!.map(child => renderCategoryItem(child, depth + 1))}
          </div>
        )}
      </div>
    )
  }

  // Flatten categories for mobile chips
  const flattenCategories = (cats: KnowledgeCategory[]): KnowledgeCategory[] => {
    const result: KnowledgeCategory[] = []
    const walk = (list: KnowledgeCategory[]) => {
      for (const cat of list) {
        result.push(cat)
        if (cat.children?.length) walk(cat.children)
      }
    }
    walk(cats)
    return result
  }

  const flatCategories = flattenCategories(categories)

  return (
    <div className="space-y-4">
      {/* Header */}
      <div>
        <h1 className="text-xl md:text-2xl font-bold">{t.knowledge.knowledgeBase}</h1>
      </div>

      {/* Search */}
      <div className="flex gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder={t.knowledge.searchArticles}
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
            className="pl-9"
          />
        </div>
      </div>

      {/* Mobile: horizontal scrollable category chips */}
      <div className="md:hidden">
        <div className="flex gap-2 overflow-x-auto pb-2 scrollbar-hide">
          <Button
            variant={!categoryId ? 'default' : 'outline'}
            size="sm"
            className="shrink-0"
            onClick={() => handleCategoryClick(undefined)}
          >
            {t.knowledge.allCategories}
          </Button>
          {flatCategories.map(cat => (
            <Button
              key={cat.id}
              variant={categoryId === cat.id.toString() ? 'default' : 'outline'}
              size="sm"
              className="shrink-0"
              onClick={() => handleCategoryClick(cat.id.toString())}
            >
              {cat.name}
            </Button>
          ))}
        </div>
      </div>

      {/* Main content: sidebar + article list */}
      <div className="flex gap-6">
        {/* Desktop: category sidebar */}
        <div className="hidden md:block w-56 shrink-0">
          <Card>
            <CardContent className="p-2">
              <div className="space-y-0.5">
                <div
                  className={cn(
                    'flex items-center gap-1.5 px-3 py-2 rounded-md cursor-pointer text-sm transition-colors',
                    !categoryId ? 'bg-primary text-primary-foreground' : 'hover:bg-accent',
                  )}
                  onClick={() => handleCategoryClick(undefined)}
                >
                  <span className="w-[18px] shrink-0" />
                  <BookOpen className="h-4 w-4 shrink-0" />
                  <span>{t.knowledge.allCategories}</span>
                </div>
                {categories.map(cat => renderCategoryItem(cat))}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Article list */}
        <div className="flex-1 min-w-0">
          {isLoading ? (
            <div className="space-y-3">
              {[...Array(3)].map((_, i) => (
                <Card key={i} className="animate-pulse">
                  <CardContent className="p-4 space-y-2">
                    <div className="h-5 bg-muted rounded w-3/4" />
                    <div className="h-4 bg-muted rounded w-1/4" />
                  </CardContent>
                </Card>
              ))}
            </div>
          ) : articles.length === 0 ? (
            <Card>
              <CardContent className="text-center py-12">
                <BookOpen className="h-12 w-12 mx-auto text-muted-foreground mb-4" />
                <p className="text-muted-foreground">
                  {search ? t.knowledge.noSearchResults : t.knowledge.noArticlesDesc}
                </p>
              </CardContent>
            </Card>
          ) : (
            <>
              <div className="space-y-3">
                {articles.map((article: any) => (
                  <Link key={article.id} href={`/knowledge/${article.id}`} className="block">
                    <Card className="hover:bg-accent/50 transition-colors cursor-pointer">
                      <CardContent className="p-4">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0 flex-1">
                            <h3 className="font-medium text-sm md:text-base truncate">
                              {article.title}
                            </h3>
                            <div className="flex items-center gap-2 mt-2 flex-wrap">
                              {article.category && (
                                <Badge variant="secondary" className="text-xs">
                                  {article.category.name}
                                </Badge>
                              )}
                              <span className="text-xs text-muted-foreground">
                                {format(
                                  new Date(article.created_at),
                                  'yyyy-MM-dd',
                                  { locale: locale === 'zh' ? zhCN : undefined }
                                )}
                              </span>
                            </div>
                          </div>
                          <ChevronRight className="h-4 w-4 text-muted-foreground shrink-0 mt-1" />
                        </div>
                      </CardContent>
                    </Card>
                  </Link>
                ))}
              </div>

              {/* Pagination */}
              {totalPages > 1 && (
                <div className="flex items-center justify-center gap-2 pt-4">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage(p => Math.max(1, p - 1))}
                    disabled={page === 1}
                  >
                    <ChevronLeft className="h-4 w-4" />
                  </Button>
                  <span className="text-sm text-muted-foreground px-2">
                    {page} / {totalPages}
                  </span>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                    disabled={page === totalPages}
                  >
                    <ChevronRight className="h-4 w-4" />
                  </Button>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  )
}
