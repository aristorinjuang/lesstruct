# Python — DDD idioms

Loaded when the project is Python (`requirements.txt` / `pyproject.toml`). Adapt to your model — don't copy verbatim.

## Tactical shapes
- **Aggregate root** — class (often `@dataclass` with `__post_init__` enforcing invariants, or attrs/Pydantic); convention-private attributes (leading underscore); classmethod factory constructors; command methods validate and append to an internal `_events` list.
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

# --- Value object: OrderLine ---
@dataclass(frozen=True)
class OrderLine:
    product_id: str
    quantity: int
    unit_price: Money

    def __post_init__(self):
        if self.quantity <= 0:
            raise ValueError("quantity must be positive")

    def line_total(self) -> Money:
        return Money(self.unit_price.amount * self.quantity, self.unit_price.currency)

# --- Domain event ---
@dataclass(frozen=True)
class OrderPlaced:
    order_id: str
    occurred_at: datetime = field(default_factory=datetime.utcnow)

# --- Aggregate: Order ---
class Status(Enum):
    PLACED = "placed"
    CONFIRMED = "confirmed"

@dataclass
class Order(AggregateRoot):
    id: str
    lines: List[OrderLine]

    def __post_init__(self):
        super().__init__()
        if not self.lines:
            raise ValueError("order needs a line")
        self.lines = list(self.lines)             # defensive copy
        self._status = Status.PLACED
        self._record(OrderPlaced(self.id))

    @classmethod
    def place(cls, id: str, lines: List[OrderLine]) -> "Order":
        return cls(id=id, lines=lines)            # __post_init__ records OrderPlaced

    def confirm(self) -> None:
        if self._status != Status.PLACED:
            raise ValueError("only placed orders can confirm")
        self._status = Status.CONFIRMED

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
        for line in order.lines:
            gross = line.line_total() if gross is None else gross.plus(line.line_total())
        discount = policy.discount_for(order)
        return Money(gross.amount - discount.amount, gross.currency)

# --- Repository (port) — Protocol for structural typing ---
class OrderRepository(Protocol):
    def find_by_id(self, id: str) -> Optional[Order]: ...
    def save(self, order: Order) -> None: ...
```

## Pitfalls
- Frozen dataclasses give `__eq__`/`__hash__` for VOs for free; normalize in `__post_init__` via `object.__setattr__` (frozen blocks direct assignment).
- Privacy is by **convention** (leading underscore) — not enforced; rely on discipline.
- Use `Decimal` (never `float`) for money.
- `Protocol` (PEP 544) for structural ports; `ABC` when you want nominal enforcement and shared logic.
- Publish events via the **unit-of-work** pattern after commit (see *Cosmic Python*), not from inside the aggregate.
