# The ingredient dictionary

Fond turns "1 cup flour" into "120 g" from a dictionary that ships inside the
app: `web/src/data/foods.json`, about 250 foods and 3,500 names. Nothing is
looked up at run time and nobody has to maintain a database.

| File | What it is |
| --- | --- |
| `foods.txt` | The curated list: which names cooks write, and which source records answer for them. Edit this. |
| `build.py` | Reads `foods.txt`, pulls the measures out of the source databases, cross-checks them and writes `foods.json`. |
| `report.md` | Generated. Every food with what each source says, so a change can be reviewed. |

## Rebuilding

```sh
python3 scripts/foods/build.py              # downloads the sources on first run (~80 MB, cached in .cache/)
python3 scripts/foods/build.py find shallot # search the source records while editing foods.txt
cd web && npm test                          # the golden set tells you what moved
```

Only the Python standard library is needed. If a change to the dictionary is
intended, refresh the golden set with `FOND_GOLDEN_UPDATE=1 npm test -- golden`
and read the diff.

## Where the numbers come from

- **USDA FoodData Central, SR Legacy** (2018) is the backbone: laboratory
  portion weights for cups, spoons and counted items. Public domain (CC0).
- **USDA FNDDS** (2021–2023) fills the long tail. Public domain (CC0).
- **AUSNUT 2023 food measures** is an independent second opinion.
  © Food Standards Australia New Zealand, used under CC BY 4.0.
- **Published kitchen charts** (King Arthur Baking, America's Test Kitchen)
  are quoted value by value as `ref:` and `pin:` entries where test kitchens
  have settled on a convention the laboratories do not capture, above all how
  a cup of flour is filled.

For each food the build takes the median across those independent families.
Where they disagree by more than 10 % the food is marked as an estimate and
Fond shows its weights with "≈". A `pin:` overrides the arithmetic and always
states why.

## How a line is matched

The matcher (`web/src/lib/foods.ts`) is deliberately conservative. A line is
tied to a food by its head noun; describing words it knows are harmless
("fresh", "organic", "finely chopped") are ignored; words that change what the
thing is ("dried", "cooked", "powdered", or the name of another food) stop the
match unless a name says otherwise. Anything it cannot place is left exactly
as the recipe wrote it and marked, never guessed.
