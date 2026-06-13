JUSTMART PRINT CONNECTOR (installed edition)
============================================

WHAT IS THIS?
-------------
The connector prints Justmart receipts to a printer attached to THIS PC - USB
thermal printers, or any printer installed in Windows. A network printer that
has its own IP address does NOT need the connector (set "printer: enabled: true"
and its address in config.yaml instead).


SETUP
-----
1. Install your printer in Windows (Settings -> Bluetooth & devices -> Printers
   & scanners) and confirm it prints a Windows test page.

2. Turn on connector mode in Justmart:
   - Open   C:\ProgramData\Justmart\config.yaml   in Notepad (as Administrator).
   - In the "connector:" section change   mode: tcp   to   mode: connector .
   - Save, then restart the Justmart server: open Services (services.msc),
     right-click "justmart-server" -> Restart.

3. Start the connector:
   - Use the "Justmart Connector" shortcut (Start menu / Desktop).
   - A window opens and lists your printers. Keep it open while you print.

4. Pick the printer in Justmart:
   - Sign in as owner -> Settings -> Printing -> choose the printer -> Save
     (or leave the connector on "Auto" if there is only one).

Done - the "Print" button on a completed sale now prints to your printer.


START WITH WINDOWS (optional)
-----------------------------
Copy the "Justmart Connector" shortcut into the Startup folder: press Win+R,
type   shell:startup   , press Enter, and paste the shortcut there.


CONFIG FILES
------------
  C:\ProgramData\Justmart\config.yaml            (server: connector.mode)
  C:\ProgramData\Justmart\connector\config.yaml  (connector: server_url)


TROUBLESHOOTING
---------------
- "No print connector is connected" when printing -> the connector window is not
  running; launch "Justmart Connector".
- No printers under Settings -> Printing -> install the printer in Windows, then
  restart the connector.
