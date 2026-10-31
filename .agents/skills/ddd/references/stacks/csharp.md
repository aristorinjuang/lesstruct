# C# / .NET — DDD idioms

Loaded when the project is C# / .NET (`*.csproj` / `*.sln`). Adapt to your model — don't copy verbatim.

## Tactical shapes
- **Aggregate root** — class with private setters; constructor/static factory validates invariants; command methods mutate state and append events to a `_domainEvents` collection. Typically inherits an `AggregateRoot` base (or implements `IAggregateRoot`) exposing `AddDomainEvent` / `PullDomainEvents`.
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

// --- Value object: OrderLine ---
public record OrderLine(string ProductId, int Quantity, Money UnitPrice)
{
    public OrderLine
    {
        if (Quantity <= 0) throw new ArgumentException("quantity must be positive");
    }
    public Money LineTotal() => new(UnitPrice.Amount * Quantity, UnitPrice.Currency);
}

// --- Domain event ---
public sealed record OrderPlaced(string OrderId) : IDomainEvent
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

    public static Order Place(string id, IReadOnlyList<OrderLine> lines)
    {
        if (lines.Count == 0) throw new InvalidOperationException("order needs a line");
        var o = new Order(id, lines, Status.Placed);
        o.Register(new OrderPlaced(id));
        return o;
    }

    public void Confirm()
    {
        if (_status != Status.Placed) throw new InvalidOperationException("only placed orders can confirm");
        _status = Status.Confirmed;
    }

    public IReadOnlyList<OrderLine> Lines => _lines.AsReadOnly();
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
- `sealed` aggregates; `record` for VOs (synthesizes `Equals`/`GetHashCode`).
- EF Core maps private setters and fields via backing-field conventions — don't expose public setters just for the ORM.
- Dispatch events via an EF Core `SaveChanges` interceptor or MediatR `INotification` **after** `SaveChanges`/commit.
- Command methods return `void`; events are `Register`ed.
- Use `decimal` for money (never `float`/`double`).
