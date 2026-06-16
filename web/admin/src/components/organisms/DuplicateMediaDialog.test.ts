import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DuplicateMediaDialog from './DuplicateMediaDialog.vue'
import type { Media } from '@/stores/domain/media'

const mockMedia: Media = {
  id: 42,
  userId: 1,
  filename: 'abc123def4567890.webp',
  originalFilename: 'sunset.jpg',
  mimeType: 'image/webp',
  fileSize: 204800,
  width: 800,
  height: 600,
  altText: 'A sunset over the ocean',
  isWebp: true,
  filePath: '/uploads/media/abc123def4567890.webp',
  url: 'http://localhost:8080/uploads/media/abc123def4567890.webp',
  hash: 'sha256hash',
  variants: {},
  uploadedBy: 'admin',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

describe('DuplicateMediaDialog', () => {
  it('should not render when visible is false', () => {
    const wrapper = mount(DuplicateMediaDialog, {
      props: {
        visible: false,
        existingMedia: null,
      },
    })

    expect(wrapper.find('.modal__overlay').exists()).toBe(false)
  })

  it('should render when visible is true', () => {
    const wrapper = mount(DuplicateMediaDialog, {
      props: {
        visible: true,
        existingMedia: mockMedia,
      },
    })

    expect(wrapper.find('.modal__overlay').exists()).toBe(true)
    expect(wrapper.text()).toContain('Duplicate Image Detected')
    expect(wrapper.text()).toContain('This image already exists in your media library')
  })

  it('should display existing media thumbnail and filename', () => {
    const wrapper = mount(DuplicateMediaDialog, {
      props: {
        visible: true,
        existingMedia: mockMedia,
      },
    })

    expect(wrapper.find('.duplicate-dialog__thumbnail').exists()).toBe(true)
    expect(wrapper.find('.duplicate-dialog__thumbnail').attributes('src')).toBe(mockMedia.url)
    expect(wrapper.find('.duplicate-dialog__thumbnail').attributes('alt')).toBe(mockMedia.altText)
    expect(wrapper.find('.duplicate-dialog__filename').text()).toBe('sunset.jpg')
    expect(wrapper.find('.duplicate-dialog__size').text()).toBe('200.0 KB')
  })

  it('should show Use Existing button when showUseExisting is true', () => {
    const wrapper = mount(DuplicateMediaDialog, {
      props: {
        visible: true,
        existingMedia: mockMedia,
        showUseExisting: true,
      },
    })

    const buttons = wrapper.findAll('button')
    const buttonTexts = buttons.map((b) => b.text())
    expect(buttonTexts).toContain('Use Existing')
  })

  it('should hide Use Existing button when showUseExisting is false', () => {
    const wrapper = mount(DuplicateMediaDialog, {
      props: {
        visible: true,
        existingMedia: mockMedia,
        showUseExisting: false,
      },
    })

    const buttons = wrapper.findAll('button')
    const buttonTexts = buttons.map((b) => b.text())
    expect(buttonTexts).not.toContain('Use Existing')
    expect(buttonTexts).toContain('Upload Anyway')
    expect(buttonTexts).toContain('Cancel')
  })

  it('should emit use-existing with media when Use Existing is clicked', async () => {
    const wrapper = mount(DuplicateMediaDialog, {
      props: {
        visible: true,
        existingMedia: mockMedia,
        showUseExisting: true,
      },
    })

    const useExistingButton = wrapper.findAll('button').find((b) => b.text() === 'Use Existing')
    await useExistingButton!.trigger('click')

    expect(wrapper.emitted('use-existing')).toBeTruthy()
    expect(wrapper.emitted('use-existing')![0]).toEqual([mockMedia])
  })

  it('should emit upload-anyway when Upload Anyway is clicked', async () => {
    const wrapper = mount(DuplicateMediaDialog, {
      props: {
        visible: true,
        existingMedia: mockMedia,
      },
    })

    const uploadAnywayButton = wrapper.findAll('button').find((b) => b.text() === 'Upload Anyway')
    await uploadAnywayButton!.trigger('click')

    expect(wrapper.emitted('upload-anyway')).toBeTruthy()
  })

  it('should emit close when Cancel is clicked', async () => {
    const wrapper = mount(DuplicateMediaDialog, {
      props: {
        visible: true,
        existingMedia: mockMedia,
      },
    })

    const cancelButton = wrapper.findAll('button').find((b) => b.text() === 'Cancel')
    await cancelButton!.trigger('click')

    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('should emit close when overlay is clicked', async () => {
    const wrapper = mount(DuplicateMediaDialog, {
      props: {
        visible: true,
        existingMedia: mockMedia,
      },
    })

    const overlay = wrapper.find('.modal__overlay')
    await overlay.trigger('click')

    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('should handle null existingMedia gracefully', () => {
    const wrapper = mount(DuplicateMediaDialog, {
      props: {
        visible: true,
        existingMedia: null,
      },
    })

    expect(wrapper.find('.duplicate-dialog__preview').exists()).toBe(false)
    expect(wrapper.find('.duplicate-dialog__warning').exists()).toBe(true)
  })

  it('should display file size correctly for bytes', () => {
    const smallMedia = { ...mockMedia, fileSize: 512 }
    const wrapper = mount(DuplicateMediaDialog, {
      props: {
        visible: true,
        existingMedia: smallMedia,
      },
    })

    expect(wrapper.find('.duplicate-dialog__size').text()).toBe('512 B')
  })

  it('should display file size correctly for megabytes', () => {
    const largeMedia = { ...mockMedia, fileSize: 2 * 1024 * 1024 }
    const wrapper = mount(DuplicateMediaDialog, {
      props: {
        visible: true,
        existingMedia: largeMedia,
      },
    })

    expect(wrapper.find('.duplicate-dialog__size').text()).toBe('2.0 MB')
  })
})
