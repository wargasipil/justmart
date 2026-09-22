import type { PartialMessage } from "@bufbuild/protobuf";

import type { ListProductsRequest } from "../../gen/inventory_iface/v1/product_pb";
import { Product, ProductRestockLog, ProductUnit } from "../../gen/inventory_iface/v1/product_pb";
import { Manufacturer, ManufacturerProduct } from "../../gen/inventory_iface/v1/manufacturer_pb";
import { UnitBase } from "../../gen/unit_iface/v1/unit_pb";
import { Warehouse } from "../../gen/warehouse_iface/v1/warehouse_pb";

// Shared fixture data for PAGE and SCREEN stories (the screens/* tree). Component stories
// keep their own tiny inline fixtures; this module exists because several
// pages show the same catalog — the list, the detail, POS — and a product
// should look the same in each.
//
// Everything is built through the generated message classes (see storyMocks.ts
// for why), and every date is relative to now: ExpiryBadge, "expired" checks
// and "Last opname" comparisons read against today, so literal dates would
// drift into different badges as the calendar moves.

// --- time -------------------------------------------------------------------
const DAY = 86_400;
const nowUnix = () => BigInt(Math.floor(Date.now() / 1000));

/** A unix timestamp `n` days in the past. */
export const daysAgo = (n: number) => nowUnix() - BigInt(Math.round(n * DAY));

/** A local `YYYY-MM-DD` `days` from today (negative = past). */
export const dateIn = (days: number) => {
  const d = new Date(Date.now() + days * DAY * 1000);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};

// --- suppliers --------------------------------------------------------------
export const SUPPLIERS = [
  { id: "sup-1", code: "SUP-0001", name: "PT Sinar Farma" },
  { id: "sup-2", code: "SUP-0002", name: "CV Mitra Sehat" },
  { id: "sup-3", code: "SUP-0003", name: "PT Indo Grosir Jaya" },
  { id: "sup-4", code: "SUP-0004", name: "UD Makmur Sentosa" },
];

