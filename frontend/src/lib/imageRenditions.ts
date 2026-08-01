// Two-rendition image pipeline — the single implementation behind the
// two-rendition HARD RULE in CLAUDE.md.
//
// Every image the user uploads is downscaled client-side into TWO renditions
// before it ever reaches the wire:
//
//   original — bounded to ORIGINAL_MAX_EDGE on its longest side, aspect kept.
//              This is what a full-size / lightbox view reads. It is NOT the
//              raw camera file: a modern phone photo is 4-12 MB, and storing
//              that to render a 32px avatar is the thing this prevents.
//   thumb    — a THUMB_EDGE square, center-cropped. This is what EVERY avatar,
//              table cell, list row and other fast-load surface reads.
//
// Both are re-encoded as JPEG: it is universally supported, small, and gives a
// predictable single content type for both renditions. Transparent source
// pixels are composited onto white first, so a PNG with alpha does not come
// back with a black background (JPEG has no alpha channel).
//
// A new image-upload feature reuses this module. Do not hand-roll a second
// canvas resize, and do not upload a raw File.

/** Longest edge of the stored original, in CSS pixels. */
export const ORIGINAL_MAX_EDGE = 1024;
/** Side of the square thumbnail, in CSS pixels. */
export const THUMB_EDGE = 128;
/** Reject absurd source files before decoding them into memory. */
export const MAX_SOURCE_BYTES = 15 * 1024 * 1024;

/** Source types the picker accepts. Matches the backend's allowlist minus SVG
 *  (script-capable, and these bytes are handed back to the browser to render). */
export const ACCEPTED_IMAGE_TYPES = ["image/jpeg", "image/png", "image/webp"] as const;
/** Ready to drop into an <input type="file" accept={...}>. */
export const ACCEPT_ATTR = ACCEPTED_IMAGE_TYPES.join(",");

/** The content type BOTH renditions are encoded as. */
export const RENDITION_CONTENT_TYPE = "image/jpeg";
const JPEG_QUALITY = 0.85;

// Uint8Array<ArrayBuffer> (not the default Uint8Array<ArrayBufferLike>): TS 5.7+
// made the backing buffer generic, and protobuf `bytes` fields want the
// non-shared form. Being explicit here keeps the cast out of every call site.
export type ImageBytes = Uint8Array<ArrayBuffer>;

export type ImageRenditions = {
  original: ImageBytes;
  thumb: ImageBytes;
  contentType: string;
};

export class ImageRenditionError extends Error {
  /** i18n key under `imageUpload.errors.*` — callers translate, never print raw. */
  readonly i18nKey: string;
  constructor(i18nKey: string) {
    super(i18nKey);
    this.i18nKey = i18nKey;
  }
}

/**
 * Decode `file` and produce both renditions. Throws ImageRenditionError with an
 * i18n key for anything the user can fix (wrong type, too big, undecodable).
 */
export async function makeImageRenditions(file: File): Promise<ImageRenditions> {
  if (!(ACCEPTED_IMAGE_TYPES as readonly string[]).includes(file.type)) {
    throw new ImageRenditionError("imageUpload.errors.type");
  }
  if (file.size > MAX_SOURCE_BYTES) {
    throw new ImageRenditionError("imageUpload.errors.tooLarge");
  }

  const bitmap = await decode(file);
  try {
    const original = await encodeContain(bitmap, ORIGINAL_MAX_EDGE);
    const thumb = await encodeCoverSquare(bitmap, THUMB_EDGE);
    return { original, thumb, contentType: RENDITION_CONTENT_TYPE };
  } finally {
    // Free the decoded bitmap explicitly — Safari in particular holds onto
    // these until GC, and a few large picks add up fast.
    bitmap.close?.();
  }
}

async function decode(file: File): Promise<ImageBitmap> {
  try {
    return await createImageBitmap(file);
  } catch {
    throw new ImageRenditionError("imageUpload.errors.decode");
  }
}

/** Scale to fit inside `maxEdge` (aspect preserved). Never upscales. */
async function encodeContain(bitmap: ImageBitmap, maxEdge: number): Promise<ImageBytes> {
  const scale = Math.min(1, maxEdge / Math.max(bitmap.width, bitmap.height));
  const w = Math.max(1, Math.round(bitmap.width * scale));
  const h = Math.max(1, Math.round(bitmap.height * scale));

  const { canvas, ctx } = newCanvas(w, h);
  ctx.drawImage(bitmap, 0, 0, w, h);
  return toJpegBytes(canvas);
}

/** Center-crop to a square, then scale to `edge`. Never upscales past source. */
async function encodeCoverSquare(bitmap: ImageBitmap, edge: number): Promise<ImageBytes> {
  const side = Math.min(bitmap.width, bitmap.height);
  const sx = (bitmap.width - side) / 2;
  const sy = (bitmap.height - side) / 2;
  const out = Math.max(1, Math.min(edge, side));

  const { canvas, ctx } = newCanvas(out, out);
  ctx.drawImage(bitmap, sx, sy, side, side, 0, 0, out, out);
  return toJpegBytes(canvas);
}

function newCanvas(w: number, h: number) {
  const canvas = document.createElement("canvas");
  canvas.width = w;
  canvas.height = h;
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new ImageRenditionError("imageUpload.errors.decode");
  // JPEG has no alpha: composite onto white so transparent PNG pixels render
  // white rather than black.
  ctx.fillStyle = "#ffffff";
  ctx.fillRect(0, 0, w, h);
  // Better downscale quality than the default nearest-ish sampling.
  ctx.imageSmoothingEnabled = true;
  ctx.imageSmoothingQuality = "high";
  return { canvas, ctx };
}

function toJpegBytes(canvas: HTMLCanvasElement): Promise<ImageBytes> {
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => {
        if (!blob) {
          reject(new ImageRenditionError("imageUpload.errors.encode"));
          return;
        }
        blob.arrayBuffer().then((buf) => resolve(new Uint8Array(buf)), reject);
      },
      RENDITION_CONTENT_TYPE,
      JPEG_QUALITY,
    );
  });
}

/**
 * Turn raw image bytes into a `data:` URL for an <img src>. Returns "" for
 * empty bytes so a caller can fall through to its placeholder.
 *
 * A data URL rather than URL.createObjectURL on purpose: an object URL has to
 * be revoked by hand, and these strings live in the TanStack Query cache where
 * there is no unmount hook to revoke them from — the leak would be silent. A
 * 128px thumb is a few KB, so the ~33% base64 overhead is not worth a
 * lifecycle bug.
 */
export function dataUrlFromBytes(bytes: Uint8Array, contentType: string): string {
  if (!bytes || bytes.length === 0) return "";
  // Chunked: String.fromCharCode(...bytes) overflows the call stack somewhere
  // around 100-200 KB of input.
  let binary = "";
  const CHUNK = 0x8000;
  for (let i = 0; i < bytes.length; i += CHUNK) {
    binary += String.fromCharCode(...bytes.subarray(i, i + CHUNK));
  }
  return `data:${contentType};base64,${btoa(binary)}`;
}
