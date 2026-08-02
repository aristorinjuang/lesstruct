import { onMounted, onUnmounted, ref, watch, type Ref } from 'vue'

export interface InfiniteScrollOptions {
  /**
   * How far (in px) before the sentinel enters the viewport the callback should fire.
   * A positive margin prefetches the next page just before it becomes visible.
   */
  rootMargin?: string
  /**
   * When true, the callback is suppressed even if the sentinel intersects.
   */
  disabled?: Ref<boolean>
}

/**
 * Composable that fires a callback when a sentinel element scrolls near the viewport
 * bottom — the primitive behind infinite (lazy-load) lists.
 *
 * Returns a `sentinel` ref to bind to a trailing element in the list:
 *
 * @example
 * const { sentinel } = useInfiniteScroll(() => mediaStore.loadMore(), {
 *   disabled: computed(() => !mediaStore.hasMore || mediaStore.isLoadingMore),
 * })
 * // <div ref="sentinel" />
 *
 * The observer watches the ref, so the sentinel may live inside v-if/v-for branches —
 * when the element is (re)created, the observer is re-armed automatically. The viewport
 * is used as the root, which works for both window scrolling and inner scroll containers
 * (`overflow-y: auto` ancestors): intersection is computed from the rendered position.
 */
export function useInfiniteScroll(onLoadMore: () => void, options: InfiniteScrollOptions = {}) {
  const sentinel = ref<HTMLElement | null>(null)
  let observer: IntersectionObserver | null = null

  function observe() {
    if (!sentinel.value || observer || typeof IntersectionObserver === 'undefined') return

    observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting && !options.disabled?.value) {
          onLoadMore()
        }
      },
      {
        root: null,
        rootMargin: options.rootMargin ?? '200px',
        threshold: 0,
      },
    )
    observer.observe(sentinel.value)
  }

  function disconnect() {
    observer?.disconnect()
    observer = null
  }

  onMounted(() => {
    observe()

    // Re-arm when the sentinel appears (v-if) or is replaced (v-for re-render).
    watch(sentinel, (element, previous) => {
      if (element !== previous) {
        disconnect()
        observe()
      }
    })
  })

  onUnmounted(disconnect)

  return { sentinel }
}
