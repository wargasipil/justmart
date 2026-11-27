// The POS prescription (resep) flow, pharmacy mode: the picker dialog, the
// Rx-required product whose add waits for a covering prescription, and the
// round-trip to the create-resep page. Split out of routes/Pos.tsx; behaviour
// unchanged.
import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";

import type { Product } from "../../gen/inventory_iface/v1/product_pb";
import { releaseModalBodyLock } from "../../lib/modalLock";
import { saveResepRoundTrip } from "../../lib/posStorage";
import type { PosCart } from "./usePosCart";

type Deferred = { product: Product; unitId: string };

export function usePosRxFlow({
  cart,
  onAdded,
}: {
  cart: Pick<PosCart, "attachPrescription" | "addLine" | "currentSale" | "keepDraftOnLeave">;
  /** Called after the deferred product landed in the cart (clears the search). */
  onAdded: () => void;
}) {
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [deferred, setDeferred] = useState<Deferred | null>(null);

  // Open the picker; `d` is the product whose add waits for the Rx (null = a
  // manual attach via F5 / the prescription bar).
  const openPicker = useCallback((d: Deferred | null = null) => {
    setDeferred(d);
    setOpen(true);
  }, []);

  const closePicker = useCallback(() => {
    setOpen(false);
    setDeferred(null);
  }, []);

  // Attach a prescription, then (if a product add was deferred pending the Rx)
  // add that product immediately — a single user gesture lands both.
  const onPick = async (prescriptionId: string) => {
    if (!(await cart.attachPrescription(prescriptionId))) return;
    setOpen(false);
    if (!deferred) return;
    const d = deferred;
    setDeferred(null);
    const added = await cart.addLine({
      productId: d.product.id,
      productUnitId: d.unitId,
      qty: 1,
    });
    if (added) onAdded();
  };

  // Persist the cart + the pending Rx-required product, then route to the
  // full create-resep page. On return (?attachRx=<id>) the POS mount restores
  // the draft and attaches the new resep. Navigation is DEFERRED until after
  // the picker dialog closes (see the effect below): navigating while a Chakra
  // Dialog is open leaves the body scroll-lock / inert state applied, which
  // freezes the whole destination page (nothing clickable/typable).
  const [pendingNav, setPendingNav] = useState<string | null>(null);
  const goCreateResep = useCallback(() => {
    const s = cart.currentSale();
    saveResepRoundTrip(
      s?.id ?? null,
      deferred ? { productId: deferred.product.id, unitId: deferred.unitId } : null,
    );
    cart.keepDraftOnLeave();
    const patient = s?.customerId ?? "";
    setOpen(false); // close the picker first so it unmounts + releases the body lock
    setPendingNav(`/prescriptions/new?returnTo=pos${patient ? `&patient=${patient}` : ""}`);
  }, [cart, deferred]);

  // Once the picker dialog has closed (committed), navigate on the next frame.
  // Chakra/Ark applies a body lock (pointer-events:none + overflow:hidden) and
  // sets aria-hidden on #root while a modal Dialog is open, and only restores
  // them on a normal close transition — NOT when the Dialog unmounts because we
  // navigated away. That leaves the destination page completely frozen. We've
  // closed the picker (so it's unmounted by now) but must release the residual
  // body lock ourselves before routing to the create-resep page.
  useEffect(() => {
    if (pendingNav == null) return;
    const id = requestAnimationFrame(() => {
      releaseModalBodyLock();
      navigate(pendingNav);
    });
    return () => cancelAnimationFrame(id);
  }, [pendingNav, navigate]);

  return {
    open,
    deferredName: deferred?.product.name ?? "",
    openPicker,
    closePicker,
    onPick,
    goCreateResep,
  };
}
