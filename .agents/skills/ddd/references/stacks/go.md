# Go — DDD idioms

Loaded when the project is Go (`go.mod`). Adapt to your model — don't copy verbatim.

## Tactical shapes
- **Aggregate root** — struct with unexported fields; a `NewOrder(...)` constructor function enforces invariants; command methods (`func (o *Order) Confirm()`) enforce rules and append events. Reuse via **struct embedding**: embed an `AggregateRoot` base whose `record`/`PullEvents` methods are promoted onto the aggregate (Go's composition-as-inheritance).
- **Entity (internal)** — a mutable struct **owned by** its root, with identity **local to the aggregate** (`lineID` unique only within its `Order`); no repository of its own; addressed only via root commands (`o.ChangeLineQuantity(lineID, …)`). Always a **pointer type** (`*OrderLine`): looked up, shared, mutated in place through pointer receivers. Equality by id.
- **Value object** — immutable struct with a value-equality method, or a typed wrapper around a primitive (typed alias) for single-value VOs. Passed **by value**, never mutated in place.
- **Repository** — domain interface `type OrderRepository interface { FindByID(ctx, id) (*Order, error); Save(ctx, *Order) error }`; a concrete adapter (database/sql, GORM, sqlc) in `infrastructure/`. Interfaces belong to the **consumer** side in idiomatic Go.
- **Domain events** — collected on the embedded base; pulled and published by the application/use-case layer after `Save` (ports-and-adapters; the domain never imports a broker).

## AggregateRoot base
Embedded, not inherited — methods are promoted onto the embedding aggregate:

```go
type DomainEvent interface{ OccurredAt() time.Time }

type AggregateRoot struct {
	events []DomainEvent
}

func (a *AggregateRoot) record(e DomainEvent) { a.events = append(a.events, e) }
func (a *AggregateRoot) PullEvents() []DomainEvent {
	out := make([]DomainEvent, len(a.events))
	copy(out, a.events)
	a.events = a.events[:0]
	return out
}
```

## Worked example: ordering
```go
package order

import (
	"context"
	"errors"
	"strconv"
	"time"
)

// --- Value object: Money (stored as minor units — avoid float drift) ---
type Money struct {
	amount   int64
	currency string
}

func (m Money) Amount() int64      { return m.amount }
func (m Money) Currency() string   { return m.currency }
func (m Money) Plus(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, errors.New("mismatch")
	}
	return Money{amount: m.amount + other.amount, currency: m.currency}, nil
}
func (m Money) Equals(other Money) bool {
	return m.amount == other.amount && m.currency == other.currency
}

func NewMoney(amount int64, currency string) (Money, error) {
	if amount < 0 {
		return Money{}, errors.New("negative")
	}
	if len(currency) != 3 {
		return Money{}, errors.New("bad currency")
	}
	return Money{amount: amount, currency: currency}, nil
}

// --- Entity (internal): OrderLine — owned by Order, local identity ---
// Not a value object: it is addressed individually over its lifecycle
// (change its quantity), so it carries a lineID meaningful only within its Order.
type OrderLine struct {
	lineID    string
	productID string
	quantity  int
	unitPrice Money
}

func (l *OrderLine) changeQuantity(qty int) error {
	if qty <= 0 {
		return errors.New("quantity must be positive")
	}
	l.quantity = qty
	return nil
}

func (l *OrderLine) LineID() string    { return l.lineID }
func (l *OrderLine) ProductID() string { return l.productID }
func (l *OrderLine) Quantity() int     { return l.quantity }
func (l *OrderLine) LineTotal() Money {
	return Money{amount: l.unitPrice.amount * int64(l.quantity), currency: l.unitPrice.currency}
}
func (l *OrderLine) Equals(other *OrderLine) bool {
	if other == nil {
		return false
	}
	return l.lineID == other.lineID
}

func NewOrderLine(lineID, productID string, qty int, price Money) (*OrderLine, error) {
	if qty <= 0 {
		return nil, errors.New("quantity must be positive")
	}
	return &OrderLine{lineID: lineID, productID: productID, quantity: qty, unitPrice: price}, nil
}

// --- Value object: LineItem — the facts the caller supplies to place an order ---
// Input VO, not an entity: no identity. The root builds one OrderLine per LineItem.
type LineItem struct {
	ProductID string
	Quantity  int
	UnitPrice Money
}

func NewLineItem(productID string, qty int, price Money) (LineItem, error) {
	if qty <= 0 {
		return LineItem{}, errors.New("quantity must be positive")
	}
	return LineItem{ProductID: productID, Quantity: qty, UnitPrice: price}, nil
}

// LineSnapshot is the read-only data handed out about a line —
// callers get data, never the live mutable *OrderLine.
type LineSnapshot struct {
	LineID    string
	ProductID string
	Quantity  int
	UnitPrice Money
}

func (s LineSnapshot) LineTotal() Money {
	return Money{amount: s.UnitPrice.amount * int64(s.Quantity), currency: s.UnitPrice.currency}
}

// --- Aggregate: Order ---
type orderStatus int

const (
	StatusPlaced orderStatus = iota
	StatusConfirmed
)

type OrderPlaced struct {
	OrderID string
	at      time.Time
}

func (e OrderPlaced) OccurredAt() time.Time { return e.at }

type OrderLineQuantityChanged struct {
	OrderID     string
	LineID      string
	NewQuantity int
	at          time.Time
}

func (e OrderLineQuantityChanged) OccurredAt() time.Time { return e.at }

type Order struct {
	AggregateRoot
	id     string
	lines  []*OrderLine
	status orderStatus
}

func (o *Order) Confirm() error {
	if o.status != StatusPlaced {
		return errors.New("only placed orders can confirm")
	}
	o.status = StatusConfirmed
	return nil
}

// ChangeLineQuantity addresses an internal entity by its LOCAL identity.
// The root — never the caller — looks the line up, enforces invariants, mutates it.
func (o *Order) ChangeLineQuantity(lineID string, qty int) error {
	if o.status != StatusPlaced {
		return errors.New("only placed orders can change lines")
	}
	for _, l := range o.lines {
		if l.lineID == lineID {
			if err := l.changeQuantity(qty); err != nil {
				return err
			}
			o.record(OrderLineQuantityChanged{OrderID: o.id, LineID: lineID, NewQuantity: qty, at: time.Now()})
			return nil
		}
	}
	return errors.New("unknown line")
}
func (o *Order) ID() string { return o.id }

// Lines returns snapshots — data about internal entities, never the live *OrderLine.
func (o *Order) Lines() []LineSnapshot {
	out := make([]LineSnapshot, len(o.lines))
	for i, l := range o.lines {
		out[i] = LineSnapshot{LineID: l.lineID, ProductID: l.productID, Quantity: l.quantity, UnitPrice: l.unitPrice}
	}
	return out
}

func NewOrder(id string, items []LineItem) (*Order, error) { // factory: builds entities from input VOs
	if len(items) == 0 {
		return nil, errors.New("order needs a line")
	}
	lines := make([]*OrderLine, len(items))
	for i, it := range items {
		l, err := NewOrderLine(id+"-"+strconv.Itoa(i+1), it.ProductID, it.Quantity, it.UnitPrice)
		if err != nil {
			return nil, err
		}
		lines[i] = l
	}
	o := &Order{id: id, lines: lines, status: StatusPlaced}
	o.record(OrderPlaced{OrderID: id, at: time.Now()})
	return o, nil
}

// --- Domain service: PricingService (pure, stateless, domain layer) ---
type DiscountPolicy interface {
	DiscountFor(o *Order) (Money, error)
}
type PricingService interface {
	CalculateTotal(o *Order, p DiscountPolicy) (Money, error)
}

type StandardPricingService struct{}

func (StandardPricingService) CalculateTotal(o *Order, p DiscountPolicy) (Money, error) {
	var gross Money
	for i, l := range o.Lines() {
		t := l.LineTotal()
		if i == 0 {
			gross = t
			continue
		}
		g, err := gross.Plus(t)
		if err != nil {
			return Money{}, err
		}
		gross = g
	}
	disc, err := p.DiscountFor(o)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: gross.amount - disc.amount, currency: gross.currency}, nil
}

// --- Repository (port) — defined on the consumer (application) side ---
type OrderRepository interface {
	FindByID(ctx context.Context, id string) (*Order, error)
	Save(ctx context.Context, o *Order) error
}
```

## Pitfalls
- Embed `AggregateRoot` for method promotion (composition-as-inheritance); don't build OOP class hierarchies.
- Roots and internal entities are **pointers** (`*Order`, `*OrderLine`); value objects are **values** (`Money`). Command methods use pointer receivers and return `error`; don't mutate through value receivers.
- Only roots get repositories — never create a repository for an internal entity.
- Store money as integer minor units (or use `shopspring/decimal`); floats cause drift.
- Define repository interfaces where they're **consumed** (application layer), not in the aggregate's package.
- Exported accessors (`ID()`, `Lines()`) replace public fields; hand out snapshots/copies of internal entities, never the live `*OrderLine`.
