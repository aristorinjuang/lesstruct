# C# / .NET — DDD idioms

Loaded when the project is C# / .NET (`*.csproj` / `*.sln`). Adapt to your model — don't copy verbatim.

## Tactical shapes
- **Aggregate root** — class with private setters; constructor/static factory validates invariants; command methods mutate state and append events to a `_domainEvents` collection. Typically inherits an `AggregateRoot` base (or implements `IAggregateRoot`) exposing `AddDomainEvent` / `PullDomainEvents`.
- **Entity (internal)** — a mutable `sealed class` **owned by** its root, with identity **local to the aggregate** (`LineId` unique only within its `Order`); no repository of its own; mutated only via root commands (`order.ChangeLineQuantity(lineId, …)`). Equality by id — never a `record` (records synthesize value equality).
- **Value object** — immutable class/record overriding equality members, or a `ValueObject` base class comparing equality components. C# 9+ `record` (and `record struct`) is ideal — value equality is synthesized.
- **Repository** — domain interface `IOrderRepository { Task<Order?> FindByIdAsync(...); Task SaveAsync(...); }`; an EF Core adapter (`DbContext` + `DbSet<Order>`) in `infrastructure/`.
- **Domain events** — collected on the aggregate; dispatched by an EF Core `SaveChanges` interceptor, a unit-of-work, or MediatR (`INotification`) after the transaction commits.

## AggregateRoot base
```csharp
public interface IDomainEvent { DateTime OccurredAt { get; } }

public abstract class AggregateRoot
{
    private readonly List<IDomainEvent> _events = new();
    protected void Register(IDomainEvent e) => _events.Add(e);
    public IReadOnlyList<IDomainEvent> PullEvents()
    {
        var copy = _events.ToList(); _events.Clear(); return copy;
    }
}
```

## Worked example: ordering
```csharp
using System;
using System.Collections.Generic;
using System.Linq;
using System.Text.RegularExpressions;
using System.Threading;
using System.Threading.Tasks;

// --- Value object: Money (record → value equality synthesized) ---
public record Money(decimal Amount, string Currency)
{
    public Money
    {
        if (Amount < 0) throw new ArgumentException("negative");
        if (!Regex.IsMatch(Currency, "^[A-Z]{3}$")) throw new ArgumentException("bad currency");
        Amount = Math.Round(Amount, 2);
    }
    public Money Plus(Money other)
    {
        if (Currency != other.Currency) throw new ArgumentException("mismatch");
        return this with { Amount = Amount + other.Amount };
    }
}

// --- Entity (internal): OrderLine — owned by Order, local identity ---
// Sealed class, not a record: entities mutate in place and compare by id.
public sealed class OrderLine
{
    public string LineId { get; }
    public string ProductId { get; }
    public int Quantity { get; private set; }
    public Money UnitPrice { get; }

    public OrderLine(string lineId, string productId, int quantity, Money unitPrice)
    {
        if (quantity <= 0) throw new ArgumentException("quantity must be positive");
        LineId = lineId; ProductId = productId; Quantity = quantity; UnitPrice = unitPrice;
    }
    internal void ChangeQuantity(int qty)
    {
        if (qty <= 0) throw new ArgumentException("quantity must be positive");
        Quantity = qty;
    }
    public Money LineTotal() => new(UnitPrice.Amount * Quantity, UnitPrice.Currency);
    public override bool Equals(object? o) => o is OrderLine other && LineId == other.LineId;
    public override int GetHashCode() => LineId.GetHashCode();

    // Snapshot handed out to callers — data about the line, never the live entity.
    public sealed record Snapshot(string LineId, string ProductId, int Quantity, Money UnitPrice)
    {
        public Money LineTotal() => new(UnitPrice.Amount * Quantity, UnitPrice.Currency);
    }
    public Snapshot ToSnapshot() => new(LineId, ProductId, Quantity, UnitPrice);
}

// --- Value object: LineItem — the facts the caller supplies to place an order ---
// Input VO, not an entity: no identity. The root builds one OrderLine per LineItem.
public sealed record LineItem(string ProductId, int Quantity, Money UnitPrice)
{
    public LineItem
    {
        if (Quantity <= 0) throw new ArgumentException("quantity must be positive");
    }
}

// --- Domain events ---
public sealed record OrderPlaced(string OrderId) : IDomainEvent
{
    public DateTime OccurredAt { get; } = DateTime.UtcNow;
}
public sealed record OrderLineQuantityChanged(string OrderId, string LineId, int NewQuantity) : IDomainEvent
{
    public DateTime OccurredAt { get; } = DateTime.UtcNow;
}

// --- Aggregate: Order ---
public sealed class Order : AggregateRoot
{
    public enum Status { Placed, Confirmed }

    public string Id { get; }
    private readonly List<OrderLine> _lines;
    private Status _status;

    private Order(string id, IEnumerable<OrderLine> lines, Status status)
    {
        Id = id; _lines = lines.ToList(); _status = status;
    }

    public static Order Place(string id, IReadOnlyList<LineItem> items)
    {
        if (items.Count == 0) throw new InvalidOperationException("order needs a line");
        var lines = items.Select((it, i) =>
            new OrderLine($"{id}-{i + 1}", it.ProductId, it.Quantity, it.UnitPrice)).ToList();
        var o = new Order(id, lines, Status.Placed);
        o.Register(new OrderPlaced(id));
        return o;
    }

    public void Confirm()
    {
        if (_status != Status.Placed) throw new InvalidOperationException("only placed orders can confirm");
        _status = Status.Confirmed;
    }

    // Root addresses its internal entity by LOCAL identity — the sole mutator.
    public void ChangeLineQuantity(string lineId, int qty)
    {
        if (_status != Status.Placed) throw new InvalidOperationException("only placed orders can change lines");
        var line = _lines.FirstOrDefault(l => l.LineId == lineId)
            ?? throw new ArgumentException("unknown line");
        line.ChangeQuantity(qty);
        Register(new OrderLineQuantityChanged(Id, lineId, qty));
    }

    public IReadOnlyList<OrderLine.Snapshot> Lines => _lines.Select(l => l.ToSnapshot()).ToList().AsReadOnly();
}

// --- Domain service: PricingService (pure, stateless, domain layer) ---
public interface IDiscountPolicy { Money DiscountFor(Order order); }
public interface IPricingService { Money CalculateTotal(Order order, IDiscountPolicy policy); }

public sealed class StandardPricingService : IPricingService
{
    public Money CalculateTotal(Order order, IDiscountPolicy policy)
    {
        var gross = order.Lines.Aggregate((Money?)null,
            (acc, l) => acc is null ? l.LineTotal() : acc.Plus(l.LineTotal()))
            ?? throw new InvalidOperationException();
        return gross with { Amount = gross.Amount - policy.DiscountFor(order).Amount };
    }
}

// --- Repository (port) ---
public interface IOrderRepository
{
    Task<Order?> FindByIdAsync(string id, CancellationToken ct = default);
    Task SaveAsync(Order order, CancellationToken ct = default);
}
```

## Pitfalls
- `sealed` aggregates and entities; `record` for VOs (synthesizes `Equals`/`GetHashCode`) — never `record` for entities (value equality is wrong for identity).
- Only roots get repositories — never create a repository for an internal entity.
- EF Core maps private setters and fields via backing-field conventions — don't expose public setters just for the ORM.
- Dispatch events via an EF Core `SaveChanges` interceptor or MediatR `INotification` **after** `SaveChanges`/commit.
- Command methods return `void`; events are `Register`ed. Hand out snapshots of internal entities, never the live entity.
- Use `decimal` for money (never `float`/`double`).