// --- manufacturers ----------------------------------------------------------
// Pabrik, not pemasok: real Indonesian makers, so the list reads like a shop's
// own. Deliberately disjoint from SUPPLIERS above (which are distributors):
// the whole reason this page exists is that the maker and the seller are
// different parties, and a fixture that reused one set of names for both would
// hide exactly that. 30 rows, so the pager has a second page at the default 25.
export const MANUFACTURERS: Manufacturer[] = [
  { code: "MFR-0001", name: "PT Kalbe Farma Tbk", phone: "021-4287-3888", contactEmail: "care@kalbe.co.id", address: "Jl. Letjen Suprapto Kav. 4, Jakarta Pusat", note: "Prinsipal" },
  { code: "MFR-0002", name: "PT Dexa Medica", phone: "0711-5710-000", contactEmail: "info@dexa-medica.com", address: "Jl. Letjen Bambang Utoyo 138, Palembang", note: "" },
  { code: "MFR-0003", name: "PT SOHO Industri Pharmasi", phone: "021-4682-2222", contactEmail: "", address: "Jl. Pulogadung 6, Jakarta Timur", note: "" },
  { code: "MFR-0004", name: "PT Kimia Farma Tbk", phone: "021-4239-8000", contactEmail: "", address: "Jl. Veteran 9, Jakarta Pusat", note: "BUMN" },
  { code: "MFR-0005", name: "PT Tempo Scan Pacific Tbk", phone: "021-2921-8888", contactEmail: "", address: "Tempo Scan Tower, Jakarta Selatan", note: "" },
  { code: "MFR-0006", name: "PT Darya-Varia Laboratoria Tbk", phone: "021-8082-6600", contactEmail: "", address: "Talavera Office Park, Jakarta Selatan", note: "" },
  { code: "MFR-0007", name: "PT Phapros Tbk", phone: "024-7605-000", contactEmail: "", address: "Jl. Simongan 131, Semarang", note: "" },
  { code: "MFR-0008", name: "PT Combiphar", phone: "021-2793-3888", contactEmail: "", address: "Graha Atrium, Jakarta Pusat", note: "" },
  { code: "MFR-0009", name: "PT Pharos Indonesia", phone: "021-8779-0777", contactEmail: "", address: "Jl. Limo 42, Jakarta Selatan", note: "" },
  { code: "MFR-0010", name: "PT Novell Pharmaceutical Laboratories", phone: "021-8779-5000", contactEmail: "", address: "Jl. Pos Pengumben, Jakarta Barat", note: "" },
  { code: "MFR-0011", name: "PT Interbat", phone: "031-891-0000", contactEmail: "", address: "Jl. HR Muhammad 41, Sidoarjo", note: "" },
  { code: "MFR-0012", name: "PT Otto Pharmaceutical Industries", phone: "022-278-6000", contactEmail: "", address: "Jl. Cimareme 151, Bandung Barat", note: "" },
  { code: "MFR-0013", name: "PT Konimex", phone: "0271-714-422", contactEmail: "", address: "Desa Sanggrahan, Sukoharjo", note: "" },
  { code: "MFR-0014", name: "PT Bio Farma (Persero)", phone: "022-203-3755", contactEmail: "", address: "Jl. Pasteur 28, Bandung", note: "Vaksin \u2014 rantai dingin" },
  { code: "MFR-0015", name: "PT Sido Muncul Tbk", phone: "024-7692-8811", contactEmail: "", address: "Klepu, Bergas, Semarang", note: "Jamu / herbal" },
  { code: "MFR-0016", name: "PT Deltomed Laboratories", phone: "0273-321-001", contactEmail: "", address: "Nambangan, Wonogiri", note: "Jamu / herbal" },
  { code: "MFR-0017", name: "PT Industri Jamu Borobudur", phone: "024-7628-8000", contactEmail: "", address: "Jl. Hasanudin 1, Semarang", note: "Jamu / herbal" },
  { code: "MFR-0018", name: "PT Indofood Sukses Makmur Tbk", phone: "021-5795-8822", contactEmail: "", address: "Sudirman Plaza, Jakarta Selatan", note: "" },
  { code: "MFR-0019", name: "PT Mayora Indah Tbk", phone: "021-5655-311", contactEmail: "", address: "Jl. Tomang Raya 21, Jakarta Barat", note: "" },
  { code: "MFR-0020", name: "PT Garudafood Putra Putri Jaya Tbk", phone: "021-5366-3800", contactEmail: "", address: "Wisma Garudafood, Jakarta Barat", note: "" },
  { code: "MFR-0021", name: "PT Nutrifood Indonesia", phone: "021-8779-6000", contactEmail: "", address: "Jl. Rawabali II/3, Jakarta Timur", note: "" },
  { code: "MFR-0022", name: "PT Ultrajaya Milk Industry Tbk", phone: "022-866-00700", contactEmail: "", address: "Jl. Raya Cimareme 131, Bandung Barat", note: "" },
  { code: "MFR-0023", name: "PT Frisian Flag Indonesia", phone: "021-799-2222", contactEmail: "", address: "Jl. Raya Bogor KM 5, Jakarta Timur", note: "" },
  { code: "MFR-0024", name: "PT Sari Husada Generasi Mahardhika", phone: "0274-512-990", contactEmail: "", address: "Jl. Kusumanegara 173, Yogyakarta", note: "" },
  { code: "MFR-0025", name: "PT Unilever Indonesia Tbk", phone: "0800-1-558000", contactEmail: "", address: "BSD Green Office Park, Tangerang", note: "" },
  { code: "MFR-0026", name: "PT Wings Surya", phone: "031-749-6000", contactEmail: "", address: "Jl. Kalisosok Kidul 2, Surabaya", note: "" },
  { code: "MFR-0027", name: "PT Kino Indonesia Tbk", phone: "021-8082-8000", contactEmail: "", address: "Kino Tower, Tangerang", note: "" },
  { code: "MFR-0028", name: "PT Paragon Technology and Innovation", phone: "021-5435-0111", contactEmail: "", address: "Jl. Kabupaten 19, Tangerang", note: "" },
  // Archived: the shop stopped stocking these. Only visible behind the
  // "show archived" switch, which is what that story checks.
  { code: "MFR-0029", name: "CV Aneka Pangan Lestari", phone: "", contactEmail: "", address: "Cikarang, Bekasi", note: "Tidak dipakai lagi", active: false },
  { code: "MFR-0030", name: "UD Sumber Rejeki Abadi", phone: "", contactEmail: "", address: "Kudus", note: "Tutup 2025", active: false },
].map(
  (m, i) =>
    new Manufacturer({
      id: `mfr-${i + 1}`,
      active: true,
      createdAt: daysAgo(400 - i * 12),
      ...m,
    }),
);

/**
 * The server's ListManufacturers predicate, as the story's fake: the archived
 * switch and a case-insensitive name/code search, ordered by name. Mocking the
 * RPC as a function of the request (rather than a canned page) is what lets the
 * search box, the switch and the pager actually work in the story — and what
 * keeps the summary tile aggregating the SAME set the table shows.
 */
export function filterManufacturers(
  rows: Manufacturer[],
  req: { includeInactive: boolean; query: string },
): Manufacturer[] {
  const q = req.query.trim().toLowerCase();
  return rows
    .filter((m) => (req.includeInactive ? true : m.active))
    .filter((m) => !q || m.name.toLowerCase().includes(q) || m.code.toLowerCase().includes(q))
    .sort((a, b) => a.name.localeCompare(b.name));
}

