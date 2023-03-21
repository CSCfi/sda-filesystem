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

var errExpected = errors.New("Expected error for test")

// testReader implements loginReader and contains password
type testReader struct {
	pwd    string
	err    error
	stream io.Reader
}

func (r testReader) readPassword() (string, error) {
	return r.pwd, r.err
}

func (r testReader) getStream() io.Reader {
	return r.stream
}

func (r testReader) getState() error {
	return r.err
}

func (r testReader) restoreState() error {
	return nil
}

// testStream is an io.Reader given to testReader
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

func newTestReader(input []string, password string, sErr error, rErr error) *testReader {
	return &testReader{
		stream: &testStream{data: input, err: sErr},
		pwd:    password,
		err:    rErr,
	}
}

func TestMain(m *testing.M) {
	logs.SetSignal(func(l string, s []string) {})
	os.Exit(m.Run())
}

func TestUserChooseUpdate(t *testing.T) {
	finished := false
	reader := strings.NewReader("continue\nhello\nupdate")

	go func() {
		userChooseUpdate(reader)
		finished = true
	}()

	time.Sleep(10 * time.Millisecond)

	if !finished {
		t.Fatal("Function did not read input correctly. Input 'update' did not stop function")
	}
}

func TestUserChooseUpdate_Error(t *testing.T) {
	finished := false
	buf := &testStream{err: errExpected}

	var level string
	var strs []string
	logs.SetSignal(func(ll string, s []string) {
		level, strs = ll, s
	})
	defer logs.SetSignal(func(l string, s []string) {})

	go func() {
		userChooseUpdate(buf)
		finished = true
	}()

	time.Sleep(10 * time.Millisecond)

	err := []string{"Could not read input", errExpected.Error()}
	if level != logrus.ErrorLevel.String() {
		t.Fatal("Function did not log an error")
	}
	if !reflect.DeepEqual(strs, err) {
		t.Fatalf("Logged output incorrect\nExpected: %q\nReceived: %q", err, strs)
	}
	if finished {
		t.Error("Function did not read input correctly. Input should not have stopped function")
	}
}

func TestAskForLogin(t *testing.T) {
	var tests = []struct {
		testname, username, password string
		streamError, readerError     error
		errorText                    string
	}{
		{
			"OK", "Jones", "567ghk789", nil, nil, "",
		},
		{
			"FAIL_SCANNER", "Jim", "xtykr6ofcyul", errExpected, nil,
			"Could not read username: " + errExpected.Error(),
		},
		{
			"FAIL_READER", "Groot", "567ghk789", nil, errExpected,
			"Could not read password: " + errExpected.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			// Ignore prints to stdout
			null, _ := os.Open(os.DevNull)
			sout := os.Stdout
			os.Stdout = null

			r := newTestReader([]string{tt.username}, tt.password, tt.streamError, tt.readerError)
			str1, str2, exists, err := askForLogin(r)

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
			case exists:
				t.Errorf("Function says the environment variables exist.")
			case str1 != tt.username:
				t.Errorf("Username incorrect. Expected=%s, received=%s", tt.username, str1)
			case str2 != tt.password:
				t.Errorf("Password incorrect. Expected=%s, received=%s", tt.password, str2)
			}

		})
	}
}

func TestAskForLogin_Envs(t *testing.T) {
	// Ignore prints to stdout
	null, _ := os.Open(os.DevNull)
	sout := os.Stdout
	os.Stdout = null

	username := "rumplestiltskin"
	password := "very-secret-password"
	os.Setenv("CSC_USERNAME", username)
	os.Setenv("CSC_PASSWORD", password)

	r := newTestReader([]string{}, "", errors.New("stream error"), errors.New("reader error"))
	str1, str2, exists, err := askForLogin(r)

	os.Unsetenv("CSC_USERNAME")
	os.Unsetenv("CSC_PASSWORD")

	os.Stdout = sout
	null.Close()

	switch {
	case err != nil:
		t.Errorf("Function returned error: %s", err.Error())
	case !exists:
		t.Errorf("Function says the environment variables do not exist.")
	case str1 != username:
		t.Errorf("Username incorrect. Expected=%s, received=%s", username, str1)
	case str2 != password:
		t.Errorf("Password incorrect. Expected=%s, received=%s", password, str2)
	}
}

