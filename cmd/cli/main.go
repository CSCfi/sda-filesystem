package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"sd-connect-fuse/internal/api"
	"sd-connect-fuse/internal/filesystem"
	"sd-connect-fuse/internal/logs"
	"syscall"

	"github.com/billziss-gh/cgofuse/fuse"
	"golang.org/x/term"
)

var mount string

// mountPoint constructs a path to the user's home directory for mounting FUSE
func mountPoint() string {
	home, err := os.UserHomeDir()
	if err != nil {
		logs.Fatal("Could not find user home directory", err)
	}
	p := filepath.FromSlash(filepath.ToSlash(home) + "/Projects")
	return p
}

func askForLogin() (string, string) {
	fmt.Print("Enter username: ")
	scanner := bufio.NewScanner(os.Stdin)
	var username string
	if scanner.Scan() {
		username = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		logs.Fatal(err)
	}

	fmt.Print("Enter password: ")
	password, err := term.ReadPassword(syscall.Stdin)
	fmt.Println()
	if err != nil {
		logs.Fatal(err)
	}

	return username, string(password)
}

func GetUSTokens() {
	err := api.GetUToken()
	if err != nil {
		logs.Fatal(err)
	}

	projects, err := api.GetProjects(false)
	if err != nil {
		logs.Fatal(err)
	}
	if len(projects) == 0 {
		logs.Fatal("No project permissions found")
	}

	err = api.GetSTokens(projects)
	if err != nil {
		logs.Fatal(err)
	}
}

func init() {
	var logLevel string
	var timeout int
	flag.StringVar(&mount, "mount", mountPoint(), "Path to FUSE mount point")
	flag.StringVar(&logLevel, "loglevel", "info", "Logging level. Possible value: {debug,info,error}")
	flag.IntVar(&timeout, "http_timeout", 10, "Number of seconds to wait before timing out an HTTP request")
	flag.Parse()

	api.SetRequestTimeout(timeout)
	logs.SetLevel(logLevel)

	// Verify mount point directory
	if _, err := os.Stat(mount); os.IsNotExist(err) {
		// In other OSs except Windows, the mount point must exist and be empty
		if runtime.GOOS != "windows" {
			logs.Debugf("Mount point %s does not exist, so it will be created", mount)
			if err = os.Mkdir(mount, 0777); err != nil {
				logs.Fatalf("Could not create directory %s", mount)
			}
		}
	} else {
		// Mount directory must not already exist in Windows
		if runtime.GOOS == "windows" {
			logs.Fatalf("Mount point %s already exists, remove the directory or use another mount point", mount)
		}
		// Check that the mount point is empty if it already exists
		dir, err := os.Open(mount)
		if err != nil {
			logs.Fatalf("Could not open mount point %s", mount)
		}
		defer dir.Close()
		// Verify dir is empty
		if _, err = dir.Readdir(1); err != io.EOF {
			logs.Fatalf("Mount point %s must be empty", mount)
		}
	}

	logs.Debugf("Filesystem will be mounted at %s", mount)
}

func shutdown() <-chan bool {
	done := make(chan bool)
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-s
		logs.Info("Shutting down SD-Connect Filesystem")
		done <- true
	}()
	return done
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	done := shutdown()

	err := api.GetEnvs()
	if err != nil {
		logs.Fatal(err)
	}

	api.CreateToken(askForLogin())
	//fmt.Println(runtime.NumGoroutine(), runtime.GOMAXPROCS(-1))
	err = api.InitializeClient()
	if err != nil {
		logs.Fatal(err)
	}
	//fmt.Println(runtime.NumGoroutine())
	GetUSTokens()
	connectfs := filesystem.CreateFileSystem()
	//fmt.Println(runtime.NumGoroutine())
	host := fuse.NewFileSystemHost(connectfs)
	options := []string{}
	if runtime.GOOS == "darwin" {
		options = append(options, "-o", "defer_permissions")
		options = append(options, "-o", "volname="+path.Base(mount))
	}
	host.Mount(mount, options)

	<-done
}
