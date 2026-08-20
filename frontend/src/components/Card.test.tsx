import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Card } from './Card'

describe('Card', () => {
  it('renders the kicker and children', () => {
    render(<Card kicker="Вход">Hello</Card>)
    expect(screen.getByText('Вход')).toHaveClass('card-kicker')
    expect(screen.getByText('Hello')).toBeInTheDocument()
  })
})
