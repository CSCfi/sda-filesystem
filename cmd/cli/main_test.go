package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"

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

func TestDroppedRepository(t *testing.T) {
	var tests = []struct {
		testname string
		input    []string
		drop     bool
		err      error
	}{
		{
			testname: "OK_NO_1",
			input:    []string{"no"},
			drop:     true,
		},
		{
			testname: "OK_NO_2",
			input:    []string{"on", "n"},
			drop:     true,
		},
		{
			testname: "OK_YES_1",
			input:    []string{"yes"},
		},
		{
			testname: "OK_YES_2",
			input:    []string{"mmm", "y"},
		},
		{
			testname: "FAIL_SCANNER",
			input:    []string{"yes"},
			drop:     true,
			err:      errors.New("Stream error occurred"),
		},
	}

	origRemoveRepository := api.RemoveRepository
	defer func() { api.RemoveRepository = origRemoveRepository }()

	var removed bool
	api.RemoveRepository = func(r string) {
		removed = true
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			removed = false

			// Ignore prints to stdout
			null, _ := os.Open(os.DevNull)
			sout := os.Stdout
			os.Stdout = null

			r := newTestReader(tt.input, "", tt.err, nil)
			dropped := droppedRepository(r, "Test Repository")

			os.Stdout = sout
			null.Close()

			if tt.drop {
				if !removed {
					t.Errorf("Function should have called api.RemoveRepository()")
				} else if !dropped {
					t.Errorf("Function should have returned 'true'")
				}
			} else {
				if removed {
					t.Errorf("Function should not have called api.RemoveRepository()")
				} else if dropped {
					t.Errorf("Function should have returned 'false'")
				}
			}
		})
	}
}

func TestLogin(t *testing.T) {
	var count int
	var tests = []struct {
		testname          string
		readerError       error
		dropped           bool
		errorText         string
		mockAskForLogin   func(loginReader) (string, string, error)
		mockValidateLogin func(string, ...string) error
	}{
		{
			"OK", nil, false, "",
			func(lr loginReader) (string, string, error) {
				if count > 0 {
					return "", "", fmt.Errorf("Function did not approve login during first loop")
				}
				count++
				return "dumbledore", "345fgj78", nil
			},
			func(rep string, auth ...string) error {
				username, password := "dumbledore", "345fgj78"
				if auth[0] != username {
					return fmt.Errorf("Incorrect username. Expected=%s, received=%s", username, auth[0])
				}
				if auth[1] != password {
					return fmt.Errorf("Incorrect password. Expected=%s, received=%s", password, auth[1])
				}
				return nil
			},
		},
		{
			"OK_DROPPED", nil, true, "",
			func(lr loginReader) (string, string, error) {
				if count > 0 {
					return "", "", fmt.Errorf("Function did not return during first loop")
				}
				count++
				return "", "", nil
			},
			func(rep string, auth ...string) error {
				return &api.RequestError{StatusCode: 401}
			},
		},
		{
			"OK_401_ONCE", nil, false, "",
			func(lr loginReader) (string, string, error) {
				usernames, passwords := []string{"Smith", "Doris"}, []string{"hwd82bkwe", "pwd"}
				if count > 1 {
					return "", "", fmt.Errorf("Function in infinite loop")
				}
				count++
				return usernames[count-1], passwords[count-1], nil
			},
			func(rep string, auth ...string) error {
				if auth[0] == "Doris" && auth[1] == "pwd" {
					return nil
				}
				return &api.RequestError{StatusCode: 401}
			},
		},
		{
			"FAIL_STATE", errExpected, false,
			"Failed to get terminal state: " + errExpected.Error(),
			func(lr loginReader) (string, string, error) {
				return "", "", fmt.Errorf("Function should not have called askForLogin()")
			},
			func(rep string, auth ...string) error {
				return fmt.Errorf("Function should not have called api.ValidateLogin()")
			},
		},
		{
			"FAIL_ASK", nil, false, errExpected.Error(),
			func(lr loginReader) (string, string, error) {
				return "", "", errExpected
			},
			func(rep string, auth ...string) error {
				return fmt.Errorf("Function should not have called api.ValidateLogin()")
			},
		},
		{
			"FAIL_VALIDATE", nil, false,
			"Failed to log in",
			func(lr loginReader) (string, string, error) {
				if count > 0 {
					return "", "", fmt.Errorf("Function in infinite loop")
				}
				count++
				return "", "", nil
			},
			func(rep string, auth ...string) error {
				return errExpected
			},
		},
	}

	origAskForLogin := askForLogin
	origValidateLogin := api.ValidateLogin
	origDroppedRepository := droppedRepository

	defer func() {
		askForLogin = origAskForLogin
		api.ValidateLogin = origValidateLogin
		droppedRepository = origDroppedRepository
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			count = 0
			askForLogin = tt.mockAskForLogin
			api.ValidateLogin = tt.mockValidateLogin
			droppedRepository = func(lr loginReader, rep string) bool {
				return tt.dropped
			}

			// Ignore prints to stdout
			null, _ := os.Open(os.DevNull)
			sout := os.Stdout
			os.Stdout = null

			r := newTestReader([]string{""}, "", nil, tt.readerError)
			err := login(r, "Test Repository")

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

func TestLoginToAll(t *testing.T) {
	var tests = []struct {
		testname              string
		reps, removedReps     []string
		errLogin, errValidate error
		loginMethods          map[string]api.LoginMethod
	}{
		{
			"OK", []string{"Rep45", "Rep0"}, []string{}, nil, nil,
			map[string]api.LoginMethod{"Rep45": api.Token, "Rep0": api.Password},
		},
		{
			"FAIL_PASSWORD", []string{"Rep5", "Rep8", "Rep89"}, []string{"Rep5", "Rep8", "Rep89"},
			fmt.Errorf("Error occurred"), nil,
			map[string]api.LoginMethod{"Rep5": api.Password, "Rep8": api.Password, "Rep89": api.Password},
		},
		{
			"FAIL_TOKEN", []string{"Rep4", "Rep78"}, []string{"Rep4", "Rep78"},
			nil, fmt.Errorf("Error occurred"),
			map[string]api.LoginMethod{"Rep4": api.Token, "Rep78": api.Token},
		},
		{
			"FAIL_MIXED", []string{"Rep50", "Rep8", "Rep9"}, []string{"Rep8"},
			nil, fmt.Errorf("Error occurred"),
			map[string]api.LoginMethod{"Rep50": api.Password, "Rep8": api.Token, "Rep9": 3},
		},
	}

	origGetEnabledRepositories := api.GetEnabledRepositories
	origGetLoginMethod := api.GetLoginMethod
	origLogin := login
	origValidateLogin := api.ValidateLogin
	origRemoveRepository := api.RemoveRepository

	defer func() {
		api.GetEnabledRepositories = origGetEnabledRepositories
		api.GetLoginMethod = origGetLoginMethod
		login = origLogin
		api.ValidateLogin = origValidateLogin
		api.RemoveRepository = origRemoveRepository
	}()

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			api.GetEnabledRepositories = func() []string {
				return tt.reps
			}
			api.GetLoginMethod = func(rep string) api.LoginMethod {
				return tt.loginMethods[rep]
			}
			login = func(lr loginReader, rep string) error {
				return tt.errLogin
			}
			api.ValidateLogin = func(rep string, auth ...string) error {
				return tt.errValidate
			}
			removed := []string{}
			api.RemoveRepository = func(r string) {
				removed = append(removed, r)
			}

			loginToAll()

			if !reflect.DeepEqual(tt.removedReps, removed) {
				t.Errorf("Incorrect repositories removed\nExpected %v\nReceived %v", tt.removedReps, removed)
			}
		})
	}
}

