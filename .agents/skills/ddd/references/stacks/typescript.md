# TypeScript — DDD idioms

Loaded when the project is TypeScript / Node (`package.json` + `tsconfig.json`). Adapt to your model — don't copy verbatim.

## Tactical shapes
- **Aggregate root** — a class with private fields, a static factory that validates, and command methods like `place(...)` that enforce rules and push events to an internal `record(event)` list. No public setters.
- **Entity (internal)** — a mutable class **owned by** its root, with identity **local to the aggregate** (`lineId` unique only within its `Order`); no repository of its own; mutated only via root commands (`order.changeLineQuantity(lineId, …)`). Equality by id, never structural equality.
- **Value object** — an immutable class/object with structural equality (or branded primitive types for IDs).
- **Repository** — `interface OrderRepository { findById(id): Promise<Order | null>; save(order): Promise<void> }` in the domain; a concrete impl in an `infrastructure/` adapter.
- **Domain events** — plain typed objects; pulled after `save` and published by an application service.

## AggregateRoot base
TS has single inheritance — prefer a plain `abstract class AggregateRoot` (mirrors Java/C#/Python). Use the **mixin** below only when an aggregate must also extend another base (e.g. an ORM `Entity`), so you don't burn the single inheritance slot.

```ts
export interface DomainEvent { readonly occurredAt: Date; }

// Default: a base class. Simple, readable, strong IDE support.
export abstract class AggregateRoot {
  private _events: DomainEvent[] = [];
  protected record(e: DomainEvent): void { this._events.push(e); }
  pullEvents(): DomainEvent[] {
    const copy = [...this._events]; this._events = []; return copy;
  }
}

// Escape hatch: compose event recording onto *another* base class.
export function withAggregateRoot<Base extends new (...a: any[]) => any>(base: Base) {
  return class extends base {
    private _events: DomainEvent[] = [];
    protected record(e: DomainEvent): void { this._events.push(e); }
    pullEvents(): DomainEvent[] {
      const copy = [...this._events]; this._events = []; return copy;
    }
  };
}
```

## Worked example: ordering
```ts
// --- Value object: Money (immutable, value equality) ---
export class Money {
  private constructor(public readonly amount: number, public readonly currency: string) {}
  static of(amount: number, currency: string): Money {
    if (amount < 0) throw new Error('Money cannot be negative');
    if (!/^[A-Z]{3}$/.test(currency)) throw new Error('Invalid ISO currency');
    return new Money(Math.round(amount * 100) / 100, currency);
  }
  plus(other: Money): Money {
    if (this.currency !== other.currency) throw new Error('Currency mismatch');
    return Money.of(this.amount + other.amount, this.currency);
  }
  equals(other: Money): boolean {
    return this.amount === other.amount && this.currency === other.currency;
  }
}

// --- Entity (internal): OrderLine — owned by Order, local identity ---
// Mutable, compared by lineId: NOT a value object. Only the root mutates it.
export class OrderLine {
  private constructor(public readonly lineId: string, public readonly productId: string,
    private quantity: number, public readonly unitPrice: Money) {
    if (quantity <= 0) throw new Error('Quantity must be positive');
  }
  static create(lineId: string, productId: string, quantity: number, unitPrice: Money): OrderLine {
    return new OrderLine(lineId, productId, quantity, unitPrice);
  }
  /** Package-style guard: called only by Order.changeLineQuantity. */
  changeQuantity(qty: number): void {
    if (qty <= 0) throw new Error('Quantity must be positive');
    this.quantity = qty;
  }
  getQuantity(): number { return this.quantity; }
  lineTotal(): Money { return Money.of(this.unitPrice.amount * this.quantity, this.unitPrice.currency); }
  equals(other: OrderLine): boolean { return this.lineId === other.lineId; }
  snapshot(): OrderLineSnapshot {
    return { lineId: this.lineId, productId: this.productId, quantity: this.quantity, unitPrice: this.unitPrice };
  }
}

// Snapshot handed out to callers — data about the line, never the live entity.
export interface OrderLineSnapshot {
  readonly lineId: string; readonly productId: string;
  readonly quantity: number; readonly unitPrice: Money;
}

// --- Value object: LineItem — the facts the caller supplies to place an order ---
// Input VO, not an entity: no identity. The root builds one OrderLine per LineItem.
export class LineItem {
  private constructor(public readonly productId: string, public readonly quantity: number,
    public readonly unitPrice: Money) {}
  static of(productId: string, quantity: number, unitPrice: Money): LineItem {
    if (quantity <= 0) throw new Error('Quantity must be positive');
    return new LineItem(productId, quantity, unitPrice);
  }
}

// --- Aggregate: Order ---
enum OrderStatus { Placed, Confirmed }

export class OrderPlaced implements DomainEvent {
  readonly occurredAt = new Date();
  constructor(public readonly orderId: string) {}
}

export class OrderLineQuantityChanged implements DomainEvent {
  readonly occurredAt = new Date();
  constructor(public readonly orderId: string, public readonly lineId: string,
    public readonly newQuantity: number) {}
}

export class Order extends AggregateRoot {
  private lines: OrderLine[];
  private constructor(public readonly id: string, lines: OrderLine[], private status: OrderStatus) {
    super(); this.lines = [...lines];
  }
  static place(id: string, items: LineItem[]): Order {
    if (items.length === 0) throw new Error('Order needs at least one line');
    const lines = items.map((it, i) =>
      OrderLine.create(`${id}-${i + 1}`, it.productId, it.quantity, it.unitPrice));
    const o = new Order(id, lines, OrderStatus.Placed);
    o.record(new OrderPlaced(id));
    return o;
  }
  confirm(): void {
    if (this.status !== OrderStatus.Placed) throw new Error('Only placed orders can confirm');
    this.status = OrderStatus.Confirmed;
  }
  // Root addresses its internal entity by LOCAL identity — the sole mutator.
  changeLineQuantity(lineId: string, qty: number): void {
    if (this.status !== OrderStatus.Placed) throw new Error('Only placed orders can change lines');
    const line = this.lines.find(l => l.lineId === lineId);
    if (!line) throw new Error('Unknown line');
    line.changeQuantity(qty);
    this.record(new OrderLineQuantityChanged(this.id, lineId, qty));
  }
  lineItems(): readonly OrderLineSnapshot[] { return this.lines.map(l => l.snapshot()); }
}

// --- Domain service: PricingService (pure, stateless, domain layer) ---
// Logic spans Order + DiscountPolicy → doesn't belong on Order alone.
export interface DiscountPolicy { discountFor(order: Order): Money; }
export interface PricingService { calculateTotal(order: Order, policy: DiscountPolicy): Money; }

export class StandardPricingService implements PricingService {
  calculateTotal(order: Order, policy: DiscountPolicy): Money {
    const gross = order.lineItems().reduce<Money | null>(
      (acc, s) => {
        const t = Money.of(s.unitPrice.amount * s.quantity, s.unitPrice.currency);
        return acc ? acc.plus(t) : t;
      }, null,
    ) ?? Money.of(0, 'USD');
    return Money.of(gross.amount - policy.discountFor(order).amount, gross.currency);
  }
}

// --- Repository (port) ---
export interface OrderRepository {
  findById(id: string): Promise<Order | null>;
  save(order: Order): Promise<void>;
}
```

## Pitfalls
- Single inheritance: default to `extends AggregateRoot`; reach for the mixin only when composing with another base. Never build a deep `AggregateRoot` hierarchy.
- Internal entities are mutable classes with identity equality — never `readonly` frozen objects. Only roots get repositories.
- Mutate state inside command methods, but keep them `void` — return nothing, record events. Hand out snapshots of internal entities, never the live object.
- IDs as branded types (`type OrderId = string & { __brand: 'OrderId' }`) prevent cross-identity mix-ups.
- Use integer minor units or a money library in production — `number` here is illustrative.
