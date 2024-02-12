# Usage of Data Gateway

Data Gateway intended use is mainly on Linux, thus the deployment guide is customised for that.
For other operating systems the instructions below need to be adapted accordingly.

## Prerequisites

For linux, fuse package is required e.g. Ubuntu 22.04: `sudo apt-get install -qq -y fuse`.
Set the download links from the release, for each of the variables `gui_client`, `cli_client`, `airlock_client`.

## Installation

### Data Gateway GUI

or download the release:
```bash
sudo mkdir -p /etc/sda-fuse
cd /etc/sda-fuse/
wget "${gui_client}"
```

Install the software:
```bash
sudo mv "/etc/sda-fuse/${gui_client}" /etc/sda-fuse/data-gateway 
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

### Airlock and FUSE CLI

```bash
echo "Install SDA Filesystem CLI"
sudo mkdir -p /opt/fuse-deploy
cd /opt/fuse-deploy
sudo wget "${cli_client}" > /dev/null 2>&1

sudo mv "/opt/fuse-deploy/${cli_client}" /usr/bin/go-fuse
sudo chmod 755 /usr/bin/go-fuse
cd /opt/
sudo rm -rf /opt/fuse-deploy


echo "Setup Airlock client"

sudo mkdir -p /opt/airlock-deploy
cd /opt/airlock-deploy
sudo wget "${airlock_client}" > /dev/null 2>&1

sudo unzip -qq go-airlock-cli-golang1.20-linux-amd64.zip

sudo mv "/opt/airlock-deploy/${airlock_client}" /usr/bin/airlock-client
sudo chmod 755 /usr/bin/airlock-client
cd /opt/
sudo rm -rf /opt/airlock-deploy
```