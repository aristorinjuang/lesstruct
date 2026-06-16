<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  title: string
  description: string
  image?: string
  url?: string
}

const props = withDefaults(defineProps<Props>(), {
  image: '',
  url: ''
})

const googlePreview = computed(() => ({
  title: props.title || 'Your Page Title',
  description: props.description || 'Your meta description will appear here. Make it compelling to improve click-through rates.',
  url: props.url || 'https://example.com/your-page-url'
}))

const socialPreview = computed(() => ({
  title: props.title || 'Your Page Title',
  description: props.description || 'Your meta description will appear here.',
  image: props.image || 'https://via.placeholder.com/1200x630?text=Preview+Image'
}))
</script>

<template>
  <div class="seo-preview">
    <h3 class="seo-preview__title">SEO Preview</h3>

    <div class="seo-preview__section">
      <h4 class="seo-preview__section-title">Google Search Result</h4>
      <div class="seo-preview__google">
        <div class="seo-preview__google-url">
          {{ googlePreview.url }}
        </div>
        <div class="seo-preview__google-title">
          {{ googlePreview.title }}
        </div>
        <div class="seo-preview__google-description">
          {{ googlePreview.description }}
        </div>
      </div>
    </div>

    <div class="seo-preview__section">
      <h4 class="seo-preview__section-title">Social Media Card</h4>
      <div class="seo-preview__social">
        <div class="seo-preview__social-image">
          <img :src="socialPreview.image" alt="Preview" />
        </div>
        <div class="seo-preview__social-content">
          <div class="seo-preview__social-url">
            {{ googlePreview.url }}
          </div>
          <div class="seo-preview__social-title">
            {{ socialPreview.title }}
          </div>
          <div class="seo-preview__social-description">
            {{ socialPreview.description }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.seo-preview {
  border: 1px solid var(--brand-light-2);
  border-radius: 0.5rem;
  padding: 1.5rem;
  background-color: var(--color-white);
}

.seo-preview__title {
  font-size: 1.125rem;
  font-weight: 600;
  margin-bottom: 1rem;
  color: var(--brand-dark-2);
}

.seo-preview__section {
  margin-bottom: 1.5rem;
}

.seo-preview__section:last-child {
  margin-bottom: 0;
}

.seo-preview__section-title {
  font-size: 0.875rem;
  font-weight: 600;
  margin-bottom: 0.75rem;
  color: var(--color-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.seo-preview__google {
  font-family: Arial, sans-serif;
  max-width: 600px;
}

.seo-preview__google-url {
  font-size: 0.875rem;
  color: #202124;
  margin-bottom: 0.25rem;
}

.seo-preview__google-title {
  font-size: 1.25rem;
  color: #1a0dab;
  text-decoration: none;
  margin-bottom: 0.25rem;
  cursor: pointer;
}

.seo-preview__google-title:hover {
  text-decoration: underline;
}

.seo-preview__google-description {
  font-size: 0.875rem;
  color: #545454;
  line-height: 1.4;
}

.seo-preview__social {
  display: flex;
  max-width: 500px;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.5rem;
  overflow: hidden;
}

.seo-preview__social-image {
  width: 120px;
  height: 120px;
  flex-shrink: 0;
}

.seo-preview__social-image img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.seo-preview__social-content {
  flex: 1;
  padding: 0.75rem;
  font-family: Helvetica, Arial, sans-serif;
}

.seo-preview__social-url {
  font-size: 0.75rem;
  color: #606770;
  text-transform: uppercase;
  margin-bottom: 0.25rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.seo-preview__social-title {
  font-size: 1rem;
  font-weight: 600;
  color: #1d2129;
  margin-bottom: 0.25rem;
  line-height: 1.2;
}

.seo-preview__social-description {
  font-size: 0.875rem;
  color: #606770;
  line-height: 1.3;
}
</style>
