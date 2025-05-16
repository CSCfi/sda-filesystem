package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"
	"sda-filesystem/internal/mountpoint"

	"github.com/sirupsen/logrus"
)

var errExpected = errors.New("expected error for test")

// testReader implements loginReader and contains password
type testReader struct {
	pwd string
	err error
}

func (r testReader) readPassword() (string, error) {
	return r.pwd, r.err
}

func (r testReader) getState() error {
	return r.err
}

func (r testReader) restoreState() error {
	return nil
}

// testStream is an io.Reader
type testStream struct {
	data []string
	done bool
	idx  int
	err  error
}

func (s *testStream) Read(p []byte) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	if s.done || s.idx == len(s.data) {
		s.done = false

		return 0, io.EOF
	}
	content := []byte(s.data[s.idx])
	copy(p, content)
	s.done = true
	s.idx++

	return len(content), nil
}

func TestMain(m *testing.M) {
	logs.SetSignal(func(l string, s []string) {})
	os.Exit(m.Run())
}

func TestUserInput(t *testing.T) {
	finished := false
	expectedOutput := [][]string{{"continue"}, {"hello", "sunshine"}, {"update"}}
	reader := strings.NewReader("continue\nhello sunshine\nupdate")

	var ch = make(chan []string)
	go func() {
		userInput(reader, ch)
	}()
	go func() {
		for i := range expectedOutput {
			nextLine := <-ch
			if !reflect.DeepEqual(nextLine, expectedOutput[i]) {
				return
			}
		}
		finished = true
	}()

	time.Sleep(10 * time.Millisecond)

	if !finished {
		t.Fatal("Function did not read input correctly.")
	}
}

func TestUserInput_Error(t *testing.T) {
	buf := &testStream{err: errExpected}

	var level string
	var strs []string
	logs.SetSignal(func(ll string, s []string) {
		level, strs = ll, s
	})
	defer logs.SetSignal(func(l string, s []string) {})

	var ch = make(chan []string)
	go func() {
		userInput(buf, ch)
	}()

	time.Sleep(10 * time.Millisecond)

	err := []string{"Could not read input", errExpected.Error()}
	if level != logrus.ErrorLevel.String() {
		t.Fatal("Function did not log an error")
	}
	if !reflect.DeepEqual(strs, err) {
		t.Fatalf("Logged output incorrect\nExpected: %q\nReceived: %q", err, strs)
	}
}

func TestAskForPassword(t *testing.T) {
	var tests = []struct {
		testname, password string
		readerError        error
		errorText          string
	}{
		{
			"OK", "567ghk789", nil, "",
		},
		{
			"FAIL_READER", "567ghk789", errExpected,
			"could not read password: " + errExpected.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			// Ignore prints to stdout
			null, _ := os.Open(os.DevNull)
			sout := os.Stdout
			os.Stdout = null

			r := testReader{tt.password, tt.readerError}
			password, err := askForPassword(r)

			os.Stdout = sout
			null.Close()

			switch {
			case tt.testname != "OK":
				if err == nil {
					t.Error("Function should have returned error")
				} else if err.Error() != tt.errorText {
					t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errorText, err.Error())
				}
			case err != nil:
				t.Errorf("Function returned error: %s", err.Error())
			case password != tt.password:
				t.Errorf("Password incorrect. Expected=%s, received=%s", tt.password, password)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	var count int
	var tests = []struct {
		testname           string
		readerError        error
		errorText          string
		password           string // For CSC_PASSWORD env
		mockAskForPassword func(loginReader) (string, error)
		mockAuthenticate   func(string) error
	}{
		{
			"OK", nil, "", "",
			func(_ loginReader) (string, error) {
				if count > 0 {
					return "", fmt.Errorf("function did not approve login during first loop")
				}
				count++

				return "345fgj78", nil
			},
			func(password string) error {
				if password != "345fgj78" {
					return fmt.Errorf("incorrect password. Expected=345fgj78, received=%s", password)
				}

				return nil
			},
		},
		{
			"OK_EXISTS", nil, "", "89bf5cifu6vo",
			func(_ loginReader) (string, error) {
				return "", fmt.Errorf("should not have called askForPassword()")
			},
			func(password string) error {
				expected := "89bf5cifu6vo" // #nosec G101
				if password != expected {
					return fmt.Errorf("incorrect password. Expected=89bf5cifu6vo, received=%s", password)
				}

				return nil
			},
		},
		{
			"OK_401_ONCE", nil, "", "",
			func(_ loginReader) (string, error) {
				passwords := []string{"pwd", "hwd82bkwe"}
				if count > 1 {
					return "", fmt.Errorf("function in infinite loop")
				}
				count++

				return passwords[count-1], nil
			},
			func(password string) error {
				expected := "hwd82bkwe" // #nosec G101
				if password == expected {
					return nil
				}

				return &api.CredentialsError{}
			},
		},
		{
			"FAIL_401_EXISTS", nil, "Incorrect password", "v689cft",
			func(_ loginReader) (string, error) {
				return "", fmt.Errorf("should not have called askForPassword()")
			},
			func(_ string) error {
				return &api.CredentialsError{}
			},
		},
		{
			"FAIL_STATE", errExpected,
			"failed to get terminal state: " + errExpected.Error(), "",
			func(_ loginReader) (string, error) {
				return "", fmt.Errorf("function should not have called askForPassword()")
			},
			func(_ string) error {
				return fmt.Errorf("function should not have called api.Authenticate()")
			},
		},
		{
			"FAIL_ASK", nil, errExpected.Error(), "",
			func(_ loginReader) (string, error) {
				return "", errExpected
			},
			func(_ string) error {
				return fmt.Errorf("function should not have called api.Authenticate()")
			},
		},
		{
			"FAIL_AUTHENTICATE", nil, errExpected.Error(), "",
			func(_ loginReader) (string, error) {
				if count > 0 {
					return "", fmt.Errorf("function in infinite loop")
				}
				count++

				return "", nil
			},
			func(_ string) error {
				return errExpected
			},
		},
	}

	origAskForPassword := askForPassword
	origAuthenticate := api.Authenticate

	defer func() {
		askForPassword = origAskForPassword
		api.Authenticate = origAuthenticate
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			count = 0
			askForPassword = tt.mockAskForPassword
			api.Authenticate = tt.mockAuthenticate

			if tt.password == "" {
				os.Unsetenv("CSC_PASSWORD")
			} else {
				os.Setenv("CSC_PASSWORD", tt.password)
			}

			// Ignore prints to stdout
			null, _ := os.Open(os.DevNull)
			sout := os.Stdout
			os.Stdout = null

			err := login(testReader{"", tt.readerError})

			os.Stdout = sout
			null.Close()

			switch {
			case tt.errorText == "":
				if err != nil {
					t.Errorf("Returned unexpected error: %s", err.Error())
				}
			case err == nil:
				t.Error("Function should have returned error")
			case err.Error() != tt.errorText:
				t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errorText, err.Error())
			}
		})
	}
}

