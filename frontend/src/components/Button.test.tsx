import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Button } from './Button'

describe('Button', () => {
  it('applies the primary variant class by default', () => {
    render(<Button>Click me</Button>)
    const button = screen.getByRole('button', { name: 'Click me' })
    expect(button.className).toContain('btn-primary')
  })

  it('applies the requested variant and block modifier', () => {
    render(
      <Button variant="ghost" block>
        Cancel
      </Button>,
    )
    const button = screen.getByRole('button', { name: 'Cancel' })
    expect(button.className).toContain('btn-ghost')
    expect(button.className).toContain('btn-block')
  })
})
