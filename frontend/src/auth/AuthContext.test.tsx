import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from './AuthContext'
import { useAuth } from './useAuth'
import * as client from '../api/client'

afterEach(() => {
  vi.restoreAllMocks()
})

function Probe() {
  const { session } = useAuth()
  if (session === 'loading') return <p>loading</p>
  if (session === null) return <p>logged out</p>
  return <p>logged in as {session.email}</p>
}

describe('AuthProvider', () => {
  it('restores the session when GET /api/auth/me succeeds', async () => {
    vi.spyOn(client, 'me').mockResolvedValue({ email: 'parent@example.com' })

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    )

    expect(screen.getByText('loading')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByText('logged in as parent@example.com')).toBeInTheDocument())
  })

  it('treats a failed /api/auth/me as logged out', async () => {
    vi.spyOn(client, 'me').mockRejectedValue(new Error('unauthorized'))

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    )

    await waitFor(() => expect(screen.getByText('logged out')).toBeInTheDocument())
  })
})
