# Bagaimana cara mencetak label barcode produk?

**Buka produknya, klik Cetak barcode. Barcode yang dicetak memakai SKU produk
itu — jadi hasil cetakannya pasti terbaca waktu discan di Kasir.**

[Lihat videonya](tutorial.webm)

## Ringkasnya

Di Justmart **barcode produk adalah SKU-nya**. Tidak ada kolom barcode
tersendiri, dan itu memang disengaja: Kasir mencari barang dengan mencocokkan
SKU persis, jadi label yang dicetak dari katalog dijamin ketemu barangnya. Kalau
ada dua kode yang berbeda — satu untuk dicetak, satu untuk discan — cepat atau
lambat keduanya akan berbeda isi.

Satu label berisi tiga hal saja:

- **nama produk**,
- **barcode** dari SKU, dengan SKU-nya ikut tercetak di bawah garis-garisnya
  (kalau scanner gagal membaca, kodenya masih bisa diketik manual),
- **harga** dari satuan yang Anda pilih.

Label **tidak memuat harga modal**, jadi kasir pun boleh mencetaknya.

## Dua tujuan cetak

| | Printer termal | Lembar label |
|---|---|---|
| Keluar di mana | printer struk toko (printer yang sama dipakai Kasir) | printer biasa (inkjet/laser) lewat dialog cetak browser |
| Kertas | kertas struk 58mm atau 80mm | kertas stiker A4/Letter |
| Bentuknya | satu label per potong, terpotong sendiri | banyak label dalam satu halaman, ada garis putus-putus sebagai panduan gunting |
| Batas panjang SKU | **ada** — lihat tabel di bawah | tidak ada |
| Butuh setelan printer | ya, di **Pengaturan ▸ Pencetakan** | tidak |

Kalau printer termal toko belum diatur, **Lembar label** tetap bisa dipakai —
jalurnya tidak lewat server sama sekali.

## Caranya

1. Buka **Produk** (di mode apotek namanya **Obat**), lalu klik produknya.
2. Klik **Cetak barcode** di kanan atas.
3. **Harga yang ditampilkan** — pilih satuannya. Harga di label ikut satuan ini,
   jadi label untuk box memuat harga box, bukan harga per butir.
4. **Jumlah** — berapa lembar label yang sama. Maksimal 100 sekali cetak.
5. **Cetak ke** — pilih **Printer termal** atau **Lembar label**. Untuk lembar
   label muncul isian **Per baris** (berapa label per baris di kertas).
6. Lihat **Pratinjau** — itu persis yang akan tercetak.
7. Klik **Cetak**.

## Setelah dicetak

Mencetak label **tidak mengubah data apa pun** — stok, harga, dan riwayat tidak
tersentuh. Jadi aman diulang berapa kali pun.

Yang perlu diingat: harga di label adalah harga **saat dicetak**. Kalau nanti
harga jualnya diubah, label lama di rak jadi salah — cetak ulang labelnya.

Semua peran boleh mencetak: **Owner, Admin, Kasir, dan Apoteker**. Label tidak
memuat data modal, dan orang yang menata rak biasanya bukan pemilik toko.

## Kalau tidak bisa dicetak

| Pesan | Artinya | Perbaikannya |
|---|---|---|
| **SKU ini memuat karakter yang tidak bisa dibawa barcode** | SKU-nya mengandung huruf beraksen, emoji, atau karakter aneh | pakai huruf, angka, dan tanda baca biasa saja (`A-Z`, `0-9`, `-`, `.`, `/`) |
| **SKU ini terlalu panjang untuk mencetak barcode yang bisa dipindai pada lebar kertas saat ini** | barcode-nya lebih lebar dari kertas struk | pakai kertas 80mm, perpendek SKU-nya, atau cetak lewat **Lembar label** |
| **Satuan itu sudah diarsipkan — pilih yang lain** | satuan yang dipilih sudah tidak aktif | pilih satuan lain di **Harga yang ditampilkan** |
| Pesan tentang pencetakan belum diatur | printer termal belum disetel | atur di **Pengaturan ▸ Pencetakan**, atau pakai **Lembar label** |

Tombol **Cetak** juga langsung mati (disertai peringatan di dalam dialog) kalau
SKU-nya memang tidak bisa dijadikan barcode — jadi Anda tahu sebelum mencoba.

## Berapa panjang SKU yang muat?

Lebar barcode tergantung jumlah karakter SKU, dan kertas struk itu sempit:

| Kertas | Muat sampai kira-kira |
|---|---|
| 58mm (lebar 32 karakter) | **12 karakter** |
| 80mm (lebar 48 karakter) | **21 karakter** |

Untuk SKU yang lebih pendek, garis barcode-nya otomatis dibuat lebih tebal —
makin tebal garisnya, makin gampang discan. Jadi **SKU pendek itu bukan cuma
muat, tapi juga lebih enak dibaca scanner.**

Kalau katalog Anda memakai SKU panjang dan tidak ingin diubah, jalur **Lembar
label** tidak punya batas ini.

## Kenapa ditolak, bukan dipaksakan

Kalau barcode-nya lebih lebar dari kertas, printer termal tidak memberi
peringatan apa-apa — dia **memotong** gambarnya. Hasilnya stiker yang kelihatan
normal, tertempel rapi di rak, dan **tidak terbaca sama sekali** waktu discan di
kasir. Kesalahan seperti itu baru ketahuan berminggu-minggu kemudian, di depan
antrean pembeli.

Karena itu Justmart memilih menolak dengan alasan yang jelas daripada mencetak
label yang tampak benar tapi mati.
