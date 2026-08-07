package main

import (
	"flag"
	"os"
	"os/signal"
	"path/filepath"

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
	var sdapplyOnly bool
	set := flag.NewFlagSet("import", flag.ContinueOnError)
	set.StringVar(&mount, "mount", "", "Path to Data Gateway mount point")
	set.BoolVar(&sdapplyOnly, "sdapply", false, "Connect only to SD Apply")

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

	if !sdapplyOnly && !api.SDConnectEnabled() {
		logs.Warningf("You do not have SD Connect enabled")
	}

	if sdapplyOnly {
		api.SetRepositories(map[string]bool{api.SDApply.ForPath(): true})
	}

	return 0, nil
}

func importHandler() (int, error) {
	var wait = make(chan any)
	go func() {
		<-wait // Wait for fuse to be ready
		filesystem.WaitForUpdateSignal(nil)
	}()

	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt)
	go func() {
		for range s {
			logs.Info("Shutting down Data Gateway")
			if err := filesystem.UnmountFilesystem(); err != nil {
				logs.Error(err)
			}
		}
	}()

	return filesystem.MountFilesystem(mount, nil, wait), nil
}
