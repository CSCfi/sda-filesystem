package main

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"sda-filesystem/internal/api"
	"testing"

	"github.com/sirupsen/logrus"
)

// testReader implements loginReader
type testReader struct {
	pwd    string
	err    error
	stream io.Reader
}

// testStream is an io.Reader given to testReader
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

// So that `go test` does not complain about flags
var _ = func() bool {
	testing.Init()
	return true
}()

func TestMain(m *testing.M) {
	logrus.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}

func TestMounPoint(t *testing.T) {
	fatal := false
	logrus.StandardLogger().ExitFunc = func(int) { fatal = true }

	dir := "/spirited/away"

	origHomeDir := os.Getenv("HOME")
	os.Setenv("HOME", dir)

	defer func() {
		logrus.StandardLogger().ExitFunc = nil
		os.Setenv("HOME", origHomeDir)
	}()

	ret := mountPoint()
	if fatal {
		t.Fatal("Function called Exit()")
	}
	if ret != dir+"/Projects" {
		t.Fatalf("Incorrect mount point. Expected %q, got %q", dir+"/Projects", ret)
	}
}

func TestMounPoint_Error(t *testing.T) {
	fatal := false
	logrus.StandardLogger().ExitFunc = func(int) { fatal = true }

	origHomeDir := os.Getenv("HOME")
	os.Unsetenv("HOME")

	defer func() {
		logrus.StandardLogger().ExitFunc = nil
		os.Setenv("HOME", origHomeDir)
	}()

	_ = mountPoint()
	if !fatal {
		t.Fatal("Function should have called Exit()")
	}
}

func TestAskForLogin(t *testing.T) {
	fatal := false
	logrus.StandardLogger().ExitFunc = func(int) { fatal = true }

	// Ignore prints to stdout
	null, _ := os.Open(os.DevNull)
	sout := os.Stdout
	os.Stdout = null

	defer func() { logrus.StandardLogger().ExitFunc = nil }()

	username := "Jones"
	password := "567ghk789"

	r := newTestReader(username, password, nil, nil)
	str1, str2 := askForLogin(r)
	os.Stdout = sout
	null.Close()

	if fatal {
		t.Fatal("Function called Exit()")
	}
	if str1 != username {
		t.Errorf("Username incorrect. Expected %q, got %q", username, str1)
	}
	if str2 != password {
		t.Errorf("Password incorrect. Expected %q, got %q", password, str2)
	}
}

func TestAskForLogin_Username_Fatal(t *testing.T) {
	fatal := false
	logrus.StandardLogger().ExitFunc = func(int) { fatal = true }

	// Ignore prints to stdout
	null, _ := os.Open(os.DevNull)
	sout := os.Stdout
	os.Stdout = null

	defer func() { logrus.StandardLogger().ExitFunc = nil }()

	r := newTestReader("Jim", "xtykr6ofcyul", errors.New("Cannot read from scanner"), nil)
	_, _ = askForLogin(r)
	os.Stdout = sout
	null.Close()

	if !fatal {
		t.Fatal("Function should have called Exit()")
	}
}

func TestAskForLogin_Password_Fatal(t *testing.T) {
	fatal := false
	logrus.StandardLogger().ExitFunc = func(int) { fatal = true }

	// Ignore prints to stdout
	null, _ := os.Open(os.DevNull)
	sout := os.Stdout
	os.Stdout = null

	defer func() { logrus.StandardLogger().ExitFunc = nil }()

	r := newTestReader("Groot", "567ghk789", nil, errors.New("Cannot read password"))
	_, _ = askForLogin(r)
	os.Stdout = sout
	null.Close()

	if !fatal {
		t.Fatal("Function should've called Exit()")
	}
}

func TestLogin(t *testing.T) {
	fatal := false
	logrus.StandardLogger().ExitFunc = func(int) { fatal = true }

	origAskForLogin := askForLogin
	origCreateToken := api.CreateToken
	origGetUToken := api.GetUToken

	defer func() {
		askForLogin = origAskForLogin
		api.CreateToken = origCreateToken
		api.GetUToken = origGetUToken
		logrus.StandardLogger().ExitFunc = nil
	}()

	username := "dumbledore"
	password := "345fgj78"

	var str1, str2 string

	askForLogin = func(lr loginReader) (string, string) {
		return username, password
	}
	api.CreateToken = func(username, password string) {
		str1 = username
		str2 = password
	}
	api.GetUToken = func() error {
		return nil
	}

	r := newTestReader(username, password, nil, nil)
	login(r)

	if fatal {
		t.Fatal("Function called Exit()")
	}
	if str1 != username {
		t.Errorf("Incorrect username. Expected %q, got %q", username, str1)
	}
	if str2 != password {
		t.Errorf("Incorrect password. Expected %q, got %q", password, str2)
	}
}