func TestProcessFlags(t *testing.T) {
	defaultMount := "default_dir"

	var tests = []struct {
		testname, mount, logLevel string
		timeout                   int
	}{
		{"OK_1", "/hello", "debug", 45},
		{"OK_2", "/goodbye", "warning", 87},
		{"OK_3", "/hi/hello", "error", 2},
		{"OK_4", "", "info", 20},
	}

	origDefaultMountPoint := mountpoint.DefaultMountPoint
	origCheckMountPoint := mountpoint.CheckMountPoint
	origSetRequestTimeout := api.SetRequestTimeout
	origSetLevel := logs.SetLevel

	defer func() {
		mountpoint.DefaultMountPoint = origDefaultMountPoint
		mountpoint.CheckMountPoint = origCheckMountPoint
		api.SetRequestTimeout = origSetRequestTimeout
		logs.SetLevel = origSetLevel
	}()

	var testTimeout int
	var testLevel, testMount string

	mountpoint.DefaultMountPoint = func() (string, error) {
		return defaultMount, nil
	}
	mountpoint.CheckMountPoint = func(mount string) error {
		testMount = mount

		return nil
	}
	api.SetRequestTimeout = func(timeout int) {
		testTimeout = timeout
	}
	logs.SetLevel = func(level string) {
		testLevel = level
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			mount = tt.mount
			logLevel = tt.logLevel
			requestTimeout = tt.timeout

			testTimeout = 0
			testLevel, testMount = "", ""

			err := processFlags()

			switch {
			case err != nil:
				t.Errorf("Returned unexpected error: %s", err.Error())
			case tt.timeout != testTimeout:
				t.Errorf("SetRequestTimeout() received incorrect timeout. Expected=%d, received=%d", tt.timeout, testTimeout)
			case tt.logLevel != testLevel:
				t.Errorf("SetLevel() received incorrect log level. Expected=%s, received=%s", tt.logLevel, testLevel)
			case tt.mount == "" && mount != defaultMount:
				t.Errorf("Expected default mount point %s, received=%s", defaultMount, mount)
			case tt.mount != testMount:
				t.Errorf("CheckMountPoint() received incorrect mount point. Expected=%s, received=%s", tt.mount, testMount)
			}
		})
	}
}

func TestProcessFlags_Error(t *testing.T) {
	var tests = []struct {
		testname, repository, mount        string
		checkMountError, defaultMountError error
	}{
		{"FAIL_DEFAULT_MOUNT", "misc", "", nil, errExpected},
		{"FAIL_CHECK_MOUNT", "all", "/bad/directory", errExpected, nil},
	}

	origDefaultMountPoint := mountpoint.DefaultMountPoint
	origCheckMountPoint := mountpoint.CheckMountPoint
	origSetRequestTimeout := api.SetRequestTimeout
	origSetLevel := logs.SetLevel

	defer func() {
		mountpoint.DefaultMountPoint = origDefaultMountPoint
		mountpoint.CheckMountPoint = origCheckMountPoint
		api.SetRequestTimeout = origSetRequestTimeout
		logs.SetLevel = origSetLevel
	}()

	api.SetRequestTimeout = func(timeout int) {}
	logs.SetLevel = func(level string) {}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			mount = tt.mount

			mountpoint.DefaultMountPoint = func() (string, error) {
				return "", tt.defaultMountError
			}
			mountpoint.CheckMountPoint = func(mount string) error {
				return tt.checkMountError
			}

			if err := processFlags(); err == nil {
				t.Error("Function should have returned error")
			} else if !errors.Is(err, errExpected) {
				t.Errorf("Function returned incorrect error: %s", err.Error())
			}
		})
	}
}
