import { ConnectError } from "@connectrpc/connect";
import type { FieldValues, Path, UseFormReturn } from "react-hook-form";

import i18n from "./i18n";

// Maps a backend STABLE token (dotted "<domain>.<reason>" returned by
// common.TokenError on the Go side) to a translated message and, where a form
// field exists, the field to attach it to.
//
// TOKEN CATALOG MIRROR — keep in sync with backend/internal/service/common/errors.go
// (the TokenError(...) call sites). Tokens NOT listed here are still safe: an
// unknown token-shaped message falls back to errors.generic (translateServerError),
// never leaking a raw token or DB-constraint string.
type ServerErrorEntry = { field?: string; i18nKey: string };

export const SERVER_ERRORS: Record<string, ServerErrorEntry> = {
  // product
  "product.required": { i18nKey: "validation.required" },
  "product.unit_price_negative": { field: "unitPrice", i18nKey: "validation.minValue" },
  "product.sku_taken": { field: "sku", i18nKey: "serverErrors.product.skuTaken" },
  "product.sku_not_printable": { i18nKey: "serverErrors.product.skuNotPrintable" },
  "product.sku_too_wide_for_paper": { i18nKey: "serverErrors.product.skuTooWideForPaper" },
  "product.unit_archived": { i18nKey: "serverErrors.product.unitArchived" },
  // supplier
  "supplier.required": { i18nKey: "validation.required" },
  "supplier.code_taken": { field: "code", i18nKey: "serverErrors.supplier.codeTaken" },
  "supplier.name_taken": { field: "name", i18nKey: "serverErrors.supplier.nameTaken" },
  // price agreement
  "price_agreement.required": { i18nKey: "validation.required" },
  "price_agreement.items_required": { i18nKey: "serverErrors.common.itemsRequired" },
  "price_agreement.price_invalid": { field: "price", i18nKey: "serverErrors.common.amountNegative" },
  "price_agreement.bad_dates": { field: "validUntil", i18nKey: "serverErrors.priceAgreement.badDates" },
  "price_agreement.supplier_missing": { field: "supplierId", i18nKey: "serverErrors.priceAgreement.supplierMissing" },
  "price_agreement.product_missing": { field: "productId", i18nKey: "serverErrors.priceAgreement.productMissing" },
  "price_agreement.exists": { field: "productUnitId", i18nKey: "serverErrors.priceAgreement.exists" },
  // avatar (profile picture upload)
  "avatar.image_required": { i18nKey: "serverErrors.avatar.imageRequired" },
  "avatar.thumb_required": { i18nKey: "serverErrors.avatar.thumbRequired" },
  "avatar.image_too_large": { i18nKey: "serverErrors.avatar.imageTooLarge" },
  "avatar.thumb_too_large": { i18nKey: "serverErrors.avatar.thumbTooLarge" },
  "avatar.content_type_invalid": { i18nKey: "serverErrors.avatar.contentTypeInvalid" },
  // product image (catalog photo upload)
  "product_image.image_required": { i18nKey: "serverErrors.productImage.imageRequired" },
  "product_image.thumb_required": { i18nKey: "serverErrors.productImage.thumbRequired" },
  "product_image.image_too_large": { i18nKey: "serverErrors.productImage.imageTooLarge" },
  "product_image.thumb_too_large": { i18nKey: "serverErrors.productImage.thumbTooLarge" },
  "product_image.content_type_invalid": { i18nKey: "serverErrors.productImage.contentTypeInvalid" },
  // product discount
  "product_discount.product_missing": { i18nKey: "serverErrors.productDiscount.productMissing" },
  "product_discount.value_invalid": { field: "value", i18nKey: "serverErrors.productDiscount.valueInvalid" },
  "product_discount.type_invalid": { i18nKey: "serverErrors.productDiscount.valueInvalid" },
  "product_discount.threshold_invalid": { field: "minQty", i18nKey: "serverErrors.productDiscount.thresholdInvalid" },
  "product_discount.bad_expiry": { field: "expiresAt", i18nKey: "serverErrors.productDiscount.badExpiry" },
  "product_discount.unit_invalid": { field: "minQtyUnitId", i18nKey: "serverErrors.productDiscount.unitInvalid" },

  // product price tier (grosir)
  "product_price_tier.product_missing": { i18nKey: "serverErrors.productPriceTier.productMissing" },
  "product_price_tier.unit_invalid": { field: "productUnitId", i18nKey: "serverErrors.productPriceTier.unitInvalid" },
  "product_price_tier.min_qty_invalid": { field: "minQty", i18nKey: "serverErrors.productPriceTier.minQtyInvalid" },
  "product_price_tier.price_invalid": { field: "price", i18nKey: "serverErrors.productPriceTier.priceInvalid" },
  "product_price_tier.tier_taken": { field: "minQty", i18nKey: "serverErrors.productPriceTier.tierTaken" },
  // warehouse
  "warehouse.required": { i18nKey: "validation.required" },
  "warehouse.name_required": { field: "name", i18nKey: "validation.required" },
  "warehouse.code_taken": { field: "code", i18nKey: "serverErrors.warehouse.codeTaken" },
  // customer
  "customer.name_required": { field: "name", i18nKey: "validation.required" },
  // user
  "user.email_required": { field: "email", i18nKey: "validation.required" },
  "user.password_too_short": { field: "password", i18nKey: "validation.passwordMin" },
  "user.email_taken": { field: "email", i18nKey: "serverErrors.user.emailTaken" },
  // auth (change password) — field names match ChangePasswordDialog's form
  "auth.password_too_short": { field: "next", i18nKey: "validation.passwordMin" },
  "auth.current_password_wrong": { field: "current", i18nKey: "serverErrors.auth.currentPasswordWrong" },
  // settings
  "settings.threshold_negative": { field: "lowStockThreshold", i18nKey: "validation.thresholdInvalid" },
  "settings.app_title_too_long": { field: "appTitle", i18nKey: "validation.appTitleTooLong" },
  "settings.business_type_invalid": { field: "businessType", i18nKey: "validation.businessTypeInvalid" },
  "settings.tunnel_token_invalid": { field: "token", i18nKey: "validation.tunnelTokenInvalid" },
  // stock movement
  "movement.batch_required": { field: "batchId", i18nKey: "validation.required" },
  "movement.qty_zero": { field: "qty", i18nKey: "validation.qtyNonZero" },

  // --- Long-tail (field-less; shown as a specific translated toast). The forms
  // for these domains already validate client-side via Zod; these are the rare
  // server backstops. ---
  // prescription
  "prescription.customer_required": { i18nKey: "validation.required" },
  "prescription.issuer_required": { i18nKey: "validation.required" },
  "prescription.items_required": { i18nKey: "serverErrors.common.itemsRequired" },
  "prescription.expires_before_issued": { i18nKey: "serverErrors.prescription.expiresBeforeIssued" },
  "prescription.fee_negative": { i18nKey: "serverErrors.common.amountNegative" },
  "prescription.qty_invalid": { i18nKey: "serverErrors.common.qtyInvalid" },
  "prescription.item_product_required": { i18nKey: "validation.required" },
  "prescription.already_dispensed": { i18nKey: "serverErrors.prescription.alreadyDispensed" },
  // purchasing
  "purchasing.supplier_required": { i18nKey: "validation.required" },
  "purchasing.items_required": { i18nKey: "serverErrors.common.itemsRequired" },
  "purchasing.qty_invalid": { i18nKey: "serverErrors.common.qtyInvalid" },
  "purchasing.cost_negative": { i18nKey: "serverErrors.common.amountNegative" },
  "purchasing.po_required": { i18nKey: "validation.required" },
  "purchasing.lines_required": { i18nKey: "serverErrors.common.itemsRequired" },
  "purchasing.amount_invalid": { i18nKey: "serverErrors.common.qtyInvalid" },
  "purchasing.po_voided": { i18nKey: "serverErrors.purchasing.poVoided" },
  "purchasing.discount_negative": { i18nKey: "serverErrors.common.discountInvalid" },
  "purchasing.discount_percent_range": { i18nKey: "serverErrors.common.discountInvalid" },
  "purchasing.discount_type_invalid": { i18nKey: "serverErrors.common.discountInvalid" },
  "purchasing.reason_required": { i18nKey: "serverErrors.purchasing.reasonRequired" },
  "purchasing.po_not_returnable": { i18nKey: "serverErrors.purchasing.poNotReturnable" },
  "purchasing.return_exceeds_on_hand": { i18nKey: "serverErrors.purchasing.returnExceedsOnHand" },
  "purchasing.receipt_item_not_found": { i18nKey: "serverErrors.purchasing.receiptItemNotFound" },
  "purchasing.duplicate_line": { i18nKey: "serverErrors.purchasing.duplicateLine" },
  "purchasing.no_batch": { i18nKey: "serverErrors.purchasing.noBatch" },
  // Cancel-an-accepted-restock guards. Also precomputed onto
  // PurchaseReceipt.cancel_blocked_reason so the UI can disable the action with
  // the same wording it would otherwise have failed with.
  "purchasing.receipt_required": { i18nKey: "validation.required" },
  "purchasing.receipt_lot_consumed": { i18nKey: "serverErrors.purchasing.receiptLotConsumed" },
  "purchasing.receipt_lot_in_stocktake": { i18nKey: "serverErrors.purchasing.receiptLotInStocktake" },
  "purchasing.receipt_cancel_po_closed": { i18nKey: "serverErrors.purchasing.receiptCancelPoClosed" },
  "purchasing.receipt_already_voided": { i18nKey: "serverErrors.purchasing.receiptAlreadyVoided" },
  // transfer
  "transfer.warehouse_required": { i18nKey: "validation.required" },
  "transfer.same_warehouse": { i18nKey: "serverErrors.transfer.sameWarehouse" },
  "transfer.lines_required": { i18nKey: "serverErrors.common.itemsRequired" },
  "transfer.qty_invalid": { i18nKey: "serverErrors.common.qtyInvalid" },
  // batch
  "batch.product_required": { i18nKey: "validation.required" },
  "batch.qty_negative": { i18nKey: "serverErrors.common.amountNegative" },
  "batch.cost_negative": { i18nKey: "serverErrors.common.amountNegative" },
  // stocktake
  "stocktake.count_negative": { i18nKey: "serverErrors.common.amountNegative" },
  "stocktake.disposition_invalid": { i18nKey: "serverErrors.stocktake.dispositionInvalid" },
  "stocktake.write_off_kind_required": { i18nKey: "serverErrors.stocktake.writeOffKindRequired" },
  // unit
  "unit.name_required": { i18nKey: "validation.required" },
  "unit.factor_invalid": { i18nKey: "serverErrors.unit.factorInvalid" },
  // sale (POS)
  "sale.qty_invalid": { i18nKey: "serverErrors.common.qtyInvalid" },
  "sale.fee_negative": { i18nKey: "serverErrors.common.amountNegative" },
  "sale.cart_empty": { i18nKey: "serverErrors.sale.cartEmpty" },
  "sale.paid_too_low": { i18nKey: "serverErrors.sale.paidTooLow" },
  "sale.discount_negative": { i18nKey: "serverErrors.common.discountInvalid" },
  "sale.discount_percent_range": { i18nKey: "serverErrors.common.discountInvalid" },
  "sale.discount_type_invalid": { i18nKey: "serverErrors.common.discountInvalid" },
};

