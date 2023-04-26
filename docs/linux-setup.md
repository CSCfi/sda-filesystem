# Setting up linux GUI

Build:
(if upx is not install remove the flag)
```
wails build -upx -trimpath -clean -s
```

or download the release:
```
sudo mkdir -p /etc/sda-fuse
cd /etc/sda-fuse/
export version=v2.1.0
wget "https://github.com/CSCfi/sda-filesystem/releases/download/${version}/go-fuse-gui-golang1.20-linux-amd64.zip"
```

Install the software:
```
sudo unzip -qq go-fuse-gui-golang1.20-linux-amd64.zip
sudo mv /etc/sda-fuse/data-gateway /etc/sda-fuse/sda-fuse
sudo chmod 755 /etc/sda-fuse/data-gateway
sudo ln -s /etc/sda-fuse/sda-fuse /usr/bin/sda-fuse
sudo wget https://raw.githubusercontent.com/CSCfi/sda-filesystem/refactor/wails-gui/build/appicon.png --directory-prefix=/etc/sda-fuse
sudo cat > /etc/skel/Desktop/filesystem.desktop << EOF
[Desktop Entry]
Version=2.1.0
Type=Application
Terminal=false
Exec=/usr/bin/sda-fuse
Name=Data Gateway
Comment=
Icon=/etc/sda-fuse/appicon.png
Comment[en_US.utf8]=CSC SDA-Filesystem/Data Gateway
Name[en_US]=Data Gateway
EOF
```

For the desktop icon to be functional, each user needs to enable it using:
```
chmod +x "${HOME}/Desktop/filesystem.desktop"
gio set "${HOME}/Desktop/filesystem.desktop" "metadata::trusted" yes
```
