# ddd — Domain-Driven Design, end-to-end

[![skills.sh](https://skills.sh/b/aristorinjuang/ddd-agent-skill)](https://skills.sh/aristorinjuang/ddd-agent-skill)

An [Agent Skill](https://agentskills.io/) that implements Domain-Driven Design end-to-end — and, just as importantly, decides **whether a project warrants DDD at all** before writing a single aggregate.

It follows Vernon's *Implementing Domain-Driven Design* as collaborative modeling: it asks probing questions, proposes a model, and iterates with you rather than dumping artifacts.

## When to use

- Designing a new system or modeling a domain
- Running Event Storming and carving bounded contexts
- Deriving microservice boundaries from a context map
- Designing aggregates, entities, value objects, domain events, repositories
- Refactoring an existing (anemic) codebase toward DDD
- Auditing code against a domain model

## Install

```bash
npx skills add aristorinjuang/ddd-agent-skill
```

Works with any Agent Skills-compatible agent (Claude Code, Cursor, Codex, Copilot, Windsurf, Gemini, OpenCode, Cline, Amp, Goose, and others).

## How it works

The skill runs a staged workflow and won't skip the worthiness gate:

1. **Assess** — greenfield vs brownfield, detect stack and anemic/rich model.
2. **Gate** — classify the subdomain (Core / Supporting / Generic) and recommend **Full DDD**, **Light DDD**, or **plain CRUD** (it will tell you plainly when DDD is overkill).
3. **Discover** (strategic) — Event Storming, ubiquitous language, context map, optional service topology.
4. **Model** (tactical) — aggregates, entities, value objects, domain events, repositories; one implementation-ready class diagram per bounded context.
5. **Implement** — idiomatic DDD code adapted to the detected stack.
6. **Audit** (optional) — score existing code against a domain model.

Each stage produces living artifacts (Markdown + PlantUML) from bundled templates that you can correct as you go.

## Skill structure

```
SKILL.md                      workflow + frontmatter (name, description)
references/
  assess.md                   detection, the DDD-worthiness gate, audit scorecard
  strategic-design.md         subdomains, bounded contexts, context mapping, event storming
  tactical-patterns.md        aggregates, entities, value objects, events, repositories, factories
  implement.md                detect & adapt codegen
  stacks/                     per-language implementation guidance (TS, Py, Go, Rust, Java, C#, PHP, C++)
templates/
  domain-context.md  ubiquitous-language.md  context-map.puml  service-topology.puml
  bounded-context.md  class-diagram.puml     sequence-diagram.puml
```

## License

MIT © Aristo Rinjuang
