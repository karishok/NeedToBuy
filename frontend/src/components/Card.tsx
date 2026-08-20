import type { ReactNode } from 'react'

interface CardProps {
  kicker?: string
  children: ReactNode
  className?: string
}

export function Card({ kicker, children, className }: CardProps) {
  const classes = ['card', 'elev-md', className].filter(Boolean).join(' ')
  return (
    <div className={classes}>
      {kicker ? <div className="card-kicker">{kicker}</div> : null}
      {children}
    </div>
  )
}
