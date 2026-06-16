import { describe, it, expect } from 'vitest'
import { validateCustomField, validateCustomFields } from './validation'
import type { FieldSchema } from '@/types/customfield'

describe('validateCustomField', () => {
  describe('required checks', () => {
    const field: FieldSchema = { name: 'Title', slug: 'title', type: 'text', required: true }

    it('returns error for empty required field', () => {
      expect(validateCustomField(field, '')).toBe('Title is required')
    })

    it('returns error for null required field', () => {
      expect(validateCustomField(field, null)).toBe('Title is required')
    })

    it('returns error for undefined required field', () => {
      expect(validateCustomField(field, undefined)).toBe('Title is required')
    })

    it('returns null for filled required field', () => {
      expect(validateCustomField(field, 'Hello')).toBeNull()
    })
  })

  describe('optional fields', () => {
    const field: FieldSchema = { name: 'Title', slug: 'title', type: 'text' }

    it('returns null for empty optional field', () => {
      expect(validateCustomField(field, '')).toBeNull()
    })

    it('returns null for null optional field', () => {
      expect(validateCustomField(field, null)).toBeNull()
    })
  })

  describe('text type', () => {
    it('returns error when maxLength is exceeded', () => {
      const field: FieldSchema = { name: 'Name', slug: 'name', type: 'text', maxLength: 5 }
      expect(validateCustomField(field, '123456')).toBe('Name must be 5 characters or less')
    })

    it('returns null when within maxLength', () => {
      const field: FieldSchema = { name: 'Name', slug: 'name', type: 'text', maxLength: 5 }
      expect(validateCustomField(field, '12345')).toBeNull()
    })
  })

  describe('textarea type', () => {
    it('returns error when maxLength is exceeded', () => {
      const field: FieldSchema = { name: 'Bio', slug: 'bio', type: 'textarea', maxLength: 10 }
      expect(validateCustomField(field, 'This is a long text')).toBe('Bio must be 10 characters or less')
    })

    it('returns null when within maxLength', () => {
      const field: FieldSchema = { name: 'Bio', slug: 'bio', type: 'textarea', maxLength: 10 }
      expect(validateCustomField(field, 'Short')).toBeNull()
    })
  })

  describe('number type', () => {
    it('returns error for non-numeric value', () => {
      const field: FieldSchema = { name: 'Price', slug: 'price', type: 'number' }
      expect(validateCustomField(field, 'abc')).toBe('Price must be a valid number')
    })

    it('returns error when value is below min', () => {
      const field: FieldSchema = { name: 'Price', slug: 'price', type: 'number', min: 1 }
      expect(validateCustomField(field, 0)).toBe('Price must be at least 1')
    })

    it('returns error when value is above max', () => {
      const field: FieldSchema = { name: 'Price', slug: 'price', type: 'number', max: 100 }
      expect(validateCustomField(field, 101)).toBe('Price must be at most 100')
    })

    it('returns null for valid number within range', () => {
      const field: FieldSchema = { name: 'Price', slug: 'price', type: 'number', min: 1, max: 100 }
      expect(validateCustomField(field, 50)).toBeNull()
    })

    it('accepts numeric string values', () => {
      const field: FieldSchema = { name: 'Price', slug: 'price', type: 'number' }
      expect(validateCustomField(field, '42')).toBeNull()
    })

    it('returns null for empty optional number field', () => {
      const field: FieldSchema = { name: 'Price', slug: 'price', type: 'number' }
      expect(validateCustomField(field, null)).toBeNull()
    })
  })

  describe('date type', () => {
    it('returns error for invalid date format', () => {
      const field: FieldSchema = { name: 'Start Date', slug: 'startDate', type: 'date' }
      expect(validateCustomField(field, 'not-a-date')).toBe('Start Date must be a valid date')
    })

    it('returns null for valid date', () => {
      const field: FieldSchema = { name: 'Start Date', slug: 'startDate', type: 'date' }
      expect(validateCustomField(field, '2026-05-10')).toBeNull()
    })

    it('returns null for ISO date string', () => {
      const field: FieldSchema = { name: 'Start Date', slug: 'startDate', type: 'date' }
      expect(validateCustomField(field, '2026-05-10T12:00:00Z')).toBeNull()
    })
  })

  describe('select type', () => {
    it('returns error for value not in options', () => {
      const field: FieldSchema = { name: 'Category', slug: 'category', type: 'select', options: ['A', 'B'] }
      expect(validateCustomField(field, 'C')).toBe('Category must be one of: A, B')
    })

    it('returns null for value in options', () => {
      const field: FieldSchema = { name: 'Category', slug: 'category', type: 'select', options: ['A', 'B'] }
      expect(validateCustomField(field, 'A')).toBeNull()
    })
  })

  describe('checkbox type', () => {
    it('always returns null regardless of value', () => {
      const field: FieldSchema = { name: 'Active', slug: 'active', type: 'checkbox', required: true }
      expect(validateCustomField(field, false)).toBeNull()
      expect(validateCustomField(field, true)).toBeNull()
      expect(validateCustomField(field, null)).toBeNull()
    })
  })
})

describe('validateCustomFields', () => {
  const fields: FieldSchema[] = [
    { name: 'Title', slug: 'title', type: 'text', required: true },
    { name: 'Price', slug: 'price', type: 'number', min: 1, required: true },
    { name: 'Active', slug: 'active', type: 'checkbox' },
  ]

  it('returns empty object when all fields are valid', () => {
    const result = validateCustomFields(fields, { title: 'Hello', price: 10, active: true })
    expect(result).toEqual({})
  })

  it('returns all errors for multiple invalid fields', () => {
    const result = validateCustomFields(fields, { title: '', price: 0, active: false })
    expect(result).toEqual({
      title: 'Title is required',
      price: 'Price must be at least 1',
    })
  })

  it('returns only relevant errors for partially valid data', () => {
    const result = validateCustomFields(fields, { title: 'Hello', price: 0, active: false })
    expect(result).toEqual({
      price: 'Price must be at least 1',
    })
  })
})
