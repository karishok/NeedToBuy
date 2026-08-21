import { useEffect, useState } from 'react'
import { Select } from '../components/Select'
import { Segmented } from '../components/Segmented'
import { getCatalog, ApiError, type CatalogItem } from '../api/client'
import { AGE_GROUPS, AGE_LABELS } from './ageGroups'

const CATEGORY_OPTIONS = [
  { value: '', label: 'Все' },
  { value: 'clothes', label: 'Одежда' },
  { value: 'toys', label: 'Игрушки' },
  { value: 'books', label: 'Книги' },
  { value: 'sport', label: 'Спорт' },
]

const CATEGORY_LABELS: Record<string, string> = {
  clothes: 'Одежда',
  toys: 'Игрушки',
  books: 'Книги',
  sport: 'Спорт',
}

const GENERIC_ERROR = 'Что-то пошло не так, попробуйте ещё раз'

export function CatalogPage() {
  const [ageRange, setAgeRange] = useState('')
  const [category, setCategory] = useState('')
  const [items, setItems] = useState<CatalogItem[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    getCatalog({ ageRange: ageRange || undefined, category: category || undefined })
      .then((result) => {
        if (!cancelled) setItems(result)
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof ApiError ? err.message : GENERIC_ERROR)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [ageRange, category])

  return (
    <div className="catalog-content">
      <h1>Идеи по возрасту</h1>
      <p>Курируем вручную — ссылки ведут на поиск в Ozon, а не на конкретный товар.</p>
      <div className="catalog-filters">
        <Select
          id="age-filter"
          label="Возраст"
          value={ageRange}
          onChange={(event) => setAgeRange(event.target.value)}
          groups={[
            { label: 'Все возраста', options: [{ value: '', label: 'Все' }] },
            ...AGE_GROUPS.map((group) => ({
              label: group.label,
              options: group.codes.map((code) => ({ value: code, label: AGE_LABELS[code] ?? code })),
            })),
          ]}
        />
        <Segmented
          name="category"
          ariaLabel="Категория"
          value={category}
          onChange={setCategory}
          options={CATEGORY_OPTIONS}
        />
      </div>
      {error ? <p className="error-text">{error}</p> : null}
      {!loading && !error && items.length === 0 ? (
        <p className="card-body">Пока нет идей для этого возраста и категории — попробуйте другой фильтр.</p>
      ) : null}
      <div className="catalog-grid">
        {items.map((item) => (
          <div key={item.id} className="card elev-sm">
            <div className="catalog-card-tags">
              <span className="tag tag-neutral">{item.age_range_code}</span>
              <span className="tag tag-outline">{CATEGORY_LABELS[item.category] ?? item.category}</span>
            </div>
            <div className="card-title">{item.title}</div>
            <a
              className="btn btn-ghost btn-block"
              href={item.marketplace_search_url}
              target="_blank"
              rel="noopener noreferrer"
            >
              Открыть на Ozon
            </a>
          </div>
        ))}
      </div>
    </div>
  )
}
