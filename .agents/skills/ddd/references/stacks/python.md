# Python — DDD idioms

Loaded when the project is Python (`requirements.txt` / `pyproject.toml`). Adapt to your model — don't copy verbatim.

## Tactical shapes
- **Aggregate root** — class (often `@dataclass` with `__post_init__` enforcing invariants, or attrs/Pydantic); convention-private attributes (leading underscore); classmethod factory constructors; command methods validate and append to an internal `_events` list.
- **Entity (internal)** — a mutable `@dataclass(eq=False)` **owned by** its root, with identity **local to the aggregate** (`line_id` unique only within its `Order`); no repository of its own; mutated only via root commands (`order.change_line_quantity(line_id, …)`). Equality by id (`__eq__`/`__hash__` on the id). Never `frozen=True` — that freezes VOs, not entities.
- **Value object** — `@dataclass(frozen=True)` (value equality for free) or `pydantic.BaseModel` with `model_config = ConfigDict(frozen=True)`; single-value VOs as a `str`/`int` subclass or `typing.NewType`.
- **Repository** — domain `Protocol` or `ABC` (`OrderRepository`) with abstract methods; a SQLAlchemy adapter inside a unit-of-work in `infrastructure/`.
- **Domain events** — frozen dataclasses collected on the aggregate; the application service pulls them after the unit-of-work commits and routes them through an event bus.

## AggregateRoot base
A base dataclass with a private event list; aggregates subclass it (or use a mixin):

```python
from dataclasses import dataclass, field

@dataclass
class AggregateRoot:
    _events: list = field(default_factory=list, init=False, repr=False, compare=False)

    def _record(self, event) -> None:
        self._events.append(event)

    def pull_events(self) -> list:
        out = list(self._events)
        self._events.clear()
        return out
```

