package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"sda-filesystem/internal/api"
	"sda-filesystem/internal/filesystem"
	"sda-filesystem/internal/logs"
	"syscall"

	"github.com/billziss-gh/cgofuse/fuse"
	"golang.org/x/sys/unix"
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

func login() {
	// Get the state of the terminal before running the password prompt
	originalTerminalState, err := term.GetState(int(syscall.Stdin))
	if err != nil {
		logs.Fatalf("Failed to get terminal state: %s", err.Error())
	}

	// check for ctrl+c signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		<-signalChan
		term.Restore(int(syscall.Stdin), originalTerminalState)
		os.Exit(1)
	}()

	for {
		username, password := askForLogin()
		api.CreateToken(username, password)
		err = api.GetUToken()

		if err == nil {
			break
		}

		var re *api.RequestError
		if errors.As(err, &re) && re.StatusCode == 401 {
			logs.Errorf("Incorrect username or password")
			continue
		}

		logs.Fatal(err)
	}

	// stop watching signals
	signal.Stop(signalChan)
}

func init() {
	var logLevel string
	var timeout int
	flag.StringVar(&mount, "mount", mountPoint(), "Path to FUSE mount point")
	flag.StringVar(&logLevel, "loglevel", "info", "Logging level. Possible value: {debug,info,error}")
	flag.IntVar(&timeout, "http_timeout", 20, "Number of seconds to wait before timing out an HTTP request")
	profiling := flag.Bool("profiling", false, "Code profiling on")
	flag.Parse()

	api.SetRequestTimeout(timeout)
	logs.SetLevel(logLevel)

	if *profiling {
		go func() {
			http.ListenAndServe(":8080", nil)
		}()
	}

	// Verify mount point directory
	if dir, err := os.Stat(mount); os.IsNotExist(err) {
		// In other OSs except Windows, the mount point must exist and be empty
		if runtime.GOOS != "windows" {
			logs.Debugf("Mount point %s does not exist, so it will be created", mount)
			if err = os.Mkdir(mount, 0755); err != nil {
				logs.Fatalf("Could not create directory %s", mount)
			}
		}
	} else {
		if !dir.IsDir() {
			logs.Fatalf("%s is not a directory", mount)
		}

		// Mount directory must not already exist in Windows
		if runtime.GOOS == "windows" { // ?
			logs.Fatalf("Mount point %s already exists, remove the directory or use another mount point", mount)
		}

		if unix.Access(mount, unix.W_OK) != nil { // What about windows?
			logs.Fatal("You do not have permission to write to folder ", mount)
		}

		// Check that the mount point is empty if it already exists
		dir, err := os.Open(mount)
		if err != nil {
			logs.Fatalf("Could not open mount point %s", mount)
		}
		defer dir.Close()

		// Verify dir is empty
		if _, err = dir.Readdir(1); err != io.EOF {
			if err != nil {
				logs.Fatalf("Error occurred when reading from directory %s: %s", mount, err.Error())
			}
			logs.Fatalf("Mount point %s must be empty", mount)
		}
	}

	logs.Debugf("Filesystem will be mounted at %s", mount)
}

func shutdown() <-chan bool {
	done := make(chan bool)
	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt)
	go func() {
		<-s
		logs.Info("Shutting down SDA Filesystem")
		done <- true
	}()
	return done
}

func main() {
	err := api.GetEnvs()
	if err != nil {
		logs.Fatal(err)
	}

	err = api.InitializeClient()
	if err != nil {
		logs.Fatal(err)
	}

	login()
	api.SetLoggedIn()

	done := shutdown()
	api.FetchTokens()

	connectfs := filesystem.CreateFileSystem()
	host := fuse.NewFileSystemHost(connectfs)
	options := []string{}
	if runtime.GOOS == "darwin" {
		options = append(options, "-o", "defer_permissions")
		options = append(options, "-o", "volname="+path.Base(mount))
		options = append(options, "-o", "attr_timeout=0")
		options = append(options, "-o", "iosize=262144") // Value not optimized
	} else if runtime.GOOS == "linux" {
		options = append(options, "-o", "attr_timeout=0") // This causes the fuse to call getattr between open and read
		options = append(options, "-o", "auto_unmount")
	} // Still needs windows options

	host.Mount(mount, options)

	<-done
}
