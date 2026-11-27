# C++ — DDD idioms

Loaded when the project is C++ or C (`CMakeLists.txt`, `meson.build`, `BUILD`/`MODULE.bazel` (Bazel); packages via `conanfile.*` / `vcpkg.json`). Adapt to your model — don't copy verbatim. A dedicated **C** section (§5) covers plain-`.c` projects; for mixed projects treat C++ as the realization language.

## Tactical shapes
- **Aggregate root** — class with private data members; a constructor or static factory (named-constructor idiom) validates invariants; command methods (`void confirm()`) enforce rules and `register_event(...)` into a `std::vector` member. C++ has real single inheritance, so the classic Vernon `AggregateRoot` base works (unlike Rust traits / Go embedding).
- **Entity (internal)** — a mutable class **owned by** its root, with identity **local to the aggregate**; no repository of its own; mutated only via root commands. Equality by id, never defaulted value equality.
- **Value object** — immutable class/struct with `operator==` (add a `std::hash` specialization if keyed in a set/map). Use **strong types** for branded primitives: a distinct `struct OrderId { std::string value; }` with explicit conversions (or a strong-type library) — never a bare `std::string` at an aggregate boundary.
- **Repository** — an abstract base class (or a C++20 concept-constrained polymorphic interface) declared **in the domain**; a concrete adapter lives in `infrastructure/` (libpqxx, sqlpp11, ODB ORM, soci). The domain header links no DB client.
- **Domain events** — either a polymorphic `DomainEvent` base with concrete event classes, or a single `std::variant<OrderPlaced, OrderConfirmed, …>` of plain event structs — pick whichever matches the codebase. Events are collected on the aggregate; the application/use-case layer pulls and publishes them after `save`. The domain never links a broker.
- **Error handling** — prefer `std::expected<T,E>` (C++23) or `tl::expected` (C++17/20) for fallible factories/commands (parallels Rust's `Result`, Go's `error`). Exceptions are acceptable if the project already uses them, but keep fallible VO construction behind a static `make()` returning `expected` rather than throwing from a constructor.