/**
 * Which pabrik makes the product at catalog index `i`.
 *
 * Concentrated rather than spread evenly: a shop's shelf is dominated by a
 * handful of makers, so five of the thirty carry the whole catalog and the
 * rest carry nothing. That is what makes MFR-0006's detail page an honest
 * empty state instead of a contrived one.
 */
function makerOf(i: number): string {
  if (i % 2 === 0) return "mfr-1"; // PT Kalbe Farma — the big one
  if (i % 3 === 0) return "mfr-2"; // PT Dexa Medica
  if (i % 5 === 0) return "mfr-3"; // PT SOHO
  if (i % 7 === 0) return "mfr-4"; // PT Kimia Farma
  return "mfr-5"; // PT Tempo Scan
}

/**
 * Stamp each catalog product with its maker, from the SAME `makerOf` that
 * `manufacturerProductsFor` reads. One assignment, so a product's Pabrik field
 * on the detail page and the manufacturer's own product list can never disagree
 * — which is exactly the kind of drift a second hand-written mapping invites.
 */
function attachMakers(catalog: Product[]): Product[] {
  catalog.forEach((p, i) => {
    p.manufacturerId = makerOf(i);
  });
  return catalog;
}

/**
 * The catalog one pabrik makes, as ListManufacturerProducts returns it —
 * denormalized rows, no cost fields. Derived from the SAME catalog the product
 * pages use, so a product reads identically wherever it appears.
 */
export function manufacturerProductsFor(
  manufacturerId: string,
  catalog: Product[],
): ManufacturerProduct[] {
  return catalog
    .map((p, i) => ({ p, owner: makerOf(i) }))
    .filter(({ owner }) => owner === manufacturerId)
    .map(
      ({ p }) =>
        new ManufacturerProduct({
          productId: p.id,
          name: p.name,
          sku: p.sku,
          baseUnit: p.unit,
          unitPrice: p.unitPrice,
          readyStock: p.readyStock,
          active: p.active,
        }),
    );
}

/** The server's ListManufacturerProducts predicate, as the story's fake. */
export function filterManufacturerProducts(
  rows: ManufacturerProduct[],
  req: { query: string; includeArchived: boolean },
): ManufacturerProduct[] {
  const q = req.query.trim().toLowerCase();
  return rows
    .filter((p) => (req.includeArchived ? true : p.active))
    .filter((p) => !q || p.name.toLowerCase().includes(q) || p.sku.toLowerCase().includes(q))
    .sort((a, b) => a.name.localeCompare(b.name));
}

// --- products ---------------------------------------------------------------

/** A larger pack of the base unit: name, base units per pack, sell price. */
type Pack = [name: string, factor: bigint, price: bigint];

/**
 * The unit ladder a product sells in: the base unit at the product's price,
 * then each pack. Ids are derived from the product id so a tier or a unit
 * price can point at `${productId}-u2` without a lookup.
 */
export function unitsFor(productId: string, base: string, basePrice: bigint, packs: Pack[]): ProductUnit[] {
  return [
    new ProductUnit({ id: `${productId}-u1`, productId, name: base, factor: 1n, isBase: true, sellPrice: basePrice, sellable: true, sortOrder: 0, active: true }),
    ...packs.map(
      ([name, factor, price], i) =>
        new ProductUnit({
          id: `${productId}-u${i + 2}`,
          productId,
          name,
          factor,
          sellPrice: price,
          sellable: true,
          // The biggest pack is what the shop buys in.
          purchasable: i === packs.length - 1,
          sortOrder: i + 1,
          active: true,
        }),
    ),
  ];
}

/**
 * One believable product. `cost` is the per-base-unit cost the valuations are
 * derived from, so Ready/Ongoing tiles and the summary row always agree with
 * the counts they sit under.
 */
