Fond 3.0 — plan the week, shop from a list that adds up, and keep cookbooks that read and print like books.

- **Meal plan.** A week of recipes and notes by day and meal, with servings. **Shop for this week** puts the ingredients on the shopping list, scaled.
- **A shopping list that adds itself up.** The same ingredient from different recipes becomes one line, in weights, exact cups and spoons, or counts, sorted into aisles. Merge, separate and delete lines one at a time or several at once.
- **Nutrition** per serving, estimated from the ingredients with USDA FoodData Central figures. Off in Settings if you would rather not see it.
- **Read a cookbook like a book.** The leather cover opens, pages turn on the spine, the contents jump to a recipe, and every photograph faces its recipe. One page at a time with a swipe on a phone.
- **Printing, rebuilt.** The preview is the real pages at the size of the paper: sheets, book pages and recipe cards, in two styles and three text sizes, with photo, notes, weights, nutrition, source and QR code as you choose. Whole cookbooks print as books. Recipes export as Markdown, text or schema.org JSON.
- **Move in from anywhere.** Imports from Mealie, Tandoor, Paprika and Recipe Sage, and Fond backups restore in one upload, merged or replacing.
- **Two-step sign-in** with an authenticator app and recovery codes.
- Fixes for see-through bars on Chrome and Android, and for sheets that slipped behind the keyboard on phones.

```sh
docker run -d --name fond -p 8080:8080 -v ./data:/data ghcr.io/au12/fond:3.0.0
```

Upgrading from 2.x needs nothing but the new image: the database is migrated on first start. Back up the data folder first if you want a way back.

See the [README](https://github.com/au12/fond#readme) for how the shopping list, nutrition and printing work, and the [changelog](https://github.com/au12/fond/blob/main/CHANGELOG.md) for everything in this release.

Nutrition and weights use USDA FoodData Central (public domain) and AUSNUT 2023 (© Food Standards Australia New Zealand, CC BY 4.0).
