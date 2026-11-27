// The POS payment panel state: payment source, amount paid, and the editable
// biaya jasa (service fee) + cart-discount inputs mirrored from the draft.
// Split out of routes/Pos.tsx; behaviour unchanged. The commits go through the
// cart (usePosCart), so they queue behind any in-flight cart op.
import { useEffect, useState } from "react";

import type { DiscountType } from "../../components/DiscountField";
import { PaymentSource, type Sale } from "../../gen/pos_iface/v1/sale_pb";
import { discountToWire, type PosCart } from "./usePosCart";

export function usePosPayment(
  sale: Sale | null,
  cart: Pick<PosCart, "setServiceFee" | "setCartDiscount">,
) {
  const [paymentSource, setPaymentSource] = useState<PaymentSource>(PaymentSource.CASH);
  const [paidAmount, setPaidAmount] = useState("0");
  // Biaya jasa — defaults from the attached resep, editable here. Local input
  // state committed on blur via SetServiceFee (avoids an RPC per keystroke);
  // kept in sync whenever the sale's fee changes (e.g. attach).
  const [feeInput, setFeeInput] = useState("0");
  // Cart-level discount — Rp/% toggle + value, committed on blur / type change
  // via SetCartDiscount. value is human (minor units for FIXED, plain % for
  // PERCENT); kept in sync with the sale (the backend re-resolves PERCENT).
  const [cartDisc, setCartDisc] = useState<{ type: DiscountType; value: number }>({
    type: "FIXED",
    value: 0,
  });

  // Keep the fee input mirrored to the sale (attach snapshots the resep's fee).
  useEffect(() => {
    setFeeInput(String(Number(sale?.biayaJasa ?? 0n)));
  }, [sale?.biayaJasa]);

  // Keep the cart-discount inputs mirrored to the sale. The sale stores the raw
  // value (basis points when PERCENT); convert back to human percent here.
  useEffect(() => {
    const type = (sale?.cartDiscountType || "FIXED") as DiscountType;
    const raw = Number(sale?.cartDiscountValue ?? 0n);
    setCartDisc({ type, value: type === "PERCENT" ? raw / 100 : raw });
  }, [sale?.id, sale?.cartDiscountType, sale?.cartDiscountValue]);

  const commitFee = async () => {
    if (!sale) return;
    const v = Math.max(0, Number(feeInput || "0") || 0);
    if (v === Number(sale.biayaJasa)) return;
    await cart.setServiceFee(BigInt(v));
  };

  const commitCartDiscount = async (type: DiscountType, human: number) => {
    if (!sale) return;
    const wire = discountToWire(type, human);
    if (type === (sale.cartDiscountType || "FIXED") && wire === Number(sale.cartDiscountValue)) {
      return;
    }
    await cart.setCartDiscount(type, human);
  };

  const total = Number(sale?.total ?? 0n);
  const paidNum = Number(paidAmount || "0") || 0;
  const canComplete =
    !!sale &&
    sale.items.length > 0 &&
    (paymentSource !== PaymentSource.CASH || paidNum >= total);

  return {
    paymentSource,
    setPaymentSource,
    paidAmount,
    setPaidAmount,
    paidNum,
    feeInput,
    setFeeInput,
    commitFee,
    cartDisc,
    setCartDisc,
    commitCartDiscount,
    total,
    change: paidNum - total,
    canComplete,
    /** Back to the defaults for the next customer (after a completed sale). */
    resetPayment: () => {
      setPaidAmount("0");
      setPaymentSource(PaymentSource.CASH);
    },
  };
}
