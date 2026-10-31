# Go — DDD idioms

Loaded when the project is Go (`go.mod`). Adapt to your model — don't copy verbatim.

## Tactical shapes
- **Aggregate root** — struct with unexported fields; a `NewOrder(...)` constructor function enforces invariants; command methods (`func (o *Order) Confirm()`) enforce rules and append events. Reuse via **struct embedding**: embed an `AggregateRoot` base whose `record`/`PullEvents` methods are promoted onto the aggregate (Go's composition-as-inheritance).
- **Value object** — immutable struct with a value-equality method, or a typed wrapper around a primitive (typed alias) for single-value VOs.
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

// --- Value object: OrderLine ---
type OrderLine struct {
	ProductID string
	Quantity  int
	UnitPrice Money
}

func (l OrderLine) LineTotal() Money {
	return Money{amount: l.UnitPrice.amount * int64(l.Quantity), currency: l.UnitPrice.currency}
}

func NewOrderLine(productID string, qty int, price Money) (OrderLine, error) {
	if qty <= 0 {
		return OrderLine{}, errors.New("quantity must be positive")
	}
	return OrderLine{ProductID: productID, Quantity: qty, UnitPrice: price}, nil
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

type Order struct {
	AggregateRoot
	id     string
	lines  []OrderLine
	status orderStatus
}

func (o *Order) Confirm() error {
	if o.status != StatusPlaced {
		return errors.New("only placed orders can confirm")
	}
	o.status = StatusConfirmed
	return nil
}
func (o *Order) ID() string         { return o.id }
func (o *Order) Lines() []OrderLine { return append([]OrderLine(nil), o.lines...) }

func NewOrder(id string, lines []OrderLine) (*Order, error) { // factory
	if len(lines) == 0 {
		return nil, errors.New("order needs a line")
	}
	o := &Order{id: id, lines: append([]OrderLine(nil), lines...), status: StatusPlaced}
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
- Store money as integer minor units (or use `shopspring/decimal`); floats cause drift.
- Define repository interfaces where they're **consumed** (application layer), not in the aggregate's package.
- Command methods use pointer receivers (`*Order`) and return `error`; don't mutate through value receivers.
- Exported accessors (`ID()`, `Lines()`) replace public fields; return defensive copies of slices.
