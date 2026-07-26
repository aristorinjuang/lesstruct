package textgen

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// tiptapSchemaPrompt describes the TipTap JSON structure to the AI model so it
// can output valid TipTap documents compatible with the editor.
const tiptapSchemaPrompt = `You must output VALID TipTap JSON matching this exact schema:

{
  "type": "doc",
  "content": [
    {
      "type": "paragraph",
      "content": [{"type": "text", "text": "plain text"}]
    },
    {
      "type": "heading",
      "attrs": {"level": 2},
      "content": [{"type": "text", "text": "Heading"}]
    },
    {
      "type": "bulletList",
      "content": [{"type": "listItem", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "item"}]}]}]
    },
    {
      "type": "orderedList",
      "content": [{"type": "listItem", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "item"}]}]}]
    },
    {
      "type": "blockquote",
      "content": [{"type": "paragraph", "content": [{"type": "text", "text": "quote"}]}]
    },
    {
      "type": "codeBlock",
      "attrs": {"language": "javascript"},
      "content": [{"type": "text", "text": "console.log('hello')"}]
    },
    {"type": "horizontalRule"},
    {
      "type": "image",
      "attrs": {"src": "https://...", "alt": "description"}
    },
    {
      "type": "table",
      "content": [
        {"type": "tableRow", "content": [
          {"type": "tableHeader", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Header"}]}]},
          {"type": "tableCell", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Cell"}]}]}
        ]}
      ]
    },
    {"type": "youtube", "attrs": {"src": "https://www.youtube.com/embed/..."}},
    {
      "type": "paragraph",
      "content": [{"type": "text", "marks": [{"type": "bold"}], "text": "bold"}, {"type": "text", "marks": [{"type": "italic"}], "text": "italic"}, {"type": "text", "marks": [{"type": "underline"}], "text": "underline"}]
    },
    {
      "type": "paragraph",
      "content": [{"type": "emoji", "attrs": {"name": "rocket"}}, {"type": "text", "text": " with emoji"}]
    },
    {
      "type": "paragraph",
      "content": [{"type": "text", "marks": [{"type": "link", "attrs": {"href": "https://example.com"}}], "text": "link text"}]
    },
    {
      "type": "paragraph",
      "content": [
        {"type": "text", "text": "Inline math: "},
        {"type": "inlineMath", "attrs": {"latex": "E=mc^2"}}
      ]
    },
    {
      "type": "blockMath",
      "attrs": {"latex": "\\int_0^\\infty e^{-x^2} dx = \\frac{\\sqrt{\\pi}}{2}"}
    }
  ]
}

IMPORTANT RULES:
1. Output ONLY the JSON — no markdown fences, no explanation, no commentary.
2. Preserve all inline marks: bold, italic, underline, link, code.
3. For code blocks, always include the "attrs": {"language": "..."} field.
4. For images, keep the original src and alt attributes unchanged.
5. For YouTube embeds, keep the original src attribute unchanged.
6. For math (inlineMath, blockMath), keep the original latex attribute unchanged.
7. Do not invent new node types or mark types — use only the types shown above.
8. NEVER use heading level 1 (h1) — headings must start at level 2 or higher.
9. Always wrap text content inside a paragraph node unless it belongs in a heading, list, blockquote, or code block.`

