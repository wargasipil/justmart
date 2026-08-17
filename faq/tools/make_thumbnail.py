#!/usr/bin/env python3
"""Build a YouTube thumbnail from a frame of an FAQ tutorial.

    python faq/tools/make_thumbnail.py can-an-accepted-restock-be-cancelled
    python faq/tools/make_thumbnail.py <slug> --variant b --out /tmp/try.jpg

Grabs a frame out of the video with ffmpeg, crops to the part that carries the
story, and composites it under a text panel. Output is 1280x720 JPEG — YouTube's
recommended size, well under the 2 MB limit.

The rule this is built around: **a thumbnail is read at about 168 px wide**, in a
list, by someone scrolling. So the hero line is two short words at ~100 px, the
supporting lines are there for the watch page and are allowed to be small, and
the screenshot is CROPPED TO ONE ELEMENT rather than shrunk whole — a full
1280x720 app screenshot scaled into a thumbnail is an unreadable grey smear.

Text also deliberately does NOT repeat the video title. The title asks the
question ("Salah Input Barang Masuk?"); the thumbnail answers it. Repeating it
spends the one visual asset saying something the viewer is already reading.
"""

from __future__ import annotations

import argparse
import subprocess
import sys
import tempfile
from dataclasses import dataclass, field
from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter, ImageFont

W, H = 1280, 720

BG = (15, 23, 42)  # slate-900, same ground as the recorder's cards
INK = (248, 250, 252)
MUTED = (148, 163, 184)
BLUE = (59, 130, 246)
RED = (239, 68, 68)
AMBER = (245, 158, 11)
GREEN = (34, 197, 94)

FONT_DIR = Path("C:/Windows/Fonts")
BLACK_FONT = "seguibl.ttf"  # Segoe UI Black
BOLD_FONT = "segoeuib.ttf"  # Segoe UI Bold


# Text lives left of the screenshot card. Nothing may cross this, so the hero
# size is DERIVED from it rather than trusted to a hand-picked constant — an
# earlier version hard-coded 104 px and a two-word line ran straight under the
# card.
PANEL_X = 62
PANEL_MAX_W = 455
HERO_MAX = 108
HERO_MIN = 54


def font(name: str, size: int) -> ImageFont.FreeTypeFont:
    for candidate in (FONT_DIR / name, Path("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf")):
        if candidate.exists():
            return ImageFont.truetype(str(candidate), size)
    return ImageFont.load_default(size)


def fit_font(lines: list[str], name: str, max_w: int, start: int, floor: int) -> ImageFont.FreeTypeFont:
    """Largest size at which every line fits `max_w`."""
    probe = ImageDraw.Draw(Image.new("RGB", (1, 1)))
    size = start
    while size > floor:
        f = font(name, size)
        if max(probe.textlength(ln, font=f) for ln in lines) <= max_w:
            return f
        size -= 2
    return font(name, floor)


@dataclass
class Variant:
    """One thumbnail design: which frame, which crop, and what it says."""

    at: float  # seconds into the video
    crop: tuple[int, int, int, int]  # source frame box to keep
    kicker: str
    kicker_color: tuple[int, int, int]
    hero: list[str]
    sub: str
    # Ring drawn on the cropped screenshot, in CROP coordinates: (x, y, w, h).
    ring: tuple[int, int, int, int] | None = None
    ring_color: tuple[int, int, int] = RED
    badge: str = ""
    badge_color: tuple[int, int, int] = field(default_factory=lambda: RED)


# Keyed by slug, then by variant letter: a frame time and a crop box only mean
# anything against one particular video, so a flat "a"/"b"/"c" namespace would
# have every new question fighting the last one for a letter.
VARIANTS: dict[str, dict[str, Variant]] = {}

