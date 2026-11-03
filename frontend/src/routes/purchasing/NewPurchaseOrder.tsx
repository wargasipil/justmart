import PurchaseOrderForm from "./PurchaseOrderForm";

// The create route. The form body is shared with /purchasing/:id/edit — it
// lives in PurchaseOrderForm so the two can never drift into two different
// ideas of what a restock line is.
export default function NewPurchaseOrder() {
  return <PurchaseOrderForm />;
}
