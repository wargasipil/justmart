#!/usr/bin/env python3
"""Generate the background music bed for the FAQ tutorial videos.

    python faq/tools/make_music.py           # writes faq/assets/music/corporate-bed.opus
    python faq/tools/make_music.py --wav     # also keep the intermediate .wav

Why synthesized rather than a downloaded track: the bed ships inside videos we
hand to shop owners, so it must be unambiguously free of third-party rights.
Generating it from this file makes the provenance the file itself, and makes the
music as regenerable as the recordings it sits under.

Brief: corporate, professional, simple, happy. That is a specific genre, and
each word pushed a concrete decision:

  corporate    a steady pulse rather than a floating texture — soft kick on 1
               and 3, shaker on eighths, harmony moving once per bar at 100 BPM.
  professional C - G - Am - F, the progression every explainer video is built
               on. It resolves, it never surprises, and it stays out of the way.
  simple       five voices, no counter-melody, and a 16-bar loop that is the
               same four bars four times. The ostinato is the hook.
  happy        major throughout, voiced high and close, bright low-pass and
               little reverb. The previous bed leaned on 7th chords and a long
               wash, which read as calm-but-wistful — the opposite of this.

The output is a SEAMLESS LOOP. The recorder loops it to whatever length a
tutorial happens to be (`-stream_loop -1` + explicit `-t`), so one asset serves
every question and no recording is bounded by the music.

Loop-safety is by construction: every mix-in wraps around the loop point
(`add`), and the reverb is a CIRCULAR convolution, so the tail of the last bar
feeds back into the first bar instead of being cut off at the seam. The pad
chords also OVERLAP into the following bar — an earlier version kept each
envelope inside its own bar, which guarantees a clean seam and made the pad drop
to silence on every chord change, so the bed pumped instead of breathed.
"""

from __future__ import annotations

import argparse
import subprocess
import sys
import wave
from pathlib import Path

import numpy as np

SR = 48_000  # matches Opus's native rate — no resample on encode
BPM = 100.0
BEAT = 60.0 / BPM  # 0.6 s
BAR = 4 * BEAT  # 2.4 s
BARS = 16
LOOP = BARS * BAR  # 38.4 s

# How far a pad chord runs past its own bar. This is the crossfade that keeps
# the pad continuous across a chord change. Short, because at 100 BPM a long
# overlap smears the harmony instead of connecting it.
CHORD_OVERLAP = 0.8
PAD_ATTACK = 0.3

# Target loudness of the asset itself. The recorder applies its own trim on top
# (MUSIC.gainDb in _recorder.ts), so this is "a normal music file", not "quiet
# enough for a video".
TARGET_RMS_DBFS = -20.0
PEAK_CEILING_DBFS = -1.0

# One chord per bar, four bars, repeated to fill the loop. Voiced close and high
# so the thirds ring — that is where "happy" actually lives, not in the tempo.
PROGRESSION = [
    ("C", [60, 64, 67], 48),  # C4 E4 G4 over C3
    ("G", [59, 62, 67], 43),  # B3 D4 G4 over G2
    ("Am", [60, 64, 69], 45),  # C4 E4 A4 over A2
    ("F", [60, 65, 69], 41),  # C4 F4 A4 over F2
]

# Mallet ostinato: which eighth-notes of the bar are struck, and which chord
# tone each takes. Syncopated (nothing on 4 or 8) so it drives without marching.
OSTINATO = [(0, 0), (2, 1), (3, 2), (5, 1), (6, 2)]

# Soft kick on 1 and 3 — the whole of the "corporate" pulse.
KICK_BEATS = [0.0, 2.0]

RNG = np.random.default_rng(20260811)


def midi_hz(note: float) -> float:
    return 440.0 * 2.0 ** ((note - 69) / 12.0)


def add(buf: np.ndarray, start_s: float, chunk: np.ndarray) -> None:
    """Mix `chunk` into `buf` at `start_s`, wrapping around the loop point."""
    i = int(round(start_s * SR)) % len(buf)
    n = len(chunk)
    end = i + n
    if end <= len(buf):
        buf[i:end] += chunk
    else:
        head = len(buf) - i
        buf[i:] += chunk[:head]
        buf[: n - head] += chunk[head:]


