# Ubiquitous Language — {{System / Bounded Context}}

> One term = one meaning within this bounded context. Keep it in sync with the code.

| Term | Definition (in the domain's words) | Where used (code / concept) |
|---|---|---|
| e.g. Order | A confirmed request to purchase items, distinct from a Cart. | `Order` aggregate |
| e.g. Cart | A transient, editable collection of items not yet ordered. | `Cart` aggregate |
|   |   |   |

## Disambiguation notes
Terms that sound similar but mean different things here vs another context (e.g., "Product" in Catalog ≠ "Product" in Inventory). Record the distinction so the boundary stays clean.
