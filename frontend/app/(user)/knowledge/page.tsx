'use client'

import { useState, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getKnowledgeCategoryTree, getKnowledgeArticles, KnowledgeCategory } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Search,
  ChevronLeft,
  ChevronRight,
  ChevronDown,
  BookOpen,
  Folder,
  FolderOpen,
  FileText,
  X,
} from 'lucide-react'
import Link from 'next/link'
import { format } from 'date-fns'
import { zhCN } from 'date-fns/locale'
import { cn } from '@/lib/utils'
import { useLocale } from '@/hooks/use-locale'
import { useIsMobile } from '@/hooks/use-mobile'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { PluginSlot } from '@/components/plugins/plugin-slot'

export default function KnowledgePage() {
  const { locale } = useLocale()
  const { isMobile, mounted } = useIsMobile()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.knowledge)
  const isCompactLayout = mounted ? isMobile : false

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
  const {
    data: articlesData,
    isLoading,
    isError,
    refetch,
  } = useQuery({
    queryKey: ['knowledgeArticles', page, categoryId, search],
    queryFn: () =>
      getKnowledgeArticles({
        page,
        limit,
        category_id: categoryId,
        search: search || undefined,
      }),
  })

  const categories: KnowledgeCategory[] = categoryData?.data || []
  const articles = articlesData?.data?.items || []
  const total = Number(articlesData?.data?.pagination?.total || 0)
  const totalPages = Number(articlesData?.data?.pagination?.total_pages || 0)

  const handleCategoryClick = (id?: string) => {
    setCategoryId(id)
    setPage(1)
  }

  const toggleExpand = (id: number) => {
    setExpandedCategories((prev) => {
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
            'flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-2 text-sm transition-colors',
            isActive ? 'bg-primary text-primary-foreground' : 'hover:bg-accent'
          )}
          style={{ paddingLeft: `${12 + depth * 16}px` }}
          onClick={() => handleCategoryClick(cat.id.toString())}
        >
          {hasChildren ? (
            <button
              type="button"
              className="-ml-0.5 shrink-0 p-0.5"
              onClick={(e) => {
                e.stopPropagation()
                toggleExpand(cat.id)
              }}
              aria-label={`${isExpanded ? t.common.collapse : t.common.expand} ${cat.name}`}
              title={`${isExpanded ? t.common.collapse : t.common.expand} ${cat.name}`}
            >
              {isExpanded ? (
                <ChevronDown className="h-3.5 w-3.5" />
              ) : (
                <ChevronRight className="h-3.5 w-3.5" />
              )}
            </button>
          ) : (
            <span className="w-[18px] shrink-0" />
          )}
          {hasChildren ? (
            isExpanded ? (
              <FolderOpen className="h-4 w-4 shrink-0 text-amber-500" />
            ) : (
              <Folder className="h-4 w-4 shrink-0 text-amber-500" />
            )
          ) : (
            <FileText className="h-4 w-4 shrink-0 opacity-50" />
          )}
          <span className="truncate">{cat.name}</span>
        </div>
        {hasChildren && isExpanded && (
          <div>{cat.children!.map((child) => renderCategoryItem(child, depth + 1))}</div>
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
  const hasActiveFilters = Boolean(categoryId || search)
  const knowledgeActiveFilterCount = Number(Boolean(categoryId)) + Number(Boolean(search))
  const userKnowledgePluginContext = {
    view: 'user_knowledge',
    filters: {
      page,
      category_id: categoryId,
      search: search || undefined,
    },
    pagination: {
      page,
      total,
      total_pages: totalPages,
      limit,
    },
    summary: {
      current_page_count: articles.length,
      category_count: flatCategories.length,
      active_filter_count: knowledgeActiveFilterCount,
    },
    state: {
      load_failed: isError && articles.length === 0,
      empty: !isLoading && !isError && articles.length === 0,
      has_active_filters: hasActiveFilters,
      has_categories: flatCategories.length > 0,
      has_pagination: totalPages > 1,
    },
  }

  const handleResetFilters = () => {
    setCategoryId(undefined)
    setSearch('')
    setSearchInput('')
    setPage(1)
  }

  const handleClearSearch = () => {
    setSearch('')
    setSearchInput('')
    setPage(1)
  }

  return (
    <div className="space-y-6">
      <PluginSlot slot="user.knowledge.top" context={userKnowledgePluginContext} />
      {/* Header */}
      <div>
        <h1 className={isCompactLayout ? 'text-2xl font-bold' : 'text-3xl font-bold'}>
          {t.knowledge.knowledgeBase}
        </h1>
      </div>

      {/* Search */}
      <div className="flex gap-2">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder={t.knowledge.searchArticles}
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
            className="pl-9 pr-9"
          />
          {searchInput && (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="absolute right-1 top-1/2 h-7 w-7 -translate-y-1/2 rounded-full text-muted-foreground hover:text-foreground"
              onClick={handleClearSearch}
            >
              <X className="h-4 w-4" />
              <span className="sr-only">{t.common.clear}</span>
            </Button>
          )}
        </div>
      </div>

      {hasActiveFilters ? (
        <div className="flex justify-end">
          <Button variant="ghost" size="sm" onClick={handleResetFilters}>
            {t.common.reset}
          </Button>
        </div>
      ) : null}
      <PluginSlot slot="user.knowledge.filters.after" context={userKnowledgePluginContext} />

      {/* Mobile: horizontal scrollable category chips */}
      {isCompactLayout ? (
        <div className="scrollbar-hide flex gap-2 overflow-x-auto pb-2">
          <Button
            variant={!categoryId ? 'default' : 'outline'}
            size="sm"
            className="shrink-0"
            onClick={() => handleCategoryClick(undefined)}
          >
            {t.knowledge.allCategories}
          </Button>
          {flatCategories.map((cat) => (
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
      ) : null}

      {/* Main content: sidebar + article list */}
      <div className={isCompactLayout ? 'space-y-4' : 'flex gap-6'}>
        {/* Desktop: category sidebar */}
        {!isCompactLayout ? (
          <div className="w-56 shrink-0">
            <Card>
              <CardContent className="p-2">
                <div className="space-y-0.5">
                  <div
                    className={cn(
                      'flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-2 text-sm transition-colors',
                      !categoryId ? 'bg-primary text-primary-foreground' : 'hover:bg-accent'
                    )}
                    onClick={() => handleCategoryClick(undefined)}
                  >
                    <span className="w-[18px] shrink-0" />
                    <BookOpen className="h-4 w-4 shrink-0" />
                    <span>{t.knowledge.allCategories}</span>
                  </div>
                  {categories.map((cat) => renderCategoryItem(cat))}
                </div>
              </CardContent>
            </Card>
          </div>
        ) : null}

        {/* Article list */}
        <div className="min-w-0 flex-1">
          {isLoading ? (
            <div className="space-y-3">
              {[...Array(3)].map((_, i) => (
                <Card key={i} className="animate-pulse">
                  <CardContent className="space-y-2 p-4">
                    <div className="h-5 w-3/4 rounded bg-muted" />
                    <div className="h-4 w-1/4 rounded bg-muted" />
                  </CardContent>
                </Card>
              ))}
            </div>
          ) : isError && articles.length === 0 ? (
            <Card className="border-dashed bg-muted/15">
              <CardContent className="py-12 text-center">
                <BookOpen className="mx-auto mb-4 h-12 w-12 text-muted-foreground" />
                <p className="text-base font-medium">{t.knowledge.loadFailed}</p>
                <p className="mt-2 text-sm text-muted-foreground">{t.knowledge.loadFailedDesc}</p>
                <div className="mt-4 flex flex-wrap justify-center gap-2">
                  <Button variant="outline" onClick={() => refetch()}>
                    {t.common.refresh}
                  </Button>
                  {hasActiveFilters ? (
                    <Button variant="ghost" onClick={handleResetFilters}>
                      {t.common.reset}
                    </Button>
                  ) : null}
                </div>
                <PluginSlot
                  slot="user.knowledge.load_failed"
                  context={{ ...userKnowledgePluginContext, section: 'list_state' }}
                />
              </CardContent>
            </Card>
          ) : articles.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center">
                <BookOpen className="mx-auto mb-4 h-12 w-12 text-muted-foreground" />
                <p className="text-base font-medium">
                  {search ? t.knowledge.noSearchResults : t.knowledge.noArticles}
                </p>
                {hasActiveFilters && (
                  <Button variant="outline" className="mt-4" onClick={handleResetFilters}>
                    {t.common.reset}
                  </Button>
                )}
                <PluginSlot
                  slot="user.knowledge.empty"
                  context={{ ...userKnowledgePluginContext, section: 'list_state' }}
                />
              </CardContent>
            </Card>
          ) : (
            <>
              <div className="space-y-3">
                {articles.map((article: any) => (
                  <Link key={article.id} href={`/knowledge/${article.id}`} className="block">
                    <Card className="cursor-pointer transition-colors hover:bg-accent/50">
                      <CardContent className="p-4">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0 flex-1">
                            <h3
                              className={cn(
                                'truncate text-sm font-medium',
                                !isCompactLayout && 'md:text-base'
                              )}
                            >
                              {article.title}
                            </h3>
                            <div className="mt-2 flex flex-wrap items-center gap-2">
                              {article.category && (
                                <Badge variant="secondary" className="text-xs">
                                  {article.category.name}
                                </Badge>
                              )}
                              <span className="text-xs text-muted-foreground">
                                {format(new Date(article.created_at), 'yyyy-MM-dd', {
                                  locale: locale === 'zh' ? zhCN : undefined,
                                })}
                              </span>
                            </div>
                          </div>
                          <ChevronRight className="mt-1 h-4 w-4 shrink-0 text-muted-foreground" />
                        </div>
                      </CardContent>
                    </Card>
                  </Link>
                ))}
              </div>
              <PluginSlot
                slot="user.knowledge.list.after"
                context={{ ...userKnowledgePluginContext, section: 'list' }}
              />

              {/* Pagination */}
              {totalPages > 1 && (
                <>
                  <PluginSlot
                    slot="user.knowledge.pagination.before"
                    context={{ ...userKnowledgePluginContext, section: 'pagination' }}
                  />
                  <div className="flex items-center justify-center gap-2 pt-4">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPage((p) => Math.max(1, p - 1))}
                      disabled={page === 1}
                      aria-label={t.common.prevPage}
                      title={t.common.prevPage}
                    >
                      <ChevronLeft className="h-4 w-4" />
                      <span className="sr-only">{t.common.prevPage}</span>
                    </Button>
                    <span className="px-2 text-sm text-muted-foreground">
                      {t.common.pageInfo
                        .replace('{page}', String(page))
                        .replace('{totalPages}', String(totalPages))}
                    </span>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                      disabled={page === totalPages}
                      aria-label={t.common.nextPage}
                      title={t.common.nextPage}
                    >
                      <ChevronRight className="h-4 w-4" />
                      <span className="sr-only">{t.common.nextPage}</span>
                    </Button>
                  </div>
                  <PluginSlot
                    slot="user.knowledge.pagination.after"
                    context={{ ...userKnowledgePluginContext, section: 'pagination' }}
                  />
                </>
              )}
            </>
          )}
        </div>
      </div>
      <PluginSlot slot="user.knowledge.bottom" context={userKnowledgePluginContext} />
    </div>
  )
}