export function makeProduct(
  seed: {
    id: string;
    sku: string;
    name: string;
    unit: string;
    price: bigint;
    cost: bigint;
    packs?: Pack[];
    ready?: bigint;
    onOrder?: bigint;
    active?: boolean;
    supplier?: (typeof SUPPLIERS)[number];
    /** Days since the last restock arrived; undefined = never restocked. */
    restockedDaysAgo?: number;
    /** Days since the last completed opname; undefined = never counted. */
    countedDaysAgo?: number;
    prescriptionRequired?: boolean;
  },
  overrides: PartialMessage<Product> = {},
): Product {
  const ready = seed.ready ?? 0n;
  const onOrder = seed.onOrder ?? 0n;
  const restocked = seed.restockedDaysAgo !== undefined;
  const sup = seed.supplier ?? SUPPLIERS[0];
  return new Product({
    id: seed.id,
    sku: seed.sku,
    name: seed.name,
    unit: seed.unit,
    unitPrice: seed.price,
    prescriptionRequired: seed.prescriptionRequired ?? false,
    active: seed.active ?? true,
    createdAt: daysAgo(240),
    readyStock: ready,
    totalStock: ready,
    onOrderStock: onOrder,
    stockValuation: ready * seed.cost,
    onOrderValuation: onOrder * seed.cost,
    referenceCost: seed.cost,
    lastRestockDate: restocked ? dateIn(-seed.restockedDaysAgo!) : "",
    lastRestockSupplier: restocked ? sup.name : "",
    lastRestockSupplierId: restocked ? sup.id : "",
    lastRestockPrice: restocked ? seed.cost : 0n,
    lastRestockQty: restocked ? ready + 100n - (ready % 100n) : 0n,
    lastRestockCreatedAt: restocked ? daysAgo(seed.restockedDaysAgo! + 3) : 0n,
    lastRestockArrivedAt: restocked ? daysAgo(seed.restockedDaysAgo!) : 0n,
    lastStocktakeDate: seed.countedDaysAgo !== undefined ? dateIn(-seed.countedDaysAgo) : "",
    lastStocktakeVariance: seed.countedDaysAgo !== undefined ? -BigInt(seed.countedDaysAgo % 5) : 0n,
    units: unitsFor(seed.id, seed.unit, seed.price, seed.packs ?? []),
    imageUpdatedAt: 0n, // "no picture" — ProductImage skips the fetch entirely
    ...overrides,
  });
}

/**
 * What the till receives for a product: the server blanks cost + supplier on
 * the wire (product.redactCost), so a cashier story is fed the same — a story
 * that handed the cashier view manager data would hide a UI that leaks.
 */
export function redactProductForTill(p: Product): Product {
  const r = p.clone();
  r.referenceCost = 0n;
  r.stockValuation = 0n;
  r.onOrderValuation = 0n;
  r.lastRestockPrice = 0n;
  r.lastRestockQty = 0n;
  r.lastRestockDiscountType = "";
  r.lastRestockDiscountValue = 0n;
  r.lastRestockSupplier = "";
  r.lastRestockSupplierId = "";
  return r;
}

