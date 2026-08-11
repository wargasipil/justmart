// Turn the raw capture into the file that gets uploaded.
//
// Playwright writes VP8 in a .webm with no audio track. YouTube accepts that,
// but H.264/AAC in MP4 is the format it re-encodes from most predictably, and a
// video with no audio stream at all reads as broken on some players — so this
// step transcodes and scores in one pass.
//
// The music is the same generated bed the FAQ tutorials use
// (faq/assets/music/corporate-bed.opus): synthesized rather than licensed, so a
// video published to a public channel carries no third-party rights.
//
// Usage:
//   node release/encode.mjs                 # release/video/product-tour.webm -> .mp4
//   node release/encode.mjs --no-music      # keep it silent
//   node release/encode.mjs --in <webm> --out <mp4>

import { execFile } from "node:child_process";
import { access, mkdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

const run = promisify(execFile);
const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(HERE, "..");

const args = Object.fromEntries(
  process.argv.slice(2).reduce((acc, a, i, arr) => {
    if (a.startsWith("--")) acc.push([a.slice(2), arr[i + 1]]);
    return acc;
  }, []),
);

const IN = path.resolve(args.in ?? path.join(ROOT, "release", "video", "product-tour.webm"));
const OUT = path.resolve(args.out ?? path.join(ROOT, "release", "video", "justmart-product-tour-1080p.mp4"));
const MUSIC = path.join(ROOT, "faq", "assets", "music", "corporate-bed.opus");
const WITH_MUSIC = !("no-music" in args);

// The bed is mastered quiet (-24 LUFS) so it sits under the captions of an
// inline FAQ clip. This video has no narration competing with it and plays on
// phone speakers, so it is normalized up — but only to -16 LUFS: YouTube
// attenuates anything above about -14 and never boosts anything below, so
// louder here would just be turned back down.
const LOUDNESS = "I=-16:TP=-1.5:LRA=11";
const FADE_IN = 1.5;
const FADE_OUT = 2.5;

async function probeDuration(file) {
  const { stdout } = await run("ffprobe", [
    "-v", "error",
    "-show_entries", "format=duration",
    "-of", "default=nw=1:nk=1",
    file,
  ]);
  const seconds = Number.parseFloat(stdout.trim());
  if (!Number.isFinite(seconds)) throw new Error(`ffprobe gave no duration for ${file}`);
  return seconds;
}

await access(IN).catch(() => {
  throw new Error(`no capture at ${IN} — record it first with \`make release-video\``);
});

const duration = await probeDuration(IN);
const hasMusic = WITH_MUSIC && (await access(MUSIC).then(() => true).catch(() => false));
if (WITH_MUSIC && !hasMusic) {
  console.warn(`  ! music bed missing at ${MUSIC} — encoding silent`);
}

await mkdir(path.dirname(OUT), { recursive: true });

const video = [
  // CRF 18 / preset slow with no -tune: UI text is the whole subject, and the
  // tunes that would otherwise fit a screencast (stillimage) raise deblocking
  // and soften exactly the small type the viewer is meant to read.
  "-c:v", "libx264",
  "-preset", "slow",
  "-crf", "18",
  "-profile:v", "high",
  "-pix_fmt", "yuv420p", // required by YouTube; VP8 4:2:0 already, but be explicit
  "-r", "25",            // the capture's own rate — do not resample into 30
  "-g", "50",            // 2s keyframes
];

const args_ffmpeg = hasMusic
  ? [
      "-y", "-loglevel", "error", "-stats",
      "-i", IN,
      // The bed is 38s and the tour is minutes: loop it and let the video's own
      // length decide where the output ends (-t), not the music's.
      "-stream_loop", "-1", "-i", MUSIC,
      "-filter_complex",
      `[1:a]loudnorm=${LOUDNESS},` +
        `afade=t=in:st=0:d=${FADE_IN},` +
        `afade=t=out:st=${Math.max(0, duration - FADE_OUT).toFixed(3)}:d=${FADE_OUT}[a]`,
      "-map", "0:v", "-map", "[a]",
      ...video,
      "-c:a", "aac", "-b:a", "192k", "-ar", "48000", "-ac", "2",
      "-t", duration.toFixed(3),
      "-movflags", "+faststart",
      OUT,
    ]
  : [
      "-y", "-loglevel", "error", "-stats",
      "-i", IN,
      "-map", "0:v",
      ...video,
      "-an",
      "-movflags", "+faststart",
      OUT,
    ];

console.log(`encoding ${path.basename(IN)} (${duration.toFixed(1)}s)${hasMusic ? " + music" : " (silent)"} -> ${path.basename(OUT)}`);
await run("ffmpeg", args_ffmpeg, { maxBuffer: 64 * 1024 * 1024 });

const { stdout } = await run("ffprobe", [
  "-v", "error",
  "-show_entries", "format=duration,size,bit_rate",
  "-show_entries", "stream=codec_name,width,height,avg_frame_rate,channels",
  "-of", "default=nw=1",
  OUT,
]);
console.log(`\n${OUT}\n${stdout.trim()}\n`);
