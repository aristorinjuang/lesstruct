# modules/textgen

Official Lesstruct module: AI text generation — the Enhancer and the
Translator over OpenAI-compatible APIs.

## Layout (module template)

```
module.go            # kernel.Module (single registry-facing file)
api/                 # browser handlers (legacy envelope) + mocks
domain/              # TextGenerationService, OpenAI provider (3.6a)
admin/               # admin-shell manifest (empty: TipTap buttons render data-driven)
testutil/            # kernel fixture for module tests
```

## Handlers

The browser-realm `api.TextGenHandler` serves Enhance and Translate with the
legacy `error:null` envelope. Media context injection degrades gracefully:
without a media lister the handler generates without media context. The
TipTap toolbar buttons render data-driven from the `/api/health` feature
flags, so the module declares no admin extensions. Core mounts the handler
explicitly through the structural `TextGenSurface` interface in `routes.go`
(the `textGenEnabled` feature flag gates mounting), so routes stay
module-agnostic and paths never drift.
