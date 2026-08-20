import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import * as client from './api/client'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('App routing', () => {
  it('redirects to /login when not authenticated', async () => {
    vi.spyOn(client, 'me').mockRejectedValue(new Error('not logged in'))

    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    )

    await waitFor(() => expect(screen.getByLabelText('Email')).toBeInTheDocument())
  })

  it('shows the placeholder home when authenticated', async () => {
    vi.spyOn(client, 'me').mockResolvedValue({ email: 'parent@example.com' })

    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    )

    await waitFor(() => expect(screen.getByText('Вы вошли как parent@example.com')).toBeInTheDocument())
  })
})
