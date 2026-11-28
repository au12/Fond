Fond 2.1 — cookbook pages, and a leather binding for every book.

- **Set like a cookbook.** Shared recipes, printouts and cookbooks follow a classic cookbook spread: letterspaced title, justified headnote, a narrow ingredient column beside the method, and notes in a tinted box.
- **A photograph faces its recipe.** In a cookbook each recipe is a spread with the photo on the left page.
- **One sheet when it can.** A printed recipe keeps its photo and its whole ingredient list on the first page, and is tightened (never below about 10 pt) to fit one sheet.
- **Leather covers.** Every cookbook is bound in a hide drawn from its own seed: dye, creasing, grain and finish all vary, and a cover never changes unless you deal it a new one.
- Fixes for stuck hover styles, jumping bars and a jumping edit page on phones.

```sh
docker run -d --name fond -p 8080:8080 -v ./data:/data ghcr.io/au12/fond:2.1.0
```

Upgrading needs nothing but the new image. Existing cookbooks are each dealt a hide on first start, and keep it.

See the [changelog](https://github.com/au12/fond/blob/main/CHANGELOG.md) for everything in this release.
