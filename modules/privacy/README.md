# Privacy module

Opt-in redaction via Meta SAM 3.1: scans every media and profile-picture
upload and blurs sensitive regions before anything is stored.

## Seams

- `domain.Segmenter` — finds detections, one Responses-API call per concept
  with boxes extracted. Implemented by `infrastructure.SAMService`.
- `domain.Service.RedactMedia` — blurs faces, plates, nudity (media).
- `domain.Service.RedactProfilePicture` — blurs plates and nudity, keeps
  faces visible (profile pictures).
- `media.Redactor` (`Redact`) — adapted in `internal/modules` via
  `NewMediaPrivacyRedactor`.
- `profilepicture.Redactor` (`Redact`) — adapted via
  `NewProfilePicturePrivacyRedactor`.

Fail-closed: any scan failure rejects the upload and stores nothing.
Empty `AI_SAM_API_KEY` keeps the legacy pipeline byte-identical.
