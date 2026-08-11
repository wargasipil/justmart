# Music bed

`corporate-bed.opus` — a 38.4 s seamless loop, mastered to −20 dBFS RMS.

Corporate, professional, simple, happy: C – G – Am – F at 100 BPM, one chord per
bar, with a syncopated mallet ostinato over a soft kick and shaker. Sixteen bars
is the same four bars four times — the ostinato is the hook, and there is no
counter-melody to listen to instead of the captions.

**Generated, not licensed.** It comes out of
[`faq/tools/make_music.py`](../../tools/make_music.py), so its provenance is that
file. That matters because the bed ships inside videos we hand to shop owners:
there is no attribution to carry and no licence to keep track of.

```sh
python faq/tools/make_music.py          # regenerate
python faq/tools/make_music.py --wav    # keep the uncompressed intermediate
```

The recorder loops it to whatever length a tutorial happens to be and fades it
in and out ([`_recorder.ts`](../../../frontend/tests/faq/_recorder.ts), `MUSIC`),
so one asset scores every question and no recording is ever cut to fit the
music. It lands at about −23 dBFS in the finished video — present, but under the
threshold where it would compete with reading the captions.

## Changing the mood

Edit the generator, not the file. The knobs that actually move the character are
grouped at the top: `BPM`, `PROGRESSION` (chord per bar plus its bass note),
`OSTINATO` (which eighths are struck and which chord tone each takes), and
`KICK_BEATS`. Reverb `mix` and the low-pass cutoff are what separate "calm" from
"bright" — the first version of this bed was 75 BPM, 7th chords, a long wash and
no percussion, which read as calm-but-wistful rather than upbeat.

## Using a different track

Drop any ffmpeg-readable audio file in here and point `MUSIC.asset` at it. It
does not have to loop cleanly if it is longer than the videos, but it does have
to be something we have the right to redistribute — check that before swapping.

To leave one recording silent, pass `music: null` to `stage.save()`.
