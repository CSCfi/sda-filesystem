# Setting up linux GUI

Build:
```
qtdeploy build desktop cmd/qt/main.go
tar -czf deploy.tar.gz -C cmd/qt/deploy linux
```

or download the release:
```
sudo mkdir -p /etc/sda-fuse
cd /etc/sda-fuse/
export version=v1.1.0
wget "https://github.com/CSCfi/sda-filesystem/releases/download/${version}/go-fuse-gui-golang1.17-linux-amd64.zip"
```

Install the software:
```
sudo unzip -qq go-fuse-gui-golang1.17-linux-amd64.zip
sudo chmod 755 /etc/sda-fuse/sda-fuse
sudo ln -s /etc/sda-fuse/sda-fuse /usr/bin/sda-fuse
sudo mv cmd/qt/icon.svg /etc/sda-fuse/icon.svg
sudo mv cmd/qt/filesystem.desktop /etc/skel/Desktop/filesystem.desktop
```

For the desktop icon to be functional, each user needs to enable it using:
```
chmod +x "${HOME}/Desktop/filesystem.desktop"
gio set "${HOME}/Desktop/filesystem.desktop" "metadata::trusted" yes
```
