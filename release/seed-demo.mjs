// Builds the demo shop the release video is recorded against.
//
// This is NOT a fixture for the dev database. It expects a THROWAWAY instance
// (its own SQLite file, its own port) so the video never shows a colleague's
// half-finished test rows — see release/README.md for how to stand one up.
//
// Two phases, both here because the second one only makes sense right after
// the first:
//
//   1. API — everything a real shop would do, done over the Connect API. The
//      shop it builds is therefore reachable by the same rules the app
//      enforces; nothing is written behind the product's back.
//   2. Backdate — one direct SQLite pass that moves the seeded sales, receipts
//      and stock movements into the past. This CANNOT go through the API:
//      CompleteSale stamps `now`, so an API-only shop has its entire trading
//      history at one timestamp, and every analytics chart in the video would
//      be a single spike. The video needs a month of trading behind it.
//
// Usage:
//   node release/seed-demo.mjs --base http://127.0.0.1:8099 \
//     --db <demo-dir>/demo.db --email owner@justmart.local --password demo12345
//
// Safe to re-run only against a fresh database — SKUs and supplier codes are
// fixed, so a second run fails on the uniqueness checks rather than quietly
// doubling the catalog.

import { DatabaseSync } from "node:sqlite";

// --- args --------------------------------------------------------------------

const args = Object.fromEntries(
  process.argv.slice(2).reduce((acc, a, i, arr) => {
    if (a.startsWith("--")) acc.push([a.slice(2), arr[i + 1]]);
    return acc;
  }, []),
);

const BASE = args.base ?? "http://127.0.0.1:8099";
const DB_PATH = args.db ?? null;
const EMAIL = args.email ?? "owner@justmart.local";
const PASSWORD = args.password ?? "demo12345";

/** Trading days of history to fabricate. The dashboard trend shows 7. */
const HISTORY_DAYS = 30;

// --- tiny Connect client -----------------------------------------------------

let token = "";

