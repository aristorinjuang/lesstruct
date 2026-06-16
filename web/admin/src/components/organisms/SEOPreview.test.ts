import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import SEOPreview from './SEOPreview.vue'

describe('SEOPreview', () => {
  describe('Rendering', () => {
    it('renders SEO preview component with title and description', () => {
      const wrapper = mount(SEOPreview, {
        props: {
          title: 'Test Article Title',
          description: 'This is a test meta description for the article.',
        },
      })

      expect(wrapper.find('.seo-preview__title').text()).toBe('SEO Preview')
      expect(wrapper.find('.seo-preview__google-title').text()).toBe('Test Article Title')
      expect(wrapper.find('.seo-preview__google-description').text()).toBe('This is a test meta description for the article.')
    })

    it('renders Google search result preview section', () => {
      const wrapper = mount(SEOPreview, {
        props: {
          title: 'Test Article',
          description: 'Test description',
          url: 'https://example.com/test-article',
        },
      })

      expect(wrapper.find('.seo-preview__section-title').text()).toContain('Google Search Result')
      expect(wrapper.find('.seo-preview__google-url').text()).toBe('https://example.com/test-article')
      expect(wrapper.find('.seo-preview__google-title').text()).toBe('Test Article')
      expect(wrapper.find('.seo-preview__google-description').text()).toBe('Test description')
    })

    it('renders social media card preview section', () => {
      const wrapper = mount(SEOPreview, {
        props: {
          title: 'Social Article',
          description: 'Social description',
          image: 'https://example.com/image.jpg',
        },
      })

      const socialSections = wrapper.findAll('.seo-preview__section')
      const socialSection = socialSections.find(s => s.text().includes('Social Media Card'))
      expect(socialSection?.exists()).toBe(true)
      expect(wrapper.find('.seo-preview__social-title').text()).toBe('Social Article')
      expect(wrapper.find('.seo-preview__social-description').text()).toBe('Social description')
    })

    it('displays default values when props are empty', () => {
      const wrapper = mount(SEOPreview, {
        props: {
          title: '',
          description: '',
          url: '',
        },
      })

      expect(wrapper.find('.seo-preview__google-title').text()).toBe('Your Page Title')
      expect(wrapper.find('.seo-preview__google-description').text()).toBe('Your meta description will appear here. Make it compelling to improve click-through rates.')
      expect(wrapper.find('.seo-preview__google-url').text()).toBe('https://example.com/your-page-url')
    })

    it('displays placeholder image when no image prop is provided', () => {
      const wrapper = mount(SEOPreview, {
        props: {
          title: 'Test',
          description: 'Test',
        },
      })

      const img = wrapper.find('.seo-preview__social-image img')
      expect(img.attributes('src')).toBe('https://via.placeholder.com/1200x630?text=Preview+Image')
    })

    it('displays provided image when image prop is provided', () => {
      const wrapper = mount(SEOPreview, {
        props: {
          title: 'Test',
          description: 'Test',
          image: 'https://example.com/custom-image.jpg',
        },
      })

      const img = wrapper.find('.seo-preview__social-image img')
      expect(img.attributes('src')).toBe('https://example.com/custom-image.jpg')
    })
  })

  describe('Google Search Result Preview', () => {
    it('shows correct Google search result format', () => {
      const wrapper = mount(SEOPreview, {
        props: {
          title: 'How to Write Great SEO Titles',
          description: 'Learn the best practices for writing SEO-friendly titles that drive traffic to your website.',
          url: 'https://example.com/blog/seo-titles',
        },
      })

      const googleUrl = wrapper.find('.seo-preview__google-url')
      const googleTitle = wrapper.find('.seo-preview__google-title')
      const googleDesc = wrapper.find('.seo-preview__google-description')

      expect(googleUrl.text()).toBe('https://example.com/blog/seo-titles')
      expect(googleTitle.text()).toBe('How to Write Great SEO Titles')
      expect(googleDesc.text()).toBe('Learn the best practices for writing SEO-friendly titles that drive traffic to your website.')
    })

    it('truncates long descriptions in Google preview', () => {
      const longDescription = 'A'.repeat(200)

      const wrapper = mount(SEOPreview, {
        props: {
          title: 'Test',
          description: longDescription,
        },
      })

      const googleDesc = wrapper.find('.seo-preview__google-description')
      expect(googleDesc.text().length).toBe(200)
    })
  })

  describe('Social Media Card Preview', () => {
    it('shows correct social media card format with image', () => {
      const wrapper = mount(SEOPreview, {
        props: {
          title: 'Amazing Article',
          description: 'Check out this amazing content!',
          image: 'https://example.com/featured-image.jpg',
        },
      })

      expect(wrapper.find('.seo-preview__social').exists()).toBe(true)
      expect(wrapper.find('.seo-preview__social-image').exists()).toBe(true)
      expect(wrapper.find('.seo-preview__social-content').exists()).toBe(true)
    })

    it('displays social card title and description correctly', () => {
      const wrapper = mount(SEOPreview, {
        props: {
          title: 'Breaking News',
          description: 'Important update just in',
        },
      })

      expect(wrapper.find('.seo-preview__social-title').text()).toBe('Breaking News')
      expect(wrapper.find('.seo-preview__social-description').text()).toBe('Important update just in')
    })
  })

  describe('Reactive Updates', () => {
    it('updates preview when props change', async () => {
      const wrapper = mount(SEOPreview, {
        props: {
          title: 'Original Title',
          description: 'Original description',
        },
      })

      expect(wrapper.find('.seo-preview__google-title').text()).toBe('Original Title')

      await wrapper.setProps({ title: 'Updated Title' })

      expect(wrapper.find('.seo-preview__google-title').text()).toBe('Updated Title')
    })

    it('updates description when prop changes', async () => {
      const wrapper = mount(SEOPreview, {
        props: {
          title: 'Test',
          description: 'Original description',
        },
      })

      expect(wrapper.find('.seo-preview__google-description').text()).toBe('Original description')

      await wrapper.setProps({ description: 'Updated description' })

      expect(wrapper.find('.seo-preview__google-description').text()).toBe('Updated description')
    })
  })
})
