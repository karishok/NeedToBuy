import { describe, expect, it } from 'vitest'
import { AGE_GROUPS } from './ageGroups'

describe('AGE_GROUPS', () => {
  it('covers exactly the 23 known agerange codes with no duplicates', () => {
    const all = AGE_GROUPS.flatMap((group) => group.codes)
    expect(all).toHaveLength(23)
    expect(new Set(all).size).toBe(23)
  })

  it('has 5 named groups', () => {
    expect(AGE_GROUPS).toHaveLength(5)
  })
})