VARIANTS["can-an-accepted-restock-be-cancelled"] = {
    # The action, caught mid-decision. The confirm dialog is the most
    # recognisable single frame in the whole video.
    "a": Variant(
        at=27.0,
        crop=(300, 66, 980, 460),
        kicker="SALAH CATAT?",
        kicker_color=AMBER,
        hero=["BATAL", "TERIMA"],
        sub="stok balik seperti semula",
        ring=(483, 340, 190, 46),
        badge="",
    ),
    # The outcome. Answers the title's question outright, which is what a
    # how-to thumbnail usually wants to do.
    "b": Variant(
        at=50.0,
        # The cancelled receipt row itself: RCV struck through, "Dibatalkan"
        # badge, reason underneath. A wide, short strip — that IS the evidence,
        # and including the rest of the page would shrink it to nothing.
        crop=(278, 440, 1000, 600),
        kicker="SALAH TERIMA?",
        kicker_color=AMBER,
        hero=["STOK", "BALIK"],
        sub="dokumennya tetap tercatat",
        ring=(8, 15, 222, 34),  # must clear the "Dibatalkan" badge, not clip it
        ring_color=RED,
        badge="",
    ),
    # The distinction. Not a how-to hook — use it if this becomes part of a
    # series, where the confusion itself is the draw.
    "c": Variant(
        at=27.0,
        crop=(300, 66, 980, 460),
        kicker="JANGAN KELIRU",
        kicker_color=RED,
        hero=["BATALKAN", "≠ RETUR"],
        sub="beda catatan, beda uang",
        ring=None,
        badge="",
    ),
}

VARIANTS["how-do-i-update-the-app"] = {
    # The evidence and the action in one crop: 1.1.0 against 1.1.2 with the
    # orange "Pembaruan tersedia" badge, and the button that closes the gap. The
    # crop runs 15 px past the button so the ring has room to draw outside it
    # instead of being clipped by the card edge.
    "a": Variant(
        at=20.0,
        crop=(462, 185, 1150, 480),
        kicker="VERSI BARU?",
        kicker_color=AMBER,
        hero=["CUKUP", "1 KLIK"],
        sub="dari dalam aplikasi",
        ring=(143, 240, 170, 34),
        ring_color=BLUE,
    ),
    # The confirmation, for a hook built on "it never updates behind your back"
    # rather than on how easy it is.
    "b": Variant(
        at=42.0,
        crop=(410, 255, 875, 400),
        kicker="TENANG",
        kicker_color=GREEN,
        hero=["TANYA", "DULU"],
        sub="tidak pernah update diam-diam",
        ring=None,
    ),
}


VARIANTS["how-do-i-enter-existing-stock"] = {
    # The preview table, taken BEFORE the spotlight ring is drawn — the ring
    # dims everything outside itself, and a crop of dimmed UI reads as a
    # switched-off screen at feed size. Five rows of green OK against one red
    # is the whole promise of the feature in one silhouette: you hand it a file
    # and it tells you what is wrong before anything is saved.
    "a": Variant(
        at=33.0,
        crop=(206, 246, 1074, 516),
        kicker="STOK LAMA?",
        kicker_color=AMBER,
        hero=["UNGGAH", "SEKALI"],
        sub="satu file CSV, tanpa order restock",
        ring=None,
    ),
    # The boundary instead: the second import of the same file. Use this one if
    # the entry is published alongside the restock videos, where the audience
    # already knows the feature and the question is what re-running costs.
    "b": Variant(
        at=96.0,
        crop=(192, 62, 1088, 340),
        kicker="IMPOR 2 KALI?",
        kicker_color=AMBER,
        hero=["TIDAK", "DOBEL"],
        sub="asalkan nomor batch diisi",
        ring=(19, 84, 186, 28),  # the "2 dilewati" in the summary line
        ring_color=GREEN,
    ),
}


VARIANTS["how-do-i-add-a-cloudflare-tunnel-token"] = {
    # The mistake, caught at the moment it is refused: a whole "cloudflared.exe
    # service install ..." line in the field, red border, red message. That is
    # the paste almost everyone tries first, because it is what Cloudflare puts
    # on screen — so showing it rejected answers the question faster than
    # showing it done right.
    #
    # 29.7 s is deliberate: the error appears while the click's trailing beat is
    # still running, and the spotlight lands ~0.8 s later. The ring dims
    # everything outside itself, and a crop of dimmed UI reads as a switched-off
    # screen at feed size.
    "a": Variant(
        at=29.7,
        crop=(455, 245, 995, 400),
        kicker="SALAH TEMPEL?",
        kicker_color=AMBER,
        hero=["TOKEN", "SAJA"],
        sub="bukan satu baris perintah",
        # The input holding the rejected command line. Doubling the field's own
        # red border is what makes the "this is wrong" read survive the
        # downscale; kept clear of the message underneath it.
        ring=(10, 31, 518, 41),
        ring_color=RED,
    ),
    # The gotcha instead of the how-to. Use this one if the entry is published
    # next to the other Settings videos, where the audience already knows where
    # the field is and the thing that actually catches them is that saving alone
    # does nothing.
    "b": Variant(
        at=54.0,
        crop=(455, 145, 995, 320),
        kicker="SUDAH DISIMPAN?",
        kicker_color=AMBER,
        hero=["BELUM", "MENYALA"],
        sub="tunnel ikut jalan setelah restart",
        ring=(8, 78, 522, 85),  # the "jalankan ulang" notice
        ring_color=BLUE,
    ),
}


