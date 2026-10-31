# Assessment, the DDD-worthiness gate, and the audit scorecard

Load this reference for the **Assess** stage, the **Gate**, and the **Audit**.

---

## Greenfield vs brownfield

- **Greenfield** — little or no domain code yet. Start from requirements and model freely.
- **Brownfield** — existing code that encodes real decisions (and real debt). Understand before reshaping; respect what works.

How to tell: look for existing source that implements **domain behavior** (not just config, scaffolding, sample/demo modules, or a private/company boilerplate starter). A substantive `src/`, `app/`, `domain/`, `internal/`, or business module that encodes decisions **unique to this project** → brownfield. Treat generated/scaffold-only trees, sample/example entities (the classic `User`/`Post`/`Todo`), vendored common libraries, and copy-pasted boilerplate starters as **closer to greenfield** — they give you layout and conventions to respect, but no real domain model to preserve. When in doubt, ask the user whether the code is theirs or a starter they intend to replace.

## Anemic vs rich model (brownfield scan)

Scan the most behavior-heavy module.

**Anemic (red flag):**
- "Entities" are data bags — only properties/getters/setters, no methods that enforce rules.
- All behavior lives in `*Service` / `*Manager` / `*Helper` classes that read and mutate those bags.
- Invariants are absent, scattered, or enforced only at the controller/UI edge.
- Smell: `OrderService.placeOrder(order, items)` mutates an `Order` whose class has no `place(...)`.

**Rich (green flag):**
- Behavior and invariants live **on** the aggregate/entity (e.g., `order.place(items)` validates and applies).
- Services *coordinate* (load via repository, call domain methods, persist) rather than *contain* the logic.
- Construction enforces validity (factories / constructors reject bad state).

Record the finding — it tells you how much reshaping the tactical step needs.

## Repository & deployment structure (brownfield scan)

Also detect **how the code is organized and deployed** — it changes extraction-vs-greenfield advice and seeds the later service-topology step.

**Monorepo signals:**
- Workspace manifests: `pnpm-workspace.yaml`, `package.json` `"workspaces"`, `lerna.json`, `turbo.json`, `nx.json`, Bazel (`WORKSPACE`/`BUILD` roots).
- Enterprise polyglot: Maven multi-module (`<modules>` in a parent `pom.xml`), Gradle composite / `settings.gradle(.kts)` `include(...)`, .NET `*.sln` with many `*.csproj`.
- Layout smell: top-level `services/` + `apps/` + `packages/` + `libs/` (or `apps/` + `libs/`).

**Microservice signals:**
- Multiple independently deployable units — each with its own manifest/Dockerfile/build pipeline.
- Per-service module roots (e.g. `services/orders/`, `services/billing/`).
- **Datastore ownership:** a database per service (good) vs one shared DB touched by several services (the *shared-database* integration anti-pattern — flag it).
- **Inter-service transport:** direct REST/gRPC calls vs an event broker (Kafka/RabbitMQ/NATS/SQS) consuming integration events.

Record: *monorepo or polyrepo; if monorepo, the workspace layout; how many services; datastore ownership; transport.* This tells you whether bounded contexts already have physical boundaries (good) or are logical-only inside a monolith (extraction work ahead), and it seeds the service-topology step when the user is going microservice.

---

## The DDD-worthiness gate (do this before modeling)

This is the soul of the skill. DDD pays off only where the domain is worth the investment. Score four dimensions:

| Dimension | Question | Signal |
|---|---|---|
| **Subdomain type** | Core, Supporting, or Generic? | Core = differentiating heart of the business; Supporting = necessary but not differentiating; Generic = commodity (auth, email, CRUD admin, reporting) |
| **Behavioral complexity** | Are there real rules/invariants/state machines? | Many invariants & workflows = high; plain create/read/update/delete on rows = low |
| **Strategic value** | Does getting this right move the business? | High / medium / low |
| **Volatility** | How often do the rules change? | Frequent change rewards a supple model |

### Decision

- **Core + complex + high value (+ volatile)** → **Full DDD**. Invest in ubiquitous language, bounded contexts, aggregates, domain events.
- **Supporting, or moderate complexity** → **Light DDD**. Model the key aggregates and ubiquitous language; skip elaborate context mapping and ceremony.
- **Generic / simple CRUD / low complexity / low value** → **Plain CRUD. STOP.** Tell the user DDD is overkill. Recommend the simplest durable option: a CRUD/repository layer, an off-the-shelf/SaaS component, or an existing library. Do **not** invent aggregates, domain events, or context maps for it.

When uncertain, default toward the lighter option and say so. The courage to *not* model is a DDD virtue.

---

## The audit scorecard (optional)

When a `domain-context.md` (with its ubiquitous language and aggregates) exists, score the code against it. Each dimension 0–5:

1. **Language alignment** — do identifiers, types, and operations match the ubiquitous-language glossary? (0 = divergent jargon; 5 = faithful end to end.)
2. **Model richness** — behavior and invariants on aggregates vs anemic DTOs + procedural services.
3. **Aggregate boundary integrity** — other aggregates referenced by identity only; one transaction modifies one aggregate; no object-graph traversal across boundaries.
4. **Bounded-context isolation** — no leakage of another context's concepts/language; integration via Anticorruption Layer / Open Host Service where needed.
5. **Tactical-pattern coverage** — appropriate use of entities, value objects, domain events, repositories, factories.

Output a **report card** (score per dimension + overall), the 3 worst offenders with evidence (`file:line`), and a **prioritized remediation** list (highest-leverage fixes first).
