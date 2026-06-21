<script setup lang="ts">
import { ref, watch, onUnmounted, onMounted, computed, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { type Editor } from '@tiptap/vue-3'
import InputText from '@/components/atoms/InputText.vue'
import Button from '@/components/atoms/Button.vue'
import Select from '@/components/atoms/Select.vue'
import FormField from '@/components/molecules/FormField.vue'
import DeleteConfirmDialog from '@/components/molecules/DeleteConfirmDialog.vue'
import Toast from '@/components/molecules/Toast.vue'
import TipTapEditor from '@/components/organisms/TipTapEditor.vue'
import MediaPanel from '@/components/organisms/MediaPanel.vue'
import { useContentStore } from '@/stores/domain/content'
import CustomFieldRenderer from '@/components/molecules/CustomFieldRenderer.vue'
import type { Content } from '@/types/content'
import type { PostType } from '@/types/posttype'
import type { FieldSchema } from '@/types/customfield'
import type { Media } from '@/stores/domain/media'
import { validateCustomField, validateCustomFields } from '@/utils/validation'
import { useAuth } from '@/composables/useAuth'
import { useConfig } from '@/composables/useConfig'
import api from '@/utils/request'

interface Props {
  userId: number
  contentId?: number
  initialContent?: Content
}

interface Emits {
  (e: 'saved', content: Content, redirectTo?: string): void
  (e: 'deleted'): void
  (e: 'cancel'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const router = useRouter()
const contentStore = useContentStore()
const { role } = useAuth()
const { languages, fetchConfig, primaryLanguage } = useConfig()
const isAdmin = computed(() => role.value === 'Admin')

const activeLanguage = ref('en')
const primaryContentId = ref<number | null>(null)
const translationGroupId = ref<number | null>(null)
const translations = ref<Content[]>([])
const showLanguageTabs = computed(() => languages.value.length > 1)

// AI text generation availability
const textGenAvailable = ref(false)
const isEnhancing = ref(false)
const isTranslating = ref(false)

const form = ref({
  title: '',
  content: '{"type":"doc","content":[{"type":"paragraph"}]}',
  tags: [] as string[],
  status: 'draft' as 'draft' | 'published',
  postType: 'post',
  metaDescription: '',
  ogTitle: '',
  ogDescription: '',
  allowComments: true,
})

const customFields = ref<Record<string, unknown>>({})
const customFieldErrors = ref<Record<string, string>>({})
const customFieldTouched = ref<Record<string, boolean>>({})
const isSwitchingPostType = ref(false)

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function getDefaultFieldValue(field: FieldSchema): any {
  switch (field.type) {
    case 'checkbox': return false
    case 'number': return null
    default: return ''
  }
}

const isSEOSettingsOpen = ref(false)

function validateFieldOnBlur(fieldSlug: string) {
  customFieldTouched.value[fieldSlug] = true
  const field = currentFields.value.find(f => f.slug === fieldSlug)
  if (!field) return
  const err = validateCustomField(field, customFields.value[fieldSlug])
  if (err) {
    customFieldErrors.value[fieldSlug] = err
  } else {
    delete customFieldErrors.value[fieldSlug]
  }
}

function validateAllCustomFields(): boolean {
  const errors = validateCustomFields(currentFields.value, customFields.value as Record<string, unknown>)
  customFieldErrors.value = errors
  for (const field of currentFields.value) {
    customFieldTouched.value[field.slug] = true
  }
  return Object.keys(errors).length === 0
}

const slug = ref('')
const slugManuallyEdited = ref(false)
const isLoading = ref(false)
const error = ref('')
const successMessage = ref('')
const hasContentChanges = ref(false)
const hasLoadedInitialContent = ref(false)
const savedContentId = ref<number | null>(props.contentId || null)
const postTypes = ref<PostType[]>([])

let debounceTimer: ReturnType<typeof setTimeout> | null = null
let autoSaveTimer: ReturnType<typeof setInterval> | null = null

const isMediaPanelOpen = ref(false)
const editorRef = ref<{ editor: Editor | null } | null>(null)

const showDeleteDialog = ref(false)
const isDeleting = ref(false)
const deleteError = ref('')

const toastMessage = ref('')
const toastType = ref<'success' | 'error'>('success')
const toastVisible = ref(false)
const toastKey = ref(0)

function displayToast(message: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = message
  toastType.value = type
  toastKey.value++
  toastVisible.value = true
}

onUnmounted(() => {
  if (debounceTimer) clearTimeout(debounceTimer)
  if (autoSaveTimer) clearInterval(autoSaveTimer)
})

onMounted(async () => {
  // Check AI text generation availability
  try {
    const health = await api.get<{ features?: { textGeneration?: boolean } }>('/api/health')
    textGenAvailable.value = health.data.features?.textGeneration === true
  } catch {
    textGenAvailable.value = false
  }

  try {
    postTypes.value = await contentStore.fetchPostTypes()
  } catch (err) {
    console.error('Failed to load post types:', err)
  }

  try {
    await fetchConfig()
    activeLanguage.value = primaryLanguage()
  } catch (err) {
    console.error('Failed to load config:', err)
  }

  // Fetch the authoritative record when editing existing content if either:
  //  - no list summary was passed (we need the whole record), or
  //  - more than one language is configured. The list summary (initialContent)
  //    omits translations, so without this fetch the language tabs can't
  //    discover an existing translation and switchLanguage() blanks the form.
  if (props.contentId && (!props.initialContent || showLanguageTabs.value)) {
    try {
      const fetchedContent = await contentStore.getById(props.contentId)
      loadContentIntoForm(fetchedContent)
    } catch (err) {
      console.error('Failed to load content for editing:', err)
      error.value = 'Failed to load content for editing'
    }
  }
})

// Watch for late-arriving initialContent (e.g. parent list finishes loading after mount)
watch(
  () => props.initialContent,
  (newContent) => {
    if (newContent && (props.contentId || !newContent.id) && !hasLoadedInitialContent.value) {
      loadContentIntoForm(newContent)
    }
  },
  { immediate: false },
)

const isNewContent = computed(() => savedContentId.value === null)
const currentStatus = computed(() => form.value.status)
const currentPostType = computed(() => (postTypes.value ?? []).find(pt => pt.slug === form.value.postType))

const showTags = computed(() => {
  if (activeLanguage.value !== primaryLanguage()) return false
  return currentPostType.value?.supports.includes('tags') ?? false
})

const showFeaturedImage = computed(() => {
  return currentPostType.value?.supports.includes('featured_image') ?? false
})

const showExcerpt = computed(() => {
  return currentPostType.value?.supports.includes('excerpt') ?? false
})

const currentFields = computed(() => currentPostType.value?.fields ?? [])

const currentSystemFields = computed(() => currentPostType.value?.systemFields ?? [])

function getSystemFieldSlugs(): Set<string> {
  return new Set(currentSystemFields.value.map(f => f.slug))
}

function getCustomFieldsOnly(): Record<string, any> {
  const systemSlugs = getSystemFieldSlugs()
  const values: Record<string, any> = {}
  for (const [key, val] of Object.entries(customFields.value)) {
    if (!systemSlugs.has(key)) {
      values[key] = val
    }
  }
  return values
}

function getSystemFieldsOnly(): Record<string, any> {
  const systemSlugs = getSystemFieldSlugs()
  const values: Record<string, any> = {}
  for (const slug of systemSlugs) {
    if (customFields.value[slug] !== undefined) {
      values[slug] = customFields.value[slug]
    }
  }
  return values
}

async function saveSystemFieldsForAdmin(contentId: number) {
  if (!isAdmin.value || currentSystemFields.value.length === 0) return
  try {
    await contentStore.updateSystemFields(contentId, getSystemFieldsOnly())
  } catch (sysErr) {
    error.value = 'Content saved but system fields update failed: ' + (sysErr instanceof Error ? sysErr.message : 'Unknown error')
  }
}

const reservedPostTypeSlugs = new Set(['media', 'comment', 'attachment'])

const postTypeOptions = computed(() => {
  return postTypes.value
    .filter(pt => !reservedPostTypeSlugs.has(pt.slug.toLowerCase()))
    .map(pt => ({ value: pt.slug, label: pt.name }))
})

if (props.initialContent) {
  loadContentIntoForm(props.initialContent)
}

function loadContentIntoForm(c: Content) {
  form.value.title = c.title
  form.value.content = c.content
  form.value.tags = c.tags
  form.value.status = c.status
  form.value.postType = c.postType || 'post'
  form.value.metaDescription = c.metaDescription || ''
  form.value.ogTitle = c.ogTitle || ''
  form.value.ogDescription = c.ogDescription || ''
  form.value.allowComments = c.allowComments ?? true
  customFields.value = c.customFields ?? {}
  slug.value = c.slug
  savedContentId.value = c.id
  hasLoadedInitialContent.value = true
  activeLanguage.value = c.language || primaryLanguage()
  if (c.translationGroupId) {
    translationGroupId.value = c.translationGroupId
  }
  if (!translationGroupId.value && !c.translationGroupId) {
    primaryContentId.value = c.id
  }
  if (c.translations) {
    translations.value = c.translations
  }
}

async function switchLanguage(lang: string) {
  if (lang === activeLanguage.value) return
  if (isLoading.value) return

  // Switching back to primary language — reload original content
  if (lang === primaryLanguage() && primaryContentId.value) {
    try {
      isLoading.value = true
      const fetched = await contentStore.getById(primaryContentId.value)
      loadContentIntoForm(fetched)
    } catch (err) {
      error.value = 'Failed to load primary content'
    } finally {
      isLoading.value = false
    }
    return
  }

  const targetTranslation = translations.value.find(t => t.language === lang)

  if (targetTranslation) {
    // Load existing translation
    try {
      isLoading.value = true
      const fetched = await contentStore.getById(targetTranslation.id)
      loadContentIntoForm(fetched)
    } catch (err) {
      error.value = 'Failed to load translation'
    } finally {
      isLoading.value = false
    }
  } else if (translationGroupId.value || primaryContentId.value) {
    // New translation — blank form but linked to group
    const groupId = translationGroupId.value || primaryContentId.value
    savedContentId.value = null
    form.value.title = ''
    form.value.content = '{"type":"doc","content":[{"type":"paragraph"}]}'
    form.value.tags = []
    form.value.status = 'draft'
    form.value.metaDescription = ''
    form.value.ogTitle = ''
    form.value.ogDescription = ''
    customFields.value = {}
    slug.value = ''
    activeLanguage.value = lang
    translationGroupId.value = groupId
  } else {
    // New content, just switch language label
    activeLanguage.value = lang
  }
}

autoSaveTimer = setInterval(() => {
  if (hasContentChanges.value && form.value.title && !isLoading.value && !isSwitchingPostType.value) {
    saveDraft(true)
  }
}, 2000)

watch(() => form.value.title, (newTitle) => {
  if (debounceTimer) clearTimeout(debounceTimer)
  if (newTitle && !slugManuallyEdited.value) {
    debounceTimer = setTimeout(async () => {
      try {
        const result = await contentStore.generateSlug(newTitle)
        slug.value = result.slug
      } catch {
        slug.value = ''
      }
    }, 400)
  } else if (!newTitle) {
    slug.value = ''
  }
})

watch(() => form.value.content, () => {
  hasContentChanges.value = true
})

watch(() => form.value.postType, (newPostType) => {
  form.value.allowComments = newPostType !== 'page'
  customFields.value = {}
  customFieldErrors.value = {}
  customFieldTouched.value = {}
  isSwitchingPostType.value = true
  nextTick(() => { isSwitchingPostType.value = false })
}, { flush: 'sync' })

async function saveDraft(isAutoSave = false) {
  if (isLoading.value) return

  if (!isAutoSave && !validateAllCustomFields()) return

  isLoading.value = true
  error.value = ''

  if (!isAutoSave) {
    successMessage.value = ''
  }

  try {
    let saved: Content
    if (savedContentId.value !== null) {
      saved = await contentStore.update(savedContentId.value, {
        title: form.value.title,
        content: form.value.content,
        tags: form.value.tags,
        status: isAutoSave ? form.value.status : 'draft',
        postType: form.value.postType,
        metaDescription: form.value.metaDescription || undefined,
        ogTitle: form.value.ogTitle || undefined,
        ogDescription: form.value.ogDescription || undefined,
        allowComments: form.value.allowComments,
        customFields: getCustomFieldsOnly(),
      })
    } else {
      saved = await contentStore.create({
        ...form.value,
        customFields: customFields.value,
        status: 'draft',
        userId: props.userId,
        language: activeLanguage.value,
        translationGroupId: translationGroupId.value || undefined,
      })
    }
    savedContentId.value = saved.id
    if (saved.translationGroupId) {
      translationGroupId.value = saved.translationGroupId
    }
    if (!translationGroupId.value) {
      primaryContentId.value = saved.id
    }
    form.value.status = saved.status
    hasContentChanges.value = false

    if (!isAutoSave) {
      successMessage.value = 'Draft saved successfully'
      emit('saved', saved)
      setTimeout(() => { successMessage.value = '' }, 3000)
    }

    if (savedContentId.value !== null) {
      await saveSystemFieldsForAdmin(savedContentId.value)
    }
  } catch (err) {
    mapBackendErrors(err)
    error.value = err instanceof Error ? err.message : 'Failed to save draft'
  } finally {
    isLoading.value = false
  }
}

async function publish() {
  if (isLoading.value) return
  if (!validateAllCustomFields()) return
  isLoading.value = true
  error.value = ''
  form.value.status = 'published'

  try {
    let saved: Content
    if (savedContentId.value !== null) {
      saved = await contentStore.update(savedContentId.value, {
        title: form.value.title,
        content: form.value.content,
        tags: form.value.tags,
        status: 'published',
        postType: form.value.postType,
        metaDescription: form.value.metaDescription || undefined,
        ogTitle: form.value.ogTitle || undefined,
        allowComments: form.value.allowComments,
        customFields: getCustomFieldsOnly(),
      })
    } else {
      saved = await contentStore.create({
        ...form.value,
        customFields: customFields.value,
        status: 'published',
        userId: props.userId,
        language: activeLanguage.value,
        translationGroupId: translationGroupId.value || undefined,
      })
    }
    savedContentId.value = saved.id
    if (saved.translationGroupId) {
      translationGroupId.value = saved.translationGroupId
    }
    if (!translationGroupId.value) {
      primaryContentId.value = saved.id
    }
    form.value.status = saved.status
    hasContentChanges.value = false
    successMessage.value = 'Content published successfully'
    emit('saved', saved, '/content')
    setTimeout(() => {
      successMessage.value = ''
    }, 3000)

    if (savedContentId.value !== null) {
      await saveSystemFieldsForAdmin(savedContentId.value)
    }
  } catch (err) {
    mapBackendErrors(err)
    error.value = err instanceof Error ? err.message : 'Failed to publish'
  } finally {
    isLoading.value = false
  }
}

async function unpublish() {
  if (isLoading.value) return
  isLoading.value = true
  error.value = ''

  try {
    const saved = await contentStore.update(savedContentId.value!, {
      title: form.value.title,
      content: form.value.content,
      tags: form.value.tags,
      status: 'draft',
      postType: form.value.postType,
      metaDescription: form.value.metaDescription || undefined,
      ogTitle: form.value.ogTitle || undefined,
      allowComments: form.value.allowComments,
      customFields: getCustomFieldsOnly(),
    })
    form.value.status = saved.status
    hasContentChanges.value = false
    successMessage.value = 'Content unpublished successfully'
    emit('saved', saved)
    setTimeout(() => {
      successMessage.value = ''
    }, 3000)

    await saveSystemFieldsForAdmin(savedContentId.value!)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to unpublish'
  } finally {
    isLoading.value = false
  }
}

async function saveChanges() {
  if (isLoading.value) return
  if (!validateAllCustomFields()) return
  isLoading.value = true
  error.value = ''

  try {
    const saved = await contentStore.update(savedContentId.value!, {
      title: form.value.title,
      content: form.value.content,
      tags: form.value.tags,
      status: 'published',
      postType: form.value.postType,
      metaDescription: form.value.metaDescription || undefined,
      ogTitle: form.value.ogTitle || undefined,
      ogDescription: form.value.ogDescription || undefined,
      allowComments: form.value.allowComments,
      customFields: getCustomFieldsOnly(),
    })
    hasContentChanges.value = false
    successMessage.value = 'Changes saved successfully'
    emit('saved', saved)
    setTimeout(() => {
      successMessage.value = ''
    }, 3000)

    await saveSystemFieldsForAdmin(savedContentId.value!)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to save changes'
  } finally {
    isLoading.value = false
  }
}

function requestDelete() {
  deleteError.value = ''
  showDeleteDialog.value = true
}

function cancelDelete() {
  showDeleteDialog.value = false
  deleteError.value = ''
}

async function confirmDelete() {
  if (savedContentId.value === null) return
  isDeleting.value = true
  deleteError.value = ''

  if (autoSaveTimer) {
    clearInterval(autoSaveTimer)
    autoSaveTimer = null
  }

  try {
    await contentStore.deleteContent(savedContentId.value)
    showDeleteDialog.value = false
    displayToast('Content deleted successfully')
    emit('deleted')
  } catch (err) {
    deleteError.value = err instanceof Error ? err.message : 'Failed to delete content'
  } finally {
    isDeleting.value = false
  }
}

function cancel() {
  emit('cancel')
  router.push('/content')
}

function mapBackendErrors(err: unknown) {
  const details = (err as any)?.response?.data?.error?.details
  if (details) {
    for (const detail of details) {
      const fieldSlug = detail.field?.replace('customFields.', '')
      if (fieldSlug && detail.message) {
        customFieldErrors.value[fieldSlug] = detail.message
      }
    }
  }
}

function toggleMediaPanel() {
  isMediaPanelOpen.value = !isMediaPanelOpen.value
}

function handleInsertImage(media: Media) {
  const editor = editorRef.value?.editor
  if (editor) {
    editor
      .chain()
      .focus()
      .setImage({ src: media.url, alt: media.altText })
      .run()
  }
}

async function handleEnhance() {
  if (isEnhancing.value) return
  isEnhancing.value = true
  try {
    const enhanced = await contentStore.enhanceContent(form.value.content)
    const editor = editorRef.value?.editor
    if (editor) {
      editor.commands.setContent(JSON.parse(enhanced))
    }
    form.value.content = enhanced
    displayToast('Content enhanced successfully')
  } catch (err: unknown) {
    const error = err as { message?: string; response?: { data?: { error?: { message?: string } } } }
    const msg = error.response?.data?.error?.message || error.message || 'Failed to enhance content'
    displayToast(msg, 'error')
  } finally {
    isEnhancing.value = false
  }
}

async function handleTranslate() {
  if (isTranslating.value) return
  if (!primaryContentId.value || primaryContentId.value === savedContentId.value) return
  isTranslating.value = true
  try {
    let sourceContent: string
    if (primaryContentId.value) {
      const primary = await contentStore.getById(primaryContentId.value)
      sourceContent = primary.content
    } else {
      displayToast('No primary content to translate from', 'error')
      return
    }

    const sourceLang = primaryLanguage()
    const targetLang = activeLanguage.value
    const translated = await contentStore.translateContent(sourceContent, sourceLang, targetLang)
    const editor = editorRef.value?.editor
    if (editor) {
      editor.commands.setContent(JSON.parse(translated))
    }
    form.value.content = translated
    displayToast('Content translated successfully')
  } catch (err: unknown) {
    const error = err as { message?: string; response?: { data?: { error?: { message?: string } } } }
    const msg = error.response?.data?.error?.message || error.message || 'Failed to translate content'
    displayToast(msg, 'error')
  } finally {
    isTranslating.value = false
  }
}
</script>

<template>
  <form @submit.prevent="saveDraft" class="content-editor">
    <div v-if="!isNewContent" class="content-editor__links">
      <router-link
        :to="`/content/${savedContentId}/comments?slug=${slug}`"
        class="content-editor__link"
      >
        Comments
      </router-link>
    </div>

    <h1 class="content-editor__title">
      {{ isNewContent ? 'Create New Content' : 'Edit Content' }}
    </h1>

    <div v-if="showLanguageTabs" class="content-editor__lang-tabs">
      <button
        v-for="lang in languages"
        :key="lang"
        type="button"
        class="content-editor__lang-tab"
        :class="{ 'content-editor__lang-tab--active': lang === activeLanguage }"
        @click="switchLanguage(lang)"
      >
        {{ lang.toUpperCase() }}
      </button>
    </div>

    <FormField label="Title" required>
      <InputText
        v-model="form.title"
        placeholder="Enter content title..."
        size="large"
        class="content-editor__title-input"
      />
    </FormField>

    <FormField label="Slug">
      <InputText
        v-model="slug"
        placeholder="URL-friendly slug will be generated"
        disabled
      />
    </FormField>

    <FormField v-if="activeLanguage === primaryLanguage()" label="Content Type">
      <Select
        v-model="form.postType"
        :options="postTypeOptions"
      />
    </FormField>

    <div class="content-editor__media-toggle" v-if="showFeaturedImage">
      <Button
        type="button"
        variant="secondary"
        @click="toggleMediaPanel"
      >
        {{ isMediaPanelOpen ? 'Hide' : 'Show' }} Media
      </Button>
    </div>

    <MediaPanel
      :is-open="isMediaPanelOpen"
      @insert-image="handleInsertImage"
      @show-toast="displayToast"
    />

    <FormField label="Content">
      <div class="content-editor__editor-wrapper">
        <TipTapEditor
          ref="editorRef"
          v-model="form.content"
          placeholder="Start writing your content..."
          class="content-editor__tiptap"
        />
        <div v-if="hasContentChanges && form.title" class="content-editor__autosave-hint">
          Unsaved changes
        </div>
      </div>
      <!-- AI Text Generation Buttons -->
      <div class="content-editor__ai-buttons">
        <Button
          v-if="textGenAvailable && activeLanguage === primaryLanguage()"
          type="button"
          variant="secondary"
          :is-loading="isEnhancing"
          @click="handleEnhance"
        >
          {{ isEnhancing ? 'Enhancing...' : 'Enhance with AI' }}
        </Button>
        <Button
          v-if="textGenAvailable && activeLanguage !== primaryLanguage() && (primaryContentId || savedContentId !== primaryContentId)"
          type="button"
          variant="secondary"
          :is-loading="isTranslating"
          @click="handleTranslate"
        >
          {{ isTranslating ? 'Translating...' : 'Translate with AI' }}
        </Button>
      </div>
    </FormField>

    <FormField label="Tags" v-if="showTags">
      <InputText
        :model-value="(form.tags ?? []).join(', ')"
        @update:model-value="form.tags = $event.split(',').map(t => t.trim()).filter(Boolean)"
        placeholder="tag1, tag2, tag3"
      />
    </FormField>

    <div
      v-if="Object.keys(customFieldErrors).length > 0"
      class="content-editor__validation-summary"
      role="alert"
      aria-live="assertive"
    >
      <p class="content-editor__validation-summary-title">
        Please fix the following errors:
      </p>
      <ul>
        <li v-for="(msg, slug) in customFieldErrors" :key="slug">
          <a href="#" class="content-editor__validation-link" @click.prevent="() => { const el = document.querySelector(`[data-field-slug='${slug}']`)?.querySelector('input, select, textarea') as HTMLElement; el?.focus() }">{{ msg }}</a>
        </li>
      </ul>
    </div>

    <div
      v-if="currentFields.length > 0"
      class="content-editor__custom-fields"
    >
      <CustomFieldRenderer
        v-for="field in currentFields"
        :key="field.slug"
        :data-field-slug="field.slug"
        :field="field"
        :model-value="customFields[field.slug] ?? getDefaultFieldValue(field)"
        :error="customFieldErrors[field.slug] ?? ''"
        @update:model-value="(val: unknown) => { customFields[field.slug] = val; delete customFieldErrors[field.slug] }"
        @blur="validateFieldOnBlur(field.slug)"
      />
    </div>

    <div
      v-if="currentSystemFields.length > 0"
      class="content-editor__system-fields"
    >
      <CustomFieldRenderer
        v-for="field in currentSystemFields"
        :key="'system-' + field.slug"
        :data-field-slug="'system-' + field.slug"
        :field="field"
        :model-value="customFields[field.slug] ?? getDefaultFieldValue(field)"
        :disabled="!isAdmin"
        :system-field="true"
        @update:model-value="(val: unknown) => { customFields[field.slug] = val }"
      />
    </div>

    <FormField label="Excerpt" v-if="showExcerpt">
      <textarea
        v-model="form.metaDescription"
        placeholder="Short description or excerpt"
        rows="3"
        maxlength="160"
        class="content-editor__textarea"
      />
      <p class="content-editor__char-count">
        {{ form.metaDescription.length }} / 160 characters
      </p>
    </FormField>

    <FormField label="Status">
      <Select
        v-model="form.status"
        :options="[
          { value: 'draft', label: 'Draft' },
          { value: 'published', label: 'Published' }
        ]"
      />
    </FormField>

    <div class="content-editor__checkbox-field">
      <label class="content-editor__checkbox-label">
        <input
          type="checkbox"
          v-model="form.allowComments"
          class="content-editor__checkbox"
        />
        Allow comments on this content
      </label>
    </div>

    <div class="content-editor__seo-section">
      <button
        type="button"
        class="content-editor__seo-toggle"
        @click="isSEOSettingsOpen = !isSEOSettingsOpen"
      >
        {{ isSEOSettingsOpen ? '▼' : '▶' }} SEO Settings
      </button>

      <div v-if="isSEOSettingsOpen" class="content-editor__seo-fields">
        <FormField label="Meta Description (optional)">
          <textarea
            v-model="form.metaDescription"
            placeholder="Auto-generated from content first 160 characters"
            rows="3"
            maxlength="160"
            class="content-editor__textarea"
          />
          <p class="content-editor__char-count">
            {{ form.metaDescription.length }} / 160 characters
          </p>
        </FormField>

        <FormField label="OG Title (optional)">
          <InputText
            v-model="form.ogTitle"
            placeholder="Defaults to content title"
            maxlength="60"
          />
          <p class="content-editor__char-count">
            {{ form.ogTitle.length }} / 60 characters
          </p>
        </FormField>
      </div>
    </div>

    <div v-if="error" class="content-editor__error">
      {{ error }}
    </div>

    <div v-if="successMessage" class="content-editor__success">
      {{ successMessage }}
    </div>

    <div class="content-editor__actions">
      <template v-if="!isNewContent">
        <Button
          type="button"
          variant="danger"
          size="small"
          @click="requestDelete"
        >
          Delete
        </Button>
      </template>

      <Button
        type="button"
        variant="secondary"
        @click="cancel"
      >
        Cancel
      </Button>

      <template v-if="isNewContent">
        <Button
          type="button"
          variant="primary"
          :is-loading="isLoading"
          @click="() => saveDraft()"
        >
          Save Draft
        </Button>
        <Button
          type="button"
          variant="primary"
          :is-loading="isLoading"
          @click="publish"
        >
          Publish
        </Button>
      </template>

      <template v-else-if="currentStatus === 'draft'">
        <Button
          type="button"
          variant="secondary"
          :is-loading="isLoading"
          @click="() => saveDraft()"
        >
          Save Draft
        </Button>
        <Button
          type="button"
          variant="primary"
          :is-loading="isLoading"
          @click="publish"
        >
          Publish
        </Button>
      </template>

      <template v-else>
        <Button
          type="button"
          variant="secondary"
          :is-loading="isLoading"
          @click="unpublish"
        >
          Unpublish
        </Button>
        <Button
          type="button"
          variant="primary"
          :is-loading="isLoading"
          @click="saveChanges"
        >
          Save Changes
        </Button>
      </template>
    </div>

    <DeleteConfirmDialog
      title="Delete Content"
      :item-name="form.title"
      :is-open="showDeleteDialog"
      :is-loading="isDeleting"
      @confirm="confirmDelete"
      @cancel="cancelDelete"
    />

    <Toast
      v-if="toastMessage"
      :key="toastKey"
      :message="toastMessage"
      :type="toastType"
      :visible="toastVisible"
      @dismiss="toastVisible = false"
    />

    <div v-if="deleteError" class="content-editor__error">
      {{ deleteError }}
    </div>
  </form>
</template>

<style scoped>
.content-editor {
  max-width: 800px;
  margin: 0 auto;
  padding: 2rem 1rem;
}

.content-editor__links {
  margin-bottom: 1rem;
}

.content-editor__link {
  display: inline-flex;
  align-items: center;
  color: var(--brand-primary);
  text-decoration: none;
  font-weight: 500;
  transition: color 0.2s;
}

.content-editor__link:hover {
  color: var(--brand-dark-1);
  text-decoration: underline;
}

.content-editor__title {
  font-size: 1.875rem;
  font-weight: 700;
  margin: 0 0 0.5rem 0;
  color: var(--brand-dark-1);
}

.content-editor__lang-tabs {
  display: flex;
  gap: 0.25rem;
  margin-bottom: 1rem;
  border-bottom: 2px solid var(--brand-light-2);
  padding-bottom: 0;
}

.content-editor__lang-tab {
  padding: 0.5rem 1rem;
  border: none;
  background: none;
  cursor: pointer;
  font-weight: 600;
  font-size: 0.875rem;
  color: var(--brand-dark-2);
  border-bottom: 2px solid transparent;
  margin-bottom: -2px;
  transition: color 0.2s, border-color 0.2s;
}

.content-editor__lang-tab:hover {
  color: var(--brand-primary);
}

.content-editor__lang-tab--active {
  color: var(--brand-primary);
  border-bottom-color: var(--brand-primary);
}

.content-editor__editor-wrapper {
  position: relative;
  width: 100%;
}

.content-editor__tiptap {
  width: 100%;
}

.content-editor__autosave-hint {
  position: absolute;
  bottom: 0.5rem;
  right: 0.75rem;
  font-size: 0.75rem;
  color: var(--brand-dark-2);
  font-style: italic;
  background-color: var(--color-background);
  padding: 0.25rem 0.5rem;
  border-radius: 0.25rem;
  pointer-events: none;
}

.content-editor__error {
  padding: 0.75rem;
  margin-bottom: 1rem;
  background-color: var(--color-error-bg);
  border: 1px solid var(--color-error-border);
  border-radius: 0.375rem;
  color: var(--color-error-dark);
}

.content-editor__success {
  padding: 0.75rem;
  margin-bottom: 1rem;
  background-color: var(--color-success-bg);
  border: 1px solid var(--color-success-border);
  border-radius: 0.375rem;
  color: var(--color-success-dark);
}

.content-editor__title-input :deep(input) {
  font-size: 48px;
  font-weight: 700;
}

.content-editor__media-toggle {
  margin-top: 1rem;
  margin-bottom: 1rem;
}

.content-editor__custom-fields {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.content-editor__system-fields {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.content-editor__validation-summary {
  padding: 0.75rem 1rem;
  margin-bottom: 1rem;
  background-color: var(--color-error-bg);
  border: 1px solid var(--color-error-border);
  border-radius: 0.375rem;
  color: var(--color-error-dark);
}

.content-editor__validation-summary-title {
  font-weight: 600;
  margin: 0 0 0.5rem 0;
}

.content-editor__validation-summary ul {
  margin: 0;
  padding-left: 1.25rem;
}

.content-editor__validation-link {
  color: var(--color-error-dark);
  text-decoration: underline;
  cursor: pointer;
}

.content-editor__validation-link:hover {
  color: var(--color-error-dark);
}

.content-editor__ai-buttons {
  margin-top: 1rem;
}

@media (min-width: 1024px) {
  .content-editor__custom-fields {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1rem;
  }

  .content-editor__custom-fields > :deep(
    .form-field--textarea,
    .form-field--checkbox
  ) {
    grid-column: 1 / -1;
  }

  .content-editor__system-fields {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
  }

  .content-editor__system-fields > :deep(
    .form-field--textarea,
    .form-field--checkbox
  ) {
    grid-column: 1 / -1;
  }
}

.content-editor__seo-section {
  margin-bottom: 1rem;
  padding: 1rem;
  background-color: var(--brand-light-1);
  border-radius: 0.375rem;
}

.content-editor__checkbox-field {
  padding: 0.5rem 0;
}

.content-editor__checkbox-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  cursor: pointer;
  font-size: 0.9375rem;
  font-weight: 500;
  color: var(--brand-dark-1);
}

.content-editor__checkbox {
  width: 1rem;
  height: 1rem;
  cursor: pointer;
}

.content-editor__seo-toggle {
  width: 100%;
  padding: 0.75rem;
  background-color: var(--color-background);
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  cursor: pointer;
  font-weight: 600;
  color: var(--brand-dark-1);
  text-align: left;
}

.content-editor__seo-toggle:hover {
  background-color: var(--brand-light-1);
}

.content-editor__seo-fields {
  margin-top: 1rem;
}

.content-editor__textarea {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid var(--brand-light-2);
  border-radius: 0.375rem;
  font-family: inherit;
  font-size: 0.875rem;
  resize: vertical;
  background-color: var(--color-background);
  color: var(--brand-dark-1);
}

/* Mobile: Ensure 16px minimum font-size to prevent iOS auto-zoom */
@media (max-width: 767px) {
  .content-editor__textarea {
    font-size: 16px;
  }
}

.content-editor__char-count {
  margin-left: 0.5rem;
  font-size: 0.75rem;
  color: var(--brand-dark-2);
}

.content-editor__actions {
  display: flex;
  gap: 1rem;
  margin-top: 2rem;
  justify-content: flex-end;
}

@media (max-width: 640px) {
  .content-editor__actions {
    flex-direction: column;
  }

  .content-editor__actions button {
    width: 100%;
  }
}
</style>
