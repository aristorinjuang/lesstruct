# Lesstruct CLI

> **Scope.** This page documents `lesstruct-cli`, the terminal client for
> `/api/v1`: installation, authentication, output modes, every command
> group, and the two workflows that sell it (Markdown authoring, the
> commerce operator loop). Endpoint semantics live in
> [api-reference.md](api-reference.md); `lesstruct-cli <command> --help`
> remains the exhaustive flag source.

`lesstruct-cli` is a thin Cobra client over `/api/v1` for AI agents and
terminal-first humans. Markdown is a first-class input, pagination is
cursor-based, and `--output json` (errors included) keeps pipelines
parseable. On eCommerce builds the same Admin API key operates the store —
upsert catalog data, list and transition orders, set shipping, decide
payment proofs, triage registrations, suspend accounts, edit invoice
settings — without a browser session.

## Install

```bash
go install github.com/aristorinjuang/lesstruct/cmd/lesstruct-cli@latest
```

Or from source (`make install` drops the CLI into `$GOBIN` alongside the
blog edition):

```bash
make build-cli    # bin/lesstruct-cli
```

`lesstruct-cli --version` reports the shipped version (release builds bake
it in via `-ldflags`; a bare `go build` reports `dev`). A bare
`lesstruct-cli` with no arguments prints help and exits 0.

## Authentication and configuration

Every request carries one API key as a Bearer token. Create it from the
admin panel: **Profile → API keys** (`lesstruct_<keyID>_<secret>`), scoped
to your user — an Admin key unlocks the admin-only commands below; the CLI
never sends an unauthenticated request.

Resolution order for **both** the key and the base URL (first hit wins):

