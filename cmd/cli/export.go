package main

import (
	"errors"
	"flag"
	"fmt"
	"net/mail"
	"os"
	"sda-filesystem/internal/airlock"
	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"
	"slices"
)

var exportPrefix string
var selection []string

var override bool
var metadata map[string]string = make(map[string]string)

func init() {
	handlers["export"] = handlerFuncs{setup: exportSetup, execute: exportHandler}
}

// flagSortFunc sorts non-flag arguments (the ones with '-') first.
// This is so that users are able to give non-flag arguments after
// listing the bucket and files/folders.
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

// refineArgs ensures that any non-flag arguments (the ones with '-')
// given in form `--flag x` do not get separated during sorting.
func refineArgs(args []string, name string) []string {
	for {
		idx := slices.IndexFunc(args, func(a string) bool {
			return a == "-"+name || a == "--"+name
		})
		if idx != -1 {
			if idx < len(args)-1 {
				args[idx] = args[idx] + "=" + args[idx+1]
				args = append(args[:idx+1], args[idx+2:]...)
			} else {
				args[idx] = args[idx] + "="
			}
		} else {
			break
		}
	}

	return args
}

func exportSetup(args []string) (int, error) {
	aaiEmail := api.GetUserEmail()
	if aaiEmail != "" {
		logs.Debugf("Your default email is %s", aaiEmail)
	}

	var email, journalNumber string
	set := flag.NewFlagSet("export", flag.ContinueOnError)
	set.BoolVar(&override, "override", false, "Forcibly override data in SD Connect")
	set.StringVar(&email, "email", aaiEmail, "Your email (for Findata projects)")
	set.StringVar(&journalNumber, "journal-number", "", "Journal number (for Findata projects)")

	set.Usage = func() {
		fmt.Println("Usage of export:")

		set.PrintDefaults()

		fmt.Println("Examples:")
		fmt.Println(" ", os.Args[0], "export testbucket path/to/file/or/folder")
		fmt.Println(" ", os.Args[0], "export -override testbucket/subfolder path/to/file/or/folder path/to/another/file")
	}

	args = refineArgs(args, "email")
	args = refineArgs(args, "journal-number")

	// We want the non-flag arguments to be first
	slices.SortStableFunc(args, flagSortFunc)
	if err := set.Parse(args); err != nil {
		return 2, nil
	}

	args = slices.DeleteFunc(args, func(arg string) bool {
		return arg[0] == '-'
	})

	if len(args) < 2 {
		set.Usage()

		return 2, nil
	}

	exportPrefix = args[0]
	selection = args[1:]

	if ok := airlock.ExportPossible(); !ok {
		return 0, fmt.Errorf("you are not allowed to export files")
	}

	// Findata projects
	if api.GetProjectType() != "default" {
		e, err := mail.ParseAddress(email)
		if err != nil {
			return 2, fmt.Errorf("invalid email argument %q: %w", email, err)
		}
		email = e.Address

		if journalNumber == "" {
			return 2, errors.New("missing journal number argument")
		}

		metadata["journal_number"] = journalNumber
		metadata["author_email"] = email
	}

	return 0, nil
}

func exportHandler() (int, error) {
	set, err := airlock.WalkDirs(selection, nil, exportPrefix)
	if err != nil {
		return 0, fmt.Errorf("failed to select files for export: %w", err)
	}

	created, err := airlock.ValidateBucket(set.Bucket)
	if err != nil {
		return 0, fmt.Errorf("cannot use bucket %s: %w", set.Bucket, err)
	}

	if !created && !override {
		if err := airlock.CheckObjectExistences(&set, os.Stdin); err != nil {
			return 0, err
		}
	}

	if err := airlock.Upload(set, metadata); err != nil {
		return 0, err
	}

	logs.Info("Upload(s) complete")

	return 0, nil
}