// A minimarket shelf: enough rows (30 active + 3 archived) to page at 25, a
// few low-stock lines, a couple never counted, and mixed unit ladders.
const [S1, S2, S3, S4] = SUPPLIERS;
export const RETAIL_CATALOG: Product[] = [
  makeProduct({ id: "r01", sku: "8998866200011", name: "Indomie Goreng 85 g", unit: "pcs", price: 3_500n, cost: 2_900n, packs: [["dus", 40n, 132_000n]], ready: 320n, onOrder: 400n, supplier: S3, restockedDaysAgo: 6, countedDaysAgo: 20 }),
  makeProduct({ id: "r02", sku: "8998866200028", name: "Indomie Soto 70 g", unit: "pcs", price: 3_300n, cost: 2_750n, packs: [["dus", 40n, 125_000n]], ready: 145n, supplier: S3, restockedDaysAgo: 18, countedDaysAgo: 20 }),
  makeProduct({ id: "r03", sku: "8886008101053", name: "Aqua 600 ml", unit: "botol", price: 4_000n, cost: 3_100n, packs: [["karton", 24n, 84_000n]], ready: 212n, onOrder: 240n, supplier: S3, restockedDaysAgo: 4, countedDaysAgo: 20 }),
  makeProduct({ id: "r04", sku: "8886008101060", name: "Aqua 1500 ml", unit: "botol", price: 7_000n, cost: 5_600n, packs: [["karton", 12n, 78_000n]], ready: 58n, supplier: S3, restockedDaysAgo: 11, countedDaysAgo: 20 }),
  makeProduct({ id: "r05", sku: "BRS-PW-05", name: "Beras Pandan Wangi 5 kg", unit: "karung", price: 78_000n, cost: 69_500n, ready: 24n, onOrder: 20n, supplier: S4, restockedDaysAgo: 9, countedDaysAgo: 48 }),
  makeProduct({ id: "r06", sku: "GLP-1KG", name: "Gulaku Premium 1 kg", unit: "pcs", price: 18_500n, cost: 16_200n, packs: [["bal", 20n, 355_000n]], ready: 64n, supplier: S4, restockedDaysAgo: 14, countedDaysAgo: 48 }),
  makeProduct({ id: "r07", sku: "BML-2L", name: "Bimoli Minyak Goreng 2 L", unit: "pouch", price: 38_000n, cost: 34_100n, packs: [["dus", 6n, 222_000n]], ready: 7n, onOrder: 36n, supplier: S4, restockedDaysAgo: 30, countedDaysAgo: 48 }),
  makeProduct({ id: "r08", sku: "8992761111014", name: "Teh Botol Sosro 450 ml", unit: "botol", price: 5_500n, cost: 4_300n, packs: [["krat", 24n, 125_000n]], ready: 96n, supplier: S3, restockedDaysAgo: 7, countedDaysAgo: 20 }),
  makeProduct({ id: "r09", sku: "KPA-ABC-S", name: "Kopi ABC Susu 31 g", unit: "sachet", price: 2_000n, cost: 1_550n, packs: [["renceng", 10n, 18_500n], ["dus", 120n, 210_000n]], ready: 480n, supplier: S3, restockedDaysAgo: 5, countedDaysAgo: 20 }),
  makeProduct({ id: "r10", sku: "SGS-TEH-25", name: "Teh Sariwangi 25 kantong", unit: "kotak", price: 7_500n, cost: 6_100n, ready: 41n, supplier: S3, restockedDaysAgo: 22, countedDaysAgo: 48 }),
  makeProduct({ id: "r11", sku: "8999999036232", name: "Pepsodent 190 g", unit: "pcs", price: 14_500n, cost: 11_800n, packs: [["lusin", 12n, 168_000n]], ready: 36n, supplier: S1, restockedDaysAgo: 25, countedDaysAgo: 48 }),
  makeProduct({ id: "r12", sku: "LFB-SBN-110", name: "Lifebuoy Sabun Batang 110 g", unit: "pcs", price: 5_000n, cost: 3_900n, packs: [["lusin", 12n, 57_000n]], ready: 88n, supplier: S1, restockedDaysAgo: 25, countedDaysAgo: 48 }),
  makeProduct({ id: "r13", sku: "SNS-340", name: "Sunsilk Shampoo 340 ml", unit: "botol", price: 32_000n, cost: 27_300n, ready: 3n, supplier: S1, restockedDaysAgo: 60 }),
  makeProduct({ id: "r14", sku: "RNS-DET-800", name: "Rinso Anti Noda 800 g", unit: "pcs", price: 24_500n, cost: 21_000n, packs: [["dus", 12n, 285_000n]], ready: 30n, supplier: S4, restockedDaysAgo: 13, countedDaysAgo: 48 }),
  makeProduct({ id: "r15", sku: "SNL-800", name: "Sunlight Jeruk Nipis 800 ml", unit: "pouch", price: 18_000n, cost: 15_400n, packs: [["dus", 12n, 210_000n]], ready: 44n, supplier: S4, restockedDaysAgo: 13, countedDaysAgo: 48 }),
  makeProduct({ id: "r16", sku: "ULT-SSU-1L", name: "Ultra Milk Full Cream 1 L", unit: "kotak", price: 19_500n, cost: 17_100n, packs: [["karton", 12n, 228_000n]], ready: 26n, onOrder: 48n, supplier: S3, restockedDaysAgo: 8, countedDaysAgo: 20 }),
  makeProduct({ id: "r17", sku: "FRS-KNL-370", name: "Frisian Flag Kental Manis 370 g", unit: "kaleng", price: 12_500n, cost: 10_600n, packs: [["karton", 48n, 580_000n]], ready: 52n, supplier: S3, restockedDaysAgo: 16, countedDaysAgo: 20 }),
  makeProduct({ id: "r18", sku: "ROT-TWR-300", name: "Roti Tawar Sari Roti", unit: "bungkus", price: 17_000n, cost: 14_000n, ready: 12n, supplier: S4, restockedDaysAgo: 1, countedDaysAgo: 3 }),
  makeProduct({ id: "r19", sku: "TLR-AYM-10", name: "Telur Ayam Negeri", unit: "butir", price: 2_200n, cost: 1_850n, packs: [["kg", 16n, 32_000n], ["peti", 240n, 460_000n]], ready: 360n, supplier: S4, restockedDaysAgo: 2, countedDaysAgo: 3 }),
  makeProduct({ id: "r20", sku: "8991002101630", name: "Chitato Sapi Panggang 68 g", unit: "pcs", price: 11_500n, cost: 9_200n, packs: [["dus", 20n, 220_000n]], ready: 40n, supplier: S3, restockedDaysAgo: 12, countedDaysAgo: 20 }),
  makeProduct({ id: "r21", sku: "OREO-133", name: "Oreo Vanilla 133 g", unit: "pcs", price: 9_500n, cost: 7_700n, packs: [["dus", 24n, 218_000n]], ready: 2n, onOrder: 48n, supplier: S3, restockedDaysAgo: 40, countedDaysAgo: 48 }),
  makeProduct({ id: "r22", sku: "KCP-BGO-600", name: "Kecap Bango 600 ml", unit: "pouch", price: 26_000n, cost: 22_400n, packs: [["dus", 12n, 300_000n]], ready: 19n, supplier: S4, restockedDaysAgo: 21, countedDaysAgo: 48 }),
  makeProduct({ id: "r23", sku: "SMB-ABC-335", name: "Sambal ABC Asli 335 ml", unit: "botol", price: 16_500n, cost: 13_900n, ready: 23n, supplier: S4, restockedDaysAgo: 21 }),
  makeProduct({ id: "r24", sku: "MSK-RYC-230", name: "Royco Kaldu Ayam 230 g", unit: "pcs", price: 10_500n, cost: 8_800n, packs: [["dus", 24n, 240_000n]], ready: 33n, supplier: S4, restockedDaysAgo: 19, countedDaysAgo: 48 }),
  makeProduct({ id: "r25", sku: "PMP-MMY-S", name: "MamyPoko Pants S 34", unit: "pack", price: 72_000n, cost: 63_500n, ready: 9n, supplier: S1, restockedDaysAgo: 33, countedDaysAgo: 48 }),
  makeProduct({ id: "r26", sku: "TSU-PSO-250", name: "Paseo Tisu Wajah 250 s", unit: "pack", price: 14_000n, cost: 11_300n, packs: [["dus", 24n, 324_000n]], ready: 27n, supplier: S1, restockedDaysAgo: 27, countedDaysAgo: 48 }),
  makeProduct({ id: "r27", sku: "BTR-ABC-AA", name: "Baterai ABC AA (isi 2)", unit: "pack", price: 8_000n, cost: 6_000n, packs: [["box", 12n, 90_000n]], ready: 60n, supplier: S4, restockedDaysAgo: 70 }),
  makeProduct({ id: "r28", sku: "GAS-35", name: "Gas LPG 3 kg (isi ulang)", unit: "tabung", price: 22_000n, cost: 19_000n, ready: 14n, onOrder: 20n, supplier: S4, restockedDaysAgo: 3, countedDaysAgo: 3 }),
  makeProduct({ id: "r29", sku: "ES-BTU-KRS", name: "Es Batu Kristal", unit: "bungkus", price: 6_000n, cost: 3_500n, ready: 30n, supplier: S4, restockedDaysAgo: 1 }),
  makeProduct({ id: "r30", sku: "RKK-GDG-12", name: "Gudang Garam Filter 12", unit: "bungkus", price: 27_500n, cost: 25_200n, packs: [["slop", 10n, 270_000n]], ready: 80n, supplier: S3, restockedDaysAgo: 4, countedDaysAgo: 3 }),
  // Archived: discontinued lines, kept because sales history still points at them.
  makeProduct({ id: "r31", sku: "MIE-SDP-OLD", name: "Mie Sedaap Kari Spesial (lama)", unit: "pcs", price: 3_000n, cost: 2_500n, active: false, supplier: S3, restockedDaysAgo: 210 }),
  makeProduct({ id: "r32", sku: "SPR-KLG-330", name: "Sprite Kaleng 330 ml", unit: "kaleng", price: 7_000n, cost: 5_400n, active: false, supplier: S3, restockedDaysAgo: 150 }),
  makeProduct({ id: "r33", sku: "PLS-HRG-12", name: "Plastik HD Kresek 24x40", unit: "pack", price: 9_000n, cost: 6_500n, active: false, supplier: S4 }),
];

