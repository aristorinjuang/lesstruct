export interface HtmlAiPreset {
  id: string
  label: string
  prompt: string
}

export const HTML_AI_PRESETS: HtmlAiPreset[] = [
  {
    id: 'hero',
    label: 'Hero section',
    prompt: 'A bold hero section for a SaaS landing page. Large fluid headline (max 8 words), supporting subheadline (1-2 sentences), a primary CTA button and a secondary text link. Subtle gradient or geometric SVG background. Generous whitespace, mobile-first.',
  },
  {
    id: 'pricing',
    label: 'Pricing table',
    prompt: 'A pricing section with three tiers — Hobby, Pro (highlighted as most popular), Business. Each tier shows price, short feature list with SVG checkmarks, and a CTA button. The middle tier visually emphasized via scale and accent color. Responsive grid.',
  },
  {
    id: 'testimonials',
    label: 'Testimonials',
    prompt: 'A testimonials section showing three customer quotes in a responsive auto-fit grid. Each card: round avatar image, italic quote, attribution with name and role. Subtle border, hover lift effect.',
  },
  {
    id: 'features',
    label: 'Features grid',
    prompt: 'A features section with six feature cards in a responsive grid (3 columns on desktop, 2 on tablet, 1 on mobile). Each card: SVG icon badge, short title, one-line description. Consistent spacing, hover micro-interaction.',
  },
  {
    id: 'cta',
    label: 'CTA banner',
    prompt: 'A call-to-action banner with a strong headline, supporting line, and a prominent CTA button. Accent-color background or gradient, generous padding, full-width feel, centered content.',
  },
  {
    id: 'faq',
    label: 'FAQ',
    prompt: 'A frequently-asked-questions section with six questions and answers. Use details/summary for native accordion behavior (no JS). Two-column on desktop, single column on mobile. Subtle divider between items.',
  },
  {
    id: 'stats',
    label: 'Stats showcase',
    prompt: 'A stats band displaying four key metrics (e.g., 10k+ users, 99.9% uptime, 4.9/5 rating, 24/7 support). Large fluid numbers, short labels underneath. Auto-fit grid, accent color for numbers.',
  },
  {
    id: 'contact',
    label: 'Contact section',
    prompt: 'A contact section with a left column (heading, short invitation text, email link, social links) and a right column (name, email, message fields, submit button). Responsive — single column on mobile. Accessible form labels.',
  },
  {
    id: 'newsletter',
    label: 'Newsletter signup',
    prompt: 'A newsletter signup section with a heading, one-line value prop, email input, and subscribe button. Centered layout, accent color, subtle background. Mobile-friendly form.',
  },
]
