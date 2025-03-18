# Setting up linux GUI

Build:
(if upx is not install remove the flag)
```bash
wails build -upx -trimpath -clean -s
```

or download the release:
```bash
sudo mkdir -p /etc/sda-fuse
cd /etc/sda-fuse/
export version=2025.4.0
wget "https://github.com/CSCfi/sda-filesystem/releases/download/${version}/go-fuse-gui-linux-amd64.zip"
```

Install the software:
```bash
sudo unzip -qq go-fuse-gui-linux-amd64.zip
sudo mv /etc/sda-fuse/data-gateway /etc/sda-fuse/data-gateway
sudo chmod 755 /etc/sda-fuse/data-gateway
sudo ln -s /etc/sda-fuse/data-gateway  /usr/bin/data-gateway
sudo wget https://raw.githubusercontent.com/CSCfi/sda-filesystem/refactor/wails-gui/build/appicon.png --directory-prefix=/etc/sda-fuse
sudo cat > /etc/skel/Desktop/filesystem.desktop << EOF
[Desktop Entry]
Type=Application
Terminal=false
Exec=/usr/bin/data-gateway
Name=Data Gateway
Comment=
Icon=/etc/sda-fuse/appicon.png
Comment[en_US.utf8]=CSC SDA-Filesystem/Data Gateway
Name[en_US]=Data Gateway
EOF
```

For the desktop icon to be functional, each user needs to enable it using:
```bash
chmod +x "${HOME}/Desktop/filesystem.desktop"
gio set "${HOME}/Desktop/filesystem.desktop" "metadata::trusted" yes
```
