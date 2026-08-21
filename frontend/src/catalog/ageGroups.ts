export interface AgeGroup {
  label: string
  codes: string[]
}

// Mirrors the age grid from docs/mvp-decisions.md, grouping the 23 fixed
// bucket codes from backend/internal/agerange into 5 named periods for
// display. Purely presentational — the backend has no concept of these
// coarser groups, only the fine-grained codes themselves.
export const AGE_GROUPS: AgeGroup[] = [
  { label: '0–6 мес.', codes: ['0m', '1m', '2m', '3m', '4m', '5m'] },
  { label: '6–18 мес.', codes: ['6m', '9m', '12m', '15m'] },
  { label: '18 мес.–3 года', codes: ['18m', '24m', '30m'] },
  { label: '3–12 лет', codes: ['3y', '4y', '5y', '6y', '7y', '8y', '9y', '10y', '11y'] },
  { label: '12+', codes: ['12y+'] },
]

// Short Russian display labels for each of the 23 fixed agerange codes.
// Keep this in sync with AGE_GROUPS — ageGroups.test.ts asserts both have
// 23 entries so the two can't silently drift apart.
export const AGE_LABELS: Record<string, string> = {
  '0m': 'с рождения',
  '1m': '1 мес.',
  '2m': '2 мес.',
  '3m': '3 мес.',
  '4m': '4 мес.',
  '5m': '5 мес.',
  '6m': '6 мес.',
  '9m': '9 мес.',
  '12m': '1 год',
  '15m': '1 год 3 мес.',
  '18m': '1 год 6 мес.',
  '24m': '2 года',
  '30m': '2 года 6 мес.',
  '3y': '3 года',
  '4y': '4 года',
  '5y': '5 лет',
  '6y': '6 лет',
  '7y': '7 лет',
  '8y': '8 лет',
  '9y': '9 лет',
  '10y': '10 лет',
  '11y': '11 лет',
  '12y+': '12 лет и старше',
}
