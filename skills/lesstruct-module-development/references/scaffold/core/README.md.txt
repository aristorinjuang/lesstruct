# __MODULE_NAME__

<What the module owns in one paragraph: the feature, its surfaces
(routes/admin/post types/capabilities/migrations), and its seams.>

## Seams

- `domain/` — <entities/services/ports, e.g. WidgetService over WidgetStore>
- `api/` — <handlers + seam interfaces, e.g. WidgetStore, Mailer>
- `store/` — <single-source `*sql.DB + kernel.Dialect` adapter, or "none (no tables)">
- `admin/` — <manifest entries, or "none (data-driven surfaces only)">

## Requires

<Hard dependencies and the cascade they design, or "none (leaf)".>

## Disable semantics

<What `[modules] disabled = ["__MODULE_NAME__"]` drops and what keeps
serving (lifecycle-level vs route unmount).>
