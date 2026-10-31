# PHP — DDD idioms

Loaded when the project is PHP (`composer.json`). Covers **Symfony**, **Laravel**, and **framework-agnostic** PHP. Adapt to your model — don't copy verbatim.

## Tactical shapes
- **Aggregate root** — PHP class with private/protected properties, named constructors (static factory methods like `Order::place(...)`), and command methods enforcing invariants and registering events. Reuse via a `RecordsDomainEvents` trait or base class providing `record()` / `pullEvents()`. PHP 8.1+ `readonly` properties (or `readonly class` in 8.2+) for safe construction.
  - *Symfony* — Doctrine-mapped aggregate; layers across `src/Domain/`, `src/Application/`, `src/Infrastructure/` (often per-bundle).
  - *Laravel* — either a plain PHP aggregate persisted via a repository (more DDD-pure) or an Eloquent model re-purposed as the aggregate; events via `Illuminate\Foundation\Events\Dispatchable` or Spatie's `laravel-event-sourcing`.
  - *Framework-agnostic* — plain PSR-4 classes (`App\Domain\Order`); persistence entirely behind the repository interface.
- **Value object** — `final readonly class` (PHP 8.2+) or class with `readonly` props, implementing `equals(self): bool`; single-value VOs as a typed wrapper with `__toString()` and an `of()` factory.
- **Repository** — domain interface `OrderRepository` in the domain layer.
  - *Symfony* — Doctrine ORM repository in infrastructure.
  - *Laravel* — repository backed by Eloquent queries or the `DB` facade.
  - *Framework-agnostic* — a PDO / DBAL adapter behind the interface.
- **Domain events** — plain immutable classes implementing a `DomainEvent` contract; collected on the aggregate, dispatched by the application service after the unit-of-work flushes.
  - *Symfony* — Event Dispatcher or Messenger bus (Messenger doubles as an outbox when transports are async).
  - *Laravel* — Laravel events on a queued connection, or an explicit outbox job for reliability.
  - *Framework-agnostic* — a PSR-14 event listener provider / a small in-process dispatcher.

