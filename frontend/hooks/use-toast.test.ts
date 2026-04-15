import { useToast } from './use-toast'

describe('useToast', () => {
  it('returns a stable api reference across calls', () => {
    const first = useToast()
    const second = useToast()

    expect(second).toBe(first)
    expect(typeof first.success).toBe('function')
    expect(typeof first.error).toBe('function')
    expect(typeof first.loading).toBe('function')
    expect(typeof first.promise).toBe('function')
    expect(typeof first.dismiss).toBe('function')
  })
})
