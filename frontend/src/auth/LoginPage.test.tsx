import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { LoginPage } from './LoginPage'
import { AuthProvider } from './AuthContext'
import * as client from '../api/client'

afterEach(() => {
  vi.restoreAllMocks()
})

function renderLoginPage() {
  vi.spyOn(client, 'me').mockRejectedValue(new Error('not logged in'))
  return render(
    <MemoryRouter>
      <AuthProvider>
        <LoginPage />
      </AuthProvider>
    </MemoryRouter>,
  )
}

describe('LoginPage', () => {
  it('requests a code, then verifies it', async () => {
    const requestOtp = vi.spyOn(client, 'requestOtp').mockResolvedValue({ status: 'sent' })
    const verifyOtp = vi.spyOn(client, 'verifyOtp').mockResolvedValue({ status: 'ok' })
    const user = userEvent.setup()
    renderLoginPage()

    await user.type(screen.getByLabelText('Email'), 'parent@example.com')
    await user.click(screen.getByRole('button', { name: 'Получить код' }))

    await waitFor(() => expect(requestOtp).toHaveBeenCalledWith('parent@example.com'))
    expect(await screen.findByLabelText('Код из письма')).toBeInTheDocument()

    await user.type(screen.getByLabelText('Код из письма'), '123456')
    await user.click(screen.getByRole('button', { name: 'Подтвердить' }))

    await waitFor(() => expect(verifyOtp).toHaveBeenCalledWith('parent@example.com', '123456'))
  })

  it('shows the server error message when the code is rejected', async () => {
    vi.spyOn(client, 'requestOtp').mockResolvedValue({ status: 'sent' })
    vi.spyOn(client, 'verifyOtp').mockRejectedValue(
      new client.ApiError('bad_request', 'invalid or expired code', 400),
    )
    const user = userEvent.setup()
    renderLoginPage()

    await user.type(screen.getByLabelText('Email'), 'parent@example.com')
    await user.click(screen.getByRole('button', { name: 'Получить код' }))
    await screen.findByLabelText('Код из письма')

    await user.type(screen.getByLabelText('Код из письма'), '000000')
    await user.click(screen.getByRole('button', { name: 'Подтвердить' }))

    expect(await screen.findByText('invalid or expired code')).toBeInTheDocument()
  })

  it('shows the cooldown message on a 429 from requesting a code', async () => {
    vi.spyOn(client, 'requestOtp').mockRejectedValue(
      new client.ApiError('too_many_requests', 'code already sent, try again shortly', 429),
    )
    const user = userEvent.setup()
    renderLoginPage()

    await user.type(screen.getByLabelText('Email'), 'parent@example.com')
    await user.click(screen.getByRole('button', { name: 'Получить код' }))

    expect(await screen.findByText('code already sent, try again shortly')).toBeInTheDocument()
  })
})