| Precedence | API key | Base URL |
|---|---|---|
| 1. Flag | `--api-key` (or `--api-key-stdin`, which reads the key from stdin's first line instead) | `--base-url` |
| 2. Environment | `LESSTRUCT_API_KEY` | `LESSTRUCT_BASE_URL` |
| 3. Config file | `[default] api_key` | `[default] base_url` |
| 4. Fallback | — (hard error) | `http://localhost:8080` |

Prefer the environment variable or the config file — a `--api-key` value
is visible to other local users in the process list (and lands in shell
history). `--api-key-stdin` keeps secrets out of argv entirely and composes
with pipes: `vault read -field=key ... | lesstruct-cli orders list --api-key-stdin`.
It cannot be combined with `--api-key`.

The config file is TOML, and only the `[default]` table is read:

```toml
[default]
api_key = "lesstruct_<keyID>_<secret>"
base_url = "https://cms.example.com"
```

It lives at `$LESSTRUCT_CONFIG`, else `$XDG_CONFIG_HOME/lesstruct/config.toml`,
else `~/.config/lesstruct/config.toml`. A missing file is fine (the chain
falls through); an unreadable or invalid one is a hard error. A key-bearing
config file that is group/world-readable triggers a warning on stderr
(`chmod 600` it). With no base
URL anywhere, commands target the local server — `lesstruct-cli content
list` just works against `http://localhost:8080` once the key resolves.

## Output modes and exit codes

`--output text` (default) prints a human line per result; `--output json`
prints the raw `{data, meta}` envelope. Failures go to stderr, stdout stays
empty, and `--output json` renders the failure as
`{"error": {"message": "…", "exitCode": N}}` so `stdout`-as-JSON pipelines
don't break.

| Code | Meaning |
|---|---|
| 0 | Success (for `media delete` and `comment delete`, exit 0 *is* the signal — nothing prints) |
| 1 | Generic: missing API key, network error, cancelled or failed import job |
| 2 | Usage: bad flags or arguments, invalid `--output` / `--poll-interval`, mutually exclusive flags |
| 3 | Auth: HTTP 401 |
| 4 | Not found: HTTP 404 |
| 5 | Validation: other 4xx, or a bad local `--source` / `--file` |
| 6 | Rate-limited: HTTP 429 |
| 7 | Server: 5xx |

## Command reference

Global flags (every command): `--api-key`, `--base-url`, `--output
text|json`. All flags are long-only; there are no shorthands.

### `content` — author and manage content

```bash
echo "# Launch day" | lesstruct-cli content create --post-type post --tags launch,news
lesstruct-cli content create --file draft.md --title "Launch day" --published
lesstruct-cli content list --status draft --limit 20
lesstruct-cli content publish 42
```

- `create [markdown]` (alias `new`) — body from `--file`, the positional
  arg, or piped stdin (in that order). Title defaults to the first heading
  or line; `--slug` is create-only and immutable after. Repeatable `--field
  key=value` (auto-typed: `true`/`false`→bool, numbers→number, else string)
  and `--field-json key='<json>'` for arrays/objects (wins on conflict).
  `--translation-of <id>` joins a translation group; `--format
  markdown|tiptap|html` (default `markdown`); `--published` publishes
  immediately.
- `update <id> [markdown]` — patch semantics: omitted `--post-type`,
  `--tags`, `--language`, `--published` keep server values (pass
  `--published=false` to unpublish). `--field`/`--field-json` **replace**
  all custom fields — omit both to preserve them. Empty body allows
  flags-only edits (retitle, retag).
- `list` — your own content, cursor-paginated (`--limit`, max 100;
  `--cursor`). Filters AND together; multiple `--tags` require every tag.
  `--search` needs ≥2 characters; `--author` is admin-only (403 otherwise).
- `get <id>` / `delete <id>` — single fetch; hard delete.
- `publish <id>` / `unpublish <id>` — publishing a draft auto-generates
  SEO and fires the `AfterPublish` plugin hook; no-ops answer 200.
- `system-fields <id> --field editorial_status=review` — admin-managed
  system fields (`editorial_status`, `internal_notes`, … per the post-type
  schema). **Admin only.**

### `media` — upload and manage files

```bash
lesstruct-cli media upload --file hero.webp --alt-text "Store hero"
lesstruct-cli media list --limit 20
```

- `upload` — `--file` is required. `--alt-text` or `--metadata
  '<json>'` (mutually exclusive; the server persists `altText` from
  either). Re-uploading identical bytes answers 409 instead of storing a
  duplicate. Requires a role with the `media` capability (403 otherwise).
- `list` — cursor-paginated (`--limit`, `--cursor`).
- `get <id>` — includes generated variants (thumbnails) when present.
- `delete <id>` — owners delete their own; anyone else's requires Admin
  (403, or 404 with no disclosure for non-owners).

### `catalog` — commerce data for products (eCommerce)

```bash
lesstruct-cli catalog upsert 12 --price 299000 --sku LSM-TS-001 --track-inventory --stock 40
lesstruct-cli catalog upsert 12 --variants-json '[{"sku":"LSM-TS-001-S",...}]'
lesstruct-cli catalog get 12
```

The id is the product's **content** id. Upsert merges: the record is read
first and only changed flags overlay, so omitted flags keep server values;
`--options-json` / `--variants-json` replace the variant matrix wholesale.
On a product with no record yet `--price` is required. Prices are integer
minor units; `--sale-price` must not exceed `--price` and cannot be cleared
back to unset from the CLI.

### `orders` — the fulfillment queue (eCommerce)

```bash
lesstruct-cli orders list --status pending
lesstruct-cli orders get 7          # numeric admin id, not ORD-00007
lesstruct-cli orders shipping 7 --amount 15000
lesstruct-cli orders transition 7 packed
```

- `get` takes the numeric admin id — `ORD-…` display numbers are rejected.
- `shipping` sets the admin shipping line in minor units (`--amount`,
  required, ≥0). Pending orders only; settled orders refuse with 409.
- `transition` moves the lifecycle forward — but never to `paid` (refused
  outright): payment confirmation is the only path into paid. Illegal moves
  read 409.

### `payments` — manual transfer proofs (eCommerce)

```bash
lesstruct-cli payments list --status pending
lesstruct-cli payments verify 8
```

`get` shows the proof with its order context (order number, status, payable
total). `verify` flips the order to paid through the payment-gated path;
`reject` leaves the order pending so the customer can resubmit. Decided
proofs, and proofs whose order already left pending, refuse with 409.

### `users` — registrations and moderation

```bash
lesstruct-cli users pending
lesstruct-cli users approve 9
lesstruct-cli users reject 10 --confirm
lesstruct-cli users suspend 11 --reason "Chargeback fraud"
```

- `pending` lists the registration queue with each row's
  email-verification state — approve only verified rows when the store
  requires admin approval (`[registration] admin_approval`; the server
  refuses the rest with 409, and with the default open registration
  approval activates the account directly).
- `approve` needs no confirmation; `reject` and `mark-spam` are
  destructive and final (the row is **deleted**, there is no rejected
  status to audit later) and require `--confirm`. Spam-marking also adds
  the email to the blocklist.
- `suspend`/`unsuspend` toggle access (`--reason` is emailed best-effort);
  `delete` is a soft-delete with an optional recorded `--reason`.

Account creation and role changes stay in the admin panel.

### `invoice-settings` — seller identity and PPN rate (eCommerce)

```bash
lesstruct-cli invoice-settings set --seller-name "Lesstruct Merch" --tax-rate 12
lesstruct-cli invoice-settings get
```

`set` merges like `catalog upsert`: the live block is read first, only
changed flags overlay, at least one flag is required. `tax_rate` is a
percent 0–100 (`0` = non-PKP, no tax line). Applies to the next rendered
invoice with no restart.

### `comment` — discussion on content

```bash
lesstruct-cli comment create 42 "First!" --output json
lesstruct-cli comment approve 42 7
```

Body from `--file`, the positional arg, or stdin. New comments start
`pending`; the content must be published (or owned) with comments enabled.
`list` shows every status the caller may see. `delete` removes your own,
or any with an Admin key. `approve` / `reject` / `spam` are admin-only
status transitions (no confirmation prompt).

### `import` — Hugo and WordPress migration

```bash
lesstruct-cli import hugo --source /path/to/hugo-site
lesstruct-cli import wordpress --file export.xml --skip-media
```

Both upload, then poll the server job with live progress (`--poll-interval`,
default `3s`). Hugo accepts a project directory (only `content/` and
`static/` are archived) or a `.tar.gz`. `--skip-media` imports text only —
images stay linked to their original URLs. The first ≤10 job errors print
to stderr.

### `export` — Hugo-compatible archive

```bash
lesstruct-cli export --output-dir /path/to/site
```

Downloads `lesstruct-export-<timestamp>.tar.gz` with `content/` source
files (YAML frontmatter + HTML body) plus the deduplicated referenced
media under `static/uploads/media/`. Large exports are bounded:
`--timeout` (default `30m`) caps the whole download and
`--max-download-size` (default `2GiB`, `0` disables) caps its bytes;
tripping either fails loudly instead of hanging or filling the disk. The
archive downloads to a `.part` file first and is renamed into place only
on success, so an interrupted run never leaves a truncated `.tar.gz`
masquerading as a complete export.

### `ssg` — static site generation

```bash
lesstruct-cli ssg --extract-dir /path/to/site
```

Downloads the fully rendered site (pages, AMP variants, listings, author
and tag pages, CSS, media, `sitemap.xml`, `robots.txt`, `404.html`, RSS).
Default writes `lesstruct-site-<timestamp>.tar.gz` into `--output-dir`;
`--extract-dir` unpacks in place instead (the two flags are mutually
exclusive). Same download bounds as `export`: `--timeout` (default `30m`)
and `--max-download-size` (default `2GiB`, `0` disables); an interrupted
download leaves at most a `.part` file behind.

## Workflows

**Authoring loop** — write Markdown, create, publish, iterate:

```bash
lesstruct-cli content create --file post.md --post-type post --tags release --published
lesstruct-cli content update 42 --tags release,featured
```

**Commerce operator loop** (eCommerce, no browser session) — triage the
queue end to end:

```bash
lesstruct-cli orders list --status pending
lesstruct-cli orders shipping 7 --amount 15000   # before payment settles
lesstruct-cli payments list --status pending
lesstruct-cli payments verify 8
lesstruct-cli orders transition 7 packed
lesstruct-cli users pending && lesstruct-cli users approve 9
```

See [getting-started-ecommerce.md](getting-started-ecommerce.md) for what
each of these steps means in the store workflow.

## See also

- [api-reference.md](api-reference.md) — the `/api/v1` contract the CLI
  speaks (authentication, envelope, error catalog).
- [built-in-modules.md](built-in-modules.md) — which module owns each
  surface, and what disabling it does to the CLI.
