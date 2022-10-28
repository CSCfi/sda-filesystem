package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"
	"sda-filesystem/internal/mountpoint"
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
	logs.SetSignal(func(i int, s []string) {})
	os.Exit(m.Run())
}

func TestUserChooseUpdate(t *testing.T) {
	finished := false
	buf := bytes.NewBufferString("continue\nhello\nupdate")

	go func() {
		userChooseUpdate(buf)
		finished = true
	}()

	time.Sleep(10 * time.Millisecond)

	if !finished {
		t.Fatal("Function did not read input correctly. Input 'update' did not stop function")
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
			str1, str2, err := askForLogin(r)

			os.Stdout = sout
			null.Close()

			if tt.testname != "OK" {
				if err == nil {
					t.Error("Function should have returned error")
				} else if err.Error() != tt.errorText {
					t.Errorf("Function returned incorrect error\nExpected=%s\nReceived=%s", tt.errorText, err.Error())
				}
			} else if err != nil {
				t.Errorf("Function returned error: %s", err.Error())
			} else if str1 != tt.username {
				t.Errorf("Username incorrect. Expected=%s, received=%s", tt.username, str1)
			} else if str2 != tt.password {
				t.Errorf("Password incorrect. Expected=%s, received=%s", tt.password, str2)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	var count int
	var tests = []struct {
		testname          string
		readerError       error
		errorText         string
		mockAskForLogin   func(loginReader) (string, string, error)
		mockValidateLogin func(string, string) (bool, error)
	}{
		{
			"OK", nil, "",
			func(lr loginReader) (string, string, error) {
				if count > 0 {
					return "", "", fmt.Errorf("Function did not approve login during first loop")
				}
				count++
				return "dumbledore", "345fgj78", nil
			},
			func(uname, pwd string) (bool, error) {
				username, password := "dumbledore", "345fgj78"
				if uname != username {
					return false, fmt.Errorf("Incorrect username. Expected=%s, received=%s", username, uname)
				}
				if pwd != password {
					return false, fmt.Errorf("Incorrect password. Expected=%s, received=%s", password, pwd)
				}
				return true, nil
			},
		},
		{
			"OK_VALIDATE_ERROR", nil, "",
			func(lr loginReader) (string, string, error) {
				if count > 0 {
					return "", "", fmt.Errorf("Function did not approve login during first loop")
				}
				count++
				return "sandman", "89bf5cifu6vo", nil
			},
			func(uname, pwd string) (bool, error) {
				username, password := "sandman", "89bf5cifu6vo" // #nosec G101
				if uname != username {
					return false, fmt.Errorf("Incorrect username. Expected=%s, received=%s", username, uname)
				}
				if pwd != password {
					return false, fmt.Errorf("Incorrect password. Expected=%s, received=%s", password, pwd)
				}
				return true, errExpected
			},
		},
		{
			"OK_401_ONCE", nil, "",
			func(lr loginReader) (string, string, error) {
				usernames, passwords := []string{"Smith", "Doris"}, []string{"hwd82bkwe", "pwd"}
				if count > 1 {
					return "", "", fmt.Errorf("Function in infinite loop")
				}
				count++
				return usernames[count-1], passwords[count-1], nil
			},
			func(uname, pwd string) (bool, error) {
				if uname == "Doris" && pwd == "pwd" {
					return true, nil
				}
				return false, &api.RequestError{StatusCode: 401}
			},
		},
		{
			"FAIL_STATE", errExpected,
			"Failed to get terminal state: " + errExpected.Error(),
			func(lr loginReader) (string, string, error) {
				return "", "", fmt.Errorf("Function should not have called askForLogin()")
			},
			func(uname, pwd string) (bool, error) {
				return false, fmt.Errorf("Function should not have called api.ValidateLogin()")
			},
		},
		{
			"FAIL_ASK", nil, errExpected.Error(),
			func(lr loginReader) (string, string, error) {
				return "", "", errExpected
			},
			func(uname, pwd string) (bool, error) {
				return false, fmt.Errorf("Function should not have called api.ValidateLogin()")
			},
		},
		{
			"FAIL_VALIDATE", nil, errExpected.Error(),
			func(lr loginReader) (string, string, error) {
				if count > 0 {
					return "", "", fmt.Errorf("Function in infinite loop")
				}
				count++
				return "", "", nil
			},
			func(uname, pwd string) (bool, error) {
				return false, errExpected
			},
		},
	}

	origAskForLogin := askForLogin
	origValidateLogin := api.ValidateLogin

	defer func() {
		askForLogin = origAskForLogin
		api.ValidateLogin = origValidateLogin
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			count = 0
			askForLogin = tt.mockAskForLogin
			api.ValidateLogin = tt.mockValidateLogin

			// Ignore prints to stdout
			null, _ := os.Open(os.DevNull)
			sout := os.Stdout
			os.Stdout = null

			r := newTestReader([]string{""}, "", nil, tt.readerError)
			err := login(r)

			os.Stdout = sout
			null.Close()

			if tt.errorText == "" {
				if err != nil {
					t.Errorf("Returned unexpected error: %s", err.Error())
				}
			} else if err == nil {
				t.Error("Function should have returned error")
			} else if err.Error() != tt.errorText {
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

			if err != nil {
				t.Errorf("Returned unexpected error: %s", err.Error())
			} else if tt.timeout != testTimeout {
				t.Errorf("SetRequestTimeout() received incorrect timeout. Expected=%d, received=%d", tt.timeout, testTimeout)
			} else if tt.logLevel != testLevel {
				t.Errorf("SetLevel() received incorrect log level. Expected=%s, received=%s", tt.logLevel, testLevel)
			} else if tt.mount == "" && mount != defaultMount {
				t.Errorf("Expected default mount point %s, received=%s", defaultMount, mount)
			} else if tt.mount != testMount {
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
