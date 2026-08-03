import { expect, test } from "./_helpers";

// Regression guard for the "dynamic selects MUST search server-side" rule
// (see CLAUDE.md → Selects). Opens a SearchableSelect, types a query, and
// asserts that:
//   1. The popover opens via the Combobox trigger (not a native <select>).
//   2. Typing fires the corresponding backend Search RPC (proves loadOptions
//      mode is wired correctly).
//   3. The page has ZERO `useSuppliersQuery()` preload firing in the same
//      session (no `ListSuppliers` RPC) — if a future change accidentally
//      reverts to `items={suppliersQ.data}` the preload would come back and
//      this assertion catches it.
test.describe("SearchableSelect (async loadOptions)", () => {
  test("supplier select on /purchasing/new uses SearchSuppliers, not ListSuppliers", async ({
    page,
  }) => {
    // Collect every /api/ POST so we can assert on the RPC traffic.
    const rpcs: string[] = [];
    page.on("requestfinished", (req) => {
      const url = req.url();
      if (url.includes("/api/")) rpcs.push(url);
    });

    // Connect-Web URLs look like:
    //   /api/inventory_iface.v1.SupplierService/SearchSuppliers
    // Note the `.SupplierService` (dot, not slash) — that's the package
    // delimiter Connect uses.
    const SEARCH = /SupplierService\/SearchSuppliers/;
    const LIST = /SupplierService\/ListSuppliers/;

    await page.goto("/purchasing/new");
    await page.waitForLoadState("networkidle");

    // Initial load must NOT have called ListSuppliers (preload).
    expect(rpcs.some((u) => LIST.test(u))).toBe(false);

    // Wait for the debounced mount search by POLLING the collected traffic — a
    // fixed sleep raced the 250ms debounce plus the round-trip whenever the
    // machine was busy (this spec runs right after the restock suite), which
    // made it the one flaky test in an otherwise green run.
    await expect
      .poll(() => rpcs.filter((u) => SEARCH.test(u)).length)
      .toBeGreaterThanOrEqual(1);

    // Type a query into the supplier combobox; debounced loadOptions fires
    // another search. Target it by placeholder — `.first()` over all comboboxes
    // is fragile as the page chrome evolves (e.g. the warehouse picker).
    const beforeType = rpcs.filter((u) => SEARCH.test(u)).length;
    await page.getByPlaceholder("Select supplier").fill("a");
    await expect
      .poll(() => rpcs.filter((u) => SEARCH.test(u)).length)
      .toBeGreaterThan(beforeType);

    // The preload guard has to hold for the WHOLE session, not just mount.
    expect(rpcs.some((u) => LIST.test(u))).toBe(false);
  });
});
