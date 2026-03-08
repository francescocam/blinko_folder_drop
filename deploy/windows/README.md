# Windows deployment

1. Build binary:
```powershell
go build -o blinko-folder-drop.exe ./cmd/blinko-folder-drop
```

2. Place files:
- Binary: `C:\Program Files\BlinkoFolderDrop\blinko-folder-drop.exe`
- Config: `C:\ProgramData\BlinkoFolderDrop\config.yaml`

3. Create service:
```powershell
sc.exe create blinko-folder-drop binPath= '"C:\Program Files\BlinkoFolderDrop\blinko-folder-drop.exe" run --config "C:\ProgramData\BlinkoFolderDrop\config.yaml"' start= auto
```

4. Configure recovery:
```powershell
sc.exe failure blinko-folder-drop reset= 86400 actions= restart/5000/restart/5000/restart/5000
```

5. Start service:
```powershell
sc.exe start blinko-folder-drop
```

6. Optional service account:
```powershell
sc.exe config blinko-folder-drop obj= ".\\blinko-drop" password= "<password>"
```