## Worked example: ordering
```python
from __future__ import annotations
from dataclasses import dataclass, field
from datetime import datetime
from decimal import Decimal
from enum import Enum
from typing import List, Optional, Protocol
import re

# --- Value object: Money (frozen → value equality for free) ---
@dataclass(frozen=True)
class Money:
    amount: Decimal
    currency: str

    def __post_init__(self):
        if self.amount < 0:
            raise ValueError("negative")
        if not re.fullmatch(r"[A-Z]{3}", self.currency):
            raise ValueError("bad currency")
        object.__setattr__(self, "amount", self.amount.quantize(Decimal("0.01")))

    def plus(self, other: "Money") -> "Money":
        if self.currency != other.currency:
            raise ValueError("mismatch")
        return Money(self.amount + other.amount, self.currency)

# --- Entity (internal): OrderLine — owned by Order, local identity ---
# Mutable, equality by line_id: NOT frozen. Only the root mutates it.
@dataclass(eq=False)
class OrderLine:
    line_id: str       # unique only within its Order
    product_id: str
    quantity: int
    unit_price: Money

    def __post_init__(self):
        if self.quantity <= 0:
            raise ValueError("quantity must be positive")

    def change_quantity(self, qty: int) -> None:   # called only by Order.change_line_quantity
        if qty <= 0:
            raise ValueError("quantity must be positive")
        self.quantity = qty

    def line_total(self) -> Money:
        return Money(self.unit_price.amount * self.quantity, self.unit_price.currency)

    def __eq__(self, o): return isinstance(o, OrderLine) and self.line_id == o.line_id
    def __hash__(self): return hash(self.line_id)

    @dataclass(frozen=True)   # snapshot handed out — data, never the live entity
    class Snapshot:
        line_id: str
        product_id: str
        quantity: int
        unit_price: Money
        def line_total(self) -> Money:
            return Money(self.unit_price.amount * self.quantity, self.unit_price.currency)

    def snapshot(self) -> "OrderLine.Snapshot":
        return OrderLine.Snapshot(self.line_id, self.product_id, self.quantity, self.unit_price)

# --- Value object: LineItem — the facts the caller supplies to place an order ---
# Input VO, not an entity: no identity. The root builds one OrderLine per LineItem.
@dataclass(frozen=True)
class LineItem:
    product_id: str
    quantity: int
    unit_price: Money

    def __post_init__(self):
        if self.quantity <= 0:
            raise ValueError("quantity must be positive")

# --- Domain events ---
@dataclass(frozen=True)
class OrderPlaced:
    order_id: str
    occurred_at: datetime = field(default_factory=datetime.utcnow)

@dataclass(frozen=True)
class OrderLineQuantityChanged:
    order_id: str
    line_id: str
    new_quantity: int
    occurred_at: datetime = field(default_factory=datetime.utcnow)

# --- Aggregate: Order ---
class Status(Enum):
    PLACED = "placed"
    CONFIRMED = "confirmed"

@dataclass(init=False)
class Order(AggregateRoot):
    id: str
    _lines: List[OrderLine]

    def __init__(self, id: str, lines: List[OrderLine]):
        super().__init__()
        if not lines:
            raise ValueError("order needs a line")
        self.id = id
        self._lines = list(lines)             # defensive copy; private — never handed out
        self._status = Status.PLACED
        self._record(OrderPlaced(self.id))

    @classmethod
    def place(cls, id: str, items: List[LineItem]) -> "Order":
        if not items:
            raise ValueError("order needs a line")
        lines = [OrderLine(line_id=f"{id}-{i + 1}", product_id=it.product_id,
                           quantity=it.quantity, unit_price=it.unit_price)
                 for i, it in enumerate(items)]
        return cls(id=id, lines=lines)        # __init__ records OrderPlaced

    def confirm(self) -> None:
        if self._status != Status.PLACED:
            raise ValueError("only placed orders can confirm")
        self._status = Status.CONFIRMED

    # Root addresses its internal entity by LOCAL identity — the sole mutator.
    def change_line_quantity(self, line_id: str, qty: int) -> None:
        if self._status != Status.PLACED:
            raise ValueError("only placed orders can change lines")
        for line in self._lines:
            if line.line_id == line_id:
                line.change_quantity(qty)
                self._record(OrderLineQuantityChanged(self.id, line_id, qty))
                return
        raise ValueError("unknown line")

    def line_snapshots(self) -> List["OrderLine.Snapshot"]:
        return [l.snapshot() for l in self._lines]   # data out, never the live entity

    @property
    def lines(self) -> List["OrderLine.Snapshot"]:    # read-only view; shadows nothing — _lines holds state
        return self.line_snapshots()

    @property
    def status(self) -> Status:
        return self._status

# --- Domain service: PricingService (pure, stateless, domain layer) ---
class DiscountPolicy(Protocol):
    def discount_for(self, order: Order) -> Money: ...

class PricingService(Protocol):
    def calculate_total(self, order: Order, policy: DiscountPolicy) -> Money: ...

class StandardPricingService:
    def calculate_total(self, order: Order, policy: DiscountPolicy) -> Money:
        gross: Optional[Money] = None
        for snap in order.line_snapshots():
            gross = snap.line_total() if gross is None else gross.plus(snap.line_total())
        discount = policy.discount_for(order)
        return Money(gross.amount - discount.amount, gross.currency)

# --- Repository (port) — Protocol for structural typing ---
class OrderRepository(Protocol):
    def find_by_id(self, id: str) -> Optional[Order]: ...
    def save(self, order: Order) -> None: ...
```

## Pitfalls
- Frozen dataclasses give `__eq__`/`__hash__` for VOs for free; normalize in `__post_init__` via `object.__setattr__` (frozen blocks direct assignment). Entities are the opposite: mutable, `eq=False`, equality by local id.
- Only roots get repositories — never create a repository for an internal entity.
- Privacy is by **convention** (leading underscore) — not enforced; rely on discipline. Hand out snapshots of internal entities, never the live object.
- Use `Decimal` (never `float`) for money.
- `Protocol` (PEP 544) for structural ports; `ABC` when you want nominal enforcement and shared logic.
- Publish events via the **unit-of-work** pattern after commit (see *Cosmic Python*), not from inside the aggregate.