// An apotek shelf for pharmacy-mode stories — the same shape, medicine names,
// strip/box ladders, and a few Rx-only lines.
export const PHARMACY_CATALOG: Product[] = [
  makeProduct({ id: "prod-paracetamol", sku: "PCT-500", name: "Paracetamol 500 mg", unit: "tablet", price: 1_000n, cost: 700n, packs: [["strip", 10n, 9_500n], ["box", 100n, 90_000n]], ready: 1_240n, onOrder: 500n, supplier: S1, restockedDaysAgo: 12, countedDaysAgo: 30 }),
  makeProduct({ id: "p02", sku: "AMX-500", name: "Amoxicillin 500 mg", unit: "kapsul", price: 1_500n, cost: 950n, packs: [["strip", 10n, 14_000n], ["box", 100n, 135_000n]], ready: 620n, supplier: S1, restockedDaysAgo: 20, countedDaysAgo: 30, prescriptionRequired: true }),
  makeProduct({ id: "p03", sku: "CTM-4", name: "CTM 4 mg", unit: "tablet", price: 300n, cost: 150n, packs: [["strip", 10n, 2_800n]], ready: 900n, supplier: S2, restockedDaysAgo: 45, countedDaysAgo: 30 }),
  makeProduct({ id: "p04", sku: "OBH-100", name: "OBH Combi Batuk Berdahak 100 ml", unit: "botol", price: 21_000n, cost: 16_800n, ready: 18n, supplier: S2, restockedDaysAgo: 15, countedDaysAgo: 30 }),
  makeProduct({ id: "p05", sku: "MTF-500", name: "Metformin 500 mg", unit: "tablet", price: 600n, cost: 320n, packs: [["strip", 10n, 5_500n], ["box", 100n, 52_000n]], ready: 1_500n, onOrder: 1_000n, supplier: S1, restockedDaysAgo: 8, countedDaysAgo: 30, prescriptionRequired: true }),
  makeProduct({ id: "p06", sku: "AML-5", name: "Amlodipine 5 mg", unit: "tablet", price: 800n, cost: 420n, packs: [["strip", 10n, 7_500n], ["box", 30n, 21_500n]], ready: 4n, onOrder: 300n, supplier: S1, restockedDaysAgo: 50, countedDaysAgo: 30, prescriptionRequired: true }),
  makeProduct({ id: "p07", sku: "PRM-FRT-20", name: "Promag Tablet", unit: "tablet", price: 1_000n, cost: 720n, packs: [["strip", 12n, 11_000n]], ready: 240n, supplier: S2, restockedDaysAgo: 10 }),
  makeProduct({ id: "p08", sku: "BTD-60", name: "Betadine Antiseptik 60 ml", unit: "botol", price: 42_000n, cost: 35_500n, ready: 11n, supplier: S2, restockedDaysAgo: 35, countedDaysAgo: 30 }),
  makeProduct({ id: "p09", sku: "VTC-IPI", name: "Vitamin C IPI 50", unit: "botol", price: 9_000n, cost: 6_900n, ready: 37n, supplier: S2, restockedDaysAgo: 28, countedDaysAgo: 30 }),
  makeProduct({ id: "p10", sku: "SLM-KPS", name: "Salbutamol 2 mg", unit: "tablet", price: 400n, cost: 210n, packs: [["strip", 10n, 3_800n]], ready: 2n, supplier: S1, restockedDaysAgo: 90, prescriptionRequired: true }),
  makeProduct({ id: "p11", sku: "MSK-3PLY", name: "Masker Medis 3 Ply", unit: "pcs", price: 1_000n, cost: 450n, packs: [["box", 50n, 35_000n]], ready: 800n, supplier: S2, restockedDaysAgo: 60 }),
  makeProduct({ id: "p12", sku: "DXM-15", name: "Dextromethorphan 15 mg", unit: "tablet", price: 350n, cost: 180n, packs: [["strip", 10n, 3_200n]], ready: 0n, active: false, supplier: S2, restockedDaysAgo: 180 }),
];