def swell(n: int, attack: float, release: float) -> np.ndarray:
    """Raised-cosine in / out. Reaches exactly 0 at both ends."""
    env = np.ones(n)
    a = min(int(attack * SR), n // 2)
    r = min(int(release * SR), n // 2)
    if a:
        env[:a] = 0.5 - 0.5 * np.cos(np.linspace(0, np.pi, a))
    if r:
        env[n - r :] = 0.5 + 0.5 * np.cos(np.linspace(0, np.pi, r))
    return env


def pad_voice(freq: float, dur: float) -> np.ndarray:
    """Warm sustained tone. Two barely-detuned layers for width, no more."""
    t = np.arange(int(dur * SR)) / SR
    out = np.zeros_like(t)
    for cents in (-3.0, 3.0):
        f = freq * 2.0 ** (cents / 1200.0)
        out += np.sin(2 * np.pi * f * t) + 0.22 * np.sin(2 * np.pi * 2 * f * t)
    return out / 2.0


def mallet(freq: float, dur: float = 1.1) -> np.ndarray:
    """Bright, short, wooden — the ostinato voice. Harmonics decay fastest."""
    t = np.arange(int(dur * SR)) / SR
    tone = (
        1.00 * np.sin(2 * np.pi * freq * t) * np.exp(-t * 4.5)
        + 0.42 * np.sin(2 * np.pi * 2 * freq * t) * np.exp(-t * 8.0)
        + 0.20 * np.sin(2 * np.pi * 3.01 * freq * t) * np.exp(-t * 13.0)
        + 0.08 * np.sin(2 * np.pi * 4.99 * freq * t) * np.exp(-t * 20.0)
    )
    n_at = int(0.003 * SR)
    tone[:n_at] *= np.linspace(0, 1, n_at)
    return tone


def bass_note(freq: float, dur: float = 1.0) -> np.ndarray:
    """Plucked, not a drone — a drone would blur the once-per-bar chord change."""
    t = np.arange(int(dur * SR)) / SR
    tone = np.sin(2 * np.pi * freq * t) * np.exp(-t * 3.2) + 0.25 * np.sin(
        2 * np.pi * 2 * freq * t
    ) * np.exp(-t * 6.0)
    n_at = int(0.006 * SR)
    tone[:n_at] *= np.linspace(0, 1, n_at)
    return tone


def kick(dur: float = 0.32) -> np.ndarray:
    """Pitch-swept sine. Felt more than heard at the level it sits."""
    t = np.arange(int(dur * SR)) / SR
    freq = 115.0 * np.exp(-t * 32.0) + 46.0
    tone = np.sin(2 * np.pi * np.cumsum(freq) / SR) * np.exp(-t * 12.0)
    n_at = int(0.002 * SR)
    tone[:n_at] *= np.linspace(0, 1, n_at)
    return tone


def shaker(dur: float = 0.085) -> np.ndarray:
    """Differentiated noise burst — cheap, and bright enough to read as a shaker."""
    n = int(dur * SR)
    noise = RNG.normal(0.0, 1.0, n)
    noise = np.diff(np.concatenate([[0.0], noise]))  # crude high-pass
    return noise * np.exp(-np.arange(n) / SR * 52.0)


def lowpass(x: np.ndarray, cutoff_hz: float) -> np.ndarray:
    """One-pole low-pass, run circularly so the filter state matches at the seam."""
    a = np.exp(-2 * np.pi * cutoff_hz / SR)
    doubled = np.concatenate([x, x])
    y = np.empty_like(doubled)
    prev = 0.0
    for i, v in enumerate(doubled):
        prev = (1 - a) * v + a * prev
        y[i] = prev
    return y[len(x) :]


def circular_reverb(x: np.ndarray, decay_s: float = 1.1, mix: float = 0.13) -> np.ndarray:
    """Convolve with a decaying-noise IR, wrapping around the loop.

    Short and low: a corporate bed wants air, not a hall. The calm version used
    nearly twice this and it turned the pulse to mush.
    """
    rng = np.random.default_rng(4242)
    n_ir = int(decay_s * SR)
    t = np.arange(n_ir) / SR
    ir = rng.normal(0, 1, n_ir) * np.exp(-t * (5.0 / decay_s))
    ir[: int(0.008 * SR)] = 0.0  # pre-delay
    ir /= np.sqrt(np.sum(ir**2))

    n = len(x)
    ir_p = np.zeros(n)
    ir_p[: min(n_ir, n)] = ir[: min(n_ir, n)]
    wet = np.real(np.fft.ifft(np.fft.fft(x) * np.fft.fft(ir_p)))
    peak = np.max(np.abs(wet))
    if peak > 0:
        wet *= np.max(np.abs(x)) / peak
    return (1 - mix) * x + mix * wet


def render() -> np.ndarray:
    n = int(LOOP * SR)
    left = np.zeros(n)
    right = np.zeros(n)

    def stereo_add(at: float, sig: np.ndarray, gain: float, pan: float = 0.0) -> None:
        add(left, at, sig * gain * (1 - 0.5 * pan))
        add(right, at, sig * gain * (1 + 0.5 * pan))

    voice_dur = BAR + CHORD_OVERLAP
    env = swell(int(voice_dur * SR), attack=PAD_ATTACK, release=CHORD_OVERLAP)

    for bar in range(BARS):
        _name, voicing, bass = PROGRESSION[bar % len(PROGRESSION)]
        bar_at = bar * BAR

        # Pad — the harmony, sitting under everything.
        for vi, note in enumerate(voicing):
            pan = -0.4 + 0.8 * vi / (len(voicing) - 1)
            stereo_add(bar_at, pad_voice(midi_hz(note), voice_dur) * env, 0.15, pan)

        # Bass — root on the downbeat, and again on 3 to keep the bar moving.
        stereo_add(bar_at, bass_note(midi_hz(bass)), 0.30)
        stereo_add(bar_at + 2 * BEAT, bass_note(midi_hz(bass), 0.7), 0.18)

        # Ostinato — chord tones an octave above the pad.
        for eighth, tone_idx in OSTINATO:
            note = voicing[tone_idx] + 12
            # Alternate the field bar to bar so four repeats do not sit still.
            pan = 0.22 if bar % 2 == 0 else -0.22
            stereo_add(bar_at + eighth * BEAT / 2, mallet(midi_hz(note)), 0.17, pan)

        # A single high chime opening each 4-bar phrase.
        if bar % 4 == 0:
            stereo_add(bar_at, mallet(midi_hz(voicing[1] + 24), 1.8), 0.085)

        for beat in KICK_BEATS:
            stereo_add(bar_at + beat * BEAT, kick(), 0.20)

        for eighth in range(8):
            gain = 0.040 if eighth % 2 == 0 else 0.024  # light downbeat accent
            stereo_add(bar_at + eighth * BEAT / 2, shaker(), gain, 0.3)

    # Brighter than the calm bed (2.6 kHz): "happy" is largely upper harmonics.
    left = circular_reverb(lowpass(left, 5200.0))
    right = circular_reverb(lowpass(right, 5200.0))

    stereo = np.stack([left, right], axis=1)

    rms = np.sqrt(np.mean(stereo**2))
    stereo *= 10 ** (TARGET_RMS_DBFS / 20) / rms
    peak = np.max(np.abs(stereo))
    ceiling = 10 ** (PEAK_CEILING_DBFS / 20)
    if peak > ceiling:
        stereo *= ceiling / peak
    return stereo


def write_wav(path: Path, audio: np.ndarray) -> None:
    pcm = np.clip(audio, -1.0, 1.0)
    pcm = (pcm * 32767.0).astype("<i2")
    path.parent.mkdir(parents=True, exist_ok=True)
    with wave.open(str(path), "wb") as w:
        w.setnchannels(2)
        w.setsampwidth(2)
        w.setframerate(SR)
        w.writeframes(pcm.tobytes())


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--out", default="faq/assets/music/corporate-bed.opus")
    ap.add_argument("--wav", action="store_true", help="keep the intermediate .wav")
    args = ap.parse_args()

    repo = Path(__file__).resolve().parents[2]
    out = (repo / args.out).resolve()
    wav = out.with_suffix(".wav")

    audio = render()
    write_wav(wav, audio)

    rms_db = 20 * np.log10(np.sqrt(np.mean(audio**2)))
    peak_db = 20 * np.log10(np.max(np.abs(audio)))
    seam = float(np.max(np.abs(audio[-1] - audio[0])))
    print(f"  loop     {LOOP:.2f}s @ {BPM:g} BPM, {BARS} bars")
    print(f"  level    RMS {rms_db:.1f} dBFS, peak {peak_db:.1f} dBFS")
    print(f"  seam     |last - first| = {seam:.5f} (0 = perfectly continuous)")

    try:
        subprocess.run(
            ["ffmpeg", "-y", "-loglevel", "error", "-i", str(wav),
             "-c:a", "libopus", "-b:a", "96k", "-vbr", "on", str(out)],
            check=True,
        )
    except FileNotFoundError:
        print("  ffmpeg not found — wrote the .wav only", file=sys.stderr)
        return 1
    if not args.wav:
        wav.unlink(missing_ok=True)
    print(f"  wrote    {out.relative_to(repo)} ({out.stat().st_size // 1024} KB)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