## AggregateRoot base
A `trait` (PHP's `RecordsDomainEvents`) gives event recording without forcing a single base class:

```php
<?php
declare(strict_types=1);

namespace App\Domain;

interface DomainEvent
{
    public function occurredAt(): \DateTimeImmutable;
}

trait RecordsDomainEvents
{
    /** @var DomainEvent[] */
    private array $events = [];

    protected function record(DomainEvent $e): void { $this->events[] = $e; }

    /** @return DomainEvent[] */
    public function pullEvents(): array {
        $copy = $this->events;
        $this->events = [];
        return $copy;
    }
}
```

## Worked example: ordering
```php
<?php
declare(strict_types=1);

namespace App\Domain\Order;

// --- Value object: Money (final readonly class, PHP 8.2+) ---
final readonly class Money
{
    public function __construct(
        public int $amount,         // minor units in production (or use brick/money)
        public string $currency,
    ) {
        if ($amount < 0) throw new \InvalidArgumentException('negative');
        if (!preg_match('/^[A-Z]{3}$/', $currency)) throw new \InvalidArgumentException('bad currency');
    }

    public function plus(self $other): self {
        if ($this->currency !== $other->currency) throw new \InvalidArgumentException('mismatch');
        return new self($this->amount + $other->amount, $this->currency);
    }

    public function minus(self $other): self {
        if ($this->currency !== $other->currency) throw new \InvalidArgumentException('mismatch');
        return new self($this->amount - $other->amount, $this->currency);
    }

    public function equals(self $other): bool {
        return $this->amount === $other->amount && $this->currency === $other->currency;
    }
}

// --- Value object: OrderLine ---
final readonly class OrderLine
{
    public function __construct(
        public string $productId,
        public int $quantity,
        public Money $unitPrice,
    ) {
        if ($quantity <= 0) throw new \InvalidArgumentException('quantity must be positive');
    }

    public function lineTotal(): Money {
        return new Money($this->unitPrice->amount * $this->quantity, $this->unitPrice->currency);
    }
}

// --- Domain event ---
final readonly class OrderPlaced implements DomainEvent
{
    public function __construct(public string $orderId) {}

    public function occurredAt(): \DateTimeImmutable {
        return new \DateTimeImmutable();
    }
}

// --- Aggregate: Order ---
enum OrderStatus: string
{
    case Placed = 'placed';
    case Confirmed = 'confirmed';
}

final class Order
{
    use RecordsDomainEvents;

    /** @var list<OrderLine> */
    private array $lines;
    private OrderStatus $status;

    /** @param list<OrderLine> $lines */
    private function __construct(
        public readonly string $id,
        array $lines,
    ) {
        $this->lines = array_values($lines);
        $this->status = OrderStatus::Placed;
    }

    /** Named constructor / factory. @param list<OrderLine> $lines */
    public static function place(string $id, array $lines): self {
        if (count($lines) === 0) throw new \DomainException('order needs a line');
        $o = new self($id, $lines);
        $o->record(new OrderPlaced($id));
        return $o;
    }

    public function confirm(): void {
        if ($this->status !== OrderStatus::Placed) throw new \DomainException('only placed orders can confirm');
        $this->status = OrderStatus::Confirmed;
    }

    /** @return list<OrderLine> */
    public function lines(): array { return $this->lines; }
}

// --- Domain service: PricingService (pure, stateless, domain layer) ---
interface DiscountPolicy { public function discountFor(Order $order): Money; }

interface PricingService { public function calculateTotal(Order $order, DiscountPolicy $policy): Money; }

final class StandardPricingService implements PricingService
{
    public function calculateTotal(Order $order, DiscountPolicy $policy): Money {
        $gross = null;
        foreach ($order->lines() as $line) {
            $t = $line->lineTotal();
            $gross = $gross === null ? $t : $gross->plus($t);
        }
        $gross ??= new Money(0, 'USD');
        return $gross->minus($policy->discountFor($order));
    }
}

// --- Repository (port) — domain interface ---
interface OrderRepository
{
    public function findById(string $id): ?Order;
    public function save(Order $order): void;
}
```

## Framework flavors — where the plumbing lives

```php
// --- Symfony adapter (Doctrine ORM) — src/Infrastructure/OrderRepository/DoctrineOrderRepository.php ---
final class DoctrineOrderRepository implements OrderRepository
{
    public function __construct(private \Doctrine\ORM\EntityManagerInterface $em) {}

    public function findById(string $id): ?Order {
        return $this->em->find(Order::class, $id);
    }

    public function save(Order $order): void {
        $this->em->persist($order);
        $this->em->flush();
        // Dispatch Order::pullEvents() via Symfony Messenger (async transport = outbox).
    }
}

// --- Laravel adapter (Eloquent/DB) — app/Infrastructure/OrderRepository/EloquentOrderRepository.php ---
final class EloquentOrderRepository implements OrderRepository
{
    public function __construct(private \Illuminate\Database\ConnectionInterface $db) {}

    public function findById(string $id): ?Order {
        $row = $this->db->table('orders')->where('id', $id)->first();
        return $row ? $this->hydrate($row) : null;     // hydrate → Order::place(...)
    }

    public function save(Order $order): void {
        $this->db->transaction(function () use ($order) {
            $this->db->table('orders')->updateOrInsert(/* ... */);
            event(new \App\Events\OrderPersisted($order));   // queued = outbox
        });
    }
}

// --- Framework-agnostic adapter (PDO/DBAL) ---
final class PdoOrderRepository implements OrderRepository
{
    public function __construct(private \PDO $pdo) {}
    public function findById(string $id): ?Order { /* SELECT → hydrate via Order::place(...) */ }
    public function save(Order $order): void { /* INSERT/UPDATE in a transaction */ }
}
```

## Pitfalls
- PHP 8.1+ `readonly` properties / 8.2+ `readonly class` for VOs; PHP 8.1+ `enum` for status.
- Store money as integer **minor units** in production (or `brick/money`); the `int $amount` here is illustrative.
- Named constructors (static factory methods); keep the real constructor `private` to enforce invariants at creation.
- *Symfony*: keep Doctrine mapping in infrastructure; don't leak ORM annotations into the domain aggregate (a deliberate tradeoff if you do).
- *Laravel*: prefer a plain aggregate + repository over Eloquent active record for DDD purity; use events on a **queued** connection for reliability.
- Dispatch events **after** the transaction commits/flushes (Messenger async transport or a queued listener acts as the outbox).