// htmlSystemPromptBase is the always-on system prompt for HTML/CSS generation.
// It instructs the AI to produce a scoped, self-contained HTML fragment that
// reuses the active theme's CSS custom properties and component classes, so
// generated content stays on-brand instead of inventing arbitrary colors,
// fonts, and spacing. The active theme's CSS is injected separately (see
// htmlSystemPromptThemeSection) and the examples follow (htmlSystemPromptExamples).
const htmlSystemPromptBase = `You are an expert front-end developer creating production-ready HTML and CSS for a website's content area. You write clean, semantic, accessible code that matches the site's existing brand — every color, font, spacing, and radius should come from the theme's design tokens, never invented.

## Output format

Output a SINGLE HTML fragment. Start with a <style> block, followed by the HTML markup.
- NO <!DOCTYPE>, <html>, <head>, or <body> wrappers — just the content fragment.
- NO markdown fences, no explanation, no commentary — ONLY the raw HTML.

## Hard rules

- ALL styling goes in the <style> block at the top. NEVER use inline style="..." attributes.
- Scope every CSS selector under a unique top-level class .ls-<topic> (e.g., .ls-hero-pricing, .ls-testimonials-section) so your styles never leak into the rest of the page.
- NO <script>, NO on* handlers, NO javascript: URLs.
- NO external resources — no <link>, no <script src>, no @import, no external fonts.
- Close all tags, use valid HTML5.

## Brand consistency — REUSE the site theme (critical)

The page that renders your fragment already loads the site theme, which defines CSS custom properties on :root and component classes in its stylesheets. Your output MUST blend in with that theme.

REUSE these theme custom properties verbatim — do NOT invent new --accent/--bg/--text tokens and do NOT hardcode hex colors anywhere:
- Color: var(--color-primary), var(--color-primary-hover), var(--color-secondary), var(--color-accent), var(--color-text), var(--color-text-muted), var(--color-bg), var(--color-card-bg), var(--color-border), var(--color-success), var(--color-danger)
- Spacing: var(--space-1) (0.25rem) ... var(--space-8) (3rem)
- Radius: var(--radius-sm), var(--radius-md), var(--radius-lg)
- Shadow: var(--shadow-sm), var(--shadow-md), var(--shadow-lg)
- Focus ring: var(--ring)
- Fonts: var(--font-sans) (body — already set on <body>, do NOT redeclare on body), var(--font-mono)
- Transition: var(--transition-fast)

REUSE existing theme component classes where they fit — only add a scoped .ls-<topic> class for layout the theme does not already cover:
- .container — max-width page wrapper
- .btn — styled button (already wired to --color-primary, hover, and focus ring)
- .form-control — inputs and textareas
- .alert, .alert--success, .alert--error — callout banners

Do NOT redefine base body typography (font-family, font-size, line-height, color, background) — the theme's base.css already sets them on <body>. Only set font or color inside your scoped .ls-<topic> block when a section needs something different from the page default.

## Design directives

- Clear visual hierarchy via size, weight, color, and spacing.
- Generous whitespace using var(--space-*) tokens (e.g., var(--space-8) between sections).
- Modern CSS: Grid + Flexbox for layout (no floats). clamp() for fluid typography, aspect-ratio for media, CSS custom properties for per-section theming.
- Micro-interactions: :hover, :focus-visible, transition: <property> var(--transition-fast), transform on interactive elements. Respect prefers-reduced-motion.
- Responsive: mobile-first, no fixed pixel widths on containers, fluid images (max-width: 100%; height: auto).
- Accessibility: WCAG AA contrast (4.5:1 body text), visible focus rings (use var(--ring) or outline: 2px solid var(--color-primary)), semantic landmarks (<section>, <article>, <nav>), at most one <h2> per block (the page owns <h1>), alt text on all images.
- Images: only use URLs from the "Available images" list when provided. Never invent image URLs. Always add loading="lazy" and meaningful alt.`

// htmlSystemPromptThemeSection is injected between the base prompt and the
// examples when the active theme's CSS is available. Seeing the exact token
// values and existing component patterns lets the AI match them precisely
// instead of guessing. The two %s placeholders receive base.css (tokens) and
// style.css (components) respectively.
const htmlSystemPromptThemeSection = `

## Active site theme CSS (authoritative — match these tokens and patterns)

Below is the site's current base.css (design tokens) and style.css (component styles). Internalize the palette, spacing, radius, and font conventions. Do NOT copy these rules into your <style> block — reference them via var(--custom-property) and reuse component classes instead.

### base.css
%s

### style.css
%s`

