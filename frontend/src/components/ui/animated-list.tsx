"use client"

import { useEffect, useState, type ReactNode } from "react"
import { AnimatePresence, motion, useReducedMotion } from "motion/react"
import { cn } from "@/lib/utils"

/**
 * Notification-stack animation in the style of Magic UI's AnimatedList: a new item springs in at
 * the top every `delay` ms and pushes the others down (layout animation). Unlike the original it
 * loops forever over `count` items and keeps only `max` items mounted. Static under reduced motion.
 */
export function AnimatedList({
  count,
  render,
  className,
  delay = 1600,
  max = 4,
}: {
  /** How many distinct items to cycle through. */
  count: number
  /** `index` = which item (0..count-1), `position` = 0 for the newest on top. */
  render: (index: number, position: number) => ReactNode
  className?: string
  delay?: number
  max?: number
}) {
  const reduced = useReducedMotion()
  const [tick, setTick] = useState(0)

  useEffect(() => {
    if (reduced || count === 0) return
    const id = window.setInterval(() => {
      if (document.visibilityState === "visible") setTick((n) => n + 1)
    }, delay)
    return () => window.clearInterval(id)
  }, [delay, count, reduced])

  // newest first; key = running counter so the same child can come round again as a new entry
  const shown = reduced
    ? Array.from({ length: Math.min(max, count) }, (_, i) => ({ key: i, index: i }))
    : Array.from({ length: Math.min(max, tick + 1) }, (_, i) => tick - i).map((n) => ({ key: n, index: n % count }))

  return (
    <div className={cn("flex flex-col items-center gap-3", className)}>
      <AnimatePresence initial={false}>
        {shown.map(({ key, index }, position) => (
          <motion.div
            key={key}
            layout
            initial={reduced ? false : { scale: 0, opacity: 0 }}
            animate={{ scale: 1, opacity: 1, originY: 0 }}
            exit={{ scale: 0, opacity: 0 }}
            transition={{ type: "spring", stiffness: 350, damping: 40 }}
            className="mx-auto w-full"
          >
            {render(index, position)}
          </motion.div>
        ))}
      </AnimatePresence>
    </div>
  )
}