func TestLogin_State_Fatal(t *testing.T) {
	fatal := false
	logrus.StandardLogger().ExitFunc = func(int) { fatal = true }

	origAskForLogin := askForLogin
	origCreateToken := api.CreateToken
	origGetUToken := api.GetUToken

	defer func() {
		askForLogin = origAskForLogin
		api.CreateToken = origCreateToken
		api.GetUToken = origGetUToken
		logrus.StandardLogger().ExitFunc = nil
	}()

	username := "dumbledore"
	password := "345fgj78"

	askForLogin = func(lr loginReader) (string, string) {
		return username, password
	}
	api.CreateToken = func(username, password string) {
	}
	api.GetUToken = func() error {
		return nil
	}

	r := newTestReader(username, password, nil, errors.New("Error occurred"))
	login(r)

	if !fatal {
		t.Fatal("Function should have called Exit()")
	}
}

func TestLogin_Auth_401(t *testing.T) {
	fatal := false
	logrus.StandardLogger().ExitFunc = func(int) { fatal = true }

	origAskForLogin := askForLogin
	origCreateToken := api.CreateToken
	origGetUToken := api.GetUToken

	defer func() {
		askForLogin = origAskForLogin
		api.CreateToken = origCreateToken
		api.GetUToken = origGetUToken
		logrus.StandardLogger().ExitFunc = nil
	}()

	usernames := []string{"Smith", "Doris"}
	passwords := []string{"hwd82bkwe", "pwd"}
	count := 0

	var str1, str2 string

	askForLogin = func(lr loginReader) (string, string) {
		return usernames[count], passwords[count]
	}
	api.CreateToken = func(username, password string) {
		str1 = usernames[count]
		str2 = passwords[count]
	}
	api.GetUToken = func() error {
		if count == 0 {
			count++
			return &api.RequestError{StatusCode: 401}
		}
		return nil
	}

	r := newTestReader("", "", nil, nil)
	login(r)

	if fatal {
		t.Fatal("Function called Exit()")
	}
	if str1 != usernames[1] {
		t.Errorf("Username incorrect. Expected %q, got %q", usernames[1], str1)
	}
	if str2 != passwords[1] {
		t.Errorf("Passwords incorrect. Expected %q, got %q", passwords[1], str2)
	}
}

// This may have to be modified for windows
func TestCheckMountPoint(t *testing.T) {
	var tests = []struct {
		testname, name string
		dir            bool
		mode           int
	}{
		{
			"OK", "dir", true, 0755,
		},
		{
			"NOT_EXIST", "dir", true, -1,
		},
		{
			"NOT_DIR", "file", false, 0755,
		},
		{
			"NO_READ_PERM", "folder", true, 0333,
		},
		{
			"NO_WRITE_PERM", "folder", true, 0555,
		},
		{
			"NOT_EMPTY", "dir", true, 0,
		},
	}

	for _, tt := range tests {
		testname := tt.testname
		t.Run(testname, func(t *testing.T) {
			var node string
			var err error
			if tt.dir {
				node, err = ioutil.TempDir("", tt.name)

				if tt.mode == 0 {
					_, err := ioutil.TempFile(node, "file")
					if err != nil {
						t.Fatalf("Failed to create file %q", node+"/file")
					}
				} else {
					err = os.Chmod(node, os.FileMode(tt.mode))
				}
			} else {
				var file *os.File
				file, err = ioutil.TempFile("", tt.name)
				node = file.Name()
			}

			if err != nil {
				t.Fatalf("Creation of file/folder caused an error: %s", err.Error())
			}

			if tt.mode == -1 {
				os.RemoveAll(node)
			}

			fatal := false
			logrus.StandardLogger().ExitFunc = func(int) { fatal = true }

			mount = node
			checkMountPoint()

			if tt.testname == "OK" {
				if fatal {
					t.Error("Function called Exit()")
				}
			} else {
				if tt.mode == -1 {
					if _, err := os.Stat(node); os.IsNotExist(err) {
						t.Errorf("Directory was not created")
					}
				} else if !fatal {
					t.Error("Function should have called Exit()")
				}
			}

			logrus.StandardLogger().ExitFunc = nil
			os.RemoveAll(node)
		})
	}
}
