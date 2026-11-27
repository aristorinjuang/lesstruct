# Tactical design

Load this reference for the **Model** stage: designing aggregates, entities, value objects, domain events, repositories, and factories inside a bounded context. Source: Vernon, *Implementing DDD*.

---

## Aggregates

An **aggregate** is a cluster of domain objects treated as one unit for data changes, with one **aggregate root** as the single entry point. The root enforces the aggregate's invariants and guarantees consistency **within a single transaction**.

### Aggregate design — rules of thumb
1. **Model true invariants.** Group objects only if they must be consistent together in one transaction. Don't cluster merely because things are "related."
2. **Design small aggregates.** Prefer a root that references other aggregates *by identity*, not by holding the whole object graph. Oversized aggregates cause contention, locking, performance problems, and false consistency requirements.
3. **Reference other aggregates by identity** (not direct object references). This keeps contexts decoupled and avoids loading huge graphs.
4. **One transaction modifies one aggregate.** If a change must cascade to another aggregate, use **eventual consistency** via a **domain event** — never a cross-aggregate transaction.
5. **Enforce invariants on construction and on every mutating operation.** Reject invalid state in constructors/factories and in command methods; don't rely on callers or a later "validation" step.
6. **Prefer eventual consistency outside the boundary.** Publish a domain event when an aggregate changes; let subscribers (other aggregates, other contexts) react.

When an aggregate seems large, that's usually a signal to split into smaller aggregates coordinated by events.

## Root entities, internal entities, value objects

An aggregate contains exactly three kinds of parts. Vernon (*IDDD*, ch. 5–6, 10) distinguishes the **root entity** from **internal entities** — both have identity, but their identity scopes differ:

- **Aggregate root (the root entity)** — top-level; **not owned by any other model**. Has **global identity** (meaningful across the bounded context), a lifecycle of its own, and **owns the repository** (one repository per root). Example: `Order`, `BacklogItem`, `Discussion`.
- **Internal entity** — **owned by** another model: its lifecycle (create, mutate, destroy) is controlled by its aggregate root. Has **identity local to the aggregate** — meaningful only *within* that aggregate instance and meaningless outside it. Equality by id. Never has its own repository. Examples: `OrderLine` (with `OrderLineId`) inside `Order`; Vernon's `Task` (with `TaskId`) inside `BacklogItem`; `Post` (with `PostId`) inside `Discussion`.
- **Value object** — also a member of another model, but has **no identity**; defined entirely by its attributes; **immutable**; equality by value; describes or measures something. Replace rather than mutate. (e.g., `Money`, `Address`, `DateRange`)

**Decision rule:**

| Is it owned by another model? | Does something need to address it individually over its lifecycle? | It is… |
|---|---|---|
| No (top-level) | — | **Aggregate root** |
| Yes, owned (lifecycle controlled by the root) | Yes (rename it, re-estimate it, change its quantity) | **Internal entity** — give it a local id |
| Yes, owned | No (just a descriptive measure) | **Value object** |

**Favor value objects.** They're simpler, safer (immutable, no aliasing bugs), and compose well. Promote an internal part to entity **only when something must address it individually over its lifecycle** (rename, re-estimate, change quantity); otherwise keep it a value object. An order line that is never addressed after creation is a perfectly valid value object — it becomes an entity the moment `changeLineQuantity(lineId, …)` must find *that* line.

**Owned vs referenced.** "Owned by" means the root controls the part's lifecycle. A part that merely *references* another aggregate by identity (`OrderLine → ProductId`) does **not** own it — `Product` stays a separate root. Ownership is lifecycle control, not pointer-holding.

**The root is the sole entry point.** External code addresses an internal entity **only through its root, by local identity** — e.g. `order.changeLineQuantity(lineId, qty)` (mirroring Vernon's `backlogItem.changeTaskStatus(taskId, status)`). The root looks the entity up, enforces the aggregate's invariants, mutates it, and records the domain event. Hand out *data about* internal entities (snapshots/copies), never live mutable references to them.

**Reference vs value semantics.** In pointer/reference languages (Go, Java, C#, TS, Python, PHP), model roots and internal entities as **reference/pointer types**: they are looked up, shared, and mutated in place (`*Order`, `*OrderLine`). Model value objects as **value types**: immutable, copied, replaced wholesale (`Money`). In Rust/C++, the same distinction holds via ownership/borrowing (`&mut` commands on entities; `Clone`d `Copy`-like values for VOs).

## Domain events

- Represent something **meaningful that happened** in the domain; named in the past tense and in the ubiquitous language (`OrderPlaced`, `PaymentCaptured`).
- **Immutable**, carrying the facts subscribers need (and ideally a timestamp/trace id).
- Used to coordinate **across aggregates and across bounded contexts** with eventual consistency — the mechanism behind rule of thumb #4.
- Publish from the aggregate when its state changes; other aggregates/contexts subscribe via policies.

## Repositories

- Provide **collection-like** access to aggregates, **by identity**.
- **One repository per aggregate root.** Don't create repositories for non-aggregates (entities/values are reached via their aggregate root).
- Keep the domain-facing repository as an **interface** in the domain; put the persistence implementation in infrastructure (hexagonal/ports-and-adapters). The domain must not depend on persistence details.

## Factories

- Encapsulate the **creation** of complex aggregates and enforce that newly created instances satisfy all invariants.
- Often a **factory method on the aggregate root** itself, or a dedicated factory for complex construction. Keep construction valid: never hand the caller a way to build an aggregate in an illegal state.

## Scenario walkthrough (optional sequence diagram)

Once aggregates exist, optionally **walk one key scenario** through them to pressure-test the boundaries — Vernon (IDDD) and Evans both treat this "conversation with the model" as the real validation that structure holds up under concrete use. Use `templates/sequence-diagram.puml`.

**The rule a DDD-valid sequence diagram obeys** (this is rule of thumb #4 made visual):

| Where the step goes | How to draw it |
|---|---|
| **Within one aggregate** | Ordinary synchronous call (application service → root → owned entities). All of it is one transaction. |
| **Across aggregates** | **Never a direct call.** The root publishes a **domain event** (past tense) onto the broker; the other aggregate's **policy** reacts in its *own* transaction. |

If a step in your diagram shows Aggregate A calling a method on Aggregate B, that is a **rule-#4 violation** — a direct cross-aggregate transaction. Fix it by splitting at the boundary: publish an event, subscribe a policy. Don't keep the violation and don't soften the language around it; the diagram's job is to make these slips visible.

**When to produce one:**
- For **one** representative scenario per Core aggregate (e.g. the money-making path), not exhaustively.
- When a boundary feels uncertain — the walkthrough either confirms it or exposes a needed split.
- When onboarding readers: a scenario is often faster to grasp than the class diagram alone.

It is an *aid*, never a required deliverable. Skip it for simple/supporting contexts where the class diagram already tells the whole story.

## Event sourcing & CQRS (advanced — defer)

- **Event sourcing** — persist aggregates as the sequence of domain events that produced them, rather than current state. Powerful for audit/rewind, but operationally heavier. Out of scope for v1; revisit if the domain needs it.
- **CQRS** — separate the write model (commands/aggregates) from the read model (optimized queries). Consider only when read/write workloads diverge enough to justify it.
