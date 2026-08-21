import type { SelectHTMLAttributes } from 'react'

export interface SelectOptionGroup {
  label: string
  options: { value: string; label: string }[]
}

interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label: string
  groups: SelectOptionGroup[]
}

export function Select({ label, id, groups, className, ...rest }: SelectProps) {
  const classes = ['input', className].filter(Boolean).join(' ')
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <select id={id} className={classes} {...rest}>
        {groups.map((group) => (
          <optgroup key={group.label} label={group.label}>
            {group.options.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </optgroup>
        ))}
      </select>
    </div>
  )
}
