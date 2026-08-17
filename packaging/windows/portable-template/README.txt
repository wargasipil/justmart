JUSTMART - Portable edition (SQLite)
====================================

This is a self-contained copy of Justmart. It needs NO installation, NO
PostgreSQL, and NO internet connection. Everything (the app, the web UI, and
your data) lives inside this one folder.


HOW TO RUN
----------
1. Double-click  "Start Justmart.bat"
   (or run justmart.exe directly).
2. A black window opens and stays open while Justmart is running.
   Keep it open. Closing it stops Justmart.
3. Your web browser opens automatically at:
       http://localhost:__PORT__
   If it does not, open that address yourself.

First launch takes a moment while the database is created.

__LICENSE_README__
LOG IN
------
   Email:    __OWNER_EMAIL__
   Password: __OWNER_PASSWORD__

Change the password: open config.yaml in a text editor, edit
"owner_password:", save, and restart Justmart. (Changing it inside the app is
reset on the next start - config.yaml is the source of truth for this account.)


WHERE IS MY DATA?
-----------------
   justmart.db          <- your whole database (one file, next to justmart.exe)
   justmart.db-wal      <- temporary write-ahead log (leave it alone)
   justmart.db-shm      <- temporary shared-memory file (leave it alone)
   backups\             <- snapshots created from Settings -> Backups

To MOVE Justmart to another PC or drive: stop it, then copy this entire folder.
To BACK UP: stop Justmart and copy justmart.db somewhere safe, OR use the in-app
OWNER -> Settings -> Backups -> Create (writes a snapshot into backups\).
To RESTORE: stop Justmart, replace justmart.db with your backup copy (delete any
leftover justmart.db-wal / justmart.db-shm), then start again.


CHANGE THE PORT
---------------
Edit "port:" in config.yaml (default __PORT__), save, restart. Then browse to
http://localhost:<your-port>.


PRINT RECEIPTS TO A USB / LOCAL PRINTER
---------------------------------------
For a USB printer (or any printer installed in Windows) attached to THIS PC, the
easiest way is "usb" mode - no extra program to run:
  1. Open config.yaml.
  2. Under "connector:", set   mode: usb
  3. (Optional) set  printer_name:  to the printer's exact Windows name; leave
     it blank to use this PC's default printer.
  4. Save and restart Justmart. Receipts print straight to that printer.

A network printer with its own IP does not need any of this - set
"printer: enabled: true" + its address in config.yaml instead.

If the printer is on a DIFFERENT PC, use the bundled "connector": open the
   connector\  folder and follow  connector\CONNECTOR-SETUP.txt.


USE FROM OTHER DEVICES ON THE NETWORK
-------------------------------------
By default Justmart only answers on this PC (host: 127.0.0.1). To let other
devices on your LAN connect:
  1. In config.yaml set  host: 0.0.0.0
  2. Allow justmart.exe through Windows Firewall when prompted (or add a rule).
  3. Other devices browse to  http://<this-PC-IP>:__PORT__


NOTES
-----
- Windows SmartScreen may warn the first time because the .exe is not code-signed.
  Choose "More info" -> "Run anyway" if you trust this copy.
- This portable edition uses SQLite, which is ideal for a single shop / single PC.
  For multi-user or networked deployments, use the PostgreSQL flavor instead.