// Both catalogs carry their maker (see attachMakers). Done here rather than in
// makeProduct because the mapping is positional — it is a property of the
// catalog, not of one product's seed.
attachMakers(RETAIL_CATALOG);
attachMakers(PHARMACY_CATALOG);

/**
 * ListProducts, faithfully enough to page, search and switch tabs live in a
 * story: the same predicate as `productFilters.apply` (archived tab, name/SKU
 * substring, "last opname before X OR never counted"), ordered by name like
 * the server, then paged.
 */
export function filterProducts(
  catalog: Product[],
  req: Pick<ListProductsRequest, "onlyArchived" | "includeInactive" | "query" | "opnameBefore">,
): Product[] {
  const q = req.query.trim().toLowerCase();
  return catalog
    .filter((p) => (req.onlyArchived ? !p.active : req.includeInactive || p.active))
    .filter((p) => !q || p.name.toLowerCase().includes(q) || p.sku.toLowerCase().includes(q))
    .filter((p) => !req.opnameBefore || !p.lastStocktakeDate || p.lastStocktakeDate < req.opnameBefore)
    .sort((a, b) => a.name.localeCompare(b.name));
}

/** GetProductsSummary over the SAME filtered set the list pages through. */
export function summarizeProducts(rows: Product[]) {
  return rows.reduce(
    (acc, p) => ({
      readyStock: acc.readyStock + p.readyStock,
      readyValuation: acc.readyValuation + p.stockValuation,
      onOrderStock: acc.onOrderStock + p.onOrderStock,
      onOrderValuation: acc.onOrderValuation + p.onOrderValuation,
    }),
    { readyStock: 0n, readyValuation: 0n, onOrderStock: 0n, onOrderValuation: 0n },
  );
}

/**
 * The shop's unit catalog (Settings ▸ Units), which the list's Units popover
 * groups by. Derived from the products' own ladders so the two never disagree.
 */