VARIANTS["how-does-a-cashier-create-an-order"] = {
    # The outcome, cropped to the receipt dialog alone — it is the one element
    # in the whole video that says "a sale happened": the invoice number, the
    # two lines, the change. The frame is taken after the spotlight has been
    # released, so the page behind carries only the modal backdrop and not the
    # recorder's dimming ring.
    "a": Variant(
        at=121.0,
        crop=(378, 56, 902, 382),
        kicker="KASIR BARU?",
        kicker_color=AMBER,
        hero=["KLIK", "LALU F8"],
        sub="satu layar, dari cari sampai struk",
        ring=None,
    ),
    # The cart panel instead, for a hook built on "it does the arithmetic"
    # rather than on how few steps it is. Cropped short of the Selesaikan
    # button on purpose: the caption bar sits over its left edge at every
    # moment the cart is paid for.
    "b": Variant(
        at=110.0,
        crop=(790, 170, 1280, 620),
        kicker="BUKAN NOTA MANUAL",
        kicker_color=BLUE,
        hero=["HITUNG", "SENDIRI"],
        sub="total, kembalian, sisa stok",
        ring=(8, 258, 467, 22),  # the TOTAL row
        ring_color=BLUE,
    ),
}


VARIANTS["how-do-i-create-a-restock-order"] = {
    # The finished order, still in Draf, with the Kirim button ringed — the
    # outcome the title asks for, and the one crop that carries the whole point
    # in one look: an order exists, nothing has been received yet.
    "a": Variant(
        at=124.0,
        # x starts in the gutter, not at 264: the content column and the PO
        # number begin on the same pixel, so cropping at the column edge shaves
        # the "P" off the order number.
        crop=(252, 205, 1262, 600),
        kicker="STOK MENIPIS?",
        kicker_color=AMBER,
        hero=["PESAN", "DULU"],
        sub="stok bertambah saat barangnya diterima",
        ring=(800, 16, 87, 35),  # the Kirim button
        ring_color=BLUE,
    ),
    # The price-agreement warning instead, for a hook about the check rather
    # than the steps. Frame 74 is deliberate: it is the one second where the row
    # is already red but the recorder's own spotlight is off, so the page is not
    # dimmed under the crop.
    "b": Variant(
        at=74.0,
        crop=(280, 452, 940, 592),
        kicker="KEMAHALAN?",
        kicker_color=RED,
        hero=["HARGA", "DICEK"],
        sub="kesepakatan pemasok ikut diperiksa",
        ring=(17, 108, 133, 18),
        ring_color=RED,
    ),
}


VARIANTS["how-do-i-change-the-shop-name"] = {
    # The half people forget: the receipt header, with a real shop address in
    # it. Cropped to that one field — it reads as "the text printed on a struk"
    # even at feed size, where the words themselves are gone. The textarea is
    # focused in this frame, so its blue outline already does the ring's job.
    "a": Variant(
        at=60.0,
        # Bottom edge stops just under the textarea: two rows further down and
        # the card ends mid-way through the "Footer" label, which reads as a
        # rendering fault rather than a crop.
        crop=(450, 448, 940, 566),
        kicker="GANTI NAMA TOKO?",
        kicker_color=AMBER,
        hero=["DUA", "TEMPAT"],
        sub="di aplikasi, dan di struk",
        ring=None,
    ),
    # The other half — the app-title field with its helper line spelling out the
    # three surfaces it feeds. Use it if the hook should be "it changes
    # instantly" rather than "there are two of them".
    "b": Variant(
        at=22.0,
        crop=(455, 158, 930, 268),
        kicker="NAMA DI APLIKASI",
        kicker_color=BLUE,
        hero=["SATU", "KOLOM"],
        sub="sidebar, tab browser, login",
        ring=None,
    ),
}

