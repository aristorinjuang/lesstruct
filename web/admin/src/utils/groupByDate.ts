import type { Media } from '@/stores/domain/media'

export interface DateGroup {
  label: string
  items: Media[]
}

function getStartOfWeek(date: Date): Date {
  const d = new Date(date)
  const day = d.getDay()
  const diff = day === 0 ? 6 : day - 1 // Monday-based
  d.setDate(d.getDate() - diff)
  d.setHours(0, 0, 0, 0)
  return d
}

function getStartOfLastWeek(date: Date): Date {
  const thisWeekStart = getStartOfWeek(date)
  const lastWeekStart = new Date(thisWeekStart)
  lastWeekStart.setDate(lastWeekStart.getDate() - 7)
  return lastWeekStart
}

function isSameDay(a: Date, b: Date): boolean {
  return a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
}

export function groupByDate(media: Media[]): DateGroup[] {
  if (media.length === 0) return []

  const now = new Date()
  const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const thisWeekStart = getStartOfWeek(now)
  const lastWeekStart = getStartOfLastWeek(now)

  const groups: DateGroup[] = []
  let currentGroup: DateGroup | null = null

  for (const item of media) {
    const itemDate = new Date(item.createdAt)
    let label: string

    if (isSameDay(itemDate, now)) {
      label = 'Today'
    } else if (itemDate >= thisWeekStart) {
      label = 'This Week'
    } else if (itemDate >= lastWeekStart) {
      label = 'Last Week'
    } else {
      label = 'Older'
    }

    if (!currentGroup || currentGroup.label !== label) {
      currentGroup = { label, items: [] }
      groups.push(currentGroup)
    }
    currentGroup.items.push(item)
  }

  return groups
}
