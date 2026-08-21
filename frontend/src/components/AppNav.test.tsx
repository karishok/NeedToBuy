import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AppNav } from './AppNav'
import { AuthProvider } from '../auth/AuthContext'
import * as client from '../api/client'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('AppNav', () => {
  it('shows a login link when anonymous', async () => {
    vi.spyOn(client, 'me').mockRejectedValue(new Error('not logged in'))
    render(
      <MemoryRouter>
        <AuthProvider>
          <AppNav />
        </AuthProvider>
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText('Войти')).toBeInTheDocument())
  })

  it('shows a logout button when authenticated', async () => {
    vi.spyOn(client, 'me').mockResolvedValue({ email: 'parent@example.com' })
    render(
      <MemoryRouter>
        <AuthProvider>
          <AppNav />
        </AuthProvider>
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByRole('button', { name: 'Выйти' })).toBeInTheDocument())
  })

  it('shows neither login nor logout while the session is loading', () => {
    vi.spyOn(client, 'me').mockReturnValue(new Promise(() => {}))
    render(
      <MemoryRouter>
        <AuthProvider>
          <AppNav />
        </AuthProvider>
      </MemoryRouter>,
    )
    expect(screen.queryByText('Войти')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Выйти' })).toBeNull()
  })
})
