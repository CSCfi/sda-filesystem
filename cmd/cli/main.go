package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sda-filesystem/internal/api"
	"sda-filesystem/internal/filesystem"
	"sda-filesystem/internal/logs"
	"sda-filesystem/internal/mountpoint"
	"strings"
	"syscall"

	"golang.org/x/term"
)

var repository, mountPoint, logLevel string
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
	pwd, err := term.ReadPassword(syscall.Stdin)
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

var askForLogin = func(lr loginReader) (string, string, error) {
	fmt.Print("Enter username: ")
	scanner := bufio.NewScanner(lr.getStream())
	var username string
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

var droppedRepository = func(lr loginReader, rep string) bool {
	for {
		fmt.Print("Do you wish to try again? (yes/no) ")
		scanner := bufio.NewScanner(lr.getStream())

		var answer string
		if scanner.Scan() {
			answer = scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			logs.Errorf("Could not read input, dropping repository %s: %w", rep, err)
			break
		}

		if strings.EqualFold(answer, "yes") || strings.EqualFold(answer, "y") {
			return false
		} else if strings.EqualFold(answer, "no") || strings.EqualFold(answer, "n") {
			logs.Info("User chose to drop repository ", rep)
			break
		}
	}

	api.RemoveRepository(rep)
	return true
}

// login asks for a username and a password for repository 'rep'
var login = func(lr loginReader, rep string) error {
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

	fmt.Printf("Log in to %s\n", rep)

	for {
		username, password, err := askForLogin(lr)
		if err != nil {
			return err
		}

		err = api.ValidateLogin(rep, username, password)
		if err == nil {
			return nil
		}

		var re *api.RequestError
		if errors.As(err, &re) && re.StatusCode == 401 {
			logs.Errorf("Incorrect username or password")
			if droppedRepository(lr, rep) {
				return nil
			}
		} else {
			return fmt.Errorf("Failed to log in to %s: %w", rep, err)
		}
	}
}

// loginToAll goes through all enabled repositories and logs into each of them
func loginToAll() {
	for _, rep := range api.GetEnabledRepositories() {
		var err error
		if api.GetLoginMethod(rep) == api.Password {
			err = login(&stdinReader{}, rep)
		} else if api.GetLoginMethod(rep) == api.Token {
			err = api.ValidateLogin(rep)
		} else {
			logs.Warningf("No login function designated for %s", rep)
			continue
		}
		if err != nil {
			api.RemoveRepository(rep)
			logs.Errorf("Dropping repository %s: %w", rep, err)
		}
	}
}

func processFlags() error {
	repOptions := api.GetAllPossibleRepositories()

	found := false
	for _, op := range repOptions {
		if strings.EqualFold(repository, op) || strings.EqualFold(repository, "all") {
			found = true
			api.AddRepository(op)
		}
	}

	if !found {
		logs.Warningf("Flag -enable=%s not supported, switching to default -enable=all", repository)
		for _, op := range repOptions {
			api.AddRepository(op)
		}
	}

	if err := mountpoint.CheckMountPoint(mountPoint); err != nil {
		return err
	}

	api.SetRequestTimeout(requestTimeout)
	logs.SetLevel(logLevel)
	return nil
}

func init() {
	repOptions := api.GetAllPossibleRepositories()
	defaultMount, err := mountpoint.DefaultMountPoint()

	if err != nil {
		logs.Warning(err)
	}

	flag.StringVar(&repository, "enable", "all",
		fmt.Sprintf("Choose which repositories you wish include in the filesystem. Possible values: {%s,all}",
			strings.Join(repOptions, ",")))
	flag.StringVar(&mountPoint, "mount", defaultMount, "Path to filesystem mount point")
	flag.StringVar(&logLevel, "loglevel", "info", "Logging level. Possible values: {debug,info,warning,error}")
	flag.IntVar(&requestTimeout, "http_timeout", 20, "Number of seconds to wait before timing out an HTTP request")
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
	flag.Parse()
	err := processFlags()
	if err != nil {
		logs.Fatal(err)
	}
	err = api.GetEnvs()
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

	loginToAll()

	if len(api.GetEnabledRepositories()) == 0 {
		logs.Fatal("No repositories found. Filesystem not created")
	}

	done := shutdown()
	fs := filesystem.InitializeFileSystem()
	fs.PopulateFilesystem()
	filesystem.MountFilesystem(fs, mountPoint)
	<-done
}
