package main

import (
	"flag"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/billziss-gh/cgofuse/fuse"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"

	"github.com/cscfi/sd-connect-fuse/internal/api"
	"github.com/cscfi/sd-connect-fuse/internal/filesystem"
)

const dirName = "Projects"

var mount string
var daemonTimeout int

// mountPoint constructs a path to the user's home directory for mounting FUSE
func mountPoint() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Could not find user home directory", err)
	}
	p := filepath.FromSlash(filepath.ToSlash(home) + "/" + dirName)
	return p
}

func verifyURL(apiURL, name string) {
	// Verify repository URL is set
	if apiURL == "" {
		log.Fatalf("%s must be set with command line argument -%s=address", name, strings.ToLower(name))
	}
	// Verify that repository URL is valid
	if _, err := url.ParseRequestURI(apiURL); err != nil {
		log.Error(err)
		log.Fatalf("%s is not valid", name)
	}
}

func setLogger(inputLevel string) {
	// Configure Log Text Formatter
	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})

	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
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
}

func init() {
	var logLevel string
	//??? flag.StringVar(&api.Token, "token", os.Getenv("TOKEN"), "Authorization token, read from ENV $TOKEN by default")
	flag.StringVar(&mount, "mount", mountPoint(), "Path to FUSE mount point")
	flag.StringVar(&api.DataURL, "dataurl", "", "URL to sd-connect data API repository")
	flag.StringVar(&api.MetadataURL, "metadataurl", "", "URL to sd-connect metadata API repository")
	flag.StringVar(&api.Certificate, "certificate", "", "TLS certificates for repositories")
	flag.StringVar(&logLevel, "loglevel", "info", "Logging level. Possible value: {debug,info,error}")
	flag.IntVar(&api.RequestTimeout, "http_timeout", 3000, "Number of seconds to wait before timing out an HTTP request")
	flag.IntVar(&daemonTimeout, "daemon_timeout", 3000, "Number of seconds during which fuse has to answer kernel")
	flag.Parse()

	setLogger(logLevel)

	verifyURL(api.DataURL, "DataURL")
	verifyURL(api.MetadataURL, "MetadataURL")

	// Verify mount point directory
	if _, err := os.Stat(mount); os.IsNotExist(err) {
		// In other OSs except Windows, the mount point must exist and be empty
		if runtime.GOOS != "windows" {
			log.Debugf("Mount point %s does not exist, so it will be created", mount)
			os.Mkdir(mount, 0777)
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

	log.Debug("Filesystem will be mounted at " + mount)
}

func shutdown() <-chan bool {
	done := make(chan bool)
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	go func() {
		<-s
		log.Info("Shutting down SDA FUSE client")
		// additional clean up procedures can be done here if necessary
		done <- true
	}()
	return done
}

func main() {
	done := shutdown()

	api.CreateToken()
	connectfs := filesystem.CreateFileSystem()
	host := fuse.NewFileSystemHost(connectfs)
	options := []string{}
	if runtime.GOOS == "darwin" {
		options = append(options, "-o", "defer_permissions")
		options = append(options, "-o", "volname="+dirName)
		options = append(options, "-o", "daemon_timeout="+strconv.Itoa(daemonTimeout))
	}
	host.Mount(mount, options)

	<-done
}
