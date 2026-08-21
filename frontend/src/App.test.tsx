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
  it('shows the catalog at / regardless of session state', async () => {
    vi.spyOn(client, 'me').mockRejectedValue(new Error('not logged in'))
    vi.spyOn(client, 'getCatalog').mockResolvedValue([])

    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    )

    await waitFor(() => expect(screen.getByText('Идеи по возрасту')).toBeInTheDocument())
  })

  it('shows a login link in the nav when anonymous', async () => {
    vi.spyOn(client, 'me').mockRejectedValue(new Error('not logged in'))
    vi.spyOn(client, 'getCatalog').mockResolvedValue([])

    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    )

    await waitFor(() => expect(screen.getByText('Войти')).toBeInTheDocument())
  })

  it('shows a logout button in the nav when authenticated', async () => {
    vi.spyOn(client, 'me').mockResolvedValue({ email: 'parent@example.com' })
    vi.spyOn(client, 'getCatalog').mockResolvedValue([])

    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    )

    await waitFor(() => expect(screen.getByRole('button', { name: 'Выйти' })).toBeInTheDocument())
  })

  it('logs in successfully and lands on the catalog', async () => {
    vi.spyOn(client, 'me').mockRejectedValue(new Error('not logged in'))
    vi.spyOn(client, 'getCatalog').mockResolvedValue([])
    vi.spyOn(client, 'requestOtp').mockResolvedValue({ status: 'sent' })
    vi.spyOn(client, 'verifyOtp').mockResolvedValue({ status: 'ok' })
    const user = userEvent.setup()

    render(
      <MemoryRouter initialEntries={['/login']}>
        <App />
      </MemoryRouter>,
    )

    await user.type(await screen.findByLabelText('Email'), 'parent@example.com')
    await user.click(screen.getByRole('button', { name: 'Получить код' }))
    await user.type(await screen.findByLabelText('Код из письма'), '123456')
    await user.click(screen.getByRole('button', { name: 'Подтвердить' }))

    await waitFor(() => expect(screen.getByText('Идеи по возрасту')).toBeInTheDocument())
  })

  it('logs out and returns to the catalog with a login link', async () => {
    vi.spyOn(client, 'me').mockResolvedValue({ email: 'parent@example.com' })
    vi.spyOn(client, 'getCatalog').mockResolvedValue([])
    vi.spyOn(client, 'logout').mockResolvedValue({ status: 'ok' })
    const user = userEvent.setup()

    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    )

    await user.click(await screen.findByRole('button', { name: 'Выйти' }))

    await waitFor(() => expect(screen.getByText('Войти')).toBeInTheDocument())
  })
})
