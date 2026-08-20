import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Field } from './Field'

describe('Field', () => {
  it('associates the label with the input via id', () => {
    render(<Field id="email" label="Email" />)
    const input = screen.getByLabelText('Email')
    expect(input).toHaveClass('input')
  })
})
