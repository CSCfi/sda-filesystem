# Troubleshooting

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