VARIANTS["how-do-i-print-a-product-barcode-label"] = {
    # The label preview itself, cropped to just the three things that get
    # printed: name, barcode, price. It reads as "a price sticker" at feed size
    # even once the words are gone, which is the whole job. The dialog chrome
    # around it adds nothing at 168 px.
    "a": Variant(
        at=42.0,
        crop=(495, 375, 790, 522),
        kicker="PERLU LABEL RAK?",
        kicker_color=BLUE,
        # The title asks how to print; the answer is that there is no separate
        # barcode to manage — the SKU already is one.
        hero=["SKU", "= BARCODE"],
        sub="cetak langsung dari halaman produk",
        ring=None,
    ),
    # The trap instead: a barcode too wide for the paper, bursting out of the
    # dialog with the refusal beside it. Use this if the hook should be the
    # mistake rather than the steps — it is the more arresting frame, but it
    # answers a question the title does not ask.
    "b": Variant(
        at=77.0,
        crop=(288, 8, 1272, 500),
        kicker="BARCODE TAK TERBACA?",
        kicker_color=RED,
        hero=["SKU", "PENDEK"],
        sub="kertas struk cuma muat sekitar 12 huruf",
        ring=None,
    ),
}


VARIANTS["can-an-order-be-cancelled"] = {
    # The confirm dialog, cropped to itself. It is the one element in the video
    # that reads as "a refund is being made" even once the words are gone: a
    # small card with a toggle and an orange button. The frame is taken while
    # the spotlight is off, so the page behind it is not dimmed — a dimmed crop
    # reads as a switched-off screen at feed size.
    "a": Variant(
        at=40.0,
        crop=(416, 62, 864, 390),
        kicker="SALAH TRANSAKSI?",
        kicker_color=AMBER,
        hero=["UANG", "KEMBALI"],
        sub="stok ikut balik kalau Anda mau",
        # The switch row: the only real decision in the whole dialog. The width
        # has to clear the end of "ke stok" — at 218 the ring cut through the
        # last letter, which reads as a rendering fault rather than a highlight.
        ring=(24, 212, 236, 36),
        ring_color=BLUE,
    ),
    # The outcome instead, for a hook built on the thing people get wrong —
    # they expect the order to vanish. Crop starts below the success toast and
    # runs to the bottom of the Refund panel, so it carries the orange
    # Dikembalikan badge, the totals, and the recorded reason in one block.
    "b": Variant(
        at=56.0,
        crop=(250, 100, 1270, 585),
        kicker="BATAL BUKAN HAPUS",
        kicker_color=BLUE,
        hero=["TETAP", "TERCATAT"],
        sub="lengkap dengan alasan dan waktunya",
        ring=None,
    ),
}


def grab_frame(video: Path, at: float) -> Image.Image:
    with tempfile.TemporaryDirectory() as tmp:
        out = Path(tmp) / "frame.png"
        subprocess.run(
            ["ffmpeg", "-y", "-loglevel", "error", "-ss", f"{at}", "-i", str(video),
             "-frames:v", "1", str(out)],
            check=True,
        )
        return Image.open(out).convert("RGB").copy()


def shadowed(size: tuple[int, int], box, radius: int, blur: int, spread: int) -> Image.Image:
    """A blurred black rounded-rect, for dropping under a card."""
    layer = Image.new("L", size, 0)
    d = ImageDraw.Draw(layer)
    x0, y0, x1, y1 = box
    d.rounded_rectangle((x0 - spread, y0 - spread, x1 + spread, y1 + spread), radius, fill=190)
    return layer.filter(ImageFilter.GaussianBlur(blur))


