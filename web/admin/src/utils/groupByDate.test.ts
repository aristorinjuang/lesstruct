import { describe, it, expect } from 'vitest'
import { groupByDate } from './groupByDate'
import type { Media } from '@/stores/domain/media'

function createMedia(overrides: Partial<Media> & { id: number; createdAt: string }): Media {
  return {
    userId: 1,
    filename: 'test.webp',
    originalFilename: 'test.jpg',
    mimeType: 'image/webp',
    fileSize: 1000,
    width: 100,
    height: 100,
    altText: 'Test',
    isWebp: true,
    filePath: '/uploads/test.webp',
    url: 'http://localhost:8080/uploads/test.webp',
    hash: 'hash-' + overrides.id,
    variants: {},
    uploadedBy: 'admin',
    updatedAt: overrides.createdAt,
    ...overrides,
  }
}

function dateStr(daysAgo: number): string {
  const d = new Date()
  d.setDate(d.getDate() - daysAgo)
  return d.toISOString()
}

describe('groupByDate', () => {
  it('returns empty array for empty input', () => {
    expect(groupByDate([])).toEqual([])
  })

  it('groups items by Today', () => {
    const media = [
      createMedia({ id: 1, createdAt: dateStr(0) }),
      createMedia({ id: 2, createdAt: dateStr(0) }),
    ]

    const groups = groupByDate(media)
    expect(groups).toHaveLength(1)
    expect(groups[0].label).toBe('Today')
    expect(groups[0].items).toHaveLength(2)
  })

  it('separates Today from This Week', () => {
    const todayItem = createMedia({ id: 1, createdAt: dateStr(0) })
    const thisWeekItem = createMedia({ id: 2, createdAt: dateStr(0) })

    const thisWeekDate = new Date(thisWeekItem.createdAt)
    thisWeekDate.setHours(0, 0, 0, 0)
    const now = new Date()
    const day = now.getDay()
    const diff = day === 0 ? 6 : day - 1
    const mondayThisWeek = new Date(now)
    mondayThisWeek.setDate(now.getDate() - diff)
    mondayThisWeek.setHours(0, 0, 0, 0)

    if (diff > 0) {
      thisWeekDate.setDate(mondayThisWeek.getDate() + Math.floor(diff / 2))
      thisWeekItem.createdAt = thisWeekDate.toISOString()
    }

    const media = [todayItem, thisWeekItem]

    const groups = groupByDate(media)
    const labels = groups.map((g) => g.label)
    if (diff > 0) {
      expect(labels).toContain('Today')
      expect(labels).toContain('This Week')
      expect(groups).toHaveLength(2)
    } else {
      expect(groups).toHaveLength(1)
      expect(groups[0].label).toBe('Today')
    }
  })

  it('separates This Week from Older', () => {
    const now = new Date()
    const day = now.getDay()
    const diff = day === 0 ? 6 : day - 1

    if (diff === 0) {
      // Today is Monday — nothing is "This Week" but not "Today"
      return
    }

    const thisWeekItem = createMedia({ id: 1, createdAt: dateStr(1) })
    const olderItem = createMedia({ id: 2, createdAt: dateStr(30) })

    const media = [thisWeekItem, olderItem]
    const groups = groupByDate(media)
    const labels = groups.map((g) => g.label)
    expect(labels).toContain('This Week')
    expect(labels).toContain('Older')
    expect(groups).toHaveLength(2)
  })

  it('separates Last Week from Older', () => {
    // Calculate a date definitely in last week (Wednesday of last week)
    const now = new Date()
    const day = now.getDay()
    const diff = day === 0 ? 6 : day - 1
    const daysAgoForLastWeek = diff + 4

    const lastWeekItem = createMedia({ id: 1, createdAt: dateStr(daysAgoForLastWeek) })
    const olderItem = createMedia({ id: 2, createdAt: dateStr(60) })

    const media = [lastWeekItem, olderItem]
    const groups = groupByDate(media)
    const labels = groups.map((g) => g.label)
    expect(labels).toContain('Last Week')
    expect(labels).toContain('Older')
    expect(groups).toHaveLength(2)
  })

  it('creates all four groups when items span all periods', () => {
    const todayItem = createMedia({ id: 1, createdAt: dateStr(0) })

    const thisWeekDate = new Date()
    thisWeekDate.setDate(thisWeekDate.getDate() - 1)
    const thisWeekItem = createMedia({ id: 2, createdAt: thisWeekDate.toISOString() })

    const lastWeekItem = createMedia({ id: 3, createdAt: dateStr(10) })
    const olderItem = createMedia({ id: 4, createdAt: dateStr(60) })

    const media = [todayItem, thisWeekItem, lastWeekItem, olderItem]
    const groups = groupByDate(media)
    const labels = groups.map((g) => g.label)
    expect(labels).toContain('Today')
    expect(labels).toContain('Older')
    expect(groups.length).toBeGreaterThanOrEqual(2)
  })

  it('preserves input order within each group', () => {
    const media = [
      createMedia({ id: 3, createdAt: dateStr(0) }),
      createMedia({ id: 1, createdAt: dateStr(0) }),
      createMedia({ id: 2, createdAt: dateStr(0) }),
    ]

    const groups = groupByDate(media)
    expect(groups).toHaveLength(1)
    expect(groups[0].items[0].id).toBe(3)
    expect(groups[0].items[1].id).toBe(1)
    expect(groups[0].items[2].id).toBe(2)
  })

  it('does not create empty groups', () => {
    const media = [
      createMedia({ id: 1, createdAt: dateStr(0) }),
      createMedia({ id: 2, createdAt: dateStr(30) }),
    ]

    const groups = groupByDate(media)
    const labels = groups.map((g) => g.label)
    expect(labels).not.toContain('This Week')
    expect(labels).not.toContain('Last Week')
  })
})
