import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Segmented } from './Segmented'

describe('Segmented', () => {
  it('renders options and calls onChange with the selected value', async () => {
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(
      <Segmented
        name="cat"
        ariaLabel="Категория"
        value="all"
        onChange={onChange}
        options={[
          { value: 'all', label: 'Все' },
          { value: 'toys', label: 'Игрушки' },
        ]}
      />,
    )
    await user.click(screen.getByRole('radio', { name: 'Игрушки' }))
    expect(onChange).toHaveBeenCalledWith('toys')
  })

  it('marks the current value as checked', () => {
    render(
      <Segmented
        name="cat"
        ariaLabel="Категория"
        value="toys"
        onChange={() => {}}
        options={[
          { value: 'all', label: 'Все' },
          { value: 'toys', label: 'Игрушки' },
        ]}
      />,
    )
    expect(screen.getByRole('radio', { name: 'Игрушки' })).toBeChecked()
    expect(screen.getByRole('radio', { name: 'Все' })).not.toBeChecked()
  })
})