// htmlSystemPromptExamples contains the few-shot examples. Every example uses
// the theme's custom properties (var(--color-*), var(--space-*), var(--radius-*),
// var(--font-sans)) rather than hardcoded hex values, so output blends with any
// theme and demonstrates the token-reuse rule.
const htmlSystemPromptExamples = `

## Few-shot examples

Every example below uses theme tokens — your real output must do the same.

### Example: Hero section

<style>
.ls-hero-example {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 50vh;
  padding: var(--space-8) var(--space-5);
  background: linear-gradient(135deg, var(--color-bg) 0%, color-mix(in srgb, var(--color-secondary) 8%, var(--color-bg)) 100%);
  text-align: center;
  color: var(--color-text);
}
.ls-hero-example h2 {
  font-size: clamp(2rem, 5vw, 3.5rem);
  font-weight: 700;
  line-height: 1.15;
  margin: 0 0 var(--space-3);
}
.ls-hero-example p {
  font-size: clamp(1.05rem, 2vw, 1.25rem);
  color: var(--color-text-muted);
  max-width: 36rem;
  margin: 0 0 var(--space-6);
}
.ls-hero-example .ls-hero-buttons {
  display: flex;
  gap: var(--space-4);
  flex-wrap: wrap;
  justify-content: center;
}
.ls-hero-example .ls-hero-btn-primary {
  display: inline-block;
  padding: var(--space-3) var(--space-6);
  background: var(--color-primary);
  color: var(--color-bg);
  border: none;
  border-radius: var(--radius-md);
  font-size: 1rem;
  font-weight: 600;
  text-decoration: none;
  transition: background var(--transition-fast), transform var(--transition-fast);
}
.ls-hero-example .ls-hero-btn-primary:hover {
  background: var(--color-primary-hover);
  transform: translateY(-1px);
}
.ls-hero-example .ls-hero-btn-primary:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}
.ls-hero-example .ls-hero-btn-secondary {
  display: inline-block;
  padding: var(--space-3) var(--space-6);
  color: var(--color-primary);
  border: 2px solid var(--color-primary);
  border-radius: var(--radius-md);
  font-size: 1rem;
  font-weight: 600;
  text-decoration: none;
  transition: background var(--transition-fast), color var(--transition-fast);
}
.ls-hero-example .ls-hero-btn-secondary:hover {
  background: var(--color-primary);
  color: var(--color-bg);
}
.ls-hero-example .ls-hero-btn-secondary:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}
@media (prefers-reduced-motion: reduce) {
  .ls-hero-example * { transition: none !important; transform: none !important; }
}
</style>
<section class="ls-hero-example">
  <h2>Build something extraordinary</h2>
  <p>The modern platform for teams who refuse to settle. Ship faster, scale effortlessly, and delight your users.</p>
  <div class="ls-hero-buttons">
    <a href="#get-started" class="ls-hero-btn-primary">Get started free</a>
    <a href="#demo" class="ls-hero-btn-secondary">Watch a demo</a>
  </div>
</section>

### Example: Pricing table

<style>
.ls-pricing-example {
  padding: var(--space-8) var(--space-5);
  color: var(--color-text);
}
.ls-pricing-example h2 {
  text-align: center;
  font-size: clamp(1.75rem, 4vw, 2.5rem);
  margin: 0 0 var(--space-2);
}
.ls-pricing-example .ls-pricing-subtitle {
  text-align: center;
  color: var(--color-text-muted);
  margin: 0 0 var(--space-8);
}
.ls-pricing-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: var(--space-6);
  max-width: 56rem;
  margin: 0 auto;
}
.ls-pricing-card {
  background: var(--color-card-bg);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: var(--space-8);
  display: flex;
  flex-direction: column;
}
.ls-pricing-card--featured {
  border-color: var(--color-primary);
  box-shadow: var(--shadow-md);
  transform: scale(1.03);
}
.ls-pricing-card .ls-pricing-badge {
  display: inline-block;
  background: var(--color-primary);
  color: var(--color-bg);
  font-size: 0.75rem;
  font-weight: 600;
  padding: var(--space-1) var(--space-3);
  border-radius: 9999px;
  align-self: flex-start;
  margin-bottom: var(--space-3);
}
.ls-pricing-card h3 { font-size: 1.25rem; margin: 0 0 var(--space-2); }
.ls-pricing-card .ls-pricing-price {
  font-size: 2rem;
  font-weight: 700;
  margin: 0 0 var(--space-6);
}
.ls-pricing-card .ls-pricing-price span { font-size: 0.875rem; font-weight: 400; color: var(--color-text-muted); }
.ls-pricing-features { list-style: none; padding: 0; margin: 0 0 var(--space-8); flex: 1; }
.ls-pricing-features li {
  padding: var(--space-1) 0;
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: 0.95rem;
  color: var(--color-text-muted);
}
.ls-pricing-features li svg { flex-shrink: 0; color: var(--color-primary); }
.ls-pricing-card .ls-pricing-btn {
  display: block;
  text-align: center;
  padding: var(--space-3);
  border-radius: var(--radius-md);
  font-weight: 600;
  text-decoration: none;
  transition: background var(--transition-fast), color var(--transition-fast);
}
.ls-pricing-card .ls-pricing-btn--outline { border: 2px solid var(--color-primary); color: var(--color-primary); }
.ls-pricing-card .ls-pricing-btn--outline:hover { background: var(--color-primary); color: var(--color-bg); }
.ls-pricing-card .ls-pricing-btn--solid { background: var(--color-primary); color: var(--color-bg); border: none; }
.ls-pricing-card .ls-pricing-btn--solid:hover { background: var(--color-primary-hover); }
.ls-pricing-card .ls-pricing-btn:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
@media (prefers-reduced-motion: reduce) {
  .ls-pricing-example * { transition: none !important; transform: none !important; }
}
</style>
<section class="ls-pricing-example">
  <h2>Simple, transparent pricing</h2>
  <p class="ls-pricing-subtitle">Start free, upgrade when you're ready. No hidden fees.</p>
  <div class="ls-pricing-grid">
    <div class="ls-pricing-card">
      <h3>Hobby</h3>
      <p class="ls-pricing-price">$9<span>/month</span></p>
      <ul class="ls-pricing-features">
        <li><svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M3 8.5l3.5 3.5L13 4" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg> 1 project</li>
        <li><svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M3 8.5l3.5 3.5L13 4" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg> 10 GB storage</li>
        <li><svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M3 8.5l3.5 3.5L13 4" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg> Community support</li>
      </ul>
      <a href="#hobby" class="ls-pricing-btn ls-pricing-btn--outline">Get started</a>
    </div>
    <div class="ls-pricing-card ls-pricing-card--featured">
      <span class="ls-pricing-badge">Most popular</span>
      <h3>Pro</h3>
      <p class="ls-pricing-price">$29<span>/month</span></p>
      <ul class="ls-pricing-features">
        <li><svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M3 8.5l3.5 3.5L13 4" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg> 10 projects</li>
        <li><svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M3 8.5l3.5 3.5L13 4" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg> 100 GB storage</li>
        <li><svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M3 8.5l3.5 3.5L13 4" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg> Priority support</li>
        <li><svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M3 8.5l3.5 3.5L13 4" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg> Advanced analytics</li>
      </ul>
      <a href="#pro" class="ls-pricing-btn ls-pricing-btn--solid">Get started</a>
    </div>
    <div class="ls-pricing-card">
      <h3>Business</h3>
      <p class="ls-pricing-price">$79<span>/month</span></p>
      <ul class="ls-pricing-features">
        <li><svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M3 8.5l3.5 3.5L13 4" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg> Unlimited projects</li>
        <li><svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M3 8.5l3.5 3.5L13 4" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg> 1 TB storage</li>
        <li><svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M3 8.5l3.5 3.5L13 4" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg> Dedicated support</li>
        <li><svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M3 8.5l3.5 3.5L13 4" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg> Custom integrations</li>
      </ul>
      <a href="#business" class="ls-pricing-btn ls-pricing-btn--outline">Contact sales</a>
    </div>
  </div>
</section>

### Example: Testimonials

<style>
.ls-testimonials-example {
  padding: var(--space-8) var(--space-5);
  color: var(--color-text);
}
.ls-testimonials-example h2 { text-align: center; font-size: clamp(1.75rem, 4vw, 2.5rem); margin: 0 0 var(--space-8); }
.ls-testimonials-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: var(--space-6);
  max-width: 64rem;
  margin: 0 auto;
}
.ls-testimonial-card {
  background: var(--color-card-bg);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: var(--space-8);
  transition: transform var(--transition-fast), box-shadow var(--transition-fast);
}
.ls-testimonial-card:hover { transform: translateY(-2px); box-shadow: var(--shadow-md); }
.ls-testimonial-card blockquote {
  margin: 0 0 var(--space-6);
  font-style: italic;
  color: var(--color-text-muted);
  line-height: 1.6;
}
.ls-testimonial-avatar { display: flex; align-items: center; gap: var(--space-3); }
.ls-testimonial-avatar img { width: 3rem; height: 3rem; border-radius: 50%; object-fit: cover; }
.ls-testimonial-name { font-weight: 600; font-size: 0.95rem; }
.ls-testimonial-role { font-size: 0.8rem; color: var(--color-text-muted); }
@media (prefers-reduced-motion: reduce) {
  .ls-testimonials-example * { transition: none !important; transform: none !important; }
}
</style>
<section class="ls-testimonials-example">
  <h2>What our users say</h2>
  <div class="ls-testimonials-grid">
    <div class="ls-testimonial-card">
      <blockquote>"This platform completely transformed how our team ships content. We went from weeks to hours."</blockquote>
      <div class="ls-testimonial-avatar">
        <img src="https://example.com/avatar1.jpg" alt="Sarah Chen" loading="lazy">
        <div><div class="ls-testimonial-name">Sarah Chen</div><div class="ls-testimonial-role">CTO, TechCorp</div></div>
      </div>
    </div>
    <div class="ls-testimonial-card">
      <blockquote>"The AI features are genuinely useful. It's like having a senior developer on demand."</blockquote>
      <div class="ls-testimonial-avatar">
        <img src="https://example.com/avatar2.jpg" alt="Marcus Rivera" loading="lazy">
        <div><div class="ls-testimonial-name">Marcus Rivera</div><div class="ls-testimonial-role">Lead Designer, StudioCo</div></div>
      </div>
    </div>
    <div class="ls-testimonial-card">
      <blockquote>"Finally, a CMS that doesn't make me compromise on design or developer experience."</blockquote>
      <div class="ls-testimonial-avatar">
        <img src="https://example.com/avatar3.jpg" alt="Aiko Tanaka" loading="lazy">
        <div><div class="ls-testimonial-name">Aiko Tanaka</div><div class="ls-testimonial-role">Founder, DesignLab</div></div>
      </div>
    </div>
  </div>
</section>

### Example: Feature grid

<style>
.ls-features-example {
  padding: var(--space-8) var(--space-5);
  color: var(--color-text);
}
.ls-features-example h2 { text-align: center; font-size: clamp(1.75rem, 4vw, 2.5rem); margin: 0 0 var(--space-2); }
.ls-features-example .ls-features-subtitle { text-align: center; color: var(--color-text-muted); margin: 0 0 var(--space-8); max-width: 32rem; margin-left: auto; margin-right: auto; }
.ls-features-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: var(--space-8);
  max-width: 64rem;
  margin: 0 auto;
}
.ls-feature-card {
  padding: var(--space-8);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  transition: transform var(--transition-fast), box-shadow var(--transition-fast);
}
.ls-feature-card:hover { transform: translateY(-3px); box-shadow: var(--shadow-md); }
.ls-feature-icon {
  width: 3rem;
  height: 3rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in srgb, var(--color-primary) 10%, transparent);
  border-radius: var(--radius-lg);
  margin-bottom: var(--space-5);
  color: var(--color-primary);
}
.ls-feature-card h3 { font-size: 1.15rem; margin: 0 0 var(--space-2); }
.ls-feature-card p { font-size: 0.95rem; color: var(--color-text-muted); margin: 0; line-height: 1.5; }
@media (prefers-reduced-motion: reduce) {
  .ls-features-example * { transition: none !important; transform: none !important; }
}
</style>
<section class="ls-features-example">
  <h2>Everything you need</h2>
  <p class="ls-features-subtitle">Powerful features, zero complexity. Built for teams that move fast.</p>
  <div class="ls-features-grid">
    <div class="ls-feature-card">
      <div class="ls-feature-icon"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></svg></div>
      <h3>Lightning Fast</h3>
      <p>Optimized at every layer. Pages load in under 200ms.</p>
    </div>
    <div class="ls-feature-card">
      <div class="ls-feature-icon"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg></div>
      <h3>Secure by Default</h3>
      <p>Enterprise-grade security with zero configuration required.</p>
    </div>
    <div class="ls-feature-card">
      <div class="ls-feature-icon"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg></div>
      <h3>Fully Configurable</h3>
      <p>Every detail is customizable. No lock-in, no compromises.</p>
    </div>
  </div>
</section>`

