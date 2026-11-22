import { onMounted, onUnmounted, ref, watch, type Ref } from 'vue'

/** Width, in CSS pixels, of the virtual viewport every theme snapshot renders at. */
export const SNAPSHOT_VIEWPORT_WIDTH = 1366

/** Height, in CSS pixels, of the virtual viewport every theme snapshot renders at. */
export const SNAPSHOT_VIEWPORT_HEIGHT = 768

/**
 * Keeps a fixed-size snapshot iframe scaled to the width of its frame.
 *
 * Theme snapshots always render at a 1366×768 viewport, so the layout matches
 * an HD-class laptop regardless of how wide the admin card happens to be. The
 * returned scale is the frame's content width divided by the virtual viewport
 * width; apply it with `transform: scale(...)` and `transform-origin: top left`
 * on an iframe sized 1366×768. A ResizeObserver keeps the scale in sync as the
 * grid reflows, and the frame's overflow crops whatever falls below the fold.
 *
 * The observer watches the ref, so the frame may live inside a `v-if` branch —
 * it is re-armed whenever the element is (re)created.
 *
 * @example
 * const frame = ref<HTMLElement | null>(null)
 * const { scale } = useSnapshotScale(frame)
 * // <div ref="frame" style="overflow: hidden">
 * //   <iframe :style="{ transform: `scale(${scale})` }" />
 * // </div>
 */
export function useSnapshotScale(frame: Ref<HTMLElement | null>) {
  const scale = ref(0)

  let observer: ResizeObserver | null = null

  function applyWidth(width: number) {
    if (width > 0) {
      scale.value = width / SNAPSHOT_VIEWPORT_WIDTH
    }
  }

  function observe() {
    const element = frame.value
    if (!element || observer || typeof ResizeObserver === 'undefined') return

    observer = new ResizeObserver((entries) => {
      const entry = entries[0]
      if (entry) {
        applyWidth(entry.contentRect.width)
      }
    })
    observer.observe(element)
    applyWidth(element.clientWidth)
  }

  function disconnect() {
    observer?.disconnect()
    observer = null
  }

  onMounted(() => {
    observe()

    // Re-arm when the frame appears (v-if) or is replaced (v-for re-render).
    watch(frame, (element, previous) => {
      if (element !== previous) {
        disconnect()
        observe()
      }
    })
  })

  onUnmounted(disconnect)

  return { scale }
}
