#!/usr/bin/env python3
"""Draw the two textures that make the leather on cookbook covers.

    python3 scripts/leather.py

    web/src/assets/leather.webp   a seamless tile of fine, shallow grain
    web/src/assets/patina.webp    one cover's worth of wear: mottled dye, rubbed patches and scratches

Both are a dark or a pale colour with varying alpha, so they sit on any cover
colour without blend modes. Needs numpy and Pillow.
"""
import numpy as np
from PIL import Image, ImageDraw, ImageFilter

rng = np.random.default_rng(11)


def noise(shape, octaves):
    """Fractal noise in -1..1 from smoothed random grids (wraps at the edges)."""
    h, w = shape
    out = np.zeros(shape, np.float32)
    for (gy, gx), amp in octaves:
        g = rng.random((gy, gx)).astype(np.float32)
        im = Image.fromarray(np.tile(g, (3, 3)) * 255).convert('L').resize((3 * w, 3 * h), Image.BICUBIC)
        out += amp * (np.asarray(im, np.float32)[h:2 * h, w:2 * w] / 255 - 0.5)
    return out / np.abs(out).max()


def step(a, b, x):
    t = np.clip((x - a) / (b - a), 0, 1)
    return t * t * (3 - 2 * t)


def rgba(signed, dark, pale, dark_gain, pale_gain):
    """Negative values become `dark`, positive ones `pale`; the size becomes alpha."""
    out = np.zeros(signed.shape + (4,), np.uint8)
    neg = signed < 0
    out[..., :3] = np.where(neg[..., None], dark, pale)
    out[..., 3] = np.clip(np.where(neg, -signed * dark_gain, signed * pale_gain) * 255, 0, 255)
    return out


# ---------------------------------------------------------------- grain
N = 384
yy, xx = np.mgrid[0:N, 0:N].astype(np.float32)


def worley(count):
    """Distance to the nearest and second-nearest of `count` wrapped, jittered points."""
    g = int(np.sqrt(count))
    cell = N / g
    px = ((np.arange(g)[None, :] + rng.random((g, g))) * cell).ravel()
    py = ((np.arange(g)[:, None] + rng.random((g, g))) * cell).ravel()
    d1 = np.full((N, N), 1e9, np.float32)
    d2 = np.full((N, N), 1e9, np.float32)
    for x, y in zip(px, py):
        dx = np.abs(xx - x); dx = np.minimum(dx, N - dx)
        dy = np.abs(yy - y); dy = np.minimum(dy, N - dy)
        d = np.sqrt(dx * dx + dy * dy)
        d2 = np.where(d < d1, d1, np.minimum(d2, d))
        d1 = np.minimum(d1, d)
    return d1, d2, cell


# Small flat pebbles between hairline creases, a little uneven in depth, with pores.
height = np.zeros((N, N), np.float32)
for count, depth in ((1500, 1.0), (5600, 0.4)):
    d1, d2, cell = worley(count)
    height += depth * step(0.0, 0.3, (d2 - d1) / cell)
height *= 0.75 + 0.25 * noise((N, N), (((5, 5), 1.0), ((11, 11), 0.5)))
height += 0.22 * noise((N, N), (((90, 90), 1.0), ((180, 180), 0.8)))
pores = (rng.random((N, N)) > 0.9965).astype(np.float32)
height -= 0.5 * np.asarray(Image.fromarray(pores * 255).convert('L').filter(ImageFilter.GaussianBlur(0.8)), np.float32) / 255 * 6

gy, gx = np.gradient(np.pad(height, 2, mode='wrap'))
gx, gy = gx[2:-2, 2:-2], gy[2:-2, 2:-2]
shade = -(gx * 0.55 + gy * 0.65) + 0.3 * (height - height.mean())
shade = np.clip(shade / np.percentile(np.abs(shade), 99.5), -1, 1)
Image.fromarray(rgba(shade, (22, 13, 8), (255, 243, 222), 0.34, 0.16), 'RGBA').save('web/src/assets/leather.webp', quality=30, alpha_quality=38, method=6)

# ---------------------------------------------------------------- patina
W, H = 540, 720
S = (H, W)

# Dye that took unevenly and darkened with handling; paler where it has dried out.
mottle = noise(S, (((3, 2), 1.0), ((6, 5), 0.8), ((13, 10), 0.55), ((28, 21), 0.35), ((60, 45), 0.2)))
dark = 0.42 * np.clip(mottle, 0, 1) ** 1.3 + 0.16 * step(0.1, 0.9, noise(S, (((5, 4), 1.0), ((11, 8), 0.6), ((24, 18), 0.3))))
pale = 0.24 * np.clip(-mottle - 0.1, 0, 1) ** 1.2

# Rubbed patches: the surface abraded pale, faintly along one direction.
streaks = Image.fromarray(((noise((900, 900), (((300, 90), 1.0), ((600, 220), 0.8), ((900, 450), 0.5))) * 0.5 + 0.5) * 255).astype(np.uint8))
for angle, lo in ((24, 0.25), (-58, 0.42)):
    zone = step(lo, lo + 0.35, noise(S, (((4, 3), 1.0), ((9, 7), 0.6), ((20, 15), 0.3))))
    grain = np.asarray(streaks.rotate(angle, Image.BICUBIC).crop((180, 90, 180 + W, 90 + H)), np.float32) / 255
    pale += 0.15 * zone * step(0.45, 0.9, grain)

# Scratches and nicks, mostly pale, a few cut dark.
marks = Image.new('L', (W * 2, H * 2), 0)
cuts = Image.new('L', (W * 2, H * 2), 0)
for layer, count, reach in ((marks, 170, 80), (marks, 16, 380), (cuts, 18, 120)):
    pen = ImageDraw.Draw(layer)
    for _ in range(count):
        x, y, a = rng.random() * W * 2, rng.random() * H * 2, rng.random() * np.pi
        n = rng.random() ** 2 * reach + 8
        bend = (rng.random() - 0.5) * n * 0.25
        pts = [(x + np.cos(a) * n * t + np.sin(a) * bend * np.sin(t * np.pi), y + np.sin(a) * n * t - np.cos(a) * bend * np.sin(t * np.pi)) for t in np.linspace(0, 1, 9)]
        pen.line(pts, fill=int(70 + rng.random() * 150), width=1 if rng.random() < 0.8 else 2)
small = lambda im: np.asarray(im.filter(ImageFilter.GaussianBlur(0.9)).resize((W, H), Image.LANCZOS), np.float32) / 255
pale += 0.42 * small(marks)
dark += 0.4 * small(cuts)

# The stain runs from warm umber to a cold near-black, the wear from tan to grey.
warm = (noise(S, (((4, 3), 1.0), ((9, 7), 0.5))) * 0.5 + 0.5)[..., None]
stain = (62, 32, 12) * warm + (20, 17, 16) * (1 - warm)
worn = (214, 184, 140) * warm + (226, 222, 212) * (1 - warm)
signed = pale - dark
out = np.zeros(S + (4,), np.uint8)
out[..., :3] = np.where((signed < 0)[..., None], stain, worn)
out[..., 3] = np.clip(np.abs(signed) * 255, 0, 255)
Image.fromarray(out, 'RGBA').save('web/src/assets/patina.webp', quality=40, alpha_quality=82, method=6)
print('wrote web/src/assets/leather.webp and patina.webp')
