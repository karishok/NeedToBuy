import type { InputHTMLAttributes } from 'react'

interface FieldProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string
}

export function Field({ label, id, className, ...rest }: FieldProps) {
  const classes = ['input', className].filter(Boolean).join(' ')
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <input id={id} className={classes} {...rest} />
    </div>
  )
}
