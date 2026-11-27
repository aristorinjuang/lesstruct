# Rust — DDD idioms

Loaded when the project is Rust (`Cargo.toml`). Adapt to your model — don't copy verbatim.

## Tactical shapes
- **Aggregate root** — struct with private fields; associated functions (`Order::place`, `pub fn confirm(&mut self)`) validate and mutate `self`, pushing events into a `Vec<DomainEvent>` field. Reuse via a `trait AggregateRoot { fn record_event(&mut self, ...); fn pull_events(&mut self) -> Vec<...>; }` — traits + composition stand in for inheritance (no default field, but the trait is the contract).
- **Entity (internal)** — a mutable struct **owned by** its root, with identity **local to the aggregate** (`line_id` unique only within its `Order`); no repository of its own; mutated only via root commands (`order.change_line_quantity(line_id, …)`) using `iter_mut()`. Equality by id only — do **not** `derive(PartialEq)` on all fields (that would be value equality, i.e. VO semantics).
- **Value object** — immutable struct with `#[derive(PartialEq, Eq, Hash, Clone)]` for value equality; newtype wrappers (`struct OrderId(String);`) for branded primitives.
- **Repository** — domain trait with async fns: `trait OrderRepository { async fn find_by_id(&self, id) -> Result<Option<Order>, RepoError>; async fn save(&self, &Order) -> Result<(), RepoError>; }`; a sqlx/sea-orm adapter in `infrastructure/`.
- **Domain events** — enums or structs collected on the aggregate; pulled and published by the application service after `save`. No runtime inheritance; trait-based polymorphism throughout.

## AggregateRoot base
No inheritance, no default fields → define a **trait** for the contract; each aggregate implements it over its own `events: Vec<DomainEvent>` field:

```rust
pub trait AggregateRoot {
    fn record_event(&mut self, e: DomainEvent);
    fn pull_events(&mut self) -> Vec<DomainEvent>;
}
```

