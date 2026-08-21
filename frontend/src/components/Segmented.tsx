export interface SegmentedOption {
  value: string
  label: string
}

interface SegmentedProps {
  name: string
  ariaLabel: string
  options: SegmentedOption[]
  value: string
  onChange: (value: string) => void
}

export function Segmented({ name, ariaLabel, options, value, onChange }: SegmentedProps) {
  return (
    <div className="seg" role="radiogroup" aria-label={ariaLabel}>
      {options.map((option) => (
        <label key={option.value} className="seg-opt">
          <input
            type="radio"
            name={name}
            checked={value === option.value}
            onChange={() => onChange(option.value)}
          />
          {option.label}
        </label>
      ))}
    </div>
  )
}