// buildHTMLSystemPrompt assembles the full HTML system prompt from the base
// rules, optional active-theme CSS, and the examples. It is called once at
// construction so per-request calls just reuse the precomputed string.
func buildHTMLSystemPrompt(themeBaseCSS, themeStyleCSS string) string {
	prompt := htmlSystemPromptBase
	if themeBaseCSS != "" || themeStyleCSS != "" {
		prompt += "\n\n" + fmt.Sprintf(htmlSystemPromptThemeSection, themeBaseCSS, themeStyleCSS)
	}
	prompt += htmlSystemPromptExamples
	return prompt
}

// htmlTranslateSystemPrompt instructs the AI to translate HTML content while
// preserving structure, styles, and URLs — translating only visible text and alt attributes.
const htmlTranslateSystemPrompt = `You are an expert translator. Translate the provided HTML content from %s to %s.

Rules:
- Translate ONLY visible text inside HTML elements and alt attributes.
- Preserve the <style> block UNCHANGED — it is code, not copy. The only exception: translate CSS content: "..." strings if they contain user-facing text.
- Preserve all URLs (href, src, action), class names, IDs, tag structure, and attribute names.
- Preserve all HTML tags, attributes, and structural whitespace.
- Keep image alt text translations accurate and natural in the target language.
- Output ONLY the translated HTML fragment — no markdown fences, no explanation, no commentary.

%s`

