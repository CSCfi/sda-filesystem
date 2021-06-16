package main

import (
	"flag"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/billziss-gh/cgofuse/fuse"
	log "github.com/sirupsen/logrus"

	"github.com/cscfi/sd-connect-fuse/internal/filesystem"
)

var mount string

// mountPoint constructs a path to the user's home directory for mounting FUSE
func mountPoint() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("could not find user home directory", err)
	}
	p := filepath.FromSlash(filepath.ToSlash(home) + "/Datasets")
	return p
}

func init() {
	// Configure Log Text Formatter
	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})

	log.SetLevel(log.DebugLevel)

	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)
}

func init() {
	flag.StringVar(&mount, "mount", mountPoint(), "Path to FUSE mount point")
	flag.Parse()

	// Verify mount point directory
	if _, err := os.Stat(mount); os.IsNotExist(err) {
		// In other OSs except Windows, the mount point must exist and be empty
		if runtime.GOOS != "windows" {
			log.Debugf("Mount point %v does not exist, so it will be created", mount)
			os.Mkdir(mount, 0777)
		}
	} else {
		// Mount directory must not already exist in Windows
		if runtime.GOOS == "windows" {
			log.Fatalf("Mount point %v already exists, remove the directory or use another mount point", mount)
		}
		// Check that the mount point is empty if it already exists
		dir, err := os.Open(mount)
		if err != nil {
			log.Fatalf("Could not open mount point %v", mount)
		}
		defer dir.Close()
		// Verify dir is empty
		if _, err = dir.Readdir(1); err != io.EOF {
			log.Fatalf("Mount point %v must be empty", mount)
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

	connectfs := filesystem.CreateFileSystem()
	host := fuse.NewFileSystemHost(connectfs)
	host.Mount(mount, []string{})

	<-done
}
