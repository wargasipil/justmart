# Pesanan yang sudah selesai, bisa dibatalkan?

**Bisa — lewat tombol Refund di halaman pesanannya, maksimal 1 hari setelah
transaksi, dan hanya oleh Owner atau Admin. Pesanannya tidak dihapus: statusnya
berubah jadi Dikembalikan, lengkap dengan alasannya.**

[Lihat videonya](tutorial.webm)

## Ringkasnya

Yang dibatalkan di sini adalah **transaksi yang sudah diselesaikan di Kasir** —
yang sudah punya nomor `INV-…` dan sudah masuk Riwayat order.

Membatalkannya **bukan menghapusnya**. Pesanan itu tetap ada di riwayat, dengan:

- status **Dikembalikan**,
- **waktu refund**, **jumlah refund**, dan **alasan** yang Anda tulis,
- keterangan apakah **barangnya kembali ke stok** atau tidak.

Itu disengaja. Nomor `INV-…` sudah terlanjur dipakai dan mungkin sudah tercetak
di struk pelanggan; kalau barisnya dihapus, nomor itu hilang tanpa penjelasan
dan tidak ada lagi jejak siapa membatalkan apa.

**Yang tidak bisa dilakukan: refund sebagian.** Refund selalu satu pesanan
penuh. Kalau pelanggan hanya mengembalikan satu barang dari lima, refund seluruh
pesanannya lalu buat transaksi baru berisi empat barang yang jadi dibeli.

### Ini beda dengan keranjang yang belum selesai

Keranjang di Kasir yang **belum ditekan Selesaikan** bukan pesanan sama sekali.
Keranjang seperti itu dibuang begitu Anda keluar dari Kasir — tidak pernah masuk
Riwayat order, tidak perlu di-refund, dan tidak mengurangi stok.

## Caranya

1. Buka **Riwayat order**, lalu klik pesanannya.
2. Klik **Refund** di kanan atas.
3. Isi **Alasan** — misalnya "pelanggan berubah pikiran". Alasan ini ikut
   tersimpan di pesanannya.
4. Tentukan **Kembalikan barang ke stok** — ini keputusan yang penting, lihat di
   bawah.
5. Klik **Refund pesanan**.

## Sakelar "Kembalikan barang ke stok"

| Posisi | Artinya | Dipakai kalau |
|---|---|---|
| **Nyala** (bawaan) | barangnya masuk lagi ke stok, ke **batch yang sama** dengan waktu dijual | barangnya benar-benar kembali ke toko dan masih layak dijual |
| **Mati** | hanya uangnya yang dikembalikan; stok tidak berubah | barangnya tidak kembali, sudah rusak, atau salah input harga saja |

Kalau nyala, di mode apotek **jumlah obat yang sudah dilayani pada resep ikut
dikembalikan**, jadi sisa resep pasien kembali seperti semula. Kalau mati, obat
dianggap tetap dibawa pasien dan resepnya tidak diutak-atik.

## Setelah di-refund

- Pesanannya **hilang dari laporan omzet dan laba** — semua laporan hanya
  menghitung pesanan berstatus Selesai. Tidak perlu koreksi manual.
- Pesanan itu bisa dicari lewat tab **Dikembalikan** di Riwayat order.
- **Tombol Cetak struk hilang.** Kalau Anda butuh struknya sebagai bukti,
  cetak dulu sebelum di-refund.
- Kalau stoknya dikembalikan, barangnya masuk lagi ke batch asalnya — jadi
  tanggal kedaluwarsanya tetap benar, bukan jadi barang baru.

## Kalau tombol Refund tidak muncul

| Keadaan | Kenapa | Yang bisa dilakukan |
|---|---|---|
| Pesanan sudah **lebih dari 1 hari** | batas refund 1 hari sejak transaksi diselesaikan | tangani manual: buat penyesuaian stok kalau barangnya kembali, dan catat uangnya di luar aplikasi |
| Status sudah **Dikembalikan** | satu pesanan hanya bisa di-refund sekali | tidak ada yang perlu dilakukan — refundnya sudah tercatat |
| Anda masuk sebagai **Kasir** atau **Apoteker** | refund menyangkut uang dan stok sekaligus, jadi dibatasi ke tingkat pengelola | minta Owner atau Admin yang melakukannya |
| Status **Dibatalkan** atau **Draf** | belum pernah jadi penjualan | tidak perlu di-refund |

## Kenapa dibatasi 1 hari

Refund menyentuh dua hal sekaligus: **uang di laci** dan **stok di rak**. Selama
hari yang sama, keduanya masih bisa dicocokkan — kasnya belum ditutup dan
barangnya masih jelas asalnya. Lewat dari itu, stok sudah bergerak karena
penjualan lain, laporan harian sudah dibaca, dan menambahkan barang kembali ke
tanggal kemarin justru membuat angka yang sudah dipakai jadi tidak cocok.

Karena itu Justmart memilih menutup jalurnya daripada membiarkan koreksi lama
diam-diam mengubah laporan yang sudah jadi.