## AggregateRoot base
Real inheritance → the Vernon base carries the events collection (C++ analogue of the Java/C# base):

```cpp
#include <memory>
#include <vector>

struct DomainEvent {
    virtual ~DomainEvent() = default;
};

class AggregateRoot {
public:
    virtual ~AggregateRoot() = default;
    std::vector<std::unique_ptr<DomainEvent>> pull_events() {
        std::vector<std::unique_ptr<DomainEvent>> out;
        out.swap(events_);     // O(1) move; aggregate left empty
        return out;
    }
protected:
    void register_event(std::unique_ptr<DomainEvent> e) {
        events_.push_back(std::move(e));
    }
private:
    std::vector<std::unique_ptr<DomainEvent>> events_;
};
```

(Event-type alternative: drop the polymorphic base and store `std::variant<…>` events — same `register_event` / `pull_events` mechanics, value semantics throughout.)

## Worked example: ordering
```cpp
// C++20 + tl::expected (C++17/20) or <expected> (C++23). Adapt the standard / error strategy to the project.
#include <tl/expected.hpp>   // or #include <expected> on C++23
#include <algorithm>
#include <chrono>
#include <cstdint>
#include <memory>
#include <ranges>
#include <string>
#include <string_view>
#include <vector>

using std::expected, std::unexpected;

// --- Strong type: branded OrderId ---
struct OrderId {
    std::string value;
    static expected<OrderId, std::string> parse(std::string_view s) {
        if (s.empty()) return unexpected("empty id");
        return OrderId{std::string(s)};
    }
    friend bool operator==(OrderId const&, OrderId const&) = default;
};

// --- Value object: Money (integer minor units — never double) ---
class Money {
public:
    static expected<Money, std::string> make(std::int64_t minor, std::string currency) {
        if (minor < 0) return unexpected("negative");
        if (currency.size() != 3 ||
            !std::ranges::all_of(currency, [](char c){ return c >= 'A' && c <= 'Z'; }))
            return unexpected("bad currency");
        return Money{minor, std::move(currency)};
    }
    std::int64_t minor() const { return minor_; }
    std::string const& currency() const { return currency_; }
    expected<Money, std::string> plus(Money const& o) const {
        if (currency_ != o.currency_) return unexpected("mismatch");
        return Money{minor_ + o.minor_, currency_};
    }
    Money times(int n) const { return Money{minor_ * n, currency_}; }   // n assumed validated by caller
    friend bool operator==(Money const&, Money const&) = default;
private:
    Money(std::int64_t minor, std::string currency) : minor_{minor}, currency_{std::move(currency)} {}
    std::int64_t minor_;
    std::string currency_;
};

// --- Entity (internal): OrderLine — owned by Order, local identity ---
// Mutable, compared by line_id only: NOT value equality. Only the root mutates it.
class OrderLine {
public:
    static expected<OrderLine, std::string> make(std::string line, OrderId product, int qty, Money unit_price) {
        if (qty <= 0) return unexpected("quantity must be positive");
        return OrderLine{std::move(line), std::move(product), qty, std::move(unit_price)};
    }
    std::string const& line_id() const { return line_; }
    OrderId const& product() const { return product_; }
    int quantity() const { return qty_; }
    Money line_total() const { return unit_price_.times(qty_); }
    bool operator==(OrderLine const& o) const { return line_ == o.line_; }
    // Snapshot handed out to callers — data about the line, never a mutable alias.
    struct Snapshot { std::string line; OrderId product; int qty; Money unit_price; };
    Snapshot snapshot() const { return Snapshot{line_, product_, qty_, unit_price_}; }
private:
    friend class Order;   // only the root mutates internals
    void change_quantity(int q) { qty_ = q; }   // caller validates; root owns the invariant
    OrderLine(std::string l, OrderId p, int q, Money u)
        : line_{std::move(l)}, product_{std::move(p)}, qty_{q}, unit_price_{std::move(u)} {}
    std::string line_;   // unique only within its Order
    OrderId product_;
    int qty_;
    Money unit_price_;
};

// --- Value object: LineItem — the facts the caller supplies to place an order ---
// Input VO, not an entity: no identity. The root builds one OrderLine per LineItem.
struct LineItem {
    OrderId product;
    int qty;
    Money unit_price;
    static expected<LineItem, std::string> make(OrderId p, int q, Money u) {
        if (q <= 0) return unexpected("quantity must be positive");
        return LineItem{std::move(p), q, std::move(u)};
    }
};

// --- Domain events ---
struct OrderPlaced : DomainEvent {
    OrderId order_id;
    std::chrono::steady_clock::time_point at = std::chrono::steady_clock::now();
    explicit OrderPlaced(OrderId id) : order_id{std::move(id)} {}
};
struct OrderLineQuantityChanged : DomainEvent {
    OrderId order_id;
    std::string line_id;
    int new_quantity;
    std::chrono::steady_clock::time_point at = std::chrono::steady_clock::now();
};

// --- Aggregate: Order ---
class Order : public AggregateRoot {
public:
    enum class Status { Placed, Confirmed };

    static expected<Order, std::string> place(OrderId id, std::vector<LineItem> items) {
        if (items.empty()) return unexpected("order needs a line");
        std::vector<OrderLine> lines;
        lines.reserve(items.size());
        for (std::size_t i = 0; i < items.size(); ++i) {
            auto l = OrderLine::make(id.value + "-" + std::to_string(i + 1),
                std::move(items[i].product), items[i].qty, std::move(items[i].unit_price));
            if (!l) return unexpected(l.error());
            lines.push_back(std::move(*l));
        }
        Order o{std::move(id), std::move(lines), Status::Placed};
        o.register_event(std::make_unique<OrderPlaced>(o.id_));
        return o;
    }
    expected<void, std::string> confirm() {
        if (status_ != Status::Placed) return unexpected("only placed orders can confirm");
        status_ = Status::Confirmed;
        return {};
    }

    // Root addresses its internal entity by LOCAL identity — the sole mutator.
    expected<void, std::string> change_line_quantity(std::string_view line_id, int qty) {
        if (status_ != Status::Placed) return unexpected("only placed orders can change lines");
        if (qty <= 0) return unexpected("quantity must be positive");
        for (auto& l : lines_)
            if (l.line_id() == line_id) {
                l.change_quantity(qty);
                auto e = std::make_unique<OrderLineQuantityChanged>();
                e->order_id = id_; e->line_id = std::string(line_id); e->new_quantity = qty;
                register_event(std::move(e));
                return {};
            }
        return unexpected("unknown line");
    }

    OrderId const& id() const { return id_; }
    // Read-only views out — callers see data, never a mutable alias to internals.
    std::vector<OrderLine> const& lines() const { return lines_; }
    std::vector<OrderLine::Snapshot> line_snapshots() const {
        std::vector<OrderLine::Snapshot> out;
        out.reserve(lines_.size());
        for (auto const& l : lines_) out.push_back(l.snapshot());
        return out;
    }
private:
    Order(OrderId id, std::vector<OrderLine> lines, Status s)
        : id_{std::move(id)}, lines_{std::move(lines)}, status_{s} {}
    OrderId id_;
    std::vector<OrderLine> lines_;
    Status status_;
};

// --- Domain service: PricingService (pure, stateless, domain layer) ---
class DiscountPolicy {                       // port
public:
    virtual ~DiscountPolicy() = default;
    virtual Money discount_for(Order const&) = 0;
};
class PricingService {                       // port
public:
    virtual ~PricingService() = default;
    virtual Money calculate_total(Order const&, DiscountPolicy&) = 0;
};

class StandardPricingService final : public PricingService {
public:
    Money calculate_total(Order const& o, DiscountPolicy& policy) override {
        Money gross = o.lines().front().line_total();
        for (auto const& l : o.lines() | std::views::drop(1))
            gross = gross.plus(l.line_total()).value();   // same currency — checked at VO construction
        Money disc = policy.discount_for(o);
        return Money{gross.minor() - disc.minor(), gross.currency()};
    }
};

// --- Repository (port) — abstract base in the domain; adapter in infrastructure ---
class OrderRepository {
public:
    virtual ~OrderRepository() = default;
    virtual expected<Order, std::string> find_by_id(OrderId const& id) = 0;   // unexpected on not-found / error
    virtual expected<void, std::string> save(Order const& order) = 0;
};
```

## Pitfalls
- Internal entities are mutable classes with identity equality (compare the local id); VOs use defaulted value equality. Only roots get repositories. Hand out snapshots/read-only views, never mutable aliases to internals.
- Store money as **integer minor units** (`std::int64_t`), or `boost::multiprecision::cpp_int` for arbitrary precision — never `float`/`double`.
- `expected` over exceptions for fallible factories/commands; keep exceptions (if used) out of VO constructors in hot paths.
- Header/source split: keep the aggregate's private members in the header (needed for `AggregateRoot` inheritance and inline methods); for true opacity use the **Pimpl** idiom only where ABI stability matters, not as a default.
- `unique_ptr` events move cheaply and self-free; the publisher owns them after `pull_events()`. Don't store raw `new`-ed events.
- Pick **one** event representation per codebase — polymorphic `DomainEvent` base **or** `std::variant<…>` — not both.
- `final` on concrete aggregates/services; `= default` equality on VOs (never on entities); reserve `virtual` for ports.
- Link the DB/broker client only in `infrastructure/`; the domain target must compile with no framework deps (hexagonal discipline).

## §5. Plain C (optional)

For pure-C projects (same manifests; sources are `.c` with no C++). C has no classes, generics, exceptions, or RAII — DDD maps via **opaque-pointer ADTs** and **vtables**.

- **Aggregate root** — declare `typedef struct Order Order;` (opaque) in the header; define `struct Order { … }` in the `.c` file. Command functions `order_confirm(Order* o, OrderError* err)` enforce invariants and append events to an internal array. "Inheritance" is **composition**: embed `struct aggregate_root { domain_event* events; size_t count; size_t cap; }` as the **first** member of every aggregate and route mutations through helpers `aggregate_root_record()` / `aggregate_root_pull()` — the C analogue of an embedded base.
- **Entity (internal)** — a nested `struct order_line` with a local `line_id`, stored in the aggregate's array and mutated only via root functions (`order_change_line_quantity(o, line_id, qty, err)`); identity comparison on the id.
- **Value object** — a plain struct passed **by value**, with an explicit `money_equals()` function (`memcmp` only when the struct is bit-identical and has no padding traps). Strong IDs: `typedef struct { char value[37]; } OrderId;` — a fixed-size struct, not a bare `char*`. (Internal entities compare by id: `order_line_id_equals()`, not full-struct equality.)
- **Repository** — a **vtable struct** of function pointers carrying a `void* ctx`:
  ```c
  typedef struct {
      void* ctx;
      int   (*find_by_id)(void* ctx, const char* id, Order** out);   // 0 ok, non-zero = error code
      int   (*save)(void* ctx, const Order* order);
  } order_repo;
  ```
  The application receives a concrete `order_repo` from infrastructure; this struct *is* the port. Pass it by value (a handful of pointers).
- **Domain events** — collected on the embedded `aggregate_root`; `order_pull_events(Order*)` returns a copy the application publishes after `save`. No broker symbols in any domain `.c`/`.h`.
- **Error handling** — every fallible function returns an error code (`int`/enum) and writes results through out-parameters; use `goto cleanup;` for multi-step teardown (the canonical C idiom). Memory is freed explicitly; prefer `__attribute__((cleanup(free_fn)))` (GCC/Clang) or manual `free` for scoped cleanup — no destructors exist.
- **Money** — `int64_t` minor units; never `double`.

### C pitfalls
- Opaque struct in `.h`, full definition in `.c` — this is C's encapsulation; never expose aggregate fields in the header.
- Every command returns an error code; there are no exceptions to fall back on. Always check.
- Vtable struct (not scattered function pointers) keeps the "port" explicit and mockable.
- Embedded `aggregate_root` must be the **first** member so `offsetof`/casts between the aggregate and its base are portable.
- Memory discipline: free what you allocate; use a `cleanup` attribute to avoid leaks; never `free` an aggregate still owned by the repository.
