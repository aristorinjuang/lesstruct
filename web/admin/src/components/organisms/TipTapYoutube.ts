import { Node, nodeInputRule, mergeAttributes } from '@tiptap/core'

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    youtube: {
      insertYoutube: (options: { src: string; width?: number; height?: number }) => ReturnType
    }
  }
}

export interface YoutubeOptions {
  width: number
  height: number
}

const YOUTUBE_REGEX =
  /^(?:https?:\/\/)?(?:www\.)?(?:youtube\.com\/(?:embed\/|watch\?v=|v\/|shorts\/|live\/)|youtu\.be\/)([\w-]{11})(?:[?&].*)?$/

function extractVideoId(url: string): string | null {
  const match = url.match(YOUTUBE_REGEX)
  return match ? match[1] : null
}

function extractYoutubeSrc(url: string): string | null {
  const videoId = extractVideoId(url)
  if (!videoId) return null
  return `https://www.youtube.com/embed/${videoId}`
}

const youtubeInputRegex = /^\$youtube\s/

export const Youtube = Node.create<YoutubeOptions>({
  name: 'youtube',

  group: 'block',

  atom: true,

  addOptions() {
    return {
      width: 640,
      height: 360,
    }
  },

  addAttributes() {
    return {
      src: {
        default: null,
      },
      width: {
        default: this.options.width,
      },
      height: {
        default: this.options.height,
      },
    }
  },

  parseHTML() {
    return [
      {
        tag: 'iframe[src*="youtube.com"]',
        getAttrs: (dom) => {
          const el = dom as HTMLElement
          const src = el.getAttribute('src')
          return { src }
        },
      },
      {
        tag: 'div[data-youtube-video]',
        getAttrs: (dom) => {
          const el = dom as HTMLElement
          return {
            src: el.getAttribute('data-youtube-video'),
          }
        },
      },
    ]
  },

  renderHTML({ HTMLAttributes }) {
    const src = HTMLAttributes.src
    if (!src) return ['div', { 'data-youtube-video': '' }, '']

    return [
      'div',
      {
        'data-youtube-video': src,
        class: 'youtube-wrapper',
      },
      ['iframe', mergeAttributes(HTMLAttributes, {
        src,
        frameborder: '0',
        allowfullscreen: 'true',
        allow: 'accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture',
        style: 'width: 100%; aspect-ratio: 16/9; height: auto;',
      })],
    ]
  },

  addCommands() {
    return {
      insertYoutube:
        (options) =>
        ({ commands }) => {
          const src = extractYoutubeSrc(options.src)
          if (!src) return false
          return commands.insertContent({
            type: this.name,
            attrs: {
              src,
              width: options.width ?? this.options.width,
              height: options.height ?? this.options.height,
            },
          })
        },
    }
  },

  addInputRules() {
    return [
      nodeInputRule({
        find: youtubeInputRegex,
        type: this.type,
        getAttributes: (match) => {
          return { src: match.input?.replace(youtubeInputRegex, '').trim() }
        },
      }),
    ]
  },
})
