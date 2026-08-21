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