def build(video: Path, v: Variant) -> Image.Image:
    canvas = Image.new("RGB", (W, H), BG)
    draw = ImageDraw.Draw(canvas)

    # --- background wash: a big soft blue glow so the panel is not flat black
    glow = Image.new("RGB", (W, H), BG)
    ImageDraw.Draw(glow).ellipse((-260, 210, 620, 1010), fill=(28, 48, 92))
    canvas = Image.blend(canvas, glow.filter(ImageFilter.GaussianBlur(120)), 0.85)
    draw = ImageDraw.Draw(canvas)

    # --- the screenshot card ------------------------------------------------
    shot = grab_frame(video, v.at).crop(v.crop)
    cw, ch = shot.size

    card_w = 700
    scale = card_w / cw
    card = shot.resize((card_w, int(ch * scale)), Image.LANCZOS)

    if v.ring:
        rd = ImageDraw.Draw(card)
        rx, ry, rw, rh = (int(n * scale) for n in v.ring)
        for i, alpha_w in enumerate((7, 5)):
            rd.rounded_rectangle(
                (rx - 6 - i * 3, ry - 6 - i * 3, rx + rw + 6 + i * 3, ry + rh + 6 + i * 3),
                radius=14 + i * 3,
                outline=v.ring_color,
                width=alpha_w if i == 0 else 2,
            )

    # Rounded corners + a shadow so the card sits on the panel instead of
    # floating on it.
    mask = Image.new("L", card.size, 0)
    ImageDraw.Draw(mask).rounded_rectangle((0, 0, card.size[0] - 1, card.size[1] - 1), 18, fill=255)

    cx, cy = 545, (H - card.size[1]) // 2
    box = (cx, cy, cx + card.size[0], cy + card.size[1])
    canvas.paste(Image.new("RGB", (W, H), (0, 0, 0)), (0, 0), shadowed((W, H), box, 22, 26, 10))
    canvas.paste(card, (cx, cy), mask)
    ImageDraw.Draw(canvas).rounded_rectangle(box, 18, outline=(51, 65, 85), width=2)
    draw = ImageDraw.Draw(canvas)

    # --- text panel ----------------------------------------------------------
    x = PANEL_X
    f_kicker = fit_font([v.kicker], BOLD_FONT, PANEL_MAX_W, 34, 22)
    f_hero = fit_font(v.hero, BLACK_FONT, PANEL_MAX_W, HERO_MAX, HERO_MIN)
    f_sub = fit_font([v.sub], BOLD_FONT, PANEL_MAX_W, 32, 20)

    line_h = int(f_hero.size * 1.0)
    block_h = 66 + line_h * len(v.hero) + 22 + int(f_sub.size * 1.3)
    y = (H - block_h) // 2

    draw.rectangle((x, y + 6, x + 8, y + 40), fill=v.kicker_color)
    draw.text((x + 26, y), v.kicker, font=f_kicker, fill=v.kicker_color)
    y += 66

    for line in v.hero:
        draw.text((x, y), line, font=f_hero, fill=INK, stroke_width=3, stroke_fill=(8, 13, 26))
        y += line_h
    y += 22

    draw.text((x, y), v.sub, font=f_sub, fill=MUTED)

    if v.badge:
        f_badge = font(BLACK_FONT, 30)
        bw = draw.textlength(v.badge, font=f_badge)
        draw.rounded_rectangle((x, y + 66, x + bw + 40, y + 122), 12, fill=v.badge_color)
        draw.text((x + 20, y + 76), v.badge, font=f_badge, fill=INK)

    return canvas


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("slug", help="folder under faq/videos/")
    ap.add_argument("--variant", default="a")
    ap.add_argument("--out", default="", help="defaults to faq/videos/<slug>/thumbnail.jpg")
    args = ap.parse_args()

    repo = Path(__file__).resolve().parents[2]
    folder = repo / "faq" / "videos" / args.slug
    video = folder / "tutorial.webm"
    if not video.exists():
        print(f"no video at {video}", file=sys.stderr)
        return 1

    designs = VARIANTS.get(args.slug)
    if not designs:
        print(f"no thumbnail designs for {args.slug} — add one to VARIANTS", file=sys.stderr)
        return 1
    if args.variant not in designs:
        print(f"unknown variant {args.variant!r}; have {', '.join(sorted(designs))}", file=sys.stderr)
        return 1

    out = Path(args.out).resolve() if args.out else folder / "thumbnail.jpg"
    out.parent.mkdir(parents=True, exist_ok=True)

    img = build(video, designs[args.variant])
    img.save(out, quality=92, optimize=True) if out.suffix.lower() in (".jpg", ".jpeg") else img.save(out)

    kb = out.stat().st_size // 1024
    print(f"  variant {args.variant} -> {out} ({W}x{H}, {kb} KB)")
    if kb > 2048:
        print("  ! over YouTube's 2 MB limit — lower the quality", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
