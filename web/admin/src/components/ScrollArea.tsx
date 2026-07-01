import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type MouseEvent as ReactMouseEvent,
  type ReactNode,
} from 'react'

type ScrollAreaProps = {
  children: ReactNode
  className?: string
  tone?: 'light' | 'dark'
}

export function ScrollArea({ children, className, tone = 'light' }: ScrollAreaProps) {
  const rootRef = useRef<HTMLDivElement>(null)
  const viewportRef = useRef<HTMLDivElement>(null)
  const dragRef = useRef<{ startY: number; startScrollTop: number } | null>(null)
  const hideTimerRef = useRef<number | undefined>(undefined)
  const [active, setActive] = useState(false)
  const [needsScroll, setNeedsScroll] = useState(false)
  const [thumbHeight, setThumbHeight] = useState(0)
  const [thumbTop, setThumbTop] = useState(0)

  const updateThumb = useCallback(() => {
    const viewport = viewportRef.current
    if (!viewport) return

    const { scrollHeight, clientHeight, scrollTop } = viewport
    const scrollable = scrollHeight > clientHeight + 1
    setNeedsScroll(scrollable)

    if (!scrollable) {
      setThumbHeight(0)
      setThumbTop(0)
      return
    }

    const ratio = clientHeight / scrollHeight
    const nextThumbHeight = Math.max(clientHeight * ratio, 36)
    const maxThumbTop = clientHeight - nextThumbHeight
    const maxScroll = scrollHeight - clientHeight
    const scrollRatio = maxScroll > 0 ? scrollTop / maxScroll : 0

    setThumbHeight(nextThumbHeight)
    setThumbTop(scrollRatio * maxThumbTop)
  }, [])

  const scheduleHide = useCallback(() => {
    window.clearTimeout(hideTimerRef.current)
    hideTimerRef.current = window.setTimeout(() => {
      if (!dragRef.current) setActive(false)
    }, 900)
  }, [])

  useEffect(() => {
    const viewport = viewportRef.current
    const root = rootRef.current
    if (!viewport || !root) return

    updateThumb()

    const onScroll = () => {
      updateThumb()
      setActive(true)
      scheduleHide()
    }

    const onMouseEnter = () => setActive(true)
    const onMouseLeave = () => scheduleHide()

    viewport.addEventListener('scroll', onScroll, { passive: true })
    root.addEventListener('mouseenter', onMouseEnter)
    root.addEventListener('mouseleave', onMouseLeave)

    const resizeObserver = new ResizeObserver(() => updateThumb())
    resizeObserver.observe(viewport)
    for (const child of viewport.children) {
      resizeObserver.observe(child)
    }

    const mutationObserver = new MutationObserver(() => updateThumb())
    mutationObserver.observe(viewport, {
      childList: true,
      subtree: true,
      characterData: true,
      attributes: true,
    })

    return () => {
      viewport.removeEventListener('scroll', onScroll)
      root.removeEventListener('mouseenter', onMouseEnter)
      root.removeEventListener('mouseleave', onMouseLeave)
      resizeObserver.disconnect()
      mutationObserver.disconnect()
      window.clearTimeout(hideTimerRef.current)
    }
  }, [scheduleHide, updateThumb, children])

  const onThumbMouseDown = (event: ReactMouseEvent<HTMLDivElement>) => {
    event.preventDefault()
    const viewport = viewportRef.current
    if (!viewport) return

    dragRef.current = { startY: event.clientY, startScrollTop: viewport.scrollTop }
    setActive(true)

    const onMouseMove = (moveEvent: MouseEvent) => {
      const drag = dragRef.current
      const currentViewport = viewportRef.current
      if (!drag || !currentViewport) return

      const { scrollHeight, clientHeight, scrollTop } = currentViewport
      const maxScroll = scrollHeight - clientHeight
      const ratio = clientHeight / scrollHeight
      const nextThumbHeight = Math.max(clientHeight * ratio, 36)
      const maxThumbTop = clientHeight - nextThumbHeight
      if (maxThumbTop <= 0) return

      const scrollPerPixel = maxScroll / maxThumbTop
      currentViewport.scrollTop = drag.startScrollTop + (moveEvent.clientY - drag.startY) * scrollPerPixel
    }

    const onMouseUp = () => {
      dragRef.current = null
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
      scheduleHide()
    }

    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
  }

  const classes = [
    'scroll-area',
    tone === 'dark' ? 'scroll-area-dark' : undefined,
    active ? 'is-active' : undefined,
    className,
  ]
    .filter(Boolean)
    .join(' ')

  return (
    <div ref={rootRef} className={classes}>
      <div ref={viewportRef} className="scroll-area-viewport hide-native-scrollbar">
        {children}
      </div>
      {needsScroll ? (
        <div
          className="scroll-area-thumb"
          style={{ height: thumbHeight, transform: `translateY(${thumbTop}px)` }}
          onMouseDown={onThumbMouseDown}
          aria-hidden
        />
      ) : null}
    </div>
  )
}
