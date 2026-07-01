import {
  useEffect,
  useId,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from 'react'

export type SelectOption<T extends string> = {
  value: T
  label: string
}

type SelectProps<T extends string> = {
  id?: string
  value: T
  options: SelectOption<T>[]
  onChange: (value: T) => void
  renderValue?: (option: SelectOption<T>) => ReactNode
  renderOption?: (option: SelectOption<T>, selected: boolean) => ReactNode
  className?: string
}

export function Select<T extends string>({
  id,
  value,
  options,
  onChange,
  renderValue,
  renderOption,
  className,
}: SelectProps<T>) {
  const fallbackId = useId()
  const selectId = id ?? fallbackId
  const rootRef = useRef<HTMLDivElement>(null)
  const [open, setOpen] = useState(false)

  const selected = options.find((option) => option.value === value) ?? options[0]

  useEffect(() => {
    if (!open) return

    const onPointerDown = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false)
      }
    }

    document.addEventListener('mousedown', onPointerDown)
    return () => document.removeEventListener('mousedown', onPointerDown)
  }, [open])

  const selectOption = (next: T) => {
    onChange(next)
    setOpen(false)
  }

  const onKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    const index = options.findIndex((option) => option.value === value)

    if (event.key === 'Escape') {
      setOpen(false)
      return
    }

    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      setOpen((current) => !current)
      return
    }

    if (!open) return

    if (event.key === 'ArrowDown') {
      event.preventDefault()
      const next = options[Math.min(index + 1, options.length - 1)]
      if (next) selectOption(next.value)
    }

    if (event.key === 'ArrowUp') {
      event.preventDefault()
      const next = options[Math.max(index - 1, 0)]
      if (next) selectOption(next.value)
    }
  }

  const classes = ['custom-select', open ? 'is-open' : undefined, className]
    .filter(Boolean)
    .join(' ')

  return (
    <div ref={rootRef} className={classes}>
      <button
        type="button"
        id={selectId}
        className="custom-select-trigger"
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((current) => !current)}
        onKeyDown={onKeyDown}
      >
        <span className="custom-select-value">
          {renderValue ? renderValue(selected) : selected.label}
        </span>
        <span className="custom-select-chevron" aria-hidden />
      </button>

      {open ? (
        <ul className="custom-select-menu" role="listbox" aria-labelledby={selectId}>
          {options.map((option) => {
            const isSelected = option.value === value
            return (
              <li key={option.label} role="presentation">
                <button
                  type="button"
                  role="option"
                  aria-selected={isSelected}
                  className={isSelected ? 'custom-select-option is-selected' : 'custom-select-option'}
                  onClick={() => selectOption(option.value)}
                >
                  {renderOption ? renderOption(option, isSelected) : option.label}
                </button>
              </li>
            )
          })}
        </ul>
      ) : null}
    </div>
  )
}
