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

var mount, project, logLevel string
var requestTimeout int

type loginReader interface {
	readPassword() (string, error)
	getStream() io.Reader
	getState() error
	restoreState() error
}

// stdinReader reads username and password from stdin (implements loginReader)
type stdinReader struct {
	originalState *term.State
}

func (r *stdinReader) readPassword() (string, error) {
	pwd, err := term.ReadPassword(int(syscall.Stdin))

	return string(pwd), err
}

func (r *stdinReader) getStream() io.Reader {
	return os.Stdin
}

func (r *stdinReader) getState() (err error) {
	r.originalState, err = term.GetState(int(syscall.Stdin))

	return
}

func (r *stdinReader) restoreState() error {
	return term.Restore(int(syscall.Stdin), r.originalState)
}

func userChooseUpdate(r io.Reader) {
	scanner := bufio.NewScanner(r)
	var answer string

	for !strings.EqualFold(answer, "update") {
		if scanner.Scan() {
			answer = scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			logs.Errorf("Could not read input: %w", err)
		}
	}
}

var askForLogin = func(lr loginReader) (string, string, error) {

	username, password, exist := checkEnvVars()
	if exist {
		logs.Info("Using username and password from environment variables CSC_USERNAME and CSC_PASSWORD")
	} else {
		fmt.Printf("Log in with your CSC credentials\n")
		fmt.Print("Enter username: ")
		scanner := bufio.NewScanner(lr.getStream())

		if scanner.Scan() {
			username = scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			return "", "", fmt.Errorf("Could not read username: %w", err)
		}

		fmt.Print("Enter password: ")
		password, err := lr.readPassword()
		fmt.Println()
		if err != nil {
			return "", "", fmt.Errorf("Could not read password: %w", err)
		}

		return username, password, nil
	}

	return username, password, nil
}

func checkEnvVars() (string, string, bool) {
	username, usernameEnv := os.LookupEnv("CSC_USERNAME")
	password, passwordEnv := os.LookupEnv("CSC_PASSWORD")

	if usernameEnv && passwordEnv {
		return username, password, true
	}

	return "", "", false
}

// login asks for CSC username and password
var login = func(lr loginReader) error {
	// Get the state of the terminal before running the password prompt
	err := lr.getState()
	if err != nil {
		return fmt.Errorf("Failed to get terminal state: %w", err)
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
		username, password, err := askForLogin(lr)
		if err != nil {
			return err
		}

		success, err := api.ValidateLogin(username, password, project)
		if err == nil {
			return nil
		}
		if success {
			logs.Error(err) // If SD Submit authorization fails

			return nil
		}

		var re *api.RequestError
		if errors.As(err, &re) && re.StatusCode == 401 {
			logs.Errorf("Incorrect username or password")
		} else {
			return err
		}
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
	flag.StringVar(&project, "project", "", "The SD Connect project when using Hopsworks")
	flag.StringVar(&logLevel, "loglevel", "info", "Logging level. Possible values: {debug,info,warning,error}")
	flag.IntVar(&requestTimeout, "http_timeout", 20, "Number of seconds to wait before timing out an HTTP request")
}

func shutdown() <-chan bool {
	done := make(chan bool)
	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt)
	go func() {
		<-s
		logs.Info("Shutting down Data Gateway")
		done <- true
	}()

	return done
}

func main() {
	err := api.GetCommonEnvs()
	if err != nil {
		logs.Fatal(err)
	}
	err = api.InitializeCache()
	if err != nil {
		logs.Fatal(err)
	}
	err = api.InitializeClient()
	if err != nil {
		logs.Fatal(err)
	}

	flag.Parse()
	err = processFlags()
	if err != nil {
		logs.Fatal(err)
	}

	for _, rep := range api.GetAllRepositories() {
		if err := api.GetEnvs(rep); err != nil {
			logs.Fatal(err)
		}
	}

	err = login(&stdinReader{})
	if err != nil {
		logs.Fatal(err)
	}

	done := shutdown()
	fs := filesystem.InitializeFileSystem(nil)
	fs.PopulateFilesystem(nil)

	go func() {
		for {
			userChooseUpdate(os.Stdin)
			if fs.FilesOpen(mount) {
				logs.Warningf("You have files in use and thus updating is not possible")

				continue
			}
			newFs := filesystem.InitializeFileSystem(nil)
			newFs.PopulateFilesystem(nil)
			fs.RefreshFilesystem(newFs)
		}
	}()

	filesystem.MountFilesystem(fs, mount)
	<-done
}
