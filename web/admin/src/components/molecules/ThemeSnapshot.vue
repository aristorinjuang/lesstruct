<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  SNAPSHOT_VIEWPORT_HEIGHT,
  SNAPSHOT_VIEWPORT_WIDTH,
  useSnapshotScale,
} from '@/composables/useSnapshotScale'

interface Props {
  name: string
}

const props = defineProps<Props>()

const frame = ref<HTMLElement | null>(null)
const { scale } = useSnapshotScale(frame)

const frameStyle = {
  aspectRatio: `${SNAPSHOT_VIEWPORT_WIDTH} / ${SNAPSHOT_VIEWPORT_HEIGHT}`,
}

const iframeStyle = computed(() => ({
  width: `${SNAPSHOT_VIEWPORT_WIDTH}px`,
  height: `${SNAPSHOT_VIEWPORT_HEIGHT}px`,
  transform: `scale(${scale.value})`,
}))
</script>

<template>
  <div ref="frame" class="theme-snapshot" :style="frameStyle">
    <iframe
      :src="`/api/v1/themegen/themes/${props.name}/snapshot`"
      :title="`Snapshot of ${props.name}`"
      :style="iframeStyle"
      sandbox="allow-same-origin"
      loading="lazy"
      scrolling="no"
    />
  </div>
</template>

<style scoped>
.theme-snapshot {
  position: relative;
  overflow: hidden;
  max-height: 180px;
  border: 1px solid var(--color-border);
  border-radius: 0.5rem;
  background: var(--color-background);
}
.theme-snapshot iframe {
  position: absolute;
  top: 0;
  left: 0;
  display: block;
  border: 0;
  transform-origin: top left;
  pointer-events: none;
}
</style>
