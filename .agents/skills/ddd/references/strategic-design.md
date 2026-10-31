# Strategic design

Load this reference for the **Discover** stage: subdomains, bounded contexts, context mapping, ubiquitous language, and event storming. Source orientation: Vernon, *Implementing DDD* (strategic chapters); Evans, *Domain-Driven Design*.

---

## Subdomains

A subdomain is a slice of the **business problem**. Classify each before investing:
- **Core** — what the business does better than competitors; the differentiator. Invest the best modeling effort here.
- **Supporting** — necessary to support the Core but not differentiating. Model enough to serve the Core; don't over-engineer.
- **Generic** — commodity capability everyone needs (auth, messaging, billing). Buy / use off-the-shelf; don't build bespoke models.

Subdomains are about the **problem space**; bounded contexts (below) are about the **solution space** — they often align, but aren't identical.

## Bounded contexts

A bounded context is an explicit boundary within which a **single model and ubiquitous language** apply. Discovery signals:
- A cluster of related domain events and commands (from Event Storming).
- A team ownership boundary.
- A place where a term changes meaning (e.g., "Product" means different things in Catalog vs Inventory vs Sales — that's ≥2 contexts).

Prefer **fewer, well-bounded** contexts over many tiny ones. Each context owns its own model; do not force one model across the whole business.

## Context mapping — the 9 patterns

When contexts integrate, choose a relationship pattern:

1. **Partnership** — two teams depend on each other and coordinate closely. (Costly; reserve for genuine mutual dependence.)
2. **Shared Kernel** — two contexts share a small, explicit subset of model. Changes need both teams. Use sparingly; the coupling is real.
3. **Customer–Supplier** — upstream *Supplier* serves downstream *Customer*; supplier accommodates the customer's needs because it has priority/value.
4. **Conformist** — the Customer adopts the Supplier's model **as-is** (no leverage to negotiate). Eases integration by conforming rather than translating.
5. **Anticorruption Layer (ACL)** — a translation layer that protects a context from a foreign/legacy model. The go-to pattern for integrating with legacy or messy upstreams. Translate at the boundary; keep the foreign model out.
6. **Open Host Service (OHS)** — a context exposes one open protocol/API for all integrators (vs a custom integration per consumer).
7. **Published Language** — a well-documented, often standardized interchange language, typically used *with* OHS (e.g., a standard schema/IDL).
8. **Separate Ways** — no integration. Contexts are independent; the cost of connecting them exceeds the value. Valid and underused.
9. **Big Ball of Mud** — a legacy context with no clear structure/boundaries. Contain it; ring it with an ACL so it doesn't corrupt neighbors; don't try to perfect it all at once.

## How to draw & read the context map (`templates/context-map.puml`)

The template uses the **canonical Vernon/Socoli notation** so the map is unambiguous, not just boxes with text on arrows. Apply it consistently.

**Reading the markers:**
- **U** = Upstream (supplier / source of influence). **D** = Downstream (customer / consumer).
- **Influence flows `U --> D`.** The pattern name sits at the **upstream end**.
- Arrow direction in the template: drawn from the upstream context to the downstream context.

**Line style per pattern** (encode the pattern in the line, don't rely on text alone):

| Style | Pattern | How to write |
|---|---|---|
| solid arrow | Customer–Supplier, Conformist | `Upstream "U" --> "D" Downstream : Customer-Supplier` |
| solid, `D (C)` at the conformist end | Conformist | `Upstream "U" --> "D (C)" Downstream : Conformist` |
| dashed crimson, `D (ACL)` at the protected end | Anticorruption Layer | `Upstream "U" -[#crimson,dashed]-> "D (ACL)" Downstream : Anticorruption Layer` |
| gold double-headed, `P` both ends | Partnership | `A "P" <-[#gold]-> "P" B : Partnership` |
| gold line, `SK` both ends | Shared Kernel | `A "SK" -[#gold]-> "SK" B : Shared Kernel` |
| `U (OHS+PL)` at the upstream end | Open Host Service + Published Language | `Upstream "U (OHS+PL)" --> "D" Downstream : OHS + Published Language` |
| **no line** | Separate Ways | Draw nothing — independence is the signal. |
| context box styled `<<mud>>` (crimson border, rose fill) | Big Ball of Mud | `rectangle "Legacy ERP" as ERP <<mud>>` |

Also stereotype each context by **subdomain type** so the box color reflects investment: `<<core>>`, `<<supporting>>`, `<<generic>>`, `<<mud>>`.

**Rendering.** The map is a `.puml` file. Render it with any PlantUML frontend: the public server at `https://www.plantuml.com/plantuml`, a local `plantuml file.puml` (needs Java + `plantuml.jar`), or a plugin in VS Code / IntelliJ / GitLab. The file is plain text, so it diffs cleanly and is consumable by any agent or human even before rendering.

## Ubiquitous language

A rigorous, shared vocabulary used identically by domain experts, in conversation, in documents, and **in code**.
- Every term has one meaning within a bounded context.
- Eliminate translation between "business speak" and "code speak."
- Refine continuously; when the model changes, the language changes.
- If two experts use a term differently, you probably have two contexts (or two concepts).

## Event Storming

Run it collaboratively with the user.

**Big Picture** — map the whole domain as a timeline:
- Brainstorm **domain events** (past tense: `OrderPlaced`, `PaymentCaptured`) on the timeline.
- Add **commands** (intent: `PlaceOrder`), **actors/policies**, and **hotspots** (pain points, questions, bottlenecks).
- Cluster events/commands that change together → candidate **bounded contexts**.
- Goal: shared big-picture understanding and subdomain/context candidates.

**Design Level** — zoom into one bounded context:
- Refine events → commands → **aggregate candidates** (what handles each command, enforces the invariant, emits the event).
- Identify **policies** ("when X, do Y") that wire eventual consistency across aggregates.
- Goal: aggregate and boundary design ready for tactical modeling.

This feeds `templates/domain-context.md`, `templates/ubiquitous-language.md`, and `templates/context-map.puml`.

## Sequence diagrams as input (optional)

Users sometimes arrive with **sequence diagrams** (PlantUML, Mermaid, or an image) describing an *as-is* or *to-be* business process or a key scenario. Accept them — they're a fast way to hand over the timeline Event Storming is trying to discover. But treat them as **input only**, and remember what they are and aren't:

- A sequence diagram describes **process** (what happens over time), **not structure** (boundaries, language, ownership). It can *inform* the context map and ubiquitous language; it must never *replace* them. A perfectly coherent end-to-end sequence can still cross three contexts you haven't named yet.
- **Extract, don't copy:**
  - **Domain events** — often the labeled return messages or side-effects (`OrderPlaced`, `PaymentCaptured`). These seed the Event Storming timeline directly.
  - **Commands / intents** — the incoming messages (`placeOrder`) → candidate command methods on aggregates.
  - **Participants / lifelines** — clusters that change together are candidate **bounded contexts**; a single lifeline that "does everything" is a smell.
  - **Actors** — candidate context owners / teams.
  - **Hotspots** — any **synchronous call** across what looks like an aggregate boundary. That's the single most important finding to carry forward: it is *not* a design to replicate, it is a *symptom* (often of an anemic model or a missing event) to resolve in the **Model** stage.
- **Do not produce** a sequence diagram here. At this stage no aggregates exist to sequence; producing one would just redraw the Event Storming timeline in a different notation. Production belongs to the **Model** stage, where it validates aggregate boundaries (`references/tactical-patterns.md` → *Scenario walkthrough*).

If the supplied diagram contradicts an emerging bounded-context split, trust the split and note the contradiction as a hotspot — the diagram is documenting current reality, which is often the thing DDD is trying to change.

## From bounded contexts to deployable units

Bounded contexts are a **design** concept; microservices are a **deployment** concept. They align well but are not identical. Produce the mapping below **only when the user is actually building microservices** — for a modular monolith, stop at the context map.

**Rules of thumb (Vernon, IDDD):**
- **One bounded context = one service** is the default and the cleanest boundary. The context map already gave you the integration topology; the service topology just makes the *deployable units* explicit.
- **Co-deploying contexts is valid** (a modular monolith, or one service hosting a small cluster of tightly-coupled contexts). Prefer this over premature distribution when the contexts change together or share a transactional need.
- **Never split a bounded context across services.** A single model + ubiquitous language + consistency boundary must live in one deployable; splitting it re-introduces the very coupling contexts exist to escape.
- **Each service owns its datastore.** No shared database between services — integrate via the context-map pattern (OHS+PL, ACL, integration events), not via a common schema. A shared DB is a smell to flag, not a design.
- **Derive the transport from the context-map pattern** — don't invent it:
  - OHS + Published Language → synchronous open API (REST/gRPC) with a documented schema.
  - Customer–Supplier / Conformist → synchronous call to the supplier's OHS (or a translation adapter on the customer side).
  - Anticorruption Layer → a translation adapter at the consuming service's edge.
  - Domain events crossing a context boundary → **integration events** on a broker (async, eventual consistency). This is the primary async glue between services.
  - Separate Ways → no integration; no transport.

**What each service contains** is already described by the bounded-context canvas(es) of the context(s) assigned to it (`bounded-context-*.md`) — their aggregates, language, and class diagram. The service topology adds *which deployable hosts which context(s), its datastore, and its transport*; it does **not** re-describe the model.

Produce `templates/service-topology.puml` (derived from the context map) and fill the **Service topology** section of `templates/domain-context.md`. Reuse the context map's subdomain-type coloring so the two diagrams read consistently.
