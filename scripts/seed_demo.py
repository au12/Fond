#!/usr/bin/env python3
"""Fill a fresh Fond instance with demo recipes (used for the README screenshots).

    python3 scripts/seed_demo.py http://localhost:8080 [photo-dir]

Photos are fetched from Unsplash (free to use under the Unsplash License) unless
a directory of already-downloaded <id>.jpg files is given.
"""
import json, os, sys, urllib.request, http.cookiejar

BASE = sys.argv[1].rstrip("/") if len(sys.argv) > 1 else "http://localhost:8080"
PHOTOS = sys.argv[2] if len(sys.argv) > 2 else None
jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))


def call(method, path, body=None, raw=None, ctype="application/json"):
    data = raw if raw is not None else (json.dumps(body).encode() if body is not None else None)
    req = urllib.request.Request(BASE + "/api" + path, data=data, method=method, headers={"Content-Type": ctype, "Origin": BASE})
    with opener.open(req) as res:
        text = res.read()
        return json.loads(text) if text else None


def photo(pid):
    if PHOTOS and os.path.exists(os.path.join(PHOTOS, pid + ".jpg")):
        return open(os.path.join(PHOTOS, pid + ".jpg"), "rb").read()
    return urllib.request.urlopen(f"https://images.unsplash.com/photo-{pid}?w=1600&q=80&fm=jpg", timeout=30).read()


def L(*lines):
    return [{"text": l[2:], "h": True} if l.startswith("# ") else {"text": l} for l in lines]


