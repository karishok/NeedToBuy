import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Select } from './Select'

describe('Select', () => {
  it('renders grouped options with the input class', () => {
    render(
      <Select
        id="age"
        label="Возраст"
        groups={[
          { label: '0–6 мес', options: [{ value: '0m', label: '0m' }] },
          { label: '6–18 мес', options: [{ value: '6m', label: '6m' }] },
        ]}
      />,
    )
    const select = screen.getByLabelText('Возраст')
    expect(select).toHaveClass('input')
    expect(screen.getByRole('option', { name: '0m' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: '6m' })).toBeInTheDocument()
  })
})
