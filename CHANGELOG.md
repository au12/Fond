# Changelog

## 1.0.0

First release.

- Recipe library with instant search (titles and tags locally, full text of ingredients and steps on the server), filters, favorites, ratings and a cooking log.
- Save from a link, pasted text, or a photo (with AI assist); Android share target, iOS Shortcut and desktop bookmarklet; duplicate detection.
- Extraction from schema.org JSON-LD, microdata and RDFa, recipe-plugin markup and headings; optional OpenAI-compatible fallback.
- Plain-text ingredients with on-the-fly parsing: scaling, metric and US conversion, weight preference, author-supplied alternates, baker's percentages and hydration.
- Cooking view: tick-off ingredients and steps, step timers that complete their step, wake lock, saved progress, inline temperature conversion.
- Cookbooks with chapters; public share links for recipes and cookbooks with link previews and schema.org markup.
- Typeset print preview in three styles and four paper sizes, with QR codes.
- Shopping list grouped by aisle.
- Migration from Mealie: backup zip, recipe export zip, or live API.
- Multi-user household with roles, API tokens, session management, light and dark themes, installable PWA with offline reading.
- Hardened for public exposure: argon2id, rate limiting, CSRF protection, strict CSP, SSRF guard, scratch-based image running unprivileged.