R = [
    dict(photo="1509440159596-0249088772ff", fav=True, cooked=6, title="Country Sourdough Loaf", tags=["Bread", "Baking"], yield_="1 large loaf", servings=12, prep=40, cook=45, total=1440,
         description="An open, custardy crumb under a crackling crust. Mix in the evening, bake the next morning.",
         ingredients=L("450 g bread flour", "50 g whole wheat flour", "375 g water, at 85°F", "100 g sourdough starter, ripe and bubbly", "10 g fine sea salt", "Rice flour, for dusting the basket"),
         steps=L("Stir the flours and water together until no dry spots remain. Cover and rest for 45 minutes.",
                 "Add the starter and salt. Squeeze and fold the dough in the bowl for 3 minutes until it feels smooth and elastic.",
                 "Let the dough rise at warm room temperature for 4 to 5 hours, giving it a set of stretch-and-folds every 30 minutes for the first 2 hours.",
                 "Shape into a tight round, place seam-side up in a floured basket, cover, and refrigerate overnight.",
                 "Heat a Dutch oven in a 475°F oven for 45 minutes. Turn the loaf into the pot, score it, cover, and bake for 20 minutes.",
                 "Remove the lid, lower the oven to 450°F, and bake for 25 minutes more, until deeply browned. Cool on a rack for at least 1 hour before slicing."),
         notes="The dough is ready to shape when it has grown by about half and jiggles when you shake the bowl.\n\n**Cold kitchen?** Use 90°F water and leave the dough in the oven with only the light on.", extra={"baker": True}),
    dict(photo="1499636136210-6f4ee915583e", fav=True, cooked=9, rating=5, title="Brown Butter Chocolate Chip Cookies", tags=["Dessert", "Baking"], yield_="18 cookies", servings=18, prep=20, cook=12, total=92,
         description="Nutty browned butter, pools of dark chocolate and a shower of flaky salt.",
         ingredients=L("1 cup (227 g) unsalted butter", "1 cup packed dark brown sugar", "1/2 cup granulated sugar", "2 large eggs", "2 tsp vanilla extract", "2 1/4 cups (270 g) all-purpose flour", "1 tsp baking soda", "3/4 tsp kosher salt", "8 oz dark chocolate, chopped", "Flaky salt, to finish"),
         steps=L("Melt the butter in a light-colored pan over medium heat and cook, swirling, for 5 to 7 minutes until it smells nutty and the milk solids turn amber. Pour into a large bowl and cool for 15 minutes.",
                 "Whisk both sugars into the butter, then the eggs and vanilla, until glossy.",
                 "Fold in the flour, baking soda and salt until just combined, then the chocolate. Chill the dough for 1 hour.",
                 "Heat the oven to 350°F. Scoop 3-tablespoon mounds onto lined sheets, 3 inches apart.",
                 "Bake for 11 to 13 minutes until the edges are set and the centers still look soft. Sprinkle with flaky salt and cool on the sheet for 10 minutes."),
         notes="Scooped dough freezes well. Bake from frozen and add 2 minutes."),
    dict(photo="1585937421612-70a008356fbe", fav=True, cooked=4, rating=5, title="Butter Chicken", tags=["Dinner", "Indian"], yield_="4 servings", servings=4, prep=20, cook=35, total=55,
         description="Charred, yogurt-marinated chicken in a silky tomato and cream sauce.",
         ingredients=L("# For the chicken", "1 1/2 lb boneless chicken thighs, cut in large pieces", "1/2 cup plain yogurt", "1 tbsp garam masala", "1 tsp ground turmeric", "1 tsp kosher salt",
                       "# For the sauce", "3 tbsp butter", "1 large onion, finely chopped", "4 cloves garlic, grated", "1 tbsp grated ginger", "1 (14.5-ounce) can crushed tomatoes", "1 tsp Kashmiri chili powder", "3/4 cup heavy cream", "1 tsp dried fenugreek leaves", "Cilantro and steamed rice, to serve"),
         steps=L("Toss the chicken with the yogurt, garam masala, turmeric and salt. Marinate for 30 minutes, or overnight in the fridge.",
                 "Heat a wide skillet over high heat. Sear the chicken in batches for 3 minutes per side until charred in spots, then set aside.",
                 "Lower the heat, melt the butter, and cook the onion for 8 minutes until soft and golden. Stir in the garlic and ginger for 1 minute.",
                 "Add the tomatoes and chili powder and simmer for 10 minutes. Blend until smooth if you like a silky sauce.",
                 "Return the chicken with the cream and fenugreek. Simmer gently for 10 minutes until the chicken is cooked through. Finish with cilantro."),
         notes=""),
    dict(photo="1567620905732-2d1ec7ab7445", cooked=12, title="Buttermilk Pancakes", tags=["Breakfast"], yield_="12 pancakes", servings=4, prep=10, cook=15, total=25,
         description="Tall, tender and tangy. The batter takes five minutes.",
         ingredients=L("2 cups all-purpose flour", "2 tbsp sugar", "2 tsp baking powder", "1 tsp baking soda", "1/2 tsp salt", "2 cups buttermilk", "2 large eggs", "3 tbsp melted butter, plus more for the pan", "Maple syrup, to serve"),
         steps=L("Whisk the flour, sugar, baking powder, baking soda and salt in a large bowl.", "Whisk the buttermilk, eggs and melted butter together, pour over the dry ingredients, and fold until just combined. Lumps are good. Rest the batter for 5 minutes.",
                 "Heat a buttered griddle over medium heat. Pour 1/4-cup rounds and cook for 2 to 3 minutes until bubbles break the surface, then flip and cook 2 minutes more."), notes=""),
    dict(photo="1574071318508-1cdbab80d002", fav=True, cooked=7, title="Margherita Pizza", tags=["Dinner", "Italian"], yield_="2 pizzas", servings=4, prep=30, cook=8, total=1500,
         description="A cold-fermented dough, crushed tomatoes and fresh mozzarella. Nothing else.",
         ingredients=L("500 g tipo 00 flour", "325 g cool water", "10 g fine sea salt", "2 g instant yeast", "1 cup crushed San Marzano tomatoes", "8 oz fresh mozzarella, torn", "Basil leaves", "Olive oil"),
         steps=L("Mix the flour, water, salt and yeast until shaggy. Rest 20 minutes, then knead for 5 minutes until smooth.", "Divide in two, ball up, and refrigerate in oiled containers for 24 to 72 hours.",
                 "Heat the oven with a steel or stone to 550°F for 1 hour.", "Stretch a dough ball to 12 inches. Top with tomatoes and mozzarella.", "Bake for 6 to 8 minutes until blistered. Finish with basil and olive oil."), notes="", extra={"baker": True}),
    dict(photo="1455619452474-d2be8b1e70cd", cooked=2, title="Red Coconut Curry Soup", tags=["Dinner", "Thai", "Soup"], yield_="4 servings", servings=4, prep=15, cook=20, total=35, description="Bright, spicy and on the table in half an hour.",
         ingredients=L("2 tbsp red curry paste", "1 (13.5-ounce) can coconut milk", "3 cups chicken stock", "1 lb chicken breast, thinly sliced", "8 oz rice noodles", "2 tbsp fish sauce", "1 tbsp lime juice", "Thai basil and sliced chilies"),
         steps=L("Fry the curry paste in a splash of the coconut cream for 2 minutes until fragrant.", "Add the remaining coconut milk and stock and simmer for 10 minutes.", "Add the chicken and noodles and cook for 5 minutes. Season with fish sauce and lime juice."), notes=""),
    dict(photo="1473093295043-cdd812d0e601", title="Farfalle with Pesto and Tomatoes", tags=["Dinner", "Italian", "Vegetarian"], yield_="4 servings", servings=4, prep=10, cook=12, total=22, description="A summer pasta that is just as good cold the next day.",
         ingredients=L("12 oz farfalle", "2 cups basil leaves", "1/3 cup pine nuts, toasted", "1/2 cup grated Parmesan", "1 clove garlic", "1/2 cup olive oil", "2 cups cherry tomatoes, halved", "4 cups baby spinach"),
         steps=L("Cook the pasta in well-salted water for 11 minutes. Reserve a cup of the water.", "Blend the basil, pine nuts, Parmesan, garlic and oil to a rough pesto.", "Toss the hot pasta with the pesto, tomatoes and spinach, loosening with pasta water."), notes=""),
    dict(photo="1563379926898-05f4575a45d8", fav=True, cooked=3, title="Garlic Shrimp Spaghetti", tags=["Dinner", "Seafood"], yield_="4 servings", servings=4, prep=10, cook=15, total=25, description="",
         ingredients=L("12 oz spaghetti", "1 lb large shrimp, peeled", "6 cloves garlic, sliced", "1/2 tsp red pepper flakes", "1/4 cup olive oil", "1 pint cherry tomatoes", "1/2 cup dry white wine", "Parsley and lemon"),
         steps=L("Boil the spaghetti for 9 minutes.", "Warm the oil with the garlic and pepper flakes, add the shrimp and cook 2 minutes per side.", "Add the tomatoes and wine, simmer 3 minutes, then toss with the pasta."), notes=""),
    dict(photo="1565958011703-44f9829ba187", cooked=1, title="Raspberry Layer Cake", tags=["Dessert", "Baking"], yield_="10 slices", servings=10, prep=45, cook=30, total=195, description="Vanilla sponge, whipped mascarpone and a sharp raspberry compote.",
         ingredients=L("2 1/2 cups cake flour", "1 3/4 cups sugar", "1 tbsp baking powder", "3/4 cup butter, softened", "4 large eggs", "1 cup milk", "2 cups raspberries", "1 1/2 cups heavy cream", "8 oz mascarpone"),
         steps=L("Heat the oven to 350°F and line three 6-inch pans.", "Cream the butter and sugar for 4 minutes, beat in the eggs, then alternate the dry ingredients and milk.", "Bake for 28 to 30 minutes. Cool completely.", "Whip the cream and mascarpone. Layer with the berries and chill for 2 hours."), notes=""),
    dict(photo="1568901346375-23c9450c58cd", cooked=5, title="Smash Burgers", tags=["Dinner", "American"], yield_="4 burgers", servings=4, prep=10, cook=10, total=20, description="",
         ingredients=L("1 lb ground beef, 80% lean", "4 slices American cheese", "4 potato buns, toasted", "1 small onion, sliced paper thin", "Pickles, mustard and ketchup"),
         steps=L("Divide the beef into 8 loose balls.", "Heat a cast iron pan until smoking. Smash each ball flat with a spatula and cook for 90 seconds.", "Flip, top half with cheese, stack, and cook 30 seconds more."), notes=""),
    dict(photo="1606313564200-e75d5e30476c", fav=True, cooked=8, rating=4, title="Fudgy Brownies", tags=["Dessert", "Baking"], yield_="16 brownies", servings=16, prep=15, cook=28, total=43, description="Crackly on top, dense in the middle.",
         ingredients=L("200 g dark chocolate", "170 g butter", "250 g sugar", "3 large eggs", "100 g all-purpose flour", "30 g cocoa powder", "1/2 tsp salt"),
         steps=L("Heat the oven to 350°F and line an 8-inch square pan.", "Melt the chocolate and butter together.", "Whisk the sugar and eggs for 3 minutes until pale, fold in the chocolate, then the dry ingredients.", "Bake for 26 to 28 minutes. Cool for 1 hour before cutting."), notes=""),
    dict(photo="1512621776951-a57141f2eefd", title="Rainbow Grain Bowl", tags=["Lunch", "Vegetarian"], yield_="2 bowls", servings=2, prep=20, cook=20, total=40, description="",
         ingredients=L("1 cup quinoa", "1 sweet potato, cubed", "1 (15-ounce) can chickpeas, drained", "1 avocado", "2 cups mixed greens", "1/4 red cabbage, shredded", "2 tbsp tahini", "1 tbsp lemon juice"),
         steps=L("Simmer the quinoa for 15 minutes.", "Roast the sweet potato and chickpeas at 425°F for 20 minutes.", "Whisk the tahini with lemon juice and water. Build the bowls and drizzle."), notes=""),
    dict(photo="1484723091739-30a097e8f929", cooked=2, title="Brioche French Toast", tags=["Breakfast"], yield_="4 servings", servings=4, prep=10, cook=12, total=22, description="",
         ingredients=L("8 thick slices brioche", "4 large eggs", "1 cup milk", "1 tsp cinnamon", "2 tbsp butter", "Blueberries, banana and maple syrup"),
         steps=L("Whisk the eggs, milk and cinnamon. Soak the bread for 30 seconds per side.", "Fry in butter over medium heat for 3 minutes per side until deep golden."), notes=""),
    dict(photo="1551183053-bf91a1d81141", title="Mushroom Tagliatelle", tags=["Dinner", "Italian", "Vegetarian"], yield_="4 servings", servings=4, prep=10, cook=20, total=30, description="",
         ingredients=L("12 oz tagliatelle", "1 lb mixed mushrooms, sliced", "3 tbsp butter", "2 shallots, minced", "1/2 cup white wine", "3/4 cup heavy cream", "Thyme and Parmesan"),
         steps=L("Brown the mushrooms in butter for 8 minutes.", "Add the shallots, then the wine, and reduce by half.", "Stir in the cream, simmer 3 minutes, and toss with the cooked pasta."), notes=""),
    dict(photo="1555939594-58d7cb561ad1", title="Grilled Chicken Skewers", tags=["Dinner", "Grilling"], yield_="6 skewers", servings=6, prep=20, cook=12, total=92, description="",
         ingredients=L("2 lb chicken thighs, cubed", "1/4 cup soy sauce", "2 tbsp honey", "2 tbsp lime juice", "3 cloves garlic, grated", "1 tbsp smoked paprika"),
         steps=L("Marinate the chicken for 1 hour.", "Thread onto skewers and grill over high heat for 10 to 12 minutes, turning often."), notes=""),
    dict(photo="1549931319-a545dcf3bc73", cooked=1, title="Seeded Rye Bread", tags=["Bread", "Baking"], yield_="1 loaf", servings=14, prep=25, cook=50, total=300, description="",
         ingredients=L("300 g rye flour", "200 g bread flour", "400 g water", "10 g salt", "5 g instant yeast", "60 g sunflower seeds", "30 g flax seeds"),
         steps=L("Mix everything into a sticky dough and rest for 2 hours.", "Scrape into a loaf pan and proof for 90 minutes.", "Bake at 425°F for 50 minutes."), notes="", extra={"baker": True}),
    dict(photo="1546069901-ba9599a7e63c", title="Crispy Tofu Salad Bowl", tags=["Lunch", "Vegan"], yield_="2 bowls", servings=2, prep=15, cook=15, total=30, description="",
         ingredients=L("14 oz firm tofu, pressed", "2 tbsp cornstarch", "2 tbsp soy sauce", "4 cups chopped romaine", "1 cup shredded red cabbage", "1 bell pepper, sliced", "2 tbsp sesame oil"),
         steps=L("Toss the tofu with cornstarch and fry for 10 minutes until crisp.", "Glaze with soy sauce and serve over the vegetables."), notes=""),
    dict(photo=None, title="Quick Pickled Red Onions", tags=["Condiment"], yield_="1 jar", servings=8, prep=5, cook=0, total=35, description="",
         ingredients=L("1 large red onion, thinly sliced", "1/2 cup apple cider vinegar", "1/2 cup hot water", "1 tbsp sugar", "1 1/2 tsp kosher salt"),
         steps=L("Pack the onion into a jar.", "Stir the remaining ingredients until dissolved, pour over, and rest for 30 minutes."), notes=""),
]

