import { preparePluginHtmlForRender } from '@/lib/plugin-html-sanitize'

describe('plugin-html-sanitize', () => {
  it('sanitizes plugin html and enhances images for lazy loading', () => {
    const rendered = preparePluginHtmlForRender(
      '<div><script>alert(1)</script><img src="/demo.png" width="640" height="360"></div>'
    )

    expect(rendered).not.toContain('<script')
    expect(rendered).toContain('loading="lazy"')
    expect(rendered).toContain('decoding="async"')
    expect(rendered).toContain('aspect-ratio: 640 / 360')
    expect(rendered).toContain('max-width: 100%')
    expect(rendered).toContain('height: auto')
  })

  it('removes style tags in sanitize mode to avoid host-page css pollution', () => {
    const rendered = preparePluginHtmlForRender(
      '<div><style>svg * { fill: transparent !important; }</style><p>hello</p></div>'
    )

    expect(rendered).not.toContain('<style')
    expect(rendered).toContain('<p>hello</p>')
  })

  it('preserves explicit image loading attributes while appending responsive styles', () => {
    const rendered = preparePluginHtmlForRender(
      '<img src="/demo.png" loading="eager" decoding="sync" style="border-radius: 12px" />',
      {
        trusted: true,
      }
    )

    expect(rendered).toContain('loading="eager"')
    expect(rendered).toContain('decoding="sync"')
    expect(rendered).toContain('border-radius: 12px')
    expect(rendered).toContain('max-width: 100%')
    expect(rendered).toContain('height: auto')
  })
})
