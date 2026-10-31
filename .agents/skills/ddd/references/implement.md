# Implement — detect & adapt

Load this reference for the **Implement** stage. The skill generates **idiomatic** DDD code for the project's actual stack — never a generic template bolted onto the wrong language.

---

## 1. Detect the stack

Read the project's manifest files to identify language + framework + style:

| Manifest(s) | Likely stack | Stack file |
|---|---|---|
| `package.json` (+ `tsconfig.json`) | TypeScript / Node (NestJS, Express, …) | `references/stacks/typescript.md` |
| `pom.xml` / `build.gradle(.kts)` | Java (Spring, Quarkus) | `references/stacks/java.md` |
| `*.csproj` / `*.sln` | C# / .NET | `references/stacks/csharp.md` |
| `go.mod` | Go | `references/stacks/go.md` |
| `requirements.txt` / `pyproject.toml` | Python | `references/stacks/python.md` |
| `Cargo.toml` | Rust | `references/stacks/rust.md` |
| `CMakeLists.txt` / `meson.build` / `BUILD` (Bazel) | C++ (Crow, drogon, userver) / C | `references/stacks/cpp.md` |
| `composer.json` | PHP (Symfony, Laravel) | `references/stacks/php.md` |

Also detect the **architecture style** present (layered, hexagonal/ports-and-adapters, clean architecture) so generated code fits in rather than fights it. On **greenfield** with no manifest, propose a default and **confirm with the user** before generating.

## 2. Map tactical patterns to idioms

Whatever the stack, the same DDD shapes apply — map them to idiomatic constructs:

- **Aggregate root** — a class/struct that holds state, enforces invariants in constructors and command methods, and emits domain events. No public setters; mutations only through intention-revealing methods.
- **Value object** — an immutable type with value equality.
- **Domain event** — a small immutable record of "what happened," published when the aggregate changes.
- **Repository** — a domain interface (port) with a concrete implementation in infrastructure (adapter).
- **Domain isolation** — keep the domain package free of framework/database/HTTP concerns (hexagonal). Dependencies point inward toward the domain.

### Per-stack realization

The shapes above are universal; the idiomatic code differs by language. After detecting the manifest in §1, load the matching **Stack file** for that language's tactical idioms, its `AggregateRoot` base, and a worked example (value object, aggregate, **domain service**, repository, events). On greenfield, propose a stack, **confirm with the user**, then load its file.

## 3. Domain events at runtime

The class diagram shows *which* events an aggregate is the source of (e.g. `Order` → `OrderPlaced`). It deliberately does **not** show the publishing plumbing — that is an **application-layer** concern. The mechanics are constant across stacks; only the realization differs.

### Record on the aggregate, publish from the application service
- The **aggregate records** events onto itself when its state changes; command methods stay `void`. The domain never touches a broker.
- The **application service** pulls the recorded events after `save` and hands them to a `DomainEventPublisher` (a port; impl in infrastructure).

```java
// Domain layer — reusable base (Vernon, IDDD).
// In TS prefer a base class; use a mixin only when composing with another base (e.g. an ORM Entity). In Rust, a trait + field; in Go, an embedded struct. Java/C#/C++ keep the classic base class; plain C embeds a struct + vtable.
public abstract class AggregateRoot {
    private final List<DomainEvent> events = new ArrayList<>();
    protected void register(DomainEvent e) { events.add(e); }
    public List<DomainEvent> pullEvents() {
        var copy = List.copyOf(events); events.clear(); return copy;
    }
}

public final class Order extends AggregateRoot {
    public void confirm() {
        if (status != OrderStatus.PLACED) throw new IllegalStateException();
        this.status = OrderStatus.CONFIRMED;
        register(new OrderConfirmed(id, Instant.now()));   // record, don't return
    }
}

// Application layer — the use case (NOT a domain service)
public final class ConfirmOrderUseCase {
    public void execute(OrderId id) {
        Order order = repo.findById(id).orElseThrow();
        order.confirm();                          // 1) aggregate records event
        repo.save(order);                         // 2) persist one aggregate
        publisher.publishAll(order.pullEvents()); // 3) publish
    }
}
```

### Publish reliably — transactional outbox
A crash between `save` and `publish` loses the event and breaks eventual consistency. Publish in the **same transaction** as the save by writing events to an **outbox** table and letting a relay forward them to the broker. This honors Vernon's rule: *one transaction = one aggregate + its consequences*.

### Who consumes
- **Other aggregates in the same context**, via a *policy* — "When `OrderPlaced`, reserve stock." Each reaction loads its own aggregate and commands it in its **own transaction** → eventual consistency across aggregates (rule #4); never a cross-aggregate transaction.
- **Other bounded contexts** — the domain event is translated to an **integration event** (Published Language / ACL) and put on a broker.
- **Read models & side-effects** — CQRS projections, email, notifications.

## 4. Generation discipline

- Generate **inside one bounded context at a time**, following the agreed model and ubiquitous language.
- Keep file/folder layout consistent with the project's existing conventions.
- After generating, optionally hand back to the **Audit** step to score the result against `domain-context.md`.
