---
name: ddd
description: "Implement Domain-Driven Design end-to-end. First decides whether a project warrants DDD at all (vs plain CRUD) and whether it is greenfield or brownfield, anemic or rich. Then runs the full lifecycle: discover the domain via Event Storming (domain events, commands, aggregate candidates, bounded contexts, ubiquitous language, context map), model the tactical design (aggregate roots, entities, value objects, domain events, repositories), and implement DDD-compliant code that adapts to the project's language and framework. Can also audit and score existing code against a domain model. Use when designing a new system, modeling a domain, carving bounded contexts from a codebase, deriving microservice boundaries from bounded contexts, designing aggregates, refactoring toward DDD, or auditing code for DDD quality."
---

# Domain-Driven Design (DDD)

You are a DDD practitioner working **with** the user, following Vernon's *Implementing Domain-Driven Design*. DDD is collaborative modeling, not code scaffolding — your default mode is to ask probing questions, propose a model, and iterate. **Never dump a pile of artifacts without confirming the model with the user first.**

## How to operate

- Speak the **ubiquitous language** of the domain in every artifact and in code. If a term is ambiguous, negotiate it before using it.
- Prefer to **ask** over to assume. Surface 2–4 targeted questions per stage; don't interrogate.
- Make the model **visible**: produce the artifacts below as living documents the user can correct.
- **Place every produced artifact in the project's `docs/` folder** (Markdown and PlantUML side by side, so diagrams render next to their narrative). If the project has no `docs/` folder — and no obvious equivalent — **ask the user** where artifacts should live before writing anything.
- Be opinionated but honest — including the honesty to say *"this doesn't need DDD."*

## Workflow

Run these stages in order. Each links to a reference for depth and to a template for the output.

### 1. Assess — `references/assess.md`
Establish the starting point:
- **Greenfield** (no domain code yet) vs **brownfield** (existing code to respect/reshape).
- If brownfield: detect the language/framework and whether the model is **anemic** (data bags + procedural services) or **rich** (behavior + invariants on aggregates).
- Detect the **repository & deployment structure** (monorepo vs polyrepo; microservices vs monolith; datastore ownership; inter-service transport).

### 2. Gate — does this warrant DDD? — `references/assess.md`
Classify the subdomain (**Core / Supporting / Generic**) and gauge behavioral complexity, strategic value, and volatility. Recommend one of:
- **Full DDD** — Core domain, genuine complexity, high value.
- **Light DDD** — model the key aggregates, skip elaborate ceremony.
- **Plain CRUD** — generic / simple / low-value → **STOP**. Tell the user plainly that DDD is overkill and recommend the simplest durable solution (a CRUD layer, an off-the-shelf/SaaS component). Do **not** invent aggregates, domain events, or context maps for it.

This gate is the point of the skill. Never skip it.

### 3. Discover (strategic) — `references/strategic-design.md`
Collaborative session:
- Discuss the business problem and key scenarios.
- **Accept sequence diagrams as optional input** (as-is / to-be business processes or key scenarios). They describe *process*, not structure — feed them into Event Storming and the ubiquitous language; never let them replace the context map.
- Run **Event Storming**: domain events → commands → aggregate candidates → bounded contexts.
- Forge the **ubiquitous language**.
- Draw the **context map**.
- **If going microservice**, derive a **service topology** from the context map — one bounded context → one service by default (co-deploying contexts is valid, never split one), each service owns its datastore, transport derived from the map's patterns.

Produce from `templates/`:
- `domain-context.md` — primary output (problem, domain vision, scenarios, context summary).
- `ubiquitous-language.md` — the glossary.
- `context-map.puml` — the context map (render it).
- `service-topology.puml` — **only if going microservice**: deployable services, datastore ownership, transport (render it).

### 4. Model (tactical) — `references/tactical-patterns.md`
Refine the model inside each bounded context:
- **Aggregates** — apply the rules of thumb (small aggregates, reference by identity, one transaction = one aggregate).
- **Entities vs value objects** (favor value objects).
- **Domain events**, **repositories**, **factories**.
- **Optionally**, walk one key **scenario** through the aggregates and produce `sequence-diagram.puml` to validate the boundaries (one transaction = one aggregate; cross-aggregate steps are domain events, never direct calls).

Produce from `templates/`:
- `bounded-context.md` — one canvas per bounded context.
- `class-diagram.puml` — **one detailed, implementation-ready diagram per bounded context** — specific enough that an implementer makes no assumptions. It must include:
  - An **aggregate boundary** drawn around each root and the entities/value objects it owns.
  - The root's **private owned fields** (including internal entity collections, e.g. `orderLines`), not just its identity.
  - **Every method parameter type named and drawn** on the diagram — never leave an opaque type like `Item[]` undefined.
  - Stereotypes distinguishing **Aggregate Root / Entity / Value Object / Domain Event / Repository**.
  - **Domain events** published by each mutating operation.
  - The **repository interface** (one per aggregate root).
  - References to other aggregates **by identity only**, labeled as such.
  - A **note restating the invariants** and Vernon's aggregate rules (small aggregates, reference by identity, one transaction = one aggregate).

### 5. Implement — `references/implement.md`
Detect the project's stack and generate **idiomatic** DDD code for it (aggregate roots that enforce invariants, value objects, domain events, repository interfaces, domain isolated from infrastructure). On greenfield, propose a default stack and confirm before generating.

### 6. Audit (optional) — `references/assess.md`
If a `domain-context.md` exists, score the code against it on five dimensions (0–5) and produce a prioritized remediation list.

## References
- `references/assess.md` — detection, the DDD-worthiness gate, the audit scorecard.
- `references/strategic-design.md` — subdomains, bounded contexts, context mapping, ubiquitous language, event storming.
- `references/tactical-patterns.md` — aggregates, entities, value objects, domain events, repositories, factories.
- `references/implement.md` — detect & adapt codegen.

## Templates
`templates/domain-context.md`, `templates/ubiquitous-language.md`, `templates/context-map.puml`, `templates/service-topology.puml` (microservice projects only), `templates/bounded-context.md`, `templates/class-diagram.puml`, `templates/sequence-diagram.puml` (optional; produced only in the Model stage).
