# Domain Context: {{System / Bounded Context name}}

> Primary output of the **Discover** stage. Edit freely — this is a living document.

## Problem statement
What problem does this system/context solve, and for whom? One or two paragraphs in the domain's own language.

## Domain vision
A short, evocative statement of the intended model and why it matters to the business. (What makes this a Core / Supporting / Generic subdomain?)

## Key scenarios
The most important workflows as numbered steps (these drove the Event Storming).
1. ...
2. ...

## Subdomain classification
- Type: Core | Supporting | Generic
- DDD posture: Full DDD | Light DDD | Plain CRUD

## Bounded contexts
| Context | Responsibility | Subdomain type |
|---|---|---|
| ... | ... | Core/Supporting/Generic |

## Aggregate candidates
- **{{Aggregate}}** — invariant it protects; key commands; events it emits.

## Integrations (context map summary)
- {{Context A}} → {{Context B}}: relationship (ACL / Customer–Supplier / OHS+Published Language / Conformist / Separate Ways / …)

## Service topology (only if going microservice)
Derived from the context map. Default: one bounded context = one service; co-deploying contexts is valid, never split one. Each service owns its datastore; transport comes from the map's patterns.

| Service | Hosts context(s) | Subdomain type | Datastore | Transport (in/out) | Owner |
|---|---|---|---|---|---|
| ... | ... | Core/Supporting/Generic | ... | sync REST / async events / via ACL | ... |

See [service-topology.puml](service-topology.puml) for the diagram.

## Related artifacts
- [Ubiquitous language](ubiquitous-language.md)
- [Context map](context-map.puml)
- [Service topology](service-topology.puml) — only if going microservice
- Per-context canvas: `bounded-context-{{name}}.md`
- Per-context class diagram: `class-diagram-{{name}}.puml`
