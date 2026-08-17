# YouTube

## Judul (pilih satu)

1. **Cara Mengisi Token Tunnel Cloudflare di Pengaturan** (47)
2. **Buka Aplikasi Toko dari Luar, Tanpa Setting Router** (52)
3. **Akses Toko dari Rumah: Isi Token Tunnel Cloudflare** (51)

Pilihan 1 paling aman: kata kunci di depan, tanpa nama merek, dan persis
pertanyaan yang diketik orang yang sudah tahu istilahnya. Pilihan 2 dipakai
kalau kaitnya mau dibuat dari kebutuhannya, bukan dari nama fiturnya — orang
yang mencari "akses aplikasi kasir dari luar" belum tentu tahu kata "tunnel".

## Deskripsi

```
Token tunnel Cloudflare diisi lewat Pengaturan, tab Akses jarak jauh, lalu klik
Simpan dan jalankan ulang aplikasi. Tidak perlu mengedit file config.yaml, dan
hanya Owner yang bisa mengisinya.

Gunanya: toko bisa dibuka dari luar jaringan, dari rumah atau dari HP, lewat
satu alamat internet. Tanpa IP publik dan tanpa port forwarding di router. Ini
yang menolong toko dengan internet CGNAT, yang memang tidak bisa dibuka dari
luar sama sekali.

LANGKAHNYA
1. Buka Pengaturan, lalu tab Akses jarak jauh.
2. Tempel token tunnel Anda di kolom Token tunnel Cloudflare.
3. Klik Simpan.
4. Jalankan ulang aplikasi.

Tokennya berasal dari akun Cloudflare Anda sendiri, di bagian Install connector
milik tunnel tersebut. Pembuatan tunnel di sisi Cloudflare mengikuti panduan
Cloudflare, bukan aplikasi ini.

YANG DIISI HANYA TOKENNYA
Cloudflare tidak memberi token polos, melainkan satu baris perintah yang
bentuknya cloudflared.exe service install eyJhIjoi dan seterusnya. Yang diisi ke
aplikasi hanya bagian panjang setelah kata install. Tanpa cloudflared.exe, tanpa
service install, tanpa tanda kutip, tanpa spasi. Kalau seluruh barisnya
ditempel, aplikasi menolaknya dengan pesan "Isi tokennya saja, tanpa perintah,
tanda kutip, atau spasi". Aplikasi sengaja tidak menebak bagian mana dari
kalimat panjang itu yang merupakan token, karena salah tebak berarti menyimpan
kredensial yang tidak pernah diperiksa.

TUNNEL BARU MENYALA SETELAH APLIKASI DIJALANKAN ULANG
Token dibaca sekali waktu aplikasi mulai berjalan, jadi menyimpan token belum
menyalakan apa-apa. Selama belum dijalankan ulang, halaman Pengaturan
menampilkan pengingatnya. Pada jalan pertama setelah itu, aplikasi mengunduh
cloudflared.exe ke folder yang sama dengan aplikasinya, jadi butuh internet.

TOKEN TIDAK DITAMPILKAN UTUH LAGI
Setelah tersimpan, yang tampil hanya potongannya, cukup untuk memastikan token
yang benar sudah masuk. Kalau perlu diganti, tempel token baru. Untuk
mematikannya, klik Hapus token lalu jalankan ulang.

KALAU TUNNELNYA TIDAK MENYALA
- Pesan "Isi tokennya saja": seluruh baris perintah ikut tertempel. Hapus
  isinya, tempel ulang hanya bagian panjang setelah kata install.
- Status menyebut config.yaml: token lama di file itu yang masih dipakai, dan
  akan digantikan token baru setelah dijalankan ulang.
- Status menyebut environment variable: ada JUSTMART_CLOUDFLARE_TUNNEL_TOKEN di
  komputer itu dan nilainya menang. Hapus dulu variable-nya.
- Pesan hanya didukung di Windows: servernya Linux atau Docker, jalankan
  cloudflared sendiri di sana.
- Sudah dijalankan ulang tapi alamatnya tetap kosong: di Cloudflare, Public
  hostname tunnel itu perlu diarahkan ke http://localhost:8080.

ALAMATNYA PUBLIK, LOGINNYA TETAP
Tunnel bukan pengganti login. Siapa pun yang tahu alamatnya tetap berhenti di
halaman login, jadi pastikan password Owner kuat sebelum menyalakannya. Alamat
jaringan lokal tetap jalan seperti biasa dan tetap lebih cepat untuk kasir di
dalam toko. Kalau tunnelnya bermasalah, kasir tetap bisa berjualan lewat
jaringan lokal.

Video ini rekaman aplikasi yang benar-benar berjalan, bukan animasi. Token yang
diketik di video adalah token palsu.
```

## Tag

```
cloudflare tunnel, token tunnel, akses aplikasi dari luar, remote access toko,
tanpa port forwarding, CGNAT, buka aplikasi kasir dari rumah, cloudflared,
zero trust, pengaturan aplikasi, aplikasi kasir, aplikasi apotek, POS Indonesia,
justmart
```

## Catatan

- **Tanpa `<` dan `>`** di judul, deskripsi, dan tag. YouTube membaca kurung
  sudut sebagai markup dan diam-diam memotong teks di sekitarnya. Video ini
  isinya jalur menu semua, jadi jebakannya besar — tulis biasa atau pakai `▸`.
- **Tanpa bab/timestamp.** Panjang rekaman berubah tiap kali direkam ulang.
- Video ini **tanpa narasi suara**, jadi deskripsi ini satu-satunya teks yang
  terbaca mesin. Karena itu istilah teknis yang mungkin diketik orang
  ("cloudflared", "CGNAT", "port forwarding", "zero trust") sengaja disebut
  lengkap, walaupun tidak satu pun muncul di layar.
- Nama variable **JUSTMART_CLOUDFLARE_TUNNEL_TOKEN** sengaja ditulis utuh:
  orang yang kena kasus itu akan mencarinya persis begitu.
- Thumbnail: `thumbnail.jpg` (varian **a**) — kolom token berisi seluruh baris
  perintah dan **ditolak**, dilingkari merah, dengan kait "TOKEN SAJA". Kesalahan
  itu yang paling sering terjadi, jadi menunjukkannya ditolak menjawab
  pertanyaannya lebih cepat daripada menunjukkan yang benar. Varian
  **b** memakai pemberitahuan "jalankan ulang" dengan kait "BELUM MENYALA";
  pakai itu kalau videonya tayang berdampingan dengan video Pengaturan lain,
  di mana penontonnya sudah tahu letak kolomnya dan yang menjebak justru
  anggapan bahwa menyimpan saja sudah cukup.
