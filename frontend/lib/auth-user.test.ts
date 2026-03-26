import { normalizeAuthUser } from '@/lib/auth-user'

describe('normalizeAuthUser', () => {
  it('maps legacy auth payload fields onto the frontend shape', () => {
    expect(
      normalizeAuthUser({
        user_id: 42,
        created_at: '2026-03-16T00:00:00Z',
        is_active: true,
      })
    ).toEqual({
      id: 42,
      user_id: 42,
      createdAt: '2026-03-16T00:00:00Z',
      created_at: '2026-03-16T00:00:00Z',
      isActive: true,
      is_active: true,
    })
  })

  it('preserves already-normalized fields while backfilling aliases', () => {
    expect(
      normalizeAuthUser({
        id: 7,
        createdAt: '2026-03-16T00:00:00Z',
        isActive: false,
      })
    ).toEqual({
      id: 7,
      user_id: 7,
      createdAt: '2026-03-16T00:00:00Z',
      created_at: '2026-03-16T00:00:00Z',
      isActive: false,
      is_active: false,
    })
  })
})