async function rpc(procedure, body = {}) {
  const res = await fetch(`${BASE}/api/${procedure}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(body),
  });
  const text = await res.text();
  if (!res.ok) throw new Error(`${procedure} -> ${res.status} ${text}`);
  return text ? JSON.parse(text) : {};
}

const log = (...a) => console.log("  ", ...a);

// --- the shop ----------------------------------------------------------------

const SUPPLIERS = [
  { code: "SNS-01", name: "PT Sinar Niaga Sejahtera", phone: "022-7301188", address: "Jl. Soekarno Hatta No. 412, Bandung" },
  { code: "BRK-02", name: "CV Berkah Grosir Utama", phone: "022-6041920", address: "Jl. Kopo No. 118, Bandung" },
  { code: "ANM-03", name: "PT Anugerah Niaga Makmur", phone: "021-5501234", address: "Jl. Raya Bekasi KM 21, Jakarta" },
];

// A minimarket catalog. `pack` is the larger unit the shop BUYS in and, where
// it makes sense, also sells; `factor` is how many base units it holds.
const PRODUCTS = [
  { sku: "MI-001", name: "Indomie Goreng 85 g",            base: "pcs",     price: 3_500,  pack: { name: "dus",  factor: 40, price: 132_000 }, cost: 2_850,  supplier: 0 },
  { sku: "MI-002", name: "Mie Sedaap Soto 75 g",           base: "pcs",     price: 3_300,  pack: { name: "dus",  factor: 40, price: 124_000 }, cost: 2_700,  supplier: 0 },
  { sku: "AQ-001", name: "Aqua Botol 600 ml",              base: "botol",   price: 4_000,  pack: { name: "dus",  factor: 24, price: 90_000  }, cost: 3_300,  supplier: 1 },
  { sku: "TB-001", name: "Teh Botol Sosro 350 ml",         base: "botol",   price: 5_000,  pack: { name: "dus",  factor: 24, price: 112_000 }, cost: 4_150,  supplier: 1 },
  { sku: "MG-001", name: "Minyak Goreng Bimoli 2 L",       base: "pouch",   price: 38_000, pack: { name: "dus",  factor: 6,  price: 222_000 }, cost: 34_500, supplier: 2 },
  { sku: "GP-001", name: "Gula Pasir Gulaku 1 kg",         base: "pack",    price: 18_500, pack: { name: "dus",  factor: 12, price: 214_000 }, cost: 16_400, supplier: 2 },
  { sku: "KP-001", name: "Kopi Kapal Api Special 165 g",   base: "pack",    price: 12_500, pack: { name: "dus",  factor: 24, price: 288_000 }, cost: 10_900, supplier: 0 },
  { sku: "SU-001", name: "Susu Ultra Milk Full Cream 1 L", base: "kotak",   price: 19_000, pack: { name: "dus",  factor: 12, price: 220_000 }, cost: 16_800, supplier: 1 },
  { sku: "SB-001", name: "Sabun Lifebuoy Merah 110 g",     base: "pcs",     price: 4_500,  pack: { name: "dus",  factor: 48, price: 205_000 }, cost: 3_700,  supplier: 2 },
  { sku: "DT-001", name: "Rinso Anti Noda 770 g",          base: "pack",    price: 24_000, pack: { name: "dus",  factor: 12, price: 276_000 }, cost: 21_500, supplier: 2 },
  { sku: "SP-001", name: "Sampo Sunsilk Black 170 ml",     base: "botol",   price: 22_000, pack: null,                                          cost: 19_200, supplier: 2 },
  { sku: "RT-001", name: "Roti Tawar Sari Roti Jumbo",     base: "bungkus", price: 17_000, pack: null,                                          cost: 14_500, supplier: 1 },
];

// The grosir ladder — the shop's own wholesale prices. Keyed by SKU; each rung
// prices the BASE unit, which is what a warung buying a dozen actually picks up.
const TIERS = {
  "MI-001": [{ minQty: 12, price: 3_300 }, { minQty: 40, price: 3_100 }],
  "AQ-001": [{ minQty: 24, price: 3_700 }],
  "SB-001": [{ minQty: 12, price: 4_200 }],
  "KP-001": [{ minQty: 24, price: 11_800 }],
};

const CUSTOMERS = [
  { name: "Warung Bu Yanti", phone: "0812-2233-4455", address: "Jl. Cibaduyut No. 12" },
  { name: "Toko Rezeki Jaya", phone: "0813-8877-6655", address: "Jl. Peta No. 88" },
  { name: "Ibu Siti Rahayu", phone: "0857-1122-9090", address: "Perum Griya Asri Blok C2" },
];

// --- helpers -----------------------------------------------------------------

const money = (n) => String(n);
const ymd = (d) => d.toISOString().slice(0, 10);
const daysFromNow = (n) => {
  const d = new Date();
  d.setDate(d.getDate() + n);
  return d;
};

/** Deterministic PRNG so a re-record produces the same shop, not a new one. */
let seed = 20260811;
function rand() {
  seed = (seed * 1103515245 + 12345) % 2147483648;
  return seed / 2147483648;
}
const pick = (arr) => arr[Math.floor(rand() * arr.length)];
const between = (lo, hi) => lo + Math.floor(rand() * (hi - lo + 1));

// --- phase 1: build the shop over the API ------------------------------------

async function seedOverApi() {
  console.log("\n[1/6] signing in");
  const auth = await rpc("user_iface.v1.AuthService/Login", { email: EMAIL, password: PASSWORD });
  token = auth.accessToken;
  log("ok");

  console.log("\n[2/6] shop identity + a second warehouse");
  // Retail is where the tour starts; the pharmacy beat at the end switches it.
  // The title is pinned so the brand stays "Justmart" across that switch —
  // otherwise the sidebar would relabel itself to "Apotek" mid-video and read
  // as if the product had been renamed.
  await rpc("settings_iface.v1.SettingsService/UpdateSettings", {
    lowStockThreshold: 10,
    appTitle: "Justmart",
    businessType: "BUSSINESS_TYPE_RETAIL",
  });
  // The TopBar warehouse selector only renders when the user can reach more
  // than one warehouse, so the multi-warehouse beat needs this row to exist.
  await rpc("warehouse_iface.v1.WarehouseService/CreateWarehouse", {
    code: "GDG-02",
    name: "Gudang Cabang Cimahi",
    address: "Jl. Raya Cibabat No. 45, Cimahi",
    phone: "022-6652100",
  });
  log("retail mode, title Justmart, 2 warehouses");

  console.log("\n[3/6] suppliers, catalog, grosir ladder");
  const supplierIds = [];
  for (const s of SUPPLIERS) {
    const r = await rpc("inventory_iface.v1.SupplierService/CreateSupplier", {
      code: s.code,
      name: s.name,
      phone: s.phone,
      address: s.address,
    });
    supplierIds.push(r.supplier.id);
  }
  log(`${supplierIds.length} suppliers`);

  const products = {};
  for (const p of PRODUCTS) {
    // `units` carries only the LARGER units — the base one is derived from
    // `unit`/`unit_price`, and passing it here is rejected (factor must be > 1).
    const units = p.pack
      ? [
          {
            name: p.pack.name,
            factor: p.pack.factor,
            isBase: false,
            sellPrice: money(p.pack.price),
            sellable: true,
            purchasable: true,
            sortOrder: 1,
            active: true,
          },
        ]
      : [];
    const r = await rpc("inventory_iface.v1.ProductService/CreateProduct", {
      sku: p.sku,
      name: p.name,
      unit: p.base,
      unitPrice: money(p.price),
      prescriptionRequired: false,
      units,
    });
    products[p.sku] = { ...p, id: r.product.id, units: r.product.units ?? [] };
  }
  log(`${Object.keys(products).length} products`);

  let tierCount = 0;
  for (const [sku, rungs] of Object.entries(TIERS)) {
    const prod = products[sku];
    const baseUnit = prod.units.find((u) => u.isBase);
    for (const rung of rungs) {
      await rpc("inventory_iface.v1.ProductPriceTierService/CreateProductPriceTier", {
        productId: prod.id,
        productUnitId: baseUnit.id,
        minQty: rung.minQty,
        price: money(rung.price),
      });
      tierCount++;
    }
  }
  log(`${tierCount} grosir tiers`);

  for (const c of CUSTOMERS) {
    await rpc("customer_iface.v1.CustomerService/CreateCustomer", c);
  }
  log(`${CUSTOMERS.length} customers`);

  console.log("\n[4/6] restock orders + receiving stock");
  // Stock arrives the way it does in the product: a PO is raised, sent, and
  // received, which is what mints the batches. Quantities are sized so a month
  // of trading does not empty the shelf mid-recording.
  const stockPlan = [
    { sku: "MI-001", packs: 14, expiryDays: 240 },
    { sku: "MI-002", packs: 9,  expiryDays: 220 },
    { sku: "AQ-001", packs: 26, expiryDays: 400 },
    { sku: "TB-001", packs: 16, expiryDays: 300 },
    { sku: "MG-001", packs: 22, expiryDays: 330 },
    { sku: "GP-001", packs: 14, expiryDays: 500 },
    { sku: "KP-001", packs: 8,  expiryDays: 280 },
    { sku: "SU-001", packs: 12, expiryDays: 26  }, // deliberately near expiry
    { sku: "SB-001", packs: 5,  expiryDays: 600  },
    { sku: "DT-001", packs: 9,  expiryDays: 540  },
    { sku: "SP-001", packs: 150, expiryDays: 450, base: true },
    { sku: "RT-001", packs: 160, expiryDays: 9, base: true }, // fast mover, thin stock
  ];

  // One PO per supplier, each fully received.
  const bySupplier = new Map();
  for (const line of stockPlan) {
    const p = products[line.sku];
    if (!bySupplier.has(p.supplier)) bySupplier.set(p.supplier, []);
    bySupplier.get(p.supplier).push(line);
  }

  const receivedPoIds = [];
  for (const [supplierIdx, lines] of bySupplier) {
    const items = lines.map((line) => {
      const p = products[line.sku];
      const unit = line.base
        ? p.units.find((u) => u.isBase)
        : (p.units.find((u) => !u.isBase) ?? p.units.find((u) => u.isBase));
      return {
        productId: p.id,
        orderedQty: line.packs,
        unitCostPrice: money(p.cost),
        productUnitId: unit.id,
      };
    });

    const po = await rpc("purchasing_iface.v1.PurchaseOrderService/CreatePurchaseOrder", {
      supplierId: supplierIds[supplierIdx],
      items,
      invoiceNo: `FK/${SUPPLIERS[supplierIdx].code}/2026/0${supplierIdx + 1}`,
      note: "Restok rutin awal bulan",
      ppnEnabled: supplierIdx === 2, // one PO carries PPN, so the tour can show it
      ppnRate: 11,
    });
    await rpc("purchasing_iface.v1.PurchaseOrderService/SendPurchaseOrder", { id: po.order.id });

    await rpc("purchasing_iface.v1.PurchaseReceiptService/CreateReceipt", {
      purchaseOrderId: po.order.id,
      invoiceNo: po.order.invoiceNo,
      lines: po.order.items.map((it, i) => {
        const line = lines[i];
        return {
          purchaseOrderItemId: it.id,
          qty: line.packs,
          batchNumber: `B-2608-${String(100 + receivedPoIds.length * 10 + i)}`,
          expiryDate: ymd(daysFromNow(line.expiryDays)),
        };
      }),
    });
    receivedPoIds.push(po.order.id);
  }
  log(`${receivedPoIds.length} restock orders received`);

  // One order left SENT — this is the delivery the video receives ON CAMERA.
  // Without it there is nothing live to demonstrate; receiving an already
  // received order is not a thing the app lets you do.
  // ONE line on purpose: receiving is demonstrated live, and every extra line
  // is another batch number and expiry date typed on camera for no new
  // information. The multi-line case is already visible on the received orders.
  const openLines = [{ sku: "MI-001", packs: 6 }];
  const openPo = await rpc("purchasing_iface.v1.PurchaseOrderService/CreatePurchaseOrder", {
    supplierId: supplierIds[0],
    items: openLines.map((l) => {
      const p = products[l.sku];
      const unit = p.units.find((u) => !u.isBase) ?? p.units.find((u) => u.isBase);
      return { productId: p.id, orderedQty: l.packs, unitCostPrice: money(p.cost), productUnitId: unit.id };
    }),
    invoiceNo: "FK/SNS-01/2026/07",
    note: "Menunggu kiriman",
  });
  await rpc("purchasing_iface.v1.PurchaseOrderService/SendPurchaseOrder", { id: openPo.order.id });
  log(`1 restock order left waiting (${openPo.order.poNo}) — the one received on camera`);

  console.log(`\n[5/6] ${HISTORY_DAYS} days of trading`);
  // Sales are created at `now` and moved into the past in phase 2. Weekday
  // shape is applied here so the trend line has a rhythm rather than noise.
  const sellable = Object.values(products);
  const saleDays = [];

  for (let dayOffset = HISTORY_DAYS; dayOffset >= 1; dayOffset--) {
    const day = daysFromNow(-dayOffset);
    const dow = day.getDay();
    const busy = dow === 0 || dow === 6 ? 1.45 : dow === 5 ? 1.2 : 1;
    const count = Math.max(2, Math.round(between(3, 6) * busy));
    const ids = [];

    for (let i = 0; i < count; i++) {
      const sale = await rpc("pos_iface.v1.SaleService/StartSale", {});
      const saleId = sale.sale.id;
      const lineCount = between(1, 4);
      const used = new Set();

      for (let l = 0; l < lineCount; l++) {
        const p = pick(sellable);
        if (used.has(p.sku)) continue;
        used.add(p.sku);
        const baseUnit = p.units.find((u) => u.isBase);
        // Most baskets are retail-sized; roughly one in six is a warung buying
        // enough to cross a grosir rung, so the wholesale prices show up in the
        // numbers as well as in the POS demo.
        const qty = rand() < 0.17 ? between(12, 30) : between(1, 4);
        await rpc("pos_iface.v1.SaleService/AddItem", {
          saleId,
          productId: p.id,
          qty,
          productUnitId: baseUnit.id,
        });
      }

      const current = await rpc("pos_iface.v1.SaleService/GetSale", { id: saleId });
      const total = Number(current.sale.total ?? 0);
      if (total <= 0) {
        await rpc("pos_iface.v1.SaleService/DiscardSale", { saleId });
        continue;
      }
      const cash = rand() < 0.72;
      try {
        await rpc("pos_iface.v1.SaleService/CompleteSale", {
          saleId,
          paymentSource: cash ? "PAYMENT_SOURCE_CASH" : "PAYMENT_SOURCE_NON_CASH",
          paidAmount: money(cash ? Math.ceil(total / 5000) * 5000 : total),
        });
        ids.push(saleId);
      } catch (err) {
        // A basket the shelf cannot cover is a real outcome, not a seeding bug:
        // the fast movers are deliberately stocked thin so the low-stock tile
        // has something to say by the time we record. Drop the cart and move on
        // rather than failing the whole month of history over one line.
        if (!String(err.message).includes("insufficient stock")) throw err;
        await rpc("pos_iface.v1.SaleService/DiscardSale", { saleId }).catch(() => {});
      }
    }

    saleDays.push({ dayOffset, ids });
    if (dayOffset % 10 === 0) log(`day -${dayOffset} …`);
  }

  const totalSales = saleDays.reduce((n, d) => n + d.ids.length, 0);
  log(`${totalSales} completed sales across ${saleDays.length} days`);

  await topUpThinShelves();

  return { saleDays, receivedPoIds };
}

/**
 * A month of trading empties the fastest movers completely. An out-of-stock
 * shelf is a real state the app handles, but a promo video should not open on
 * one — and the low-stock bell has nothing to show if the survivors are all
 * comfortable.
 *
 * This lands AFTER the history, so the closing level is exactly the quantity
 * received here rather than whatever the random baskets happened to leave.
 * The result: nothing at zero, and three products sitting under the threshold.
 */
async function topUpThinShelves() {
  const TOP_UP = { "DT-001": 9, "SP-001": 8 };

  const { products } = await rpc("inventory_iface.v1.ProductService/ListProducts", { limit: 200 });
  const { suppliers } = await rpc("inventory_iface.v1.SupplierService/ListSuppliers", { limit: 50 });
  const supplier = suppliers.find((s) => s.code === "ANM-03") ?? suppliers[0];

  const wanted = products.filter((p) => p.sku in TOP_UP);
  const items = wanted.map((p) => {
    const base = (p.units ?? []).find((u) => u.isBase);
    const spec = PRODUCTS.find((x) => x.sku === p.sku);
    return {
      productId: p.id,
      orderedQty: TOP_UP[p.sku],
      unitCostPrice: money(spec.cost),
      productUnitId: base.id,
    };
  });

  const po = await rpc("purchasing_iface.v1.PurchaseOrderService/CreatePurchaseOrder", {
    supplierId: supplier.id,
    items,
    invoiceNo: "FK/ANM-03/2026/08",
    note: "Kiriman tambahan pagi ini",
  });
  await rpc("purchasing_iface.v1.PurchaseOrderService/SendPurchaseOrder", { id: po.order.id });
  await rpc("purchasing_iface.v1.PurchaseReceiptService/CreateReceipt", {
    purchaseOrderId: po.order.id,
    invoiceNo: po.order.invoiceNo,
    lines: po.order.items.map((it, i) => ({
      purchaseOrderItemId: it.id,
      qty: items[i].orderedQty,
      batchNumber: `B-2608-90${i}`,
      expiryDate: ymd(daysFromNow(420)),
    })),
  });
  log(`topped up ${wanted.map((p) => p.sku).join(", ")} — nothing left at zero`);
}

// --- phase 2: move the history into the past ---------------------------------

// SQLite's own datetime() cannot be used for this: GORM stores seven
// fractional digits ("2026-08-11 10:11:48.4600365+07:00") and datetime()
// returns NULL rather than parsing it. So the new value is computed in JS and
// the original's fraction+offset suffix is carried over verbatim — the column
// keeps exactly one textual shape, which is what the Go side reads back.
const STAMP_RE = /^(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}:\d{2})(.*)$/;

function restamp(original, target) {
  const suffix = STAMP_RE.exec(original ?? "")?.[3] ?? "";
  const p = (n) => String(n).padStart(2, "0");
  const ymdhms =
    `${target.getFullYear()}-${p(target.getMonth() + 1)}-${p(target.getDate())} ` +
    `${p(target.getHours())}:${p(target.getMinutes())}:${p(target.getSeconds())}`;
  return ymdhms + suffix;
}

function atLocal(dayOffset, hour, minute) {
  const d = new Date();
  d.setDate(d.getDate() + dayOffset);
  d.setHours(hour, minute, (minute * 7) % 60, 0);
  return d;
}

function backdate(dbPath) {
  console.log("\n[6/6] backdating the trading history");
  const db = new DatabaseSync(dbPath);
  db.exec("PRAGMA busy_timeout = 10000");

  const sample = db.prepare("SELECT created_at FROM sales LIMIT 1").get();
  log(`stored datetime looks like: ${sample?.created_at}`);

  // --- stock arrivals go BEFORE the trading window -------------------------
  // Analytics reconstructs stock as-of-end-of-day from the ledger. Leaving the
  // PURCHASE movements at `now` while the sales move into the past would have
  // a month of sales drawn from stock that had not arrived yet — every
  // historical stock level would read negative.
  const arrival = atLocal(-(HISTORY_DAYS + 1), 8, 30);
  const restampAll = (table, column, when, where = "1=1", params = []) => {
    const rows = db.prepare(`SELECT rowid AS rid, ${column} AS v FROM ${table} WHERE ${where}`).all(...params);
    const upd = db.prepare(`UPDATE ${table} SET ${column} = ? WHERE rowid = ?`);
    for (const r of rows) if (r.v != null) upd.run(restamp(r.v, when), r.rid);
    return rows.length;
  };

  db.exec("BEGIN");

  const receivedPoWhere = "id IN (SELECT purchase_order_id FROM purchase_receipts)";
  for (const col of ["created_at", "updated_at", "sent_at"]) {
    restampAll("purchase_orders", col, arrival, receivedPoWhere);
  }
  for (const col of ["created_at", "received_at"]) {
    restampAll("purchase_receipts", col, arrival);
    restampAll("batches", col, arrival);
  }
  restampAll("batches", "updated_at", arrival);
  restampAll("stock_movements", "created_at", arrival, "type = 'PURCHASE'");

  // The delivery still in transit was raised a couple of days ago, not a month.
  const ordered = atLocal(-2, 10, 15);
  for (const col of ["created_at", "updated_at", "sent_at"]) {
    restampAll("purchase_orders", col, ordered, `NOT (${receivedPoWhere})`);
  }

  // --- spread the sales across the trading window --------------------------
  // Re-derived from the database rather than from phase 1's return value, so
  // this phase can be re-run on its own against an already-seeded instance.
  const sales = db
    .prepare("SELECT id, created_at FROM sales WHERE status = 'COMPLETED' ORDER BY created_at, sale_no")
    .all();

  // Weekends and Friday carry more traffic; the trend line should have a
  // rhythm a shopkeeper recognises rather than looking like noise. Day 0 is
  // included on purpose — the dashboard's "hari ini" tiles are on camera, and
  // a shop with no sales today would show a wall of zeros.
  const days = [];
  for (let off = -(HISTORY_DAYS - 1); off <= 0; off++) {
    const dow = atLocal(off, 12, 0).getDay();
    days.push({ off, weight: dow === 0 || dow === 6 ? 1.45 : dow === 5 ? 1.2 : 1 });
  }
  const weightSum = days.reduce((n, d) => n + d.weight, 0);
  let assigned = 0;
  for (const d of days) {
    d.count = Math.max(1, Math.round((sales.length * d.weight) / weightSum));
    assigned += d.count;
  }
  // Trim the rounding drift off the oldest days so the total still matches.
  for (let i = 0; assigned > sales.length; i = (i + 1) % days.length) {
    if (days[i].count > 1) { days[i].count--; assigned--; }
  }

  const setSale = db.prepare(
    "UPDATE sales SET created_at = ?, updated_at = ?, completed_at = ? WHERE id = ?",
  );
  const setItems = db.prepare("UPDATE sale_items SET created_at = ? WHERE sale_id = ?");
  const setMovements = db.prepare(
    "UPDATE stock_movements SET created_at = ? WHERE sale_item_id IN (SELECT id FROM sale_items WHERE sale_id = ?)",
  );

  const nowHour = new Date().getHours();
  let cursor = 0;
  for (const d of days) {
    const slice = sales.slice(cursor, cursor + d.count);
    cursor += slice.length;
    for (let i = 0; i < slice.length; i++) {
      // Trading hours 09:00–20:00, but today stops at the current hour so no
      // sale is stamped in the future.
      const lastHour = d.off === 0 ? Math.max(9, Math.min(20, nowHour)) : 20;
      const span = Math.max(1, lastHour - 9);
      const hour = 9 + Math.floor((span * i) / Math.max(1, slice.length));
      const when = atLocal(d.off, hour, (i * 17) % 60);
      const stamp = restamp(slice[i].created_at, when);
      setSale.run(stamp, stamp, stamp, slice[i].id);
      setItems.run(stamp, slice[i].id);
      setMovements.run(stamp, slice[i].id);
    }
  }

  db.exec("COMMIT");
  db.close();
  log(`stock arrivals moved to day -${HISTORY_DAYS + 1}`);
  log(`${cursor} sales spread across ${days.length} trading days (including today)`);
}

// --- main --------------------------------------------------------------------

// --backdate-only re-runs just phase 2 against an instance already seeded —
// useful when a recording session spans days and the history has drifted
// forward. Stop the demo server first so nothing writes underneath it.
if ("topup-only" in args) {
  token = (await rpc("user_iface.v1.AuthService/Login", { email: EMAIL, password: PASSWORD })).accessToken;
  await topUpThinShelves();
} else if (!("backdate-only" in args)) {
  await seedOverApi();
}

if (DB_PATH && !("topup-only" in args)) backdate(DB_PATH);
else console.log("\n(no --db given: skipping backdate — analytics would show one spike)");

console.log("\ndone. demo shop is ready to record.\n");
