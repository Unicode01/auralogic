import { sanitizeAuthBrandingHtml } from '@/lib/auth-branding-html'

describe('auth-branding-html', () => {
  it('removes active content from auth branding html', () => {
    const rendered = sanitizeAuthBrandingHtml(
      '<section><script>alert(1)</script><img src="/logo.png" onerror="alert(1)" /></section>'
    )

    expect(rendered).not.toContain('<script')
    expect(rendered).not.toContain('onerror=')
    expect(rendered).toContain('<img src="/logo.png">')
  })

  it('returns empty string for blank input', () => {
    expect(sanitizeAuthBrandingHtml('   ')).toBe('')
  })
})
