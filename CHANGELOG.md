# Changelog

## Unreleased

- **Cookbook pages are re-typeset** after a classic cookbook spread: the chapter as a running head, the title in letterspaced capitals, a justified headnote over a hairline, the yield and times above a narrow sans-serif ingredient column, the method beside it with small hanging step numbers, the notes in a tinted box at the foot of the page and the recipe's number beneath. Modern and Compact share the new structure.
- **A photograph faces its recipe.** In a shared cookbook each recipe is a spread, photo on the left; on a phone the photo sits above the recipe. Printed, the photo takes a page of its own, to the edges of the paper.
- **A printed recipe keeps to one sheet when it can.** The photo and the whole ingredient list stay on the first page. The print preview is now a scale model of the sheet: it sets the recipe at up to five tightnesses, uses the roomiest that fits one page, never goes below about 10 pt, and says how many pages will print.
- **Every cookbook is bound in a leather of its own.** The hide is drawn from a random seed when the book is made: its dye within the cover colour, how unevenly the dye took, where the leather has creased pale, and the grain. No two are alike. The look is new pull-up leather, distressed in colour and lightly creased, with no scratches and no stains. A crease has no hard edge: its colour falls away gradually into the leather round it.
- **The finish is drawn too**: soft matte for about 65 books in 100, satin for 25, slightly shiny for 10.
- **A cover stays as it is once saved.** **New leather** in the cookbook's details deals another hide, **Previous** goes back to the one before, and nothing changes until you save. The same button is there when making a book.
- Cookbooks you already have are each dealt a hide the first time this version starts, and keep it.
- A new cover colour, **chestnut**, for the browns leather is known for. The seven others are deeper and more muted.
- Covers have a hinge by the spine and lettering stamped in gilt.
- New API: cookbooks carry `leather` (the seed); `GET /leather/{seed}-{colour}-{s|l}.jpg` serves the hide. Drawn hides are kept in `leather/` in the data folder.
- Fixed: printed notes could jump to a second page with room to spare; the Compact style drew a stray outline.

## 2.0.0

Fond learns to weigh, and to remember.

### Weighing

- **Weigh** on any recipe shows cups and spoons as grams, from a built-in table of about 250 foods (3,500 names). Counted things keep their count and gain a weight beside it. A weight the recipe gives itself always wins.
- The table is built from USDA FoodData Central (SR Legacy, FNDDS) and the AUSNUT 2023 food measures, cross-checked against each other and against King Arthur Baking's and America's Test Kitchen's charts. Where sources disagree by more than a tenth the weight is shown with ≈. Wheat flour follows King Arthur's 120 g cup; dip-and-sweep (142 g) is a setting, and recipes from King Arthur Baking or America's Test Kitchen are weighed by that kitchen's own method.
- A new matcher ties a line to a food by the head noun of its name instead of by any word in it, so "boiling water" is no longer weighed as oil, "butternut squash" as butter or "sugar snap peas" as sugar. Notes after the comma are ignored, states are honoured (sifted, packed, chopped, sliced), and "dried", "cooked", "powdered" or the name of a second food stop a match rather than risk a wrong one. It weighs 97.5 % of cup measures and 98.3 % of spoon measures in an 11,000-line corpus of real recipes.
- The parser now reads `1 cup plus 2 tablespoons`, `1 packed cup`, `juice of 1 lemon`, `1 stick (113 g) butter`, egg sizes, cloves, packets and stated can sizes.
- Lines Fond cannot weigh stay as written and are marked **?**. Tapping one offers the closest foods, "weigh it yourself", or "don't weigh this", for the whole household or one recipe. Settings → Weighing lists what the library still cannot weigh, and the household's corrections.
- **Kosher salt** is inferred per recipe from its salt percentage (bread about 2 % of flour, cookies 0.5–1.5 %, and so on), with the reasoning shown, the weight marked ≈ and a one-tap override. When the amount fits both Diamond Crystal and Morton, Fond says it cannot tell.
- **Weights** lists every line with its figure and its sources, and can write the weights into the recipe as `1 cup (120 g) flour`.
- Baker's percentages use the table's flour and water figures, show salt, take the starter's hydration, and say when a line had to be left out.
- A one-time note that amounts under a gram or two need a 0.01 g scale.
- With AI assist on, the model can suggest which known food an unfamiliar ingredient weighs like. It never applies anything itself.
- The edit page shows each line's weight as you type and lets you teach Fond an ingredient on the spot. Print, shared links and cookbooks weigh the same way, and shared links carry the household's corrections.

### History

- Every recipe has a version number. A save that changes the ingredients or the method starts a new version and keeps the old one; other edits do not.
- A History page lists the versions with who saved each, when and why, shows what changed line by line and word by word, and restores any of them as a new version. The first version and the latest hundred are kept.
- Editing offers an optional "what changed" note.
- Existing libraries get a version 1 for every recipe on first start. No action is needed.

### Also

- Weights a recipe gives in grams are no longer rounded to the nearest five when shown or scaled; ounces are shown to the quarter.
- "2 large eggs" halves to "1 large egg".
- The export includes each recipe's history and the household's weighing corrections.
- New API: `GET /api/recipes/{id}/versions`, `POST /api/recipes/{id}/versions/{n}/restore`, `GET|PUT /api/food-rules`, `DELETE /api/food-rules/{id}`, `POST /api/food-rules/suggest`, `GET /api/ingredient-lines`.

### Upgrading from 1.0

Pull the new image and start it. The database is migrated in place; back up the data folder first if you want a way back, because 1.0 does not know the new tables. "Weigh instead of scooping" has moved from Preferences to Settings → Weighing and keeps its setting.

AUSNUT 2023 is © Food Standards Australia New Zealand, used under CC BY 4.0. USDA data is in the public domain.

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
