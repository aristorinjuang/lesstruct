/**
 * Icon components using Heroicons SVG strings
 * Zero-dependency icons for optimal performance
 */

import IconDashboard from './IconDashboard.vue'
import IconDocument from './IconDocument.vue'
import IconImage from './IconImage.vue'
import IconMenu from './IconMenu.vue'
import IconXMark from './IconXMark.vue'
import IconChevronLeft from './IconChevronLeft.vue'
import IconChevronRight from './IconChevronRight.vue'
import IconUser from './IconUser.vue'
import IconChartBars from './IconChartBars.vue'
import IconDocumentText from './IconDocumentText.vue'
import IconPhoto from './IconPhoto.vue'
import IconUsers from './IconUsers.vue'
import IconChatBubble from './IconChatBubble.vue'
import IconArrowDownTray from './IconArrowDownTray.vue'

export const icons = {
  dashboard: IconDashboard,
  document: IconDocument,
  image: IconImage,
  menu: IconMenu,
  'x-mark': IconXMark,
  'chevron-left': IconChevronLeft,
  'chevron-right': IconChevronRight,
  user: IconUser,
  'chart-bars': IconChartBars,
  'document-text': IconDocumentText,
  photo: IconPhoto,
  users: IconUsers,
  'chat-bubble': IconChatBubble,
  'arrow-down-tray': IconArrowDownTray,
}

export type IconName = keyof typeof icons

export {
  IconDashboard,
  IconDocument,
  IconImage,
  IconMenu,
  IconXMark,
  IconChevronLeft,
  IconChevronRight,
  IconUser,
  IconChartBars,
  IconDocumentText,
  IconPhoto,
  IconUsers,
  IconChatBubble,
  IconArrowDownTray,
}

/**
 * Get icon component by name.
 * Returns null for unknown names so the caller can handle the missing case.
 */
export function getIcon(name: string) {
  return icons[name as IconName] ?? null
}
