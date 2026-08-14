# The umbra mark

Four uneven strips laid over a lit opening, with a pair of tapered slits cut
into the deepest one. It is a sibling of the coilyco org avatars and shares
their ink, mint, and lilac.

The mark says what umbra does. The strips hold the opening's whole rim and never
cross its middle, because umbra guards a boundary and leaves the work inside it
alone. The slits are the framework looking out from behind that boundary.

The files live in [assets/mark/README.md](../assets/mark/README.md).

## What ships in assets/mark

- `umbra.svg` - the canon mark on the 400 avatar canvas. Its opaque ink field is
  part of the mark, so it stands alone on any background.
- `umbra-{400,256,128}.png` - coin rasters, transparent outside a disc just
  inside the ring, so each drops onto any surface without dark corners.
- `umbra-favicon-{64,32,16}.{svg,png}` - small sizes, each drawn on its own
  pixel grid. These keep the opaque field rather than the coin mask.

Do not resize the canon for a favicon. Nothing in it lands on a pixel boundary
at small sizes, so every edge blurs across two pixels. Use the size-specific
files, or regenerate.

## Geometry

In the 160 by 100 design box: a lilac octagon 140 by 92, chamfer 22. Four mint
strips of unequal width, top 30, right 16, bottom 18, left 28, each drawn to the
full edge and laid in the order bottom, right, top, left over an ink seam of 8.
That leaves an opening of 96 by 44. Two ink slits sit on the top strip's centre
line, 26 long and 8 thick, 8 apart. Mint ring r 165.5 stroke 12, lilac ring
r 153 stroke 13.

Four numbers are load-bearing rather than cosmetic.

- Strips are drawn to the full edge rather than pulled back from the corners.
  Every strip then leaves a corner along the same 45 diagonal whatever its
  width, so neighbours always meet and the unequal widths move the opening off
  centre instead of deforming it. Pulled back, the light flares at the corners
  and the opening reads as a bone.
- The seam is 8. At 5 it is a hairline that reads as a construction line, and at
  11 the four strips stop holding together as one object.
- Each slit keeps its thickness at the outer end and tapers to a point at the
  inner end. Reversed, the pair reads startled. Levelled, it reads calm.
- The favicons enlarge the emblem by 1.35 and drop the lilac ring. The emblem
  sits inside the ring rather than crossing it, so a small form that keeps both
  has about four pixels of emblem at 16px.

The slits need no size switch. They fall out on their own once one is under a
pixel thick, which is what the generator tests.

## Regenerating

The generator is `scripts/marks/umbra_mark.py` in `agentic-os-xxx`, and its
canon output is pixel-identical to what ships here. Its comments carry the
constraints that silently change the mark when they are broken.

Two forms are outstanding. The website canvas is a redraw at 500 with an ink
filter rather than a resize, and the lockup form over the coilyco S has not been
drawn.
