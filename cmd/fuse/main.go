package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/filesystem"
	"sda-filesystem/internal/logs"
	"sda-filesystem/internal/mountpoint"

	"golang.org/x/term"
)

var mount, logLevel string
var requestTimeout int

// var sdsubmitOnly bool

type loginReader interface {
	readPassword() (string, error)
	getState() error
	restoreState() error
}

// stdinReader reads password from stdin (implements loginReader)
type stdinReader struct {
	originalState *term.State
}

func (r *stdinReader) readPassword() (string, error) {
	pwd, err := term.ReadPassword(int(syscall.Stdin))

	return string(pwd), err
}

func (r *stdinReader) getState() (err error) {
	r.originalState, err = term.GetState(int(syscall.Stdin))

	return
}

func (r *stdinReader) restoreState() error {
	return term.Restore(int(syscall.Stdin), r.originalState)
}

type credentialsError struct {
}

func (e *credentialsError) Error() string {
	return "Incorrect password"
}

// userInput reads user's input from io.Reader and sends it into a channel
func userInput(r io.Reader, ch chan<- []string) {
	scanner := bufio.NewScanner(r)
	var answer string

	for {
		if scanner.Scan() {
			answer = scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			logs.Errorf("Could not read input: %w", err)
		}
		ch <- strings.Fields(answer)
	}
}

func applyCommand(ch <-chan []string) {
	for {
		input := <-ch
		if len(input) == 0 {
			continue
		}
		switch strings.ToLower(input[0]) {
		case "update":
			if filesystem.FilesOpen() {
				logs.Errorf("You have files in use which prevents updating Data Gateway")
			} else {
				filesystem.RefreshFilesystem()
			}
		case "clear":
			if len(input) > 1 {
				path := filepath.Clean(input[1])
				if err := filesystem.ClearPath(path); err != nil {
					logs.Error(err)
				}
			} else {
				logs.Errorf("Cannot clear cache without path")
			}
		}
	}
}

func authenticate(password string) error {
	err := api.Authenticate(password)

	var re *api.RequestError
	if errors.As(err, &re) && re.StatusCode == 401 {
		return &credentialsError{}
	}

	return err
}

var askForPassword = func(lr loginReader) (string, error) {
	fmt.Print("Enter password: ")
	password, err := lr.readPassword()
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("could not read password: %w", err)
	}

	return password, nil
}

var login = func(lr loginReader) error {
	password, ok := os.LookupEnv("CSC_PASSWORD")
	if ok {
		logs.Info("Using password from environment variable CSC_PASSWORD")

		return authenticate(password)
	}

	// Get the state of the terminal before running the password prompt
	err := lr.getState()
	if err != nil {
		return fmt.Errorf("failed to get terminal state: %w", err)
	}

	// check for ctrl+c signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	defer func() { signal.Stop(signalChan) }()
	go func() {
		<-signalChan
		fmt.Println("")
		if err = lr.restoreState(); err != nil {
			logs.Warningf("Could not restore terminal to original state: %w", err)
		}
		os.Exit(1)
	}()

	for {
		password, err := askForPassword(lr)
		if err != nil {
			return err
		}

		err = authenticate(password)

		var e *credentialsError
		if errors.As(err, &e) {
			logs.Error(err)

			continue
		}

		return err
	}
}

func processFlags() error {
	if mount == "" {
		defaultMount, err := mountpoint.DefaultMountPoint()
		if err != nil {
			return err
		}
		mount = defaultMount
	} else if err := mountpoint.CheckMountPoint(mount); err != nil {
		return err
	}

	mount = filepath.Clean(mount)
	api.SetRequestTimeout(requestTimeout)
	logs.SetLevel(logLevel)

	return nil
}

func init() {
	flag.StringVar(&mount, "mount", "", "Path to Data Gateway mount point")
	flag.StringVar(&logLevel, "loglevel", "info", "Logging level. Possible values: {trace,debug,info,warning,error}")
	// flag.BoolVar(&sdsubmitOnly, "sdapply", false, "Connect only to SD Apply")
	flag.IntVar(&requestTimeout, "http_timeout", 20, "Number of seconds to wait before timing out an HTTP request")
}

func main() {
	flag.Parse()
	if err := processFlags(); err != nil {
		logs.Fatal(err)
	}
	if err := api.Setup(); err != nil {
		logs.Fatal(err)
	}

	access, err := api.GetProfile()
	if err != nil {
		logs.Fatal(err)
	}

	// if !sdsubmitOnly {
	if !api.SDConnectEnabled() {
		// logs.Warningf("You do not have SD Connect enabled")
		logs.Fatal("You do not have SD Connect enabled")
	} else if !access {
		logs.Info("Passwordless session not possible")
		if err := login(&stdinReader{}); err != nil {
			logs.Fatal(err)
		}
	}
	// }

	var wait = make(chan any)
	var cmd = make(chan []string)
	go func() {
		<-wait // Wait for fuse to be ready
		go mountpoint.WaitForUpdateSignal(cmd)
		go userInput(os.Stdin, cmd)
		go applyCommand(cmd)
	}()

	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt)
	go func() {
		<-s
		logs.Info("Shutting down Data Gateway")
		filesystem.UnmountFilesystem()
	}()

	ret := filesystem.MountFilesystem(mount, nil, wait)

	os.Exit(ret)
}
