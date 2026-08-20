import { afterEach, describe, expect, it, vi } from 'vitest'
import { requestOtp, verifyOtp, me } from './client'

afterEach(() => {
  vi.unstubAllGlobals()
})

function mockFetch(status: number, body: unknown) {
  const fetchMock = vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

describe('requestOtp', () => {
  it('posts the email and includes credentials', async () => {
    const fetchMock = mockFetch(200, { status: 'sent' })
    await requestOtp('parent@example.com')

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/otp/request',
      expect.objectContaining({
        method: 'POST',
        credentials: 'include',
        body: JSON.stringify({ email: 'parent@example.com' }),
      }),
    )
  })
})

describe('verifyOtp', () => {
  it('throws an ApiError with the server message on a 400', async () => {
    mockFetch(400, { error: { code: 'bad_request', message: 'invalid or expired code' } })

    await expect(verifyOtp('parent@example.com', '000000')).rejects.toMatchObject({
      code: 'bad_request',
      message: 'invalid or expired code',
    })
  })
})

describe('me', () => {
  it('resolves with the email on success', async () => {
    mockFetch(200, { email: 'parent@example.com' })
    await expect(me()).resolves.toEqual({ email: 'parent@example.com' })
  })

  it('throws on a 401', async () => {
    mockFetch(401, { error: { code: 'unauthorized', message: 'login required' } })
    await expect(me()).rejects.toMatchObject({ status: 401 })
  })
})

describe('network errors', () => {
  it('propagates a rejected fetch instead of swallowing it', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockRejectedValue(new TypeError('Failed to fetch')),
    )
    await expect(me()).rejects.toThrow('Failed to fetch')
  })
})
