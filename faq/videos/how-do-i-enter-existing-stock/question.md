# Bagaimana memasukkan stok yang sudah ada di toko?

**Pakai Impor Stok di halaman Batch: unggah satu file CSV berisi isi rak Anda
sekarang. Tidak perlu membuat order restock, dan tidak menimbulkan utang ke
pemasok.**

[Lihat videonya](tutorial.webm)

## Ringkasnya

Waktu pertama kali pindah dari catatan manual, barang Anda sudah ada di rak —
tinggal dicatat, bukan dibeli lagi. Itulah yang dikerjakan **Impor Stok**:
setiap baris CSV menjadi satu **batch** baru berikut satu mutasi masuk ke
**gudang aktif**, dengan alasan "Opening stock (import)".

Yang perlu dibedakan:

| | Impor Stok | Restok (order pembelian) |
|---|---|---|
| Untuk apa | barang yang **sudah** di rak | barang yang **baru dibeli** dari pemasok |
| Pemasok | tidak ada | wajib |
| Utang / pembayaran | tidak dibuat | tercatat sebagai utang pemasok |
| Dokumen | tidak ada PO maupun faktur | PO + tanda terima |

Jadi jangan membuat order restock fiktif hanya untuk memasukkan stok awal —
Anda akan berutang kepada pemasok yang sebenarnya tidak Anda utangi.

Beberapa hal yang perlu diketahui sebelum mulai:

- **Produknya harus sudah ada di katalog.** Barisnya dicocokkan lewat **SKU**.
  Kalau katalog masih kosong, impor katalog dulu di **Produk ▸ Impor CSV**, baru
  impor stoknya.
- **Stok masuk ke gudang yang sedang aktif** (pilihan gudang di kanan atas).
  Cek dulu sebelum impor kalau toko Anda punya lebih dari satu gudang.
- **Hanya Owner dan Admin** yang bisa mengimpor stok.
- Batch hasil impor **tidak punya pemasok** dan tanggal terimanya **hari impor**
  — memang begitu, karena ini saldo awal, bukan pembelian.

## Caranya

1. Buka **Inventaris ▸ Batch**.
2. Klik **Impor Stok**.
3. Klik **Unduh templat** untuk mendapat contoh file CSV, lalu isi di Excel atau
   Google Sheets.
4. Klik **Pilih file CSV** dan pilih file yang sudah diisi.
5. Aplikasi menampilkan **pratinjau semua baris** lengkap dengan status per
   baris: hijau **OK**, atau merah beserta alasannya. Baris merah tidak akan
   dikirim.
6. Klik **Impor N** (N = jumlah baris yang siap).
7. Muncul ringkasan: **X dibuat · Y dilewati · Z error**, dengan daftar SKU yang
   gagal kalau ada.

## Isi kolomnya

| Kolom | Wajib | Keterangan |
|---|---|---|
| `sku` | ya | harus sama persis dengan SKU produk di katalog |
| `quantity` | ya | bilangan bulat > 0. Dalam satuan `unit` kalau kolom itu diisi, kalau kosong dalam **unit dasar** |
| `unit` | tidak | nama paket, misal `box`. Harus salah satu satuan produk tersebut. `5` + `box` (isi 100) = 500 unit dasar |
| `cost` | tidak | harga modal **per unit dasar**, rupiah penuh — bukan per box. Kosong = 0 |
| `batch_number` | tidak | nomor batch/lot. **Sangat disarankan diisi** — lihat di bawah |
| `expiry_date` | tidak | format `YYYY-MM-DD`. **Kosong = tidak kedaluwarsa** |

Nama kolom boleh memakai padanan Indonesia: `jumlah` untuk `quantity`, `satuan`
untuk `unit`, `harga`/`modal` untuk `cost`, `kedaluwarsa` untuk `expiry_date`.

## Kalau file diimpor dua kali

Ini bagian yang paling sering menimbulkan masalah, jadi perlakuannya sengaja
dibuat berbeda:

- Baris yang **ada `batch_number`**-nya dan nomornya sudah pernah masuk akan
  **dilewati**. Aman diulang — stok tidak dobel.
- Baris yang **tanpa `batch_number`** akan **dibuat lagi setiap kali impor**.
  Mengimpor file yang sama dua kali berarti stoknya jadi dua kali lipat.

Karena itu isilah `batch_number` kalau ada — kalau file Anda perlu diperbaiki
dan diimpor ulang, nomor batch itulah yang menjaga Anda dari stok dobel.

Kalau sudah terlanjur dobel, betulkan lewat **Stok opname** — hitung ulang fisik
barangnya dan biarkan sistem yang menyesuaikan.

## Kalau ada baris yang gagal

Impor bersifat **per baris**: satu baris rusak tidak membatalkan yang lain, jadi
baris yang benar tetap masuk dan Anda tinggal memperbaiki sisanya.

| Pesan | Artinya | Perbaikannya |
|---|---|---|
| `product not found for this SKU` | SKU-nya tidak ada di katalog | impor/buat produknya dulu, atau betulkan ketikan SKU |
| `unit "..." not found for this product` | nama satuan tidak cocok dengan satuan produk itu | pakai nama satuan persis seperti di produk, atau kosongkan |
| `sku, quantity wajib diisi` | ada sel yang kosong | lengkapi barisnya |
| `quantity harus bilangan bulat > 0` | jumlahnya kosong, nol, minus, atau desimal | isi bilangan bulat positif |
| `harga modal tidak valid` | kolom cost bukan angka | tulis angka saja, tanpa "Rp" |
| `expiry_date harus YYYY-MM-DD` | formatnya salah, misal `31/12/2027` | tulis `2027-12-31`, atau kosongkan |
| `Kolom wajib hilang: sku, quantity` | baris judul CSV tidak berisi kolom itu | pakai templat yang disediakan |
| `Tidak dapat membaca file CSV.` | filenya bukan CSV (misal `.xlsx`) | simpan sebagai CSV dulu |
| status **dilewati** | nomor batch itu sudah pernah masuk untuk produk yang sama | tidak perlu diapa-apakan, memang sengaja dilewati |

Satu kali impor menampung sampai 5.000 baris. Kalau lebih, pecah filenya.

## Setelah impor

Stoknya langsung terpakai di seluruh aplikasi: muncul di daftar **Batch**, kolom
**Ready** di halaman Produk bertambah, tercatat di **Mutasi stok**, dan barangnya
sudah bisa dijual di **Kasir**. Kalau ada `expiry_date`, batch itu ikut antrean
FEFO — yang paling dekat kedaluwarsa yang terjual lebih dulu.

Aturannya dibuat ketat karena stok di Justmart adalah buku besar: setiap
perubahan tercatat sebagai mutasi dan tidak pernah dihapus. Itulah yang membuat
angka stok selalu bisa ditelusuri asalnya — termasuk saldo awal yang Anda impor
hari ini.
