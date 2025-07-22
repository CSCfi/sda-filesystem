package main

import (
	"flag"
	"fmt"
	"os"
	"sda-filesystem/internal/airlock"
	"sda-filesystem/internal/logs"
	"slices"
)

var bucket, filename string
var override bool

func init() {
	handlers["export"] = handlerFuncs{setup: exportSetup, execute: exportHandler}
}

func flagSortFunc(arg1, arg2 string) int {
	if arg1[0] == '-' && arg2[0] == '-' {
		return 0
	}
	if arg1[0] != '-' && arg2[0] != '-' {
		return 0
	}
	if arg1[0] == '-' {
		return -1
	}

	return 1
}

func exportSetup(args []string) (int, error) {
	set := flag.NewFlagSet("export", flag.ContinueOnError)
	set.BoolVar(&override, "override", false, "Forcibly override data in SD Connect")

	set.Usage = func() {
		fmt.Println("Usage of export:")

		set.PrintDefaults()

		fmt.Println("Examples:")
		fmt.Println(" ", os.Args[0], "export testbucket path/to/file")
		fmt.Println(" ", os.Args[0], "export -override testbucket/subfolder path/to/another/file")
	}

	// We want the non-flag arguments to be first
	slices.SortStableFunc(args, flagSortFunc)
	if err := set.Parse(args); err != nil {
		return 2, nil
	}

	args = slices.DeleteFunc(args, func(arg string) bool {
		return arg[0] == '-'
	})

	if len(args) != 2 {
		set.Usage()

		return 2, nil
	}

	bucket = args[0]
	filename = args[1]

	if ok := airlock.ExportPossible(); !ok {
		return 0, fmt.Errorf("you are not allowed to export files")
	}

	return 0, nil
}

func exportHandler() (int, error) {
	if !override {
		if err := airlock.CheckObjectExistence(filename, bucket, os.Stdin); err != nil {
			return 0, err
		}
	}

	if err := airlock.Upload(filename, bucket); err != nil {
		return 0, err
	}

	logs.Info("Upload(s) complete")

	return 0, nil
}