## Worked example: ordering
```rust
use rust_decimal::Decimal;   // crate: rust_decimal
use std::fmt;

// --- Domain events ---
#[derive(Clone, Debug)]
pub enum DomainEvent {
    OrderPlaced { order_id: String, at: chrono::DateTime<chrono::Utc> },
    OrderLineQuantityChanged { order_id: String, line_id: String, new_quantity: u32 },
}

// --- Value object: Money (derived equality) ---
#[derive(Clone, Debug, PartialEq, Eq, Hash)]
pub struct Money {
    amount: Decimal,
    currency: String,
}

impl Money {
    pub fn new(amount: Decimal, currency: &str) -> Result<Self, String> {
        if amount < Decimal::ZERO { return Err("negative".into()); }
        if currency.len() != 3 || !currency.chars().all(|c| c.is_ascii_uppercase()) {
            return Err("bad currency".into());
        }
        Ok(Self { amount, currency: currency.into() })
    }
    pub fn amount(&self) -> Decimal { self.amount }
    pub fn currency(&self) -> &str { &self.currency }
    pub fn plus(&self, other: &Money) -> Result<Money, String> {
        if self.currency != other.currency { return Err("mismatch".into()); }
        Ok(Money { amount: self.amount + other.amount, currency: self.currency.clone() })
    }
}

// --- Entity (internal): OrderLine — owned by Order, local identity ---
// Mutable, compared by line_id only: NOT value equality. Only the root mutates it.
#[derive(Clone, Debug)]
pub struct OrderLine {
    line_id: String,   // unique only within its Order
    product_id: String,
    quantity: u32,
    unit_price: Money,
}

impl PartialEq for OrderLine {
    fn eq(&self, other: &Self) -> bool { self.line_id == other.line_id }
}
impl Eq for OrderLine {}

impl OrderLine {
    pub fn new(line_id: String, product_id: String, quantity: u32, unit_price: Money) -> Result<Self, String> {
        if quantity == 0 { return Err("quantity must be positive".into()); }
        Ok(Self { line_id, product_id, quantity, unit_price })
    }
    fn change_quantity(&mut self, qty: u32) -> Result<(), String> {
        if qty == 0 { return Err("quantity must be positive".into()); }
        self.quantity = qty;
        Ok(())
    }
    pub fn line_id(&self) -> &str { &self.line_id }
    pub fn line_total(&self) -> Money {
        Money {
            amount: self.unit_price.amount * Decimal::from(self.quantity),
            currency: self.unit_price.currency.clone(),
        }
    }
    // Snapshot handed out to callers — data about the line, never &mut to it.
    pub fn snapshot(&self) -> LineSnapshot {
        LineSnapshot { line_id: self.line_id.clone(), product_id: self.product_id.clone(),
            quantity: self.quantity, unit_price: self.unit_price.clone() }
    }
}

// --- Value object: LineItem — the facts the caller supplies to place an order ---
// Input VO, not an entity: no identity. The root builds one OrderLine per LineItem.
#[derive(Clone, Debug)]
pub struct LineItem {
    pub product_id: String,
    pub quantity: u32,
    pub unit_price: Money,
}

impl LineItem {
    pub fn new(product_id: String, quantity: u32, unit_price: Money) -> Result<Self, String> {
        if quantity == 0 { return Err("quantity must be positive".into()); }
        Ok(Self { product_id, quantity, unit_price })
    }
}

#[derive(Clone, Debug)]
pub struct LineSnapshot {
    pub line_id: String,
    pub product_id: String,
    pub quantity: u32,
    pub unit_price: Money,
}
impl LineSnapshot {
    pub fn line_total(&self) -> Money {
        Money { amount: self.unit_price.amount * Decimal::from(self.quantity),
            currency: self.unit_price.currency.clone() }
    }
}

// --- Aggregate: Order ---
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Status { Placed, Confirmed }

#[derive(Clone, Debug)]
pub struct Order {
    id: String,
    lines: Vec<OrderLine>,
    status: Status,
    events: Vec<DomainEvent>,
}

impl AggregateRoot for Order {
    fn record_event(&mut self, e: DomainEvent) { self.events.push(e); }
    fn pull_events(&mut self) -> Vec<DomainEvent> {
        let out = self.events.clone();
        self.events.clear();
        out
    }
}

impl Order {
    pub fn place(id: String, items: Vec<LineItem>) -> Result<Self, String> {
        if items.is_empty() { return Err("order needs a line".into()); }
        let mut lines = Vec::with_capacity(items.len());
        for (i, it) in items.into_iter().enumerate() {
            lines.push(OrderLine::new(format!("{}-{}", id, i + 1), it.product_id, it.quantity, it.unit_price)?);
        }
        let mut o = Order { id, lines, status: Status::Placed, events: Vec::new() };
        o.record_event(DomainEvent::OrderPlaced { order_id: o.id.clone(), at: chrono::Utc::now() });
        Ok(o)
    }
    pub fn confirm(&mut self) -> Result<(), String> {
        if self.status != Status::Placed { return Err("only placed orders can confirm".into()); }
        self.status = Status::Confirmed;
        Ok(())
    }
    // Root addresses its internal entity by LOCAL identity — the sole mutator.
    // iter_mut() is the borrow-checker shape of the rule: one &mut path to internals.
    pub fn change_line_quantity(&mut self, line_id: &str, qty: u32) -> Result<(), String> {
        if self.status != Status::Placed { return Err("only placed orders can change lines".into()); }
        let line = self.lines.iter_mut().find(|l| l.line_id == line_id)
            .ok_or_else(|| "unknown line".to_string())?;
        line.change_quantity(qty)?;
        let id = self.id.clone();
        self.record_event(DomainEvent::OrderLineQuantityChanged { order_id: id, line_id: line_id.into(), new_quantity: qty });
        Ok(())
    }
    pub fn id(&self) -> &str { &self.id }
    // Snapshots out — never &mut to the live OrderLine.
    pub fn lines(&self) -> Vec<LineSnapshot> { self.lines.iter().map(|l| l.snapshot()).collect() }
}

// --- Domain service: PricingService (pure, stateless, domain layer) ---
pub trait DiscountPolicy {
    fn discount_for(&self, order: &Order) -> Money;
}
pub trait PricingService {
    fn calculate_total(&self, order: &Order, policy: &dyn DiscountPolicy) -> Money;
}

pub struct StandardPricingService;
impl PricingService for StandardPricingService {
    fn calculate_total(&self, order: &Order, policy: &dyn DiscountPolicy) -> Money {
        let mut gross: Option<Money> = None;
        for l in order.lines() {
            let t = l.line_total();
            gross = Some(match gross {
                None => t,
                Some(g) => g.plus(&t).unwrap(),
            });
        }
        let gross = gross.expect("non-empty");
        let disc = policy.discount_for(order);
        Money { amount: gross.amount - disc.amount, currency: gross.currency.clone() }
    }
}

// --- Repository (port) — async trait (stable since Rust 1.75) ---
#[async_trait::async_trait]   // or native: trait OrderRepository { async fn ... }
pub trait OrderRepository: Send + Sync {
    async fn find_by_id(&self, id: &str) -> Result<Option<Order>, RepoError>;
    async fn save(&self, order: &Order) -> Result<(), RepoError>;
}

#[derive(Debug)]
pub struct RepoError(pub String);
impl fmt::Display for RepoError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result { write!(f, "{}", self.0) }
}
impl std::error::Error for RepoError {}
```

## Pitfalls
- No inheritance, no default fields → a trait defines behavior; each aggregate owns its `events: Vec<DomainEvent>` and implements the trait. A macro can dedupe if it stings.
- `&mut self` for commands; `&self` for queries. Borrow checker is happy as long as command methods don't hand out `&mut` aliases — hand out snapshots/clones of internal entities, never `&mut` to them. Only roots get repositories.
- Entities compare by id (manual `PartialEq`); VOs derive full value equality. Don't mix them up.
- `Decimal` (rust_decimal) for money — never `f32`/`f64`.
- Newtype wrappers (`pub struct OrderId(String);`) for branded IDs.
- Async traits are stable since Rust 1.75; repositories are naturally async (sqlx/sea-orm).