func TestProcessFlags(t *testing.T) {
	repositories := []string{"Rep1", "Rep2", "Rep3"}
	defaultMount := "default_dir"

	var tests = []struct {
		testname, repository, mount, logLevel string
		finalReps                             []string
		timeout                               int
	}{
		{"OK_1", "Rep2", "/hello", "debug", []string{"Rep2"}, 45},
		{"OK_2", "all", "/goodbye", "warning", repositories, 87},
		{"OK_3", "wrong_repository", "/hi/hello", "error", repositories, 2},
		{"OK_4", "Rep3", "", "info", []string{"Rep3"}, 20},
	}

	origGetAllPossibleRepositories := api.GetAllPossibleRepositories
	origAddRepository := api.AddRepository
	origDefaultMountPoint := mountpoint.DefaultMountPoint
	origCheckMountPoint := mountpoint.CheckMountPoint
	origSetRequestTimeout := api.SetRequestTimeout
	origSetLevel := logs.SetLevel

	defer func() {
		api.GetAllPossibleRepositories = origGetAllPossibleRepositories
		api.AddRepository = origAddRepository
		mountpoint.DefaultMountPoint = origDefaultMountPoint
		mountpoint.CheckMountPoint = origCheckMountPoint
		api.SetRequestTimeout = origSetRequestTimeout
		logs.SetLevel = origSetLevel
	}()

	var reps []string
	var testTimeout int
	var testLevel, testMount string

	api.GetAllPossibleRepositories = func() []string {
		return repositories
	}
	api.AddRepository = func(r string) {
		reps = append(reps, r)
	}
	api.GetEnvs = func(r string) error { return nil }
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
			repository = tt.repository
			logLevel = tt.logLevel
			requestTimeout = tt.timeout

			reps = []string{}
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
			} else if !reflect.DeepEqual(reps, tt.finalReps) {
				t.Errorf("Function did not add repositories correctly\nExpected=%v\nReceived=%v", tt.finalReps, reps)
			}
		})
	}
}

func TestProcessFlags_Error(t *testing.T) {
	var tests = []struct {
		testname, repository, mount                     string
		addRepError, checkMountError, defaultMountError error
	}{
		{"FAIL_ADD_REPOSITORY_1", "Rep1", "mount", errExpected, nil, nil},
		{"FAIL_ADD_REPOSITORY_2", "Rep4", "mount", errExpected, nil, nil},
		{"FAIL_DEFAULT_MOUNT", "misc", "", nil, nil, errExpected},
		{"FAIL_CHECK_MOUNT", "all", "/bad/directory", nil, errExpected, nil},
	}

	origGetAllPossibleRepositories := api.GetAllPossibleRepositories
	origAddRepository := api.AddRepository
	origDefaultMountPoint := mountpoint.DefaultMountPoint
	origCheckMountPoint := mountpoint.CheckMountPoint
	origSetRequestTimeout := api.SetRequestTimeout
	origSetLevel := logs.SetLevel

	defer func() {
		api.GetAllPossibleRepositories = origGetAllPossibleRepositories
		api.AddRepository = origAddRepository
		mountpoint.DefaultMountPoint = origDefaultMountPoint
		mountpoint.CheckMountPoint = origCheckMountPoint
		api.SetRequestTimeout = origSetRequestTimeout
		logs.SetLevel = origSetLevel
	}()

	api.GetAllPossibleRepositories = func() []string {
		return []string{"Rep1", "Rep2", "Rep3"}
	}
	api.AddRepository = func(r string) {}
	api.SetRequestTimeout = func(timeout int) {}
	logs.SetLevel = func(level string) {}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			mount = tt.mount
			repository = tt.repository

			api.GetEnvs = func(r string) error {
				return tt.addRepError
			}
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
