package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"
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
