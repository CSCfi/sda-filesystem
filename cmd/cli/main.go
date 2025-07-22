package main

import (
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"sda-filesystem/certs"
	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"

	"golang.org/x/term"
)

var logLevel string
var requestTimeout int

var handlers = map[string]handlerFuncs{}

type handlerFuncs struct {
	setup   func([]string) (int, error)
	execute func() (int, error)
}

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

		return api.Authenticate(password)
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

		err = api.Authenticate(password)

		var e *api.CredentialsError
		if errors.As(err, &e) {
			logs.Error(err)

			continue
		}

		return err
	}
}

func init() {
	flag.Usage = func() {
		fmt.Printf("Usage: %s [arguments] [subcommand] [subcommand arguments]\n", os.Args[0])

		flag.PrintDefaults()

		fmt.Printf("\nRun %s [subcommand] -help to learn more about subcommands\n", os.Args[0])
		fmt.Println("\nAvailable subcommands:")
		fmt.Println("import: Setup a filesystem that has access to files in SD Connect and SD Apply")
		fmt.Println("export: Upload files and folders from VM to SD Connect")
		fmt.Println()
	}

	flag.StringVar(&logLevel, "loglevel", "info", "Logging level. Possible values: {trace,debug,info,warning,error}")
	flag.IntVar(&requestTimeout, "http_timeout", 20, "Number of seconds to wait before timing out an HTTP request")
}

func main() {
	flag.Parse()

	subcommandOptions := slices.Collect(maps.Keys(handlers))
	if flag.NArg() < 1 || !slices.Contains(subcommandOptions, flag.Args()[0]) {
		flag.Usage()
		os.Exit(2)
	}
	subcommand := flag.Args()[0]

	api.SetRequestTimeout(requestTimeout)
	logs.SetLevel(logLevel)

	if err := api.Setup(certs.Files); err != nil {
		logs.Fatal(err)
	}

	access, err := api.GetProfile()
	if err != nil {
		logs.Fatal(err)
	}

	code, err := handlers[subcommand].setup(flag.Args()[1:])
	if err != nil {
		logs.Fatal(err)
	}
	if code != 0 {
		os.Exit(code)
	}

	if !access {
		logs.Info("Passwordless session not possible")
		if err := login(&stdinReader{}); err != nil {
			logs.Fatal(err)
		}
	}

	code, err = handlers[subcommand].execute()
	if err != nil {
		logs.Fatal(err)
	}
	os.Exit(code)
}
