import { describe, expect, it } from 'vitest'
import { AGE_GROUPS, AGE_LABELS } from './ageGroups'

describe('AGE_GROUPS', () => {
  it('covers exactly the 23 known agerange codes with no duplicates', () => {
    const all = AGE_GROUPS.flatMap((group) => group.codes)
    expect(all).toHaveLength(23)
    expect(new Set(all).size).toBe(23)
  })

  it('has 5 named groups', () => {
    expect(AGE_GROUPS).toHaveLength(5)
  })

  it('matches the backend agerange canonical code list exactly', () => {
    const all = AGE_GROUPS.flatMap((group) => group.codes)
    expect(all).toEqual([
      '0m',
      '1m',
      '2m',
      '3m',
      '4m',
      '5m',
      '6m',
      '9m',
      '12m',
      '15m',
      '18m',
      '24m',
      '30m',
      '3y',
      '4y',
      '5y',
      '6y',
      '7y',
      '8y',
      '9y',
      '10y',
      '11y',
      '12y+',
    ])
  })
})

describe('AGE_LABELS', () => {
  it('has a label for all 23 known agerange codes', () => {
    expect(Object.keys(AGE_LABELS)).toHaveLength(23)
  })
})
