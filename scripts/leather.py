#!/usr/bin/env python3
"""Draw the seamless leather grain used on cookbook covers.

    python3 scripts/leather.py        # writes web/src/assets/leather.webp

The tile is black or white with varying alpha (shade and sheen), so it sits on
any cover colour without blend modes. Needs numpy and Pillow.
"""
import numpy as np
from PIL import Image

N, rng = 384, np.random.default_rng(7)
yy, xx = np.mgrid[0:N, 0:N].astype(np.float32)


def worley(count, jitter=1.0):
    """Distance to the nearest and second-nearest of `count` wrapped points."""
    g = int(np.sqrt(count))
    cell = N / g
    px = ((np.arange(g)[None, :] + 0.5 + (rng.random((g, g)) - 0.5) * jitter) * cell).ravel()
    py = ((np.arange(g)[:, None] + 0.5 + (rng.random((g, g)) - 0.5) * jitter) * cell).ravel()
    d1 = np.full((N, N), 1e9, np.float32)
    d2 = np.full((N, N), 1e9, np.float32)
    for x, y in zip(px, py):
        dx = np.abs(xx - x); dx = np.minimum(dx, N - dx)
        dy = np.abs(yy - y); dy = np.minimum(dy, N - dy)
        d = np.sqrt(dx * dx + dy * dy)
        d2 = np.where(d < d1, d1, np.minimum(d2, d))
        d1 = np.minimum(d1, d)
    return d1, d2, cell


def noise(octaves):
    """Tileable fractal noise from wrapped, smoothed random grids."""
    out = np.zeros((N, N), np.float32)
    for o, amp in octaves:
        g = rng.random((o, o)).astype(np.float32)
        im = Image.fromarray(np.tile(g, (3, 3)) * 255).convert('L').resize((3 * N, 3 * N), Image.BICUBIC)
        out += amp * (np.asarray(im, np.float32)[N:2 * N, N:2 * N] / 255 - 0.5)
    return out


# Pebbles: raised cells with creases between them, at two sizes.
h = np.zeros((N, N), np.float32)
for count, depth in ((520, 1.0), (2100, 0.35)):
    d1, d2, cell = worley(count)
    crease = np.clip((d2 - d1) / (cell * 0.42), 0, 1)
    h += depth * (1 - (1 - crease) ** 2.2)
h += 0.9 * noise(((6, 1.0), (14, 0.6))) + 0.25 * noise(((64, 1.0), (128, 0.7)))

# Light it from the upper left.
gy, gx = np.gradient(np.pad(h, 2, mode='wrap'))
gx, gy = gx[2:-2, 2:-2], gy[2:-2, 2:-2]
k = 5.0
nx, ny, nz = -gx * k, -gy * k, np.ones_like(h)
ln = np.sqrt(nx * nx + ny * ny + nz * nz)
L = np.array([-0.5, -0.6, 0.62]); L /= np.linalg.norm(L)
shade = (nx * L[0] + ny * L[1] + nz * L[2]) / ln
shade = shade - shade.mean() + 0.18 * (h - h.mean())
shade /= np.abs(shade).max()

rgba = np.zeros((N, N, 4), np.uint8)
rgba[..., :3] = np.where(shade > 0, 255, 0)[..., None]
a = np.abs(shade)
rgba[..., 3] = np.clip(np.where(shade > 0, a ** 1.15 * 105, a ** 0.9 * 190), 0, 255)
Image.fromarray(rgba, 'RGBA').save('web/src/assets/leather.webp', quality=40, alpha_quality=55, method=6)
print('wrote web/src/assets/leather.webp')