func TestLogin(t *testing.T) {
	var count int
	var tests = []struct {
		testname          string
		readerError       error
		errorText         string
		mockAskForLogin   func(loginReader) (string, string, bool, error)
		mockValidateLogin func(string, ...string) error
	}{
		{
			"OK", nil, "",
			func(lr loginReader) (string, string, bool, error) {
				if count > 0 {
					return "", "", false, fmt.Errorf("Function did not approve login during first loop")
				}
				count++

				return "dumbledore", "345fgj78", false, nil
			},
			func(rep string, rest ...string) error {
				if len(rest) != 2 {
					return fmt.Errorf("Too few parameters")
				}
				if rep != api.SDConnect {
					return fmt.Errorf("Repository should have been %s", api.SDConnect)
				}

				token := "ZHVtYmxlZG9yZTozNDVmZ2o3OA==" // #nosec G101
				if rest[0] != token {
					return fmt.Errorf("Incorrect token. Expected=%s, received=%s", token, rest[0])
				}

				return nil
			},
		},
		{
			"OK_EXISTS", nil, "",
			func(lr loginReader) (string, string, bool, error) {
				if count > 0 {
					return "", "", true, fmt.Errorf("Function did not approve login during first loop")
				}
				count++

				return "sandman", "89bf5cifu6vo", true, nil
			},
			func(rep string, rest ...string) error {
				if len(rest) != 2 {
					return fmt.Errorf("Too few parameters")
				}
				if rep != api.SDConnect {
					return fmt.Errorf("Repository should have been %s", api.SDConnect)
				}

				token := "c2FuZG1hbjo4OWJmNWNpZnU2dm8=" // #nosec G101
				if rest[0] != token {
					return fmt.Errorf("Incorrect token. Expected=%s, received=%s", token, rest[0])
				}

				return nil
			},
		},
		{
			"OK_401_ONCE", nil, "",
			func(lr loginReader) (string, string, bool, error) {
				usernames, passwords := []string{"Smith", "Doris"}, []string{"hwd82bkwe", "pwd"}
				if count > 1 {
					return "", "", false, fmt.Errorf("Function in infinite loop")
				}
				count++

				return usernames[count-1], passwords[count-1], false, nil
			},
			func(rep string, rest ...string) error {
				if len(rest) != 2 {
					return fmt.Errorf("Too few parameters")
				}
				if rep != api.SDConnect {
					return fmt.Errorf("Repository should have been %s", api.SDConnect)
				}

				token := "RG9yaXM6cHdk" // #nosec G101
				if rest[0] == token {
					return nil
				}

				return &api.RequestError{StatusCode: 401}
			},
		},
		{
			"FAIL_401_EXISTS", nil, "Incorrect username or password",
			func(lr loginReader) (string, string, bool, error) {
				if count > 0 {
					return "", "", true, fmt.Errorf("Function in infinite loop")
				}
				count++

				return "Jones", "v689cft", true, nil
			},
			func(rep string, rest ...string) error {
				return &api.RequestError{StatusCode: 401}
			},
		},
		{
			"FAIL_STATE", errExpected,
			"Failed to get terminal state: " + errExpected.Error(),
			func(lr loginReader) (string, string, bool, error) {
				return "", "", false, fmt.Errorf("Function should not have called askForLogin()")
			},
			func(rep string, rest ...string) error {
				return fmt.Errorf("Function should not have called api.ValidateLogin()")
			},
		},
		{
			"FAIL_ASK", nil, errExpected.Error(),
			func(lr loginReader) (string, string, bool, error) {
				return "", "", false, errExpected
			},
			func(rep string, rest ...string) error {
				return fmt.Errorf("Function should not have called api.ValidateLogin()")
			},
		},
		{
			"FAIL_VALIDATE", nil, errExpected.Error(),
			func(lr loginReader) (string, string, bool, error) {
				if count > 0 {
					return "", "", false, fmt.Errorf("Function in infinite loop")
				}
				count++

				return "", "", false, nil
			},
			func(rep string, rest ...string) error {
				return errExpected
			},
		},
	}

	origAskForLogin := askForLogin
	origValidateLogin := api.Authenticate

	defer func() {
		askForLogin = origAskForLogin
		api.Authenticate = origValidateLogin
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			count = 0
			askForLogin = tt.mockAskForLogin
			api.Authenticate = tt.mockValidateLogin

			// Ignore prints to stdout
			null, _ := os.Open(os.DevNull)
			sout := os.Stdout
			os.Stdout = null

			r := newTestReader([]string{""}, "", nil, tt.readerError)
			err := login(r)

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

func TestDetermineAccess(t *testing.T) {
	var tests = []struct {
		testname              string
		submitErr, connectErr error
		sdsubmit              bool
	}{
		{"OK_1", nil, nil, false},
		{"OK_2", nil, nil, true},
		{"OK_3", nil, errExpected, true},
		{"OK_4", nil, errExpected, false},
		{"OK_5", errExpected, nil, false},
		{"FAIL_SUBMIT", errExpected, nil, true},
		{"FAIL_ALL_1", errExpected, errExpected, false},
		{"FAIL_ALL_2", errExpected, errExpected, true},
	}

	origSDSubmit := sdsubmit
	origLogin := login
	origValidateLogin := api.Authenticate

	defer func() {
		sdsubmit = origSDSubmit
		login = origLogin
		api.Authenticate = origValidateLogin
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			sdsubmit = tt.sdsubmit
			login = func(lr loginReader) error {
				return tt.connectErr
			}
			api.Authenticate = func(rep string, auth ...string) error {
				return tt.submitErr
			}

			err := determineAccess()
			switch {
			case strings.HasPrefix(tt.testname, "OK"):
				if err != nil {
					t.Errorf("Returned unexpected error: %s", err.Error())
				}
			case err == nil:
				t.Error("Function should have returned error")
			case !errors.Is(err, errExpected):
				t.Errorf("Function returned incorrect error: %s", err.Error())
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
