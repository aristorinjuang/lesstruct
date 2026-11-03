# modules/speech

Official Lesstruct module: voice dictation — speech-to-text transcription
over Meta Model API (Muse Voice Transcribe) or any OpenAI-compatible
endpoint, enhanced into TipTap content by the text generation service.

## Layout (module template)

```
module.go            # kernel.Module (single registry-facing file)
api/                 # browser handler (legacy envelope) + mocks
domain/              # SpeechToTextService, Transcription, WAV validator
infrastructure/      # Meta + OpenAI-compatible provider adapters
admin/               # admin-shell manifest (empty: Dictate button renders data-driven)
testutil/            # kernel fixture for module tests
```

## Handlers

The browser-realm `api.SpeechHandler` serves Transcribe with the
legacy `error:null` envelope. It validates the upload (mono 16-bit PCM WAV
at 16/24 kHz, 10 MB max), transcribes it through the adopted provider, and
enhances the transcript into TipTap content with SEO metadata through the
adopted transcription enhancer (the textgen service). The Dictate button
renders data-driven from the `/api/health` feature flags, so the module
declares no admin extensions. Core mounts the handler explicitly through
the structural `SpeechSurface` interface in `routes.go` (the `speechEnabled`
feature flag gates mounting — it composes speech-to-text *and* text
generation, since the transcript is enhanced into content), so routes stay
module-agnostic and paths never drift.
