# Java — DDD idioms

Loaded when the project is Java (`pom.xml` / `build.gradle`). Vernon's own territory. Adapt to your model — don't copy verbatim.

## Tactical shapes
- **Aggregate root** — a class with package-private state, factory methods, and command methods that enforce invariants and collect domain events.
- **Value object** — an immutable class (consider `record`) with correct `equals`/`hashCode`.
- **Repository** — a domain interface (`OrderRepository`); a Spring Data / JPA adapter in infrastructure.
- **Domain events** — published via the framework's event mechanism or a domain event registry, after the transaction commits for cross-aggregate eventual consistency.

## AggregateRoot base
This is the canonical Vernon base (also referenced from `implement.md` §3). Aggregates `extends AggregateRoot`:

```java
public interface DomainEvent { Instant occurredAt(); }

public abstract class AggregateRoot {
    private final List<DomainEvent> events = new ArrayList<>();
    protected void register(DomainEvent e) { events.add(e); }
    public List<DomainEvent> pullEvents() {
        var copy = List.copyOf(events); events.clear(); return copy;
    }
}
```

## Worked example: ordering
```java
import java.math.BigDecimal;
import java.math.RoundingMode;
import java.time.Instant;
import java.util.*;

// --- Value object: Money (record → value equality synthesized) ---
public record Money(BigDecimal amount, String currency) implements DomainEvent {
    public Money {
        if (amount.signum() < 0) throw new IllegalArgumentException("negative");
        if (!currency.matches("[A-Z]{3}")) throw new IllegalArgumentException("bad currency");
        amount = amount.setScale(2, RoundingMode.HALF_UP);
    }
    public Money plus(Money other) {
        if (!currency.equals(other.currency)) throw new IllegalArgumentException("mismatch");
        return new Money(amount.add(other.amount), currency);
    }
}

// --- Value object: OrderLine ---
public record OrderLine(String productId, int quantity, Money unitPrice) {
    public OrderLine {
        if (quantity <= 0) throw new IllegalArgumentException("quantity must be positive");
    }
    public Money lineTotal() {
        return new Money(unitPrice.amount().multiply(BigDecimal.valueOf(quantity)), unitPrice.currency());
    }
}

// --- Domain event ---
public record OrderPlaced(String orderId, Instant occurredAt) implements DomainEvent {
    public OrderPlaced(String orderId) { this(orderId, Instant.now()); }
}

// --- Aggregate: Order ---
public final class Order extends AggregateRoot {
    public enum Status { PLACED, CONFIRMED }

    private final String id;
    private final List<OrderLine> lines;
    private Status status;

    private Order(String id, List<OrderLine> lines, Status status) {
        this.id = id; this.lines = new ArrayList<>(lines); this.status = status;
    }

    public static Order place(String id, List<OrderLine> lines) {
        if (lines.isEmpty()) throw new IllegalStateException("order needs a line");
        Order o = new Order(id, lines, Status.PLACED);
        o.register(new OrderPlaced(id));
        return o;
    }

    public void confirm() {
        if (status != Status.PLACED) throw new IllegalStateException("only placed orders can confirm");
        this.status = Status.CONFIRMED;
    }

    public String id() { return id; }
    public List<OrderLine> lines() { return List.copyOf(lines); }
}

// --- Domain service: PricingService (pure, stateless, domain layer) ---
public interface DiscountPolicy { Money discountFor(Order order); }
public interface PricingService { Money calculateTotal(Order order, DiscountPolicy policy); }

public final class StandardPricingService implements PricingService {
    @Override public Money calculateTotal(Order order, DiscountPolicy policy) {
        Money gross = order.lines().stream()
            .map(OrderLine::lineTotal)
            .reduce(Money::plus)
            .orElseThrow();
        return new Money(gross.amount().subtract(policy.discountFor(order).amount()), gross.currency());
    }
}

// --- Repository (port) ---
public interface OrderRepository {
    Optional<Order> findById(String id);
    void save(Order order);
}
```

## Pitfalls
- Mark aggregates `final`; keep constructors `private`, expose static factories; invariants enforced before any state is assigned.
- `record` gives `equals`/`hashCode` for VOs for free — prefer it.
- Don't put JPA annotations on the domain aggregate; map in infrastructure via a separate persistence representation (or accept the coupling as an explicit tradeoff).
- Command methods return `void`; events are `register`ed, never returned.
- Publish events only **after the transaction commits** (Spring `@TransactionalEventListener(AFTER_COMMIT)`), or via a transactional outbox.
