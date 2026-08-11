# YouTube — how-do-i-enter-existing-stock

Assets: `tutorial.webm` (102 s, no narration) · `thumbnail.jpg` (variant a)

## Title

Front-loaded with what a shop owner actually types into search ("cara input stok
awal"), no brand name — the channel name already sits under the title.

**Primary (49 chars)**

> Cara Input Stok Awal ke Aplikasi Kasir (Impor CSV)

**Alternates**

> Stok Lama di Rak, Cara Memasukkannya Sekali Unggah   (50)
> Impor Stok Awal: 1 File CSV, Tanpa Order Restock     (48)

## Description

The first two lines are all that show before "Selengkapnya", so they carry the
answer. **These videos have no narration**, so YouTube has no transcript to
index — every term someone might search has to appear in this text.

```
Barang yang sudah ada di rak tidak perlu dibeli ulang lewat order restock. Pakai
Impor Stok di halaman Batch: satu file CSV, semua stok awal masuk sekaligus.

Video ini merekam aplikasi Justmart yang sebenarnya — bukan animasi, bukan
mockup. Termasuk apa yang terjadi kalau file yang sama diimpor dua kali.

CARANYA
1. Pastikan produknya sudah ada di katalog (Produk > Impor CSV kalau masih kosong).
   Baris CSV dicocokkan lewat SKU.
2. Cek gudang aktif di kanan atas — stok masuk ke gudang itu.
3. Buka Inventaris > Batch, klik Impor Stok.
4. Klik Unduh templat, isi di Excel atau Google Sheets.
5. Klik Pilih file CSV. Semua baris ditampilkan dulu beserta statusnya.
6. Klik Impor. Muncul ringkasan: X dibuat, Y dilewati, Z error.

KOLOM CSV
sku          wajib, harus sama persis dengan SKU produk
quantity     wajib, bilangan bulat lebih dari 0
unit         opsional, nama paket (mis. box). 5 box isi 100 = 500 unit dasar
cost         opsional, harga modal PER UNIT DASAR (per tablet, bukan per box)
batch_number opsional, tapi sangat disarankan diisi
expiry_date  opsional, format YYYY-MM-DD. Kosong = tidak kedaluwarsa

Nama kolom boleh bahasa Indonesia: jumlah, satuan, harga, modal, kedaluwarsa.

KALAU FILE DIIMPOR DUA KALI
Baris yang ada nomor batch-nya akan DILEWATI, jadi aman diulang.
Baris TANPA nomor batch akan dibuat lagi — stoknya jadi dobel.
Karena itu isilah batch_number. Kalau sudah terlanjur dobel, betulkan lewat Stok
opname.

KALAU ADA BARIS YANG GAGAL
Impor bersifat per baris: satu baris rusak tidak membatalkan yang lain.
- product not found for this SKU: SKU belum ada di katalog
- unit tidak ditemukan: nama satuan tidak cocok dengan satuan produk
- quantity harus bilangan bulat lebih dari 0
- expiry_date harus YYYY-MM-DD (mis. 2027-12-31, bukan 31/12/2027)
- Kolom wajib hilang: baris judul CSV tidak sesuai templat

CATATAN
- Impor Stok tidak membuat utang ke pemasok dan tidak membuat PO maupun faktur.
  Untuk barang yang baru dibeli, pakai Restok.
- Batch hasil impor tidak punya pemasok, dan tanggal terimanya hari impor.
- Hanya Owner dan Admin yang bisa mengimpor stok.
- Satu kali impor menampung sampai 5.000 baris.
- Setelah impor, stok langsung siap dijual di Kasir dan mengikuti antrean FEFO
  (yang paling dekat kedaluwarsa terjual lebih dulu).

Justmart — aplikasi kasir dan stok untuk toko dan apotek.
```

**No chapters or timestamps.** Recording length varies run to run (this take is
102 s; an identical earlier take was 103 s), so hardcoded times go stale on the
next re-record.

## Tags

```
stok awal, cara input stok awal, impor stok, impor csv, stok opname, aplikasi kasir,
aplikasi stok barang, aplikasi apotek, manajemen stok toko, saldo awal stok,
input stok barang, aplikasi kasir toko, software apotek, justmart, batch stok,
nomor batch, kedaluwarsa, pindah dari catatan manual, stok dobel
```

## Pinned comment

```
Ringkasan tertulis + tabel penyebab baris gagal ada di FAQ. Kalau stok Anda
sudah terlanjur dobel karena file diimpor dua kali, betulkan lewat Stok opname —
hitung ulang fisiknya dan biarkan sistem yang menyesuaikan.
```
