/**
 * Tests for the notification store
 */
import { describe, it, expect, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useNotificationStore } from './notifications'

describe('useNotificationStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  describe('initial state', () => {
    it('should have zero counts for all notification types', () => {
      const store = useNotificationStore()
      expect(store.counts.pendingRegistrations).toBe(0)
      expect(store.counts.pendingComments).toBe(0)
      expect(store.counts.pendingUpdates).toBe(0)
    })

    it('should not have pending notifications initially', () => {
      const store = useNotificationStore()
      expect(store.hasPendingNotifications).toBe(false)
    })
  })

  describe('syncFromDashboard', () => {
    it('should sync pendingRegistrations from dashboard data', () => {
      const store = useNotificationStore()
      store.syncFromDashboard(5)

      expect(store.counts.pendingRegistrations).toBe(5)
    })

    it('should overwrite existing pendingRegistrations on sync', () => {
      const store = useNotificationStore()
      store.counts.pendingRegistrations = 3
      store.syncFromDashboard(10)

      expect(store.counts.pendingRegistrations).toBe(10)
    })

    it('should clamp negative values to zero', () => {
      const store = useNotificationStore()
      store.syncFromDashboard(-1)

      expect(store.counts.pendingRegistrations).toBe(0)
    })

    it('should not affect other notification type counts', () => {
      const store = useNotificationStore()
      store.counts.pendingComments = 5
      store.counts.pendingUpdates = 2
      store.syncFromDashboard(10)

      expect(store.counts.pendingRegistrations).toBe(10)
      expect(store.counts.pendingComments).toBe(5)
      expect(store.counts.pendingUpdates).toBe(2)
    })
  })

  describe('decrementCount', () => {
    it('should decrement the count for a specific notification type', () => {
      const store = useNotificationStore()
      store.counts.pendingRegistrations = 5

      store.decrementCount('pendingRegistrations')

      expect(store.counts.pendingRegistrations).toBe(4)
    })

    it('should not decrement below zero', () => {
      const store = useNotificationStore()
      store.counts.pendingRegistrations = 0

      store.decrementCount('pendingRegistrations')

      expect(store.counts.pendingRegistrations).toBe(0)
    })

    it('should decrement pendingComments count', () => {
      const store = useNotificationStore()
      store.counts.pendingComments = 3

      store.decrementCount('pendingComments')

      expect(store.counts.pendingComments).toBe(2)
    })

    it('should decrement pendingUpdates count', () => {
      const store = useNotificationStore()
      store.counts.pendingUpdates = 1

      store.decrementCount('pendingUpdates')

      expect(store.counts.pendingUpdates).toBe(0)
    })
  })

  describe('hasPendingNotifications computed', () => {
    it('should return true when any count is greater than 0', () => {
      const store = useNotificationStore()
      store.counts.pendingRegistrations = 1
      expect(store.hasPendingNotifications).toBe(true)
    })

    it('should return true when multiple counts are greater than 0', () => {
      const store = useNotificationStore()
      store.counts.pendingRegistrations = 5
      store.counts.pendingComments = 3
      expect(store.hasPendingNotifications).toBe(true)
    })

    it('should return false when all counts are 0', () => {
      const store = useNotificationStore()
      expect(store.hasPendingNotifications).toBe(false)
    })
  })

  describe('pendingRegistrations computed', () => {
    it('should return the pendingRegistrations count', () => {
      const store = useNotificationStore()
      store.counts.pendingRegistrations = 7
      expect(store.pendingRegistrations).toBe(7)
    })

    it('should react to changes in counts', () => {
      const store = useNotificationStore()
      store.counts.pendingRegistrations = 3
      expect(store.pendingRegistrations).toBe(3)

      store.decrementCount('pendingRegistrations')
      expect(store.pendingRegistrations).toBe(2)
    })

    it('should reflect syncFromDashboard changes', () => {
      const store = useNotificationStore()
      store.syncFromDashboard(8)
      expect(store.pendingRegistrations).toBe(8)

      store.decrementCount('pendingRegistrations')
      expect(store.pendingRegistrations).toBe(7)
    })
  })
})
