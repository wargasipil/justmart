/**
 * Release the body lock Chakra/Ark leaves behind when a modal Dialog is
 * unmounted via navigation instead of a normal close transition.
 *
 * While a modal Dialog is open Ark sets `pointer-events: none` +
 * `overflow: hidden` on <body> and `aria-hidden` on #root, and restores them
 * only on a proper open → false transition — NOT when the Dialog unmounts
 * because the route changed underneath it. Without this the destination page is
 * completely frozen: nothing clickable, nothing typable.
 *
 * Apply the same pattern anywhere you navigate out of a route while a Chakra
 * Dialog is open: close the dialog first, then call this on the next frame,
 * then navigate. See routes/Pos.tsx's create-resep flow for the reference use.
 */
export function releaseModalBodyLock() {
  document.body.style.removeProperty("pointer-events");
  document.body.style.removeProperty("overflow");
  document.getElementById("root")?.removeAttribute("aria-hidden");
}