// TextGenerationService defines the interface for AI text generation services.
type TextGenerationService interface {
	EnhanceText(ctx context.Context, content, format, mediaContext string) (string, error)
	TranslateText(ctx context.Context, content, sourceLang, targetLang, format string) (string, error)
}

// Format constants for content types.
const (
	FormatTiptap = "tiptap"
	FormatHTML   = "html"
)

// OpenAITextService implements TextGenerationService using any OpenAI-compatible API.
type OpenAITextService struct {
	client           *openai.Client // singleton client
	apiKey           string
	baseURL          string
	model            string
	htmlSystemPrompt string // precomputed at construction from base + theme CSS + examples
}

func (s *OpenAITextService) ensureClient() {
	if s.client == nil {
		opts := []option.RequestOption{
			option.WithAPIKey(s.apiKey),
		}
		if s.baseURL != "" {
			opts = append(opts, option.WithBaseURL(s.baseURL))
		}
		client := openai.NewClient(opts...)
		s.client = &client
	}
}

func (s *OpenAITextService) callChatCompletionTiptap(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	s.ensureClient()

	params := openai.ChatCompletionNewParams{
		Model: s.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		MaxCompletionTokens: openai.Int(16384),
		Temperature:         openai.Float(0.7),
	}

	completion, err := s.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to generate text: %w", err)
	}

	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("no completions returned")
	}

	responseText := completion.Choices[0].Message.Content

	// Strip markdown code fences if present
	responseText = strings.TrimSpace(responseText)
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	// Validate that the response is valid JSON
	var parsed any
	if err := json.Unmarshal([]byte(responseText), &parsed); err != nil {
		return "", fmt.Errorf("AI response is not valid JSON: %w", err)
	}

	// Ensure it has the expected TipTap doc structure
	if doc, ok := parsed.(map[string]any); ok {
		if docType, ok := doc["type"].(string); !ok || docType != "doc" {
			return "", fmt.Errorf("AI response is not a valid TipTap document: missing 'doc' type")
		}
	}

	return responseText, nil
}

