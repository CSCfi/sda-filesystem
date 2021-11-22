package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"
	"sda-filesystem/internal/mountpoint"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

// testReader implements loginReader and contains password
type testReader struct {
	pwd    string
	err    error
	stream io.Reader
}

// testStream is an io.Reader given to testReader and contains username
type testStream struct {
	data string
	done bool
	err  error
}

func (s *testStream) Read(p []byte) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	copy(p, []byte(s.data))
	if s.done {
		return 0, io.EOF
	}
	s.done = true
	return len([]byte(s.data)), nil
}

func (r testReader) readPassword() (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return r.pwd, nil
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

func newTestReader(username string, password string, sErr error, rErr error) *testReader {
	return &testReader{stream: &testStream{data: username, done: false, err: sErr}, pwd: password, err: rErr}
}

func TestMain(m *testing.M) {
	logrus.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}

func TestAskForLogin(t *testing.T) {
	var tests = []struct {
		testname, username, password string
		streamError, readerError     error
	}{
		{
			"OK", "Jones", "567ghk789", nil, nil,
		},
		{
			"FAIL_SCANNER", "Jim", "xtykr6ofcyul", errors.New("Scanner error"), nil,
		},
		{
			"FAIL_READER", "Groot", "567ghk789", nil, errors.New("Reader error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			// Ignore prints to stdout
			null, _ := os.Open(os.DevNull)
			sout := os.Stdout
			os.Stdout = null

			r := newTestReader(tt.username, tt.password, tt.streamError, tt.readerError)
			str1, str2, err := askForLogin(r)

			os.Stdout = sout
			null.Close()

			if tt.testname != "OK" {
				if err == nil {
					t.Error("Function should have returned non-nil error")
				}
			} else if err != nil {
				t.Errorf("Function returned error: %s", err.Error())
			} else if str1 != tt.username {
				t.Errorf("Username incorrect. Expected %q, got %q", tt.username, str1)
			} else if str2 != tt.password {
				t.Errorf("Password incorrect. Expected %q, got %q", tt.password, str2)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	var count int
	var tests = []struct {
		testname          string
		readerError       error
		mockAskForLogin   func(loginReader) (string, string, error)
		mockValidateLogin func(string, ...string) error
	}{
		{
			"OK", nil,
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
					return fmt.Errorf("Incorrect username. Expected %q, got %q", username, auth[0])
				}
				if auth[1] != password {
					return fmt.Errorf("Incorrect password. Expected %q, got %q", password, auth[1])
				}
				return nil
			},
		},
		{
			"OK_401_ONCE", nil,
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
			"FAIL_STATE", errors.New("Error occurred"),
			func(lr loginReader) (string, string, error) {
				return "", "", fmt.Errorf("Function should not have called askForLogin()")
			},
			func(rep string, auth ...string) error {
				return fmt.Errorf("Function should not have called api.ValidateLogin()")
			},
		},
		{
			"FAIL_ASK", nil,
			func(lr loginReader) (string, string, error) {
				return "", "", fmt.Errorf("Error asking input")
			},
			func(rep string, auth ...string) error {
				return nil
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

			r := newTestReader("", "", nil, tt.readerError)
			err := login(r, api.SDConnect)

			os.Stdout = sout
			null.Close()

			if strings.HasPrefix(tt.testname, "OK") {
				if err != nil {
					t.Errorf("Function returned error: %s", err.Error())
				}
			} else if err == nil {
				t.Error("Function should have returned non-nil error")
			}
		})
	}
}

func TestLogin_Validation_Fail(t *testing.T) {
	origAskForLogin := askForLogin
	origValidateLogin := api.ValidateLogin

	defer func() {
		askForLogin = origAskForLogin
		api.ValidateLogin = origValidateLogin
	}()

	count := 0
	askForLogin = func(lr loginReader) (string, string, error) {
		if count > 0 {
			return "", "", fmt.Errorf("Function in infinite loop")
		}
		count++
		return "Anna", "cgt6d8c", nil
	}
	api.ValidateLogin = func(rep string, auth ...string) error {
		return &api.RequestError{StatusCode: 500}
	}

	// Ignore prints to stdout
	null, _ := os.Open(os.DevNull)
	sout := os.Stdout
	os.Stdout = null

	r := newTestReader("", "", nil, nil)
	err := login(r, api.SDConnect)

	os.Stdout = sout
	null.Close()

	if err == nil {
		t.Fatal("Function should have returned non-nil error")
	}

	// To get the innermost error
	for errors.Unwrap(err) != nil {
		err = errors.Unwrap(err)
	}

	var re *api.RequestError
	if !(errors.As(err, &re) && re.StatusCode == 500) {
		t.Fatalf("Function did not return RequestError with status code 500\nGot: %s", err.Error())
	}
}

func TestLoginToAll(t *testing.T) {
	var tests = []struct {
		testname          string
		removedReps       []string
		mockLogin         func(loginReader, string) error
		mockValidateLogin func(string, ...string) error
	}{
		{
			"OK", []string{},
			func(lr loginReader, rep string) error {
				return nil
			},
			func(rep string, auth ...string) error {
				return nil
			},
		},
		{
			"FAIL_CONNECT", []string{api.SDConnect},
			func(lr loginReader, rep string) error {
				return fmt.Errorf("Error occurred")
			},
			func(rep string, auth ...string) error {
				return nil
			},
		},
		{
			"FAIL_SUBMIT", []string{api.SDSubmit},
			func(lr loginReader, rep string) error {
				return nil
			},
			func(rep string, auth ...string) error {
				return fmt.Errorf("Error occurred")
			},
		},
	}

	origGetEnabledRepositories := api.GetEnabledRepositories
	origLogin := login
	origValidateLogin := api.ValidateLogin
	origRemoveRepository := api.RemoveRepository

	defer func() {
		api.GetEnabledRepositories = origGetEnabledRepositories
		login = origLogin
		api.ValidateLogin = origValidateLogin
		api.RemoveRepository = origRemoveRepository
	}()

	var removed []string
	api.RemoveRepository = func(r string) {
		removed = append(removed, r)
	}
	api.GetEnabledRepositories = func() []string {
		return []string{api.SDConnect, api.SDSubmit, "dummy"}
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			login = tt.mockLogin
			api.ValidateLogin = tt.mockValidateLogin
			removed = []string{}

			loginToAll()

			if !reflect.DeepEqual(tt.removedReps, removed) {
				t.Errorf("Incorrect repositories removed. Expected %v, got %v", tt.removedReps, removed)
			}
		})
	}
}

func TestProcessFlags(t *testing.T) {
	allRepositories := []string{"Rep1", "Rep2", "Rep3"}

	var tests = []struct {
		testname, repository, mount, logLevel string
		finalReps                             []string
		timeout                               int
		mockCheckMountPoint                   func(string) error
	}{
		{
			"OK_1", "Rep2", "/hello", "debug",
			[]string{"Rep2"}, 45, nil,
		},
		{
			"OK_2", "all", "/goodbye", "warning",
			allRepositories, 87, nil,
		},
		{
			"OK_3", "wrong_repository", "/hi/hello", "error",
			allRepositories, 2, nil,
		},
		{
			"FAIL", "Rep3", "/bad/directory", "info",
			[]string{"Rep3"}, 29,
			func(mount string) error {
				return fmt.Errorf("Error occurred")
			},
		},
	}

	origGetAllPossibleRepositories := api.GetAllPossibleRepositories
	origAddRepository := api.AddRepository
	origCheckMountPoint := mountpoint.CheckMountPoint
	origSetRequestTimeout := api.SetRequestTimeout
	origSetLevel := logs.SetLevel

	defer func() {
		api.GetAllPossibleRepositories = origGetAllPossibleRepositories
		api.AddRepository = origAddRepository
		mountpoint.CheckMountPoint = origCheckMountPoint
		api.SetRequestTimeout = origSetRequestTimeout
		logs.SetLevel = origSetLevel
	}()

	var reps []string
	var testTimeout int
	var testLevel, testMount string

	api.GetAllPossibleRepositories = func() []string {
		return allRepositories
	}
	api.AddRepository = func(r string) {
		reps = append(reps, r)
	}
	mockCheckMountPoint := func(mount string) error {
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
			mountPoint = tt.mount
			repository = tt.repository
			logLevel = tt.logLevel
			requestTimeout = tt.timeout

			reps = []string{}
			testTimeout = 0
			testLevel, testMount = "", ""

			if tt.mockCheckMountPoint != nil {
				mountpoint.CheckMountPoint = tt.mockCheckMountPoint
			} else {
				mountpoint.CheckMountPoint = mockCheckMountPoint
			}

			err := processFlags()

			if tt.testname == "FAIL" {
				if err == nil {
					t.Error("Function should have returned error")
				}
			} else if err != nil {
				t.Errorf("Function returned non-nil error: %s", err.Error())
			} else if tt.timeout != testTimeout {
				t.Errorf("SetRequestTimeout() received timeout %d, expected %d", testTimeout, tt.timeout)
			} else if tt.logLevel != testLevel {
				t.Errorf("SetLevel() received log level %s, expected %s", testLevel, tt.logLevel)
			} else if tt.mount != testMount {
				t.Errorf("CheckMountPoint() received mount point %q, expected %q", testMount, tt.mount)
			} else if !reflect.DeepEqual(reps, tt.finalReps) {
				t.Errorf("Function did not add repositories correctly.\nExpected %v\nGot%v", tt.finalReps, reps)
			}
		})
	}
}
