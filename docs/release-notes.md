Fond 2.0 — weigh instead of scooping, and a history for every recipe.

- **Weigh.** One tap shows cups and spoons as grams, from a built-in table of about 250 foods drawn from USDA and AUSNUT laboratory data and cross-checked against King Arthur's and America's Test Kitchen's charts. Estimates are marked ≈; anything Fond cannot place is left as written, marked **?**, and can be taught in a tap for the whole household.
- **Kosher salt, judged.** Fond works out from a recipe's salt percentage whether Diamond Crystal or Morton is meant, shows its reasoning, and lets you overrule it.
- **Better baker's percentages.** Salt, starter hydration, and a plain warning when something could not be weighed.
- **History.** Every change to a recipe's ingredients or method is kept as a version, with a line-by-line view of what changed and a way back to any of them.

```sh
docker run -d --name fond -p 8080:8080 -v ./data:/data ghcr.io/au12/fond:2.0.0
```

Upgrading from 1.0 needs nothing but the new image: the database is migrated on first start. Back up the data folder first if you want a way back.

See the [README](https://github.com/au12/fond#readme) for how weighing works and where the numbers come from, and the [changelog](https://github.com/au12/fond/blob/main/CHANGELOG.md) for everything in this release.

AUSNUT 2023 is © Food Standards Australia New Zealand, used under CC BY 4.0.
