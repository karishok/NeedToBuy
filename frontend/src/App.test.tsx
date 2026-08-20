import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
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

  it('logs in successfully and lands on the placeholder home', async () => {
    vi.spyOn(client, 'me').mockRejectedValue(new Error('not logged in'))
    vi.spyOn(client, 'requestOtp').mockResolvedValue({ status: 'sent' })
    vi.spyOn(client, 'verifyOtp').mockResolvedValue({ status: 'ok' })
    const user = userEvent.setup()

    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    )

    await user.type(await screen.findByLabelText('Email'), 'parent@example.com')
    await user.click(screen.getByRole('button', { name: 'Получить код' }))
    await user.type(await screen.findByLabelText('Код из письма'), '123456')
    await user.click(screen.getByRole('button', { name: 'Подтвердить' }))

    await waitFor(() =>
      expect(screen.getByText('Вы вошли как parent@example.com')).toBeInTheDocument(),
    )
  })

  it('logs out and returns to the login form', async () => {
    vi.spyOn(client, 'me').mockResolvedValue({ email: 'parent@example.com' })
    vi.spyOn(client, 'logout').mockResolvedValue({ status: 'ok' })
    const user = userEvent.setup()

    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    )

    await user.click(await screen.findByRole('button', { name: 'Выйти' }))

    await waitFor(() => expect(screen.getByLabelText('Email')).toBeInTheDocument())
  })
})