try:
    call("POST", "/auth/setup", {"username": "adam", "name": "Adam", "password": "demo-password"})
except Exception:
    call("POST", "/auth/login", {"username": "adam", "password": "demo-password"})

ids = {}
for r in reversed(R):
    body = {"title": r["title"], "description": r["description"], "tags": r["tags"], "yield": r["yield_"], "servings": r["servings"], "prepMin": r["prep"], "cookMin": r["cook"],
            "totalMin": r["total"], "ingredients": r["ingredients"], "steps": r["steps"], "notes": r["notes"], "rating": r.get("rating", 0), "extra": r.get("extra")}
    rec = call("POST", "/recipes", body)
    ids[r["title"]] = rec["id"]
    if r["photo"]:
        call("POST", f"/recipes/{rec['id']}/image", raw=photo(r["photo"]), ctype="image/jpeg")
    if r.get("fav"):
        call("PUT", f"/recipes/{rec['id']}/favorite", {"on": True})
    for _ in range(r.get("cooked", 0)):
        call("POST", f"/recipes/{rec['id']}/cooked", {})
    print("added", r["title"])

book = lambda title, sub, color, style, chapters, entries, desc="": call("POST", "/cookbooks", {"title": title, "subtitle": sub, "color": color, "style": style, "chapters": chapters, "description": desc,
                                                                                                    "entries": [{"recipeId": ids[t], "chapter": c} for t, c in entries]})
