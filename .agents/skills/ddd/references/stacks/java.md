# Java — DDD idioms

Loaded when the project is Java (`pom.xml` / `build.gradle`). Vernon's own territory. Adapt to your model — don't copy verbatim.

## Tactical shapes
- **Aggregate root** — a class with package-private state, factory methods, and command methods that enforce invariants and collect domain events.
- **Entity (internal)** — a mutable class **owned by** its root, with identity **local to the aggregate** (`lineId` unique only within its `Order`); no repository of its own; mutated only via root commands (`order.changeLineQuantity(lineId, …)`). Equality by id. Always a reference type, never a `record`.
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
public record Money(BigDecimal amount, String currency) {
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

// --- Entity (internal): OrderLine — owned by Order, local identity ---
// Not a record: entities mutate in place and compare by id, not by value.
public final class OrderLine {
    private final String lineId;   // unique only within its Order
    private final String productId;
    private int quantity;
    private final Money unitPrice;

    public OrderLine(String lineId, String productId, int quantity, Money unitPrice) {
        if (quantity <= 0) throw new IllegalArgumentException("quantity must be positive");
        this.lineId = lineId; this.productId = productId;
        this.quantity = quantity; this.unitPrice = unitPrice;
    }
    void changeQuantity(int qty) {   // package-private: only the root calls this
        if (qty <= 0) throw new IllegalArgumentException("quantity must be positive");
        this.quantity = qty;
    }
    public Money lineTotal() {
        return new Money(unitPrice.amount().multiply(BigDecimal.valueOf(quantity)), unitPrice.currency());
    }
    public String lineId() { return lineId; }
    public String productId() { return productId; }
    public int quantity() { return quantity; }
    public Money unitPrice() { return unitPrice; }
    @Override public boolean equals(Object o) {
        return o instanceof OrderLine other && lineId.equals(other.lineId);
    }
    @Override public int hashCode() { return lineId.hashCode(); }

    // Snapshot handed out to callers — data about the line, never the live entity.
    public record Snapshot(String lineId, String productId, int quantity, Money unitPrice) {
        public Money lineTotal() {
            return new Money(unitPrice.amount().multiply(BigDecimal.valueOf(quantity)), unitPrice.currency());
        }
    }
    public Snapshot snapshot() {
        return new Snapshot(lineId, productId, quantity, unitPrice);
    }
}

// --- Value object: Item — the facts the caller supplies to place an order ---
// Input VO, not an entity: no identity. The root builds one OrderLine per Item.
public record Item(String productId, int quantity, Money unitPrice) {
    public Item {
        if (quantity <= 0) throw new IllegalArgumentException("quantity must be positive");
    }
}

// --- Domain events ---
public record OrderPlaced(String orderId, Instant occurredAt) implements DomainEvent {
    public OrderPlaced(String orderId) { this(orderId, Instant.now()); }
}
public record OrderLineQuantityChanged(String orderId, String lineId, int newQuantity, Instant occurredAt) implements DomainEvent {
    public OrderLineQuantityChanged(String orderId, String lineId, int newQuantity) {
        this(orderId, lineId, newQuantity, Instant.now());
    }
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

    public static Order place(String id, List<Item> items) {
        if (items.isEmpty()) throw new IllegalStateException("order needs a line");
        List<OrderLine> lines = new ArrayList<>();
        for (int i = 0; i < items.size(); i++) {
            Item it = items.get(i);
            lines.add(new OrderLine(id + "-" + (i + 1), it.productId(), it.quantity(), it.unitPrice()));
        }
        Order o = new Order(id, lines, Status.PLACED);
        o.register(new OrderPlaced(id));
        return o;
    }

    public void confirm() {
        if (status != Status.PLACED) throw new IllegalStateException("only placed orders can confirm");
        this.status = Status.CONFIRMED;
    }

    // Root addresses its internal entity by LOCAL identity — the sole mutator.
    public void changeLineQuantity(String lineId, int qty) {
        if (status != Status.PLACED) throw new IllegalStateException("only placed orders can change lines");
        OrderLine line = lines.stream().filter(l -> l.lineId().equals(lineId))
            .findFirst().orElseThrow(() -> new IllegalArgumentException("unknown line"));
        line.changeQuantity(qty);
        register(new OrderLineQuantityChanged(id, lineId, qty));
    }

    public String id() { return id; }
    // Snapshots out — never the live OrderLine.
    public List<OrderLine.Snapshot> lines() {
        return lines.stream().map(OrderLine::snapshot).toList();
    }
}

// --- Domain service: PricingService (pure, stateless, domain layer) ---
public interface DiscountPolicy { Money discountFor(Order order); }
public interface PricingService { Money calculateTotal(Order order, DiscountPolicy policy); }

public final class StandardPricingService implements PricingService {
    @Override public Money calculateTotal(Order order, DiscountPolicy policy) {
        Money gross = order.lines().stream()
            .map(OrderLine.Snapshot::lineTotal)
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
- Internal entities are **classes** (never `record`): mutable, equality by local id. `record` gives `equals`/`hashCode` for VOs for free — prefer it for VOs only.
- Only roots get repositories — never create a repository for an internal entity.
- Don't put JPA annotations on the domain aggregate; map in infrastructure via a separate persistence representation (or accept the coupling as an explicit tradeoff).
- Command methods return `void`; events are `register`ed, never returned. Hand out snapshots of internal entities, never the live entity.
- Publish events only **after the transaction commits** (Spring `@TransactionalEventListener(AFTER_COMMIT)`), or via a transactional outbox.
