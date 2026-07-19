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

// htmlDocumentSystemPrompt is the system prompt for generating and enhancing
// HTML/CSS content. It instructs the AI to produce production-ready, semantic,
// accessible, and responsive HTML fragments with all styling in a <style> block.
const htmlDocumentSystemPrompt = `You are an expert front-end developer creating production-ready HTML and CSS for a website's content area. You write clean, semantic, accessible, and visually premium code that rivals the best page builders.

## Output format

Output a SINGLE HTML fragment. Start with a <style> block, followed by the HTML markup.
- NO <!DOCTYPE>, <html>, <head>, or <body> wrappers — just the content fragment.
- NO markdown fences, no explanation, no commentary — ONLY the raw HTML.

## Hard rules

- ALL styling goes in the <style> block at the top. NEVER use inline style="..." attributes. Inline styles are terrible for performance, readability, and overrides.
- Scope every CSS selector under a unique top-level class .ls-<topic> (e.g., .ls-hero-pricing, .ls-testimonials-section).
- NO <script>, NO on* handlers, NO javascript: URLs.
- NO external resources — no <link>, no <script src>, no @import, no external fonts. Use a system font stack: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif.
- Close all tags, use valid HTML5.

## Design directives (apply to every output)

- **Visual hierarchy**: clear primary/secondary/tertiary emphasis via size, weight, color, and spacing.
- **Whitespace**: generous padding (at least clamp(1rem, 4vw, 3rem) between sections) and breathing room.
- **Color theory**: cohesive palette via CSS custom properties (--accent, --bg, --text, --muted). No raw #000/#fff for text. Honor prefers-color-scheme: dark.
- **Modern CSS**: Grid + Flexbox for layout (no floats). Use clamp() for fluid typography, aspect-ratio for media, custom properties for theming.
- **Micro-interactions**: :hover, :focus-visible, transition (150-250ms), transform on interactive elements. Respect prefers-reduced-motion.
- **Responsive**: mobile-first, no fixed pixel widths on containers, fluid images (max-width: 100%; height: auto).
- **Accessibility**: WCAG AA contrast (4.5:1 body text), visible focus rings, semantic landmarks (<section>, <article>, <nav>), single <h2> per block at most (the page owns <h1>), alt text on all images.
- **Images**: only use URLs from the "Available images" list when relevant. Never invent image URLs. Always add loading="lazy" and meaningful alt.

## Few-shot example: Hero section

<style>
.ls-hero-example {
  --accent: #4f46e5;
  --accent-hover: #4338ca;
  --text: #1e293b;
  --muted: #64748b;
  --bg: #fafafa;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 50vh;
  padding: clamp(3rem, 8vw, 6rem) clamp(1.5rem, 4vw, 3rem);
  background: linear-gradient(135deg, var(--bg) 0%, #f0f0f5 100%);
  text-align: center;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  color: var(--text);
}
.ls-hero-example h2 {
  font-size: clamp(2rem, 5vw, 3.5rem);
  font-weight: 700;
  line-height: 1.15;
  margin: 0 0 0.75rem;
}
.ls-hero-example p {
  font-size: clamp(1.05rem, 2vw, 1.25rem);
  color: var(--muted);
  max-width: 36rem;
  margin: 0 0 2rem;
}
.ls-hero-example .ls-hero-buttons {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
  justify-content: center;
}
.ls-hero-example .ls-hero-btn-primary {
  display: inline-block;
  padding: 0.85rem 2rem;
  background: var(--accent);
  color: #fff;
  border: none;
  border-radius: 0.5rem;
  font-size: 1rem;
  font-weight: 600;
  text-decoration: none;
  transition: background 200ms, transform 200ms;
}
.ls-hero-example .ls-hero-btn-primary:hover {
  background: var(--accent-hover);
  transform: translateY(-1px);
}
.ls-hero-example .ls-hero-btn-primary:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
.ls-hero-example .ls-hero-btn-secondary {
  display: inline-block;
  padding: 0.85rem 2rem;
  color: var(--accent);
  border: 2px solid var(--accent);
  border-radius: 0.5rem;
  font-size: 1rem;
  font-weight: 600;
  text-decoration: none;
  transition: background 200ms, color 200ms;
}
.ls-hero-example .ls-hero-btn-secondary:hover {
  background: var(--accent);
  color: #fff;
}
.ls-hero-example .ls-hero-btn-secondary:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
@media (prefers-reduced-motion: reduce) {
  .ls-hero-example * { transition: none !important; transform: none !important; }
}
@media (prefers-color-scheme: dark) {
  .ls-hero-example { --text: #e2e8f0; --muted: #94a3b8; --bg: #0f172a; }
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

## Few-shot example: Pricing table

<style>
.ls-pricing-example {
  --accent: #6366f1;
  --text: #1e293b;
  --muted: #64748b;
  --bg: #ffffff;
  --border: #e2e8f0;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  padding: clamp(2rem, 5vw, 4rem) clamp(1rem, 4vw, 3rem);
  color: var(--text);
}
.ls-pricing-example h2 {
  text-align: center;
  font-size: clamp(1.75rem, 4vw, 2.5rem);
  margin: 0 0 0.5rem;
}
.ls-pricing-example .ls-pricing-subtitle {
  text-align: center;
  color: var(--muted);
  margin: 0 0 3rem;
}
.ls-pricing-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 1.5rem;
  max-width: 56rem;
  margin: 0 auto;
}
.ls-pricing-card {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 1rem;
  padding: 2rem;
  display: flex;
  flex-direction: column;
}
.ls-pricing-card--featured {
  border-color: var(--accent);
  box-shadow: 0 4px 24px rgba(99,102,241,.15);
  transform: scale(1.03);
}
.ls-pricing-card .ls-pricing-badge {
  display: inline-block;
  background: var(--accent);
  color: #fff;
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.25rem 0.75rem;
  border-radius: 9999px;
  align-self: flex-start;
  margin-bottom: 0.75rem;
}
.ls-pricing-card h3 { font-size: 1.25rem; margin: 0 0 0.5rem; }
.ls-pricing-card .ls-pricing-price {
  font-size: 2rem;
  font-weight: 700;
  margin: 0 0 1.5rem;
}
.ls-pricing-card .ls-pricing-price span { font-size: 0.875rem; font-weight: 400; color: var(--muted); }
.ls-pricing-features { list-style: none; padding: 0; margin: 0 0 2rem; flex: 1; }
.ls-pricing-features li {
  padding: 0.4rem 0;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.95rem;
  color: var(--muted);
}
.ls-pricing-features li svg { flex-shrink: 0; color: var(--accent); }
.ls-pricing-card .ls-pricing-btn {
  display: block;
  text-align: center;
  padding: 0.75rem;
  border-radius: 0.5rem;
  font-weight: 600;
  text-decoration: none;
  transition: background 200ms, color 200ms;
}
.ls-pricing-card .ls-pricing-btn--outline { border: 2px solid var(--accent); color: var(--accent); }
.ls-pricing-card .ls-pricing-btn--outline:hover { background: var(--accent); color: #fff; }
.ls-pricing-card .ls-pricing-btn--solid { background: var(--accent); color: #fff; border: none; }
.ls-pricing-card .ls-pricing-btn--solid:hover { background: #4f46e5; }
.ls-pricing-card .ls-pricing-btn:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
@media (prefers-reduced-motion: reduce) {
  .ls-pricing-example * { transition: none !important; transform: none !important; }
}
@media (prefers-color-scheme: dark) {
  .ls-pricing-example { --text: #e2e8f0; --muted: #94a3b8; --bg: #1e293b; --border: #334155; }
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

## Few-shot example: Testimonials

<style>
.ls-testimonials-example {
  --accent: #8b5cf6;
  --text: #1e293b;
  --muted: #64748b;
  --bg: #f8fafc;
  --card-bg: #ffffff;
  --border: #e2e8f0;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  padding: clamp(2rem, 5vw, 4rem) clamp(1rem, 4vw, 3rem);
  color: var(--text);
}
.ls-testimonials-example h2 { text-align: center; font-size: clamp(1.75rem, 4vw, 2.5rem); margin: 0 0 2.5rem; }
.ls-testimonials-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 1.5rem;
  max-width: 64rem;
  margin: 0 auto;
}
.ls-testimonial-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 1rem;
  padding: 2rem;
  transition: transform 200ms, box-shadow 200ms;
}
.ls-testimonial-card:hover { transform: translateY(-2px); box-shadow: 0 8px 24px rgba(0,0,0,.06); }
.ls-testimonial-card blockquote {
  margin: 0 0 1.5rem;
  font-style: italic;
  color: var(--muted);
  line-height: 1.6;
}
.ls-testimonial-avatar { display: flex; align-items: center; gap: 0.75rem; }
.ls-testimonial-avatar img { width: 3rem; height: 3rem; border-radius: 50%; object-fit: cover; }
.ls-testimonial-name { font-weight: 600; font-size: 0.95rem; }
.ls-testimonial-role { font-size: 0.8rem; color: var(--muted); }
@media (prefers-reduced-motion: reduce) {
  .ls-testimonials-example * { transition: none !important; transform: none !important; }
}
@media (prefers-color-scheme: dark) {
  .ls-testimonials-example { --text: #e2e8f0; --muted: #94a3b8; --bg: #0f172a; --card-bg: #1e293b; --border: #334155; }
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

## Few-shot example: Feature grid

<style>
.ls-features-example {
  --accent: #0ea5e9;
  --text: #1e293b;
  --muted: #64748b;
  --bg: #ffffff;
  --border: #e2e8f0;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  padding: clamp(2rem, 5vw, 4rem) clamp(1rem, 4vw, 3rem);
  color: var(--text);
}
.ls-features-example h2 { text-align: center; font-size: clamp(1.75rem, 4vw, 2.5rem); margin: 0 0 0.5rem; }
.ls-features-example .ls-features-subtitle { text-align: center; color: var(--muted); margin: 0 0 3rem; max-width: 32rem; margin-left: auto; margin-right: auto; }
.ls-features-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 2rem;
  max-width: 64rem;
  margin: 0 auto;
}
.ls-feature-card {
  padding: 2rem;
  border: 1px solid var(--border);
  border-radius: 1rem;
  transition: transform 200ms, box-shadow 200ms;
}
.ls-feature-card:hover { transform: translateY(-3px); box-shadow: 0 8px 24px rgba(0,0,0,.06); }
.ls-feature-icon {
  width: 3rem;
  height: 3rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in srgb, var(--accent) 10%, transparent);
  border-radius: 0.75rem;
  margin-bottom: 1.25rem;
  color: var(--accent);
}
.ls-feature-card h3 { font-size: 1.15rem; margin: 0 0 0.5rem; }
.ls-feature-card p { font-size: 0.95rem; color: var(--muted); margin: 0; line-height: 1.5; }
@media (prefers-reduced-motion: reduce) {
  .ls-features-example * { transition: none !important; transform: none !important; }
}
@media (prefers-color-scheme: dark) {
  .ls-features-example { --text: #e2e8f0; --muted: #94a3b8; --bg: #0f172a; --border: #334155; }
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
	client  *openai.Client // singleton client
	apiKey  string
	baseURL string
	model   string
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
		systemPrompt := htmlDocumentSystemPrompt

		userPrompt := "Generate HTML/CSS based on this description:\n\n" + content
		if mediaContext != "" {
			userPrompt += "\n\n" + mediaContext
		}

		return s.callChatCompletionHTML(ctx, systemPrompt, userPrompt)
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
func NewOpenAITextService(apiKey, baseURL, model string) *OpenAITextService {
	return &OpenAITextService{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
	}
}
