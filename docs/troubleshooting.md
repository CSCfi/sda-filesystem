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

### MacOS
```
umount <path>
```

## File was updated in archive, but program displays old file
### GUI
Click on the `Update` button to clear cached files.
### CLI
Write `update` in the terminal where the process is running.
```
update
```

## Environment variables don't seem to be correct even though I am sure I am running the correct commands

Make sure you have not exported any of the environment variables in you command line. They will override any envs given to docker compose.