book("The Bread Book", "Loaves worth the wait", "bay", "classic", ["Sourdough", "Yeasted"], [("Country Sourdough Loaf", "Sourdough"), ("Seeded Rye Bread", "Yeasted"), ("Margherita Pizza", "Yeasted")],
     "Everything here is measured by weight. Turn on baker's percentages to resize any loaf.")
book("Weeknight Dinners", "On the table in under an hour", "tomato", "modern", ["Pasta", "Mains"],
     [("Garlic Shrimp Spaghetti", "Pasta"), ("Mushroom Tagliatelle", "Pasta"), ("Farfalle with Pesto and Tomatoes", "Pasta"), ("Butter Chicken", "Mains"), ("Smash Burgers", "Mains"), ("Red Coconut Curry Soup", "Mains")])
book("Sweet Things", "", "butter", "classic", [], [("Brown Butter Chocolate Chip Cookies", ""), ("Fudgy Brownies", ""), ("Raspberry Layer Cake", "")])
book("Sunday Brunch", "Slow mornings", "ocean", "classic", [], [("Buttermilk Pancakes", ""), ("Brioche French Toast", "")])
call("POST", "/shopping", {"items": [{"text": t} for t in ["2 lb chicken thighs", "1 bunch cilantro", "3/4 cup heavy cream", "1 (14.5-ounce) can crushed tomatoes", "Sourdough bread", "6 lemons", "1 dozen eggs", "Basmati rice"]]})
print("done:", len(ids), "recipes")
