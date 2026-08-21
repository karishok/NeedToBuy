import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { CatalogPage } from './CatalogPage'
import * as client from '../api/client'

afterEach(() => {
  vi.restoreAllMocks()
})

const SEED_ITEM = {
  id: 1,
  age_range_code: '18m',
  category: 'toys',
  title: 'Сортер с крупными деталями',
  marketplace_search_url: 'https://www.ozon.ru/search/?text=сортер',
}

describe('CatalogPage', () => {
  it('renders items returned by getCatalog', async () => {
    vi.spyOn(client, 'getCatalog').mockResolvedValue([SEED_ITEM])
    render(<CatalogPage />)
    await waitFor(() => expect(screen.getByText('Сортер с крупными деталями')).toBeInTheDocument())
  })

  it('shows the empty-state message when there are no results', async () => {
    vi.spyOn(client, 'getCatalog').mockResolvedValue([])
    render(<CatalogPage />)
    await waitFor(() =>
      expect(
        screen.getByText('Пока нет идей для этого возраста и категории — попробуйте другой фильтр.'),
      ).toBeInTheDocument(),
    )
  })

  it('shows the server error message when the request fails', async () => {
    vi.spyOn(client, 'getCatalog').mockRejectedValue(
      new client.ApiError('bad_request', 'category is not a known category', 400),
    )
    render(<CatalogPage />)
    await waitFor(() => expect(screen.getByText('category is not a known category')).toBeInTheDocument())
  })

  it('re-fetches with the new category when the filter changes', async () => {
    const getCatalog = vi.spyOn(client, 'getCatalog').mockResolvedValue([SEED_ITEM])
    const user = userEvent.setup()
    render(<CatalogPage />)
    await waitFor(() => expect(getCatalog).toHaveBeenCalledWith({ ageRange: undefined, category: undefined }))

    await user.click(screen.getByRole('radio', { name: 'Игрушки' }))

    await waitFor(() => expect(getCatalog).toHaveBeenCalledWith({ ageRange: undefined, category: 'toys' }))
  })
})