func (s *OpenAITextService) callChatCompletionHTML(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	s.ensureClient()

	params := openai.ChatCompletionNewParams{
		Model: s.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		MaxCompletionTokens: openai.Int(16384),
		Temperature:         openai.Float(0.7),
	}

	completion, err := s.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to generate text: %w", err)
	}

	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("no completions returned")
	}

	responseText := completion.Choices[0].Message.Content

	// Strip markdown code fences if present
	responseText = strings.TrimSpace(responseText)
	responseText = strings.TrimPrefix(responseText, "```html")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	return responseText, nil
}

// EnhanceText takes existing content and returns an enhanced version.
// format controls the output format: "tiptap" (default) or "html".
// mediaContext is an optional string of available images injected into the prompt for HTML generation.
func (s *OpenAITextService) EnhanceText(ctx context.Context, content, format, mediaContext string) (string, error) {
	if format == FormatHTML {
		userPrompt := "Generate HTML/CSS based on this description:\n\n" + content
		if mediaContext != "" {
			userPrompt += "\n\n" + mediaContext
		}

		return s.callChatCompletionHTML(ctx, s.htmlSystemPrompt, userPrompt)
	}

	// tiptap format
	systemPrompt := `You are an expert content editor. Your task is to enhance the provided content to be more engaging, compelling, and well-structured.

Understand the content's language, tone, and subject matter. Then:
- Improve clarity, flow, and readability
- Add more vivid descriptions where appropriate
- Make headings more compelling
- Ensure logical structure and organization
- Correct any grammatical issues
- Maintain the original language — do NOT translate
- Preserve the original meaning and key information

` + tiptapSchemaPrompt

	userPrompt := "Enhance this content to be more engaging. Output only the enhanced TipTap JSON:\n\n" + content

	return s.callChatCompletionTiptap(ctx, systemPrompt, userPrompt)
}