// A token-shaped message: all-lowercase dotted segments, no spaces. Real
// free-text backend prose (sentence-case, spaces) never matches this.
const TOKEN_SHAPE = /^[a-z_]+(\.[a-z_]+)+$/;

function connectMessage(err: unknown): string | null {
  if (!(err instanceof ConnectError)) return null;
  // ConnectError.message is prefixed with "[code] "; rawMessage is the bare text.
  return err.rawMessage ?? err.message;
}

// translateServerError returns a user-facing string for a backend error:
//   known token            -> its translated message
//   unknown token-shaped    -> generic ("Something went wrong") — never leak it
//   free-text / non-Connect -> null (caller falls back to err.message as-is)
export function translateServerError(err: unknown): string | null {
  const msg = connectMessage(err);
  if (msg == null) return null;
  const entry = SERVER_ERRORS[msg];
  if (entry) return i18n.t(entry.i18nKey);
  if (TOKEN_SHAPE.test(msg)) return i18n.t("errors.generic");
  return null;
}

// applyServerError attaches a known field-bearing token to its form field and
// returns true (the caller should then suppress the toast). Returns false when
// the error is not a known field token (caller falls back to a toast).
export function applyServerError<T extends FieldValues>(
  err: unknown,
  form: UseFormReturn<T>,
): boolean {
  const msg = connectMessage(err);
  if (msg == null) return false;
  const entry = SERVER_ERRORS[msg];
  if (entry?.field) {
    form.setError(entry.field as Path<T>, { message: i18n.t(entry.i18nKey) });
    return true;
  }
  return false;
}
