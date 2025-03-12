# Troubleshooting

## Cannot open GUI application on MacOS

If clicking the icon produces a message which says that application cannot be opened or the program is killed in command line, decompressing the binary might help.

Install upx with Homebrew
```
brew install upx
```

Decompress the binary
```
upx -d data-gateway.app/Contents/MacOS/data-gateway
```

## Directory is not unmounted after program crash
### Linux
```
fusermount -u <path>
```

## File was updated in archive, but program displays old file
### GUI
Click on the `Update` button to clear cached files.
### CLI
Write `update` in the terminal where the process is running.
```
update
```