export function unitBasesFor(catalog: Product[]): UnitBase[] {
  const byBase = new Map<string, Map<string, [bigint, number]>>();
  for (const p of catalog) {
    const packs = byBase.get(p.unit) ?? new Map<string, [bigint, number]>();
    for (const u of p.units) if (!u.isBase && !packs.has(u.name)) packs.set(u.name, [u.factor, u.sortOrder]);
    byBase.set(p.unit, packs);
  }
  return Array.from(byBase, ([name, packs], i) =>
    new UnitBase({
      id: `ub-${i + 1}`,
      name,
      active: true,
      createdAt: daysAgo(300),
      derivatives: Array.from(packs, ([dName, [factor, sortOrder]], j) => ({
        id: `ub-${i + 1}-d${j + 1}`,
        baseUnitId: `ub-${i + 1}`,
        name: dName,
        factor,
        sortOrder,
        active: true,
      })),
    }),
  );
}

// --- restock history --------------------------------------------------------

// The discount shapes a PO line can carry, cycled so a history shows each one:
// none · percent per item · fixed on the line · percent on the line. Percent
// is in basis points (250 = 2.5%), fixed in minor units — as on the wire.
const RESTOCK_DISCOUNTS: Array<Pick<ProductRestockLog, "discountType" | "discountValue" | "discountPerItem">> = [
  { discountType: "", discountValue: 0n, discountPerItem: false },
  { discountType: "PERCENT", discountValue: 250n, discountPerItem: true },
  { discountType: "FIXED", discountValue: 5_000n, discountPerItem: false },
  { discountType: "PERCENT", discountValue: 500n, discountPerItem: false },
];

/**
 * `count` restock log rows for a product, newest arrival first (the server's
 * order), one every ~12 days, rotating across `suppliers`.
 *
 * The price is what the row really carries — the NET cost per BASE unit — and
 * drifts: the newest equals the product's `referenceCost` (its latest batch
 * cost) and each step back in time is ~1.5% cheaper, with the second supplier
 * pricing a little under the first. Enough texture that "is this supplier
 * getting dearer / who is cheaper" is a real question to ask of the table.
 */
export function restockLogsFor(p: Product, count: number, suppliers = SUPPLIERS.slice(0, 3)): ProductRestockLog[] {
  return Array.from({ length: count }, (_, i) => {
    const sup = suppliers[i % suppliers.length];
    const drift = 1000n - BigInt(i) * 15n; // per-mille of the newest price
    const supplierSpread = BigInt(i % suppliers.length) * 10n; // later suppliers a touch cheaper
    const price = (p.referenceCost * (drift - supplierSpread)) / 1000n;
    const arrived = i * 12 + 2;
    return new ProductRestockLog({
      id: `${p.id}-rl${i + 1}`,
      supplierId: sup.id,
      price: price > 0n ? price : 1n,
      qty: BigInt(((i * 7) % 5) + 2) * 100n,
      ...RESTOCK_DISCOUNTS[i % RESTOCK_DISCOUNTS.length],
      restockCreatedAt: daysAgo(arrived + 3 + (i % 3)),
      restockArrivedAt: daysAgo(arrived),
    });
  });
}

// --- warehouses ---------------------------------------------------------------
// Two, so the TopBar renders the real picker (a single-warehouse user gets a
// read-only chip instead). MAIN is the migration-seeded default.
export const WAREHOUSES = [
  new Warehouse({ id: "wh-main", code: "MAIN", name: "Gudang Utama", isDefault: true, active: true }),
  new Warehouse({ id: "wh-toko2", code: "TK2", name: "Toko Cabang Sudirman", active: true }),
];

// --- avatars ---------------------------------------------------------------

// A drawn JPEG rather than a committed binary: the backend only ever serves
// JPEG renditions (SVG is rejected on both sides), so the fixture is one too.
export async function avatarJpeg(): Promise<Uint8Array<ArrayBuffer>> {
  const canvas = new OffscreenCanvas(128, 128);
  const ctx = canvas.getContext("2d")!;
  const g = ctx.createLinearGradient(0, 0, 128, 128);
  g.addColorStop(0, "#2563eb");
  g.addColorStop(1, "#0d9488");
  ctx.fillStyle = g;
  ctx.fillRect(0, 0, 128, 128);
  ctx.fillStyle = "rgba(255,255,255,0.85)";
  ctx.beginPath();
  ctx.arc(64, 50, 22, 0, Math.PI * 2);
  ctx.fill();
  ctx.beginPath();
  ctx.ellipse(64, 118, 42, 34, 0, 0, Math.PI * 2);
  ctx.fill();
  const blob = await canvas.convertToBlob({ type: "image/jpeg", quality: 0.85 });
  return new Uint8Array(await blob.arrayBuffer());
}
