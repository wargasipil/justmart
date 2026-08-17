# Bagaimana cara mengisi token tunnel Cloudflare?

**Buka Pengaturan ▸ Akses jarak jauh, tempel tokennya saja, klik Simpan, lalu
jalankan ulang Justmart.** Tokennya tidak perlu diketik ke file mana pun — sejak
versi ini semuanya lewat halaman Pengaturan, dan khusus Owner.

[Lihat videonya](tutorial.webm)

## Ringkasnya

Tunnel Cloudflare membuat Justmart bisa dibuka **dari luar toko** — dari rumah,
dari HP, dari cabang lain — lewat satu alamat internet, tanpa perlu IP publik
dan tanpa mengutak-atik port forwarding di router. Ini yang menyelamatkan toko
yang internetnya pakai CGNAT, yang memang tidak bisa dibuka dari luar sama
sekali.

Yang **bukan** tugas tunnel:

- **Bukan pengganti login.** Alamatnya publik; siapa pun yang tahu alamatnya
  tetap berhenti di halaman login. Karena itu password Owner jadi jauh lebih
  penting begitu tunnel menyala.
- **Bukan backup.** Data tetap di PC toko. Backup diatur terpisah di
  Pengaturan ▸ Backup.
- **Bukan alamat jaringan lokal.** Alamat LAN (misalnya 192.168.x.x) tetap
  jalan seperti biasa, dan tetap lebih cepat untuk kasir di dalam toko.

## Caranya

1. Buka **Pengaturan ▸ Akses jarak jauh**. Kalau belum pernah diisi, statusnya
   **Mati**.
2. Tempel token tunnel Anda di kolom **Token tunnel Cloudflare**.
3. Klik **Simpan**.
4. **Jalankan ulang Justmart.**

Tokennya berasal dari akun Cloudflare Anda sendiri, di bagian **Install
connector** milik tunnel tersebut. Justmart tidak mengurus pembuatan tunnel di
Cloudflare — ikuti panduan Cloudflare untuk itu.

## Yang diisi adalah tokennya saja

Cloudflare tidak menampilkan token polos, melainkan satu baris perintah yang
bentuknya seperti:

```
cloudflared.exe service install eyJhIjoiN2E4YjljMGQ...
```

Yang diisi ke Justmart **hanya bagian panjang setelah kata `install`** — tanpa
`cloudflared.exe`, tanpa `service install`, tanpa tanda kutip, dan tanpa spasi.
Kalau seluruh barisnya ditempel, Justmart menolaknya dengan pesan *"Isi tokennya
saja — tanpa perintah, tanda kutip, atau spasi"*.

Justmart sengaja tidak menebak-nebak bagian mana dari kalimat panjang itu yang
merupakan token. Salah tebak berarti sebuah kredensial tersimpan tanpa pernah
diperiksa, dan baru ketahuan salah jauh di kemudian hari — waktu tunnelnya tidak
mau menyala dan tidak jelas kenapa.

## Setelah disimpan

- **Tunnel baru menyala setelah aplikasi dijalankan ulang.** Token dibaca sekali
  waktu Justmart start, jadi menyimpan token belum menyalakan apa-apa. Selama
  belum di-restart, halaman itu menampilkan pengingatnya.
- **Token tidak pernah ditampilkan utuh lagi.** Yang tampil cuma potongan
  seperti `eyJhIj••••byJ9`, cukup untuk memastikan token yang benar tersimpan.
  Kalau tokennya perlu diganti, tempel yang baru — tidak bisa diedit sebagian.
- **Sekali jalan pertama, Justmart mengunduh `cloudflared.exe`** ke folder yang
  sama dengan `justmart.exe`. Jadi jalan pertama setelah restart butuh internet.
- Mau mematikan lagi: tombol **Hapus token**, lalu jalankan ulang.

## Kalau tokennya ditolak atau tunnelnya tidak menyala

| Yang tampil | Artinya | Yang perlu dilakukan |
|---|---|---|
| "Isi tokennya saja — tanpa perintah, tanda kutip, atau spasi" | Yang ditempel bukan token polos — biasanya seluruh baris `cloudflared.exe service install ...` ikut tersalin | Hapus isinya, tempel ulang **hanya bagian panjang setelah kata `install`** |
| Status: "Sedang memakai token dari config.yaml" | Token lama masih ada di file `config.yaml` dan itu yang sedang dipakai | Tidak apa-apa — token yang baru disimpan menggantikannya setelah restart |
| Status: "Sedang memakai token dari environment variable" | Ada `JUSTMART_CLOUDFLARE_TUNNEL_TOKEN` di komputer itu, dan itu menang | Hapus environment variable-nya, lalu jalankan ulang |
| Status: "Tunnel dimatikan lewat environment variable" | `JUSTMART_CLOUDFLARE_TUNNEL_TOKEN` diisi `off` di komputer itu | Token tetap tersimpan, tapi tidak akan dipakai sampai variable itu dihapus |
| "Tunnel otomatis baru didukung di Windows" | Servernya bukan Windows (Docker/Linux) | Jalankan cloudflared sendiri di server itu; token di halaman ini tidak dipakai |
| Sudah restart, alamatnya tetap tidak terbuka | Token benar tapi tunnel di Cloudflare belum diarahkan ke Justmart | Di Cloudflare, arahkan **Public hostname** tunnel itu ke `http://localhost:8080` (atau port yang dipakai) |

## Kenapa harus restart

Tunnel dijalankan sebagai program pendamping yang hidup selama Justmart hidup —
ikut menyala waktu aplikasi start, dan ikut mati waktu aplikasi ditutup. Kalau
tokennya bisa diganti sambil jalan, kasir yang sedang menutup transaksi bisa
kehilangan koneksi di tengah jalan. Restart sekali jauh lebih aman daripada
sambungan yang putus-nyambung sendiri.

Dan satu hal yang tidak berubah: **kalau tunnel gagal jalan, toko tetap jalan.**
Kasir tetap bisa berjualan lewat jaringan lokal — tunnel yang bermasalah dicatat
di log, bukan dijadikan alasan aplikasi berhenti.
