package main

import (
	"flag"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/billziss-gh/cgofuse/fuse"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"

	"sd-connect-fuse/internal/api"
	"sd-connect-fuse/internal/filesystem"
)

// dirName is name of the directory where the projects are stored
const dirName = "Projects"

var mount string

// mountPoint constructs a path to the user's home directory for mounting FUSE
func mountPoint() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Could not find user home directory", err)
	}
	p := filepath.FromSlash(filepath.ToSlash(home) + "/" + dirName)
	return p
}

func setLogger(inputLevel string) {
	// Configure Log Text Formatter
	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})

	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)

	// Only log given severity or above
	var m = map[string]logrus.Level{
		"debug": logrus.DebugLevel,
		"info":  logrus.InfoLevel,
		"error": logrus.ErrorLevel,
	}

	if logrusLevel, ok := m[strings.ToLower(inputLevel)]; ok {
		log.SetLevel(logrusLevel)
		return
	}

	log.Infof("-loglevel=%s is not supported, possible values are {debug,info,error}, setting fallback loglevel to 'info'", inputLevel)
	log.SetLevel(logrus.InfoLevel)
}

func init() {
	var logLevel string
	flag.StringVar(&mount, "mount", mountPoint(), "Path to FUSE mount point")
	flag.StringVar(&logLevel, "loglevel", "info", "Logging level. Possible value: {debug,info,error}")
	flag.IntVar(&api.RequestTimeout, "http_timeout", 10, "Number of seconds to wait before timing out an HTTP request")
	flag.Parse()

	setLogger(logLevel)

	// Verify mount point directory
	if _, err := os.Stat(mount); os.IsNotExist(err) {
		// In other OSs except Windows, the mount point must exist and be empty
		if runtime.GOOS != "windows" {
			log.Debugf("Mount point %s does not exist, so it will be created", mount)
			if err = os.Mkdir(mount, 0777); err != nil {
				log.Fatalf("Could not create directory %s", mount)
			}
		}
	} else {
		// Mount directory must not already exist in Windows
		if runtime.GOOS == "windows" {
			log.Fatalf("Mount point %s already exists, remove the directory or use another mount point", mount)
		}
		// Check that the mount point is empty if it already exists
		dir, err := os.Open(mount)
		if err != nil {
			log.Fatalf("Could not open mount point %s", mount)
		}
		defer dir.Close()
		// Verify dir is empty
		if _, err = dir.Readdir(1); err != io.EOF {
			log.Fatalf("Mount point %s must be empty", mount)
		}
	}

	log.Debugf("Filesystem will be mounted at %s", mount)
}

func shutdown() <-chan bool {
	done := make(chan bool)
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-s
		log.Info("Shutting down SD-Connect FUSE")
		done <- true
	}()
	return done
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	done := shutdown()

	api.CreateToken()
	//fmt.Println(runtime.NumGoroutine(), runtime.GOMAXPROCS(-1))
	api.InitializeClient()
	//fmt.Println(runtime.NumGoroutine())
	connectfs := filesystem.CreateFileSystem()
	//fmt.Println(runtime.NumGoroutine())
	host := fuse.NewFileSystemHost(connectfs)
	options := []string{}
	if runtime.GOOS == "darwin" {
		options = append(options, "-o", "defer_permissions")
		options = append(options, "-o", "volname="+dirName)
	}
	host.Mount(mount, options)

	<-done
}
