package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"sda-filesystem/internal/api"
	"sda-filesystem/internal/filesystem"
	"sda-filesystem/internal/logs"
	"sda-filesystem/internal/mountpoint"
)

var mount string

func init() {
	handlers["import"] = handlerFuncs{setup: importSetup, execute: importHandler}
}

func importSetup(args []string) (int, error) {
	// var sdapplyOnly bool
	set := flag.NewFlagSet("import", flag.ContinueOnError)
	set.StringVar(&mount, "mount", "", "Path to Data Gateway mount point")
	// set.BoolVar(&sdapplyOnly, "sdapply", false, "Connect only to SD Apply")

	if err := set.Parse(args); err != nil {
		return 2, nil
	}

	if mount == "" {
		defaultMount, err := mountpoint.DefaultMountPoint()
		if err != nil {
			return 0, err
		}
		mount = defaultMount
	} else if err := mountpoint.CheckMountPoint(mount); err != nil {
		return 0, err
	}

	mount = filepath.Clean(mount)

	// if !sdapplyOnly {
	if !api.SDConnectEnabled() {
		// logs.Warningf("You do not have SD Connect enabled")
		return 0, fmt.Errorf("you do not have %s enabled", api.SDConnect)
	}
	// }

	return 0, nil
}

// userInput reads user's input from io.Reader and sends it into a channel
func userInput(r io.Reader, ch chan<- []string) {
	scanner := bufio.NewScanner(r)
	var answer string

	for {
		if scanner.Scan() {
			answer = scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			logs.Errorf("Could not read input: %w", err)
		}
		ch <- strings.Fields(answer)
	}
}

func applyCommand(ch <-chan []string) {
	for {
		input := <-ch
		if len(input) == 0 {
			continue
		}
		switch strings.ToLower(input[0]) {
		case "update":
			if filesystem.FilesOpen() {
				logs.Errorf("You have files in use which prevents updating Data Gateway")
			} else {
				filesystem.RefreshFilesystem()
			}
		case "clear":
			if len(input) > 1 {
				path := filepath.Clean(input[1])
				if err := filesystem.ClearPath(path); err != nil {
					logs.Error(err)
				}
			} else {
				logs.Errorf("Cannot clear cache without path")
			}
		}
	}
}

func waitForUpdateSignal(ch chan<- []string) {
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGUSR2)
	for {
		<-s
		ch <- []string{"update"}
	}
}

func importHandler() (int, error) {
	var wait = make(chan any)
	var cmd = make(chan []string)
	go func() {
		<-wait // Wait for fuse to be ready
		go waitForUpdateSignal(cmd)
		go userInput(os.Stdin, cmd)
		go applyCommand(cmd)
	}()

	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt)
	go func() {
		for range s {
			logs.Info("Shutting down Data Gateway")
			filesystem.UnmountFilesystem()
		}
	}()

	return filesystem.MountFilesystem(mount, nil, wait), nil
}