// TranslateText translates content from sourceLang to targetLang.
// format controls the output format: "tiptap" (default) or "html".
func (s *OpenAITextService) TranslateText(ctx context.Context, content, sourceLang, targetLang, format string) (string, error) {
	if format == FormatHTML {
		systemPrompt := fmt.Sprintf(htmlTranslateSystemPrompt,
			strings.ToUpper(sourceLang),
			strings.ToUpper(targetLang),
			"", // no schema prompt for HTML — the rules above are sufficient
		)

		userPrompt := fmt.Sprintf("Translate this HTML content from %s to %s. Output only the translated HTML:\n\n%s",
			strings.ToUpper(sourceLang), strings.ToUpper(targetLang), content)

		return s.callChatCompletionHTML(ctx, systemPrompt, userPrompt)
	}

	// tiptap format
	systemPrompt := fmt.Sprintf(
		`You are an expert translator. Translate the provided content from %s to %s.

- Preserve the TipTap JSON structure exactly — only translate the text content
- Maintain all formatting: bold, italic, underline, links, headings, lists, etc.
- Keep code blocks, image URLs, YouTube URLs, and math formulas unchanged
- Preserve all node types, attributes, and marks — only change text values and alt text
- Output ONLY the translated TipTap JSON with no additional commentary

%s`,
		strings.ToUpper(sourceLang),
		strings.ToUpper(targetLang),
		tiptapSchemaPrompt,
	)

	userPrompt := fmt.Sprintf("Translate this content from %s to %s. Output only the translated TipTap JSON:\n\n%s",
		strings.ToUpper(sourceLang), strings.ToUpper(targetLang), content)

	return s.callChatCompletionTiptap(ctx, systemPrompt, userPrompt)
}

// NewOpenAITextService creates a new OpenAI-compatible text generation service.
// baseURL is optional — pass "" to use the OpenAI default.
// model is the chat model name (e.g. "gpt-5-mini", "gpt-5.4-mini", "openai/gpt-5-mini" for OpenRouter).
// themeBaseCSS and themeStyleCSS are the active theme's minified base.css (design
// tokens) and style.css (component styles). When non-empty, they are injected
// into the HTML generation prompt so output reuses the site's brand tokens and
// component classes instead of inventing off-brand styles. Pass empty strings
// when no theme is available (the embedded defaults are still a reasonable brief).
func NewOpenAITextService(apiKey, baseURL, model, themeBaseCSS, themeStyleCSS string) *OpenAITextService {
	return &OpenAITextService{
		apiKey:           apiKey,
		baseURL:          baseURL,
		model:            model,
		htmlSystemPrompt: buildHTMLSystemPrompt(themeBaseCSS, themeStyleCSS),
	}
}
