# Rust — DDD idioms

Loaded when the project is Rust (`Cargo.toml`). Adapt to your model — don't copy verbatim.

## Tactical shapes
- **Aggregate root** — struct with private fields; associated functions (`Order::place`, `pub fn confirm(&mut self)`) validate and mutate `self`, pushing events into a `Vec<DomainEvent>` field. Reuse via a `trait AggregateRoot { fn record_event(&mut self, ...); fn pull_events(&mut self) -> Vec<...>; }` — traits + composition stand in for inheritance (no default field, but the trait is the contract).
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

// --- Value object: OrderLine ---
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct OrderLine {
    product_id: String,
    quantity: u32,
    unit_price: Money,
}

impl OrderLine {
    pub fn new(product_id: String, quantity: u32, unit_price: Money) -> Result<Self, String> {
        if quantity == 0 { return Err("quantity must be positive".into()); }
        Ok(Self { product_id, quantity, unit_price })
    }
    pub fn line_total(&self) -> Money {
        Money {
            amount: self.unit_price.amount * Decimal::from(self.quantity),
            currency: self.unit_price.currency.clone(),
        }
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
    pub fn place(id: String, lines: Vec<OrderLine>) -> Result<Self, String> {
        if lines.is_empty() { return Err("order needs a line".into()); }
        let mut o = Order { id, lines, status: Status::Placed, events: Vec::new() };
        o.record_event(DomainEvent::OrderPlaced { order_id: o.id.clone(), at: chrono::Utc::now() });
        Ok(o)
    }
    pub fn confirm(&mut self) -> Result<(), String> {
        if self.status != Status::Placed { return Err("only placed orders can confirm".into()); }
        self.status = Status::Confirmed;
        Ok(())
    }
    pub fn id(&self) -> &str { &self.id }
    pub fn lines(&self) -> &[OrderLine] { &self.lines }
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
- `&mut self` for commands; `&self` for queries. Borrow checker is happy as long as command methods don't hand out `&mut` aliases.
- `Decimal` (rust_decimal) for money — never `f32`/`f64`.
- Newtype wrappers (`pub struct OrderId(String);`) for branded IDs.
- Async traits are stable since Rust 1.75; repositories are naturally async (sqlx/sea-orm).
