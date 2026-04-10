import {
  prepareMarkdownContentForRender,
  sanitizeMarkdownHtml,
  stripMarkdownHtmlForStaticRender,
} from '@/lib/markdown-html-sanitize'

describe('markdown-html-sanitize', () => {
  it('removes raw html during static render fallback', () => {
    const rendered = stripMarkdownHtmlForStaticRender(
      '<script>alert(1)</script><h2>Title</h2><p>Hello <strong>world</strong></p>'
    )

    expect(rendered).toBe('TitleHello world')
  })

  it('sanitizes allowed html but removes dangerous content', () => {
    const rendered = sanitizeMarkdownHtml(
      '<p><span style="color: #ef4444">safe</span><img src="javascript:alert(1)" onerror="alert(1)" /></p>'
    )

    expect(rendered).toContain('<span style="color: #ef4444">safe</span>')
    expect(rendered).not.toContain('onerror')
    expect(rendered).not.toContain('javascript:')
  })

  it('uses text-only fallback before hydration even when html is allowed', () => {
    const rendered = prepareMarkdownContentForRender(
      '<p>Hello</p><div>World</div>',
      { allowHtml: true, hydrated: false }
    )

    expect(rendered).toBe('HelloWorld')
  })

  it('renders sanitized html after hydration when html is allowed', () => {
    const rendered = prepareMarkdownContentForRender(
      '<p>Hello</p><span style="color: #22c55e">World</span>',
      { allowHtml: true, hydrated: true }
    )

    expect(rendered).toContain('<p>Hello</p>')
    expect(rendered).toContain('<span style="color: #22c55e">World</span>')
  })
})
