In case support for Windows is needed in the future, this folder contains Windows-specific code that was removed from the repository. Below are snippets that were moved from various functions. This folder also contains files `mountpoint_windows.txt` and `mountpoint_windows_test.txt` that were under the `mountpoint` package.

## Snippets

### OpenFuse()

```go
case "windows":
	cmd = exec.Command("cmd", "/C", "start", userPath)
```

### TestMain() in mountpoint tests

```go
if runtime.GOOS == "windows" {
    homeEnv = "USERPROFILE"
}
```

### SaveLogs()

```go
if runtime.GOOS == "windows" {
	newline = "\r\n"
}
```

### IsValidOpen()

```go
case "windows":
	filter := fmt.Sprintf("PID eq %d", pid)
	task := exec.Command("tasklist", "/FI", filter, "/fo", "table", "/nh")
	if res, err := task.Output(); err == nil {
		parts := strings.Fields(string(res))
		if parts[0] == "explorer.exe" {
			logs.Debug("Explorer trying to preview files")

			return false
		}
	}
```

### FilesOpen()

```go
case "windows":
	volume, _ := os.Readlink(fi.mount)
	output, err := exec.Command("handle.exe", "-a", "-nobanner", volume).Output()
	if err != nil {
		logs.Errorf("Update halted, could not determine if files are open: %w", err)

		return true
	}

	return strings.Contains(string(output), volume)
```
