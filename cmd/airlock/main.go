package main

import (
	"flag"
	"fmt"
	"os"
	"syscall"

	"sda-filesystem/internal/airlock"
	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"

	"golang.org/x/term"
)

func usage(self_path string) {
	fmt.Println("Password is read from environment variable AIRLOCK_PASSWORD")
	fmt.Println("If this variable is empty airlock requests the password interactively")
	fmt.Println("Usage:")
	fmt.Println(" ", self_path, "[-segment-size=size_in_mb] "+
		"[-journal-number=journal_number] [-original-file=unecrypted_filename] "+
		"[-quiet] "+"username container filename")
	fmt.Println("Examples:")
	fmt.Println(" ", self_path, "testuser testcontainer path/to/file")
	fmt.Println(" ", self_path, "-segment-size=100 testuser testcontainer path/to/file")
	fmt.Println(" ", self_path, "-segment-size=100 "+
		"-original-file=/path/to/original/unecrypted/file -journal-number=example124"+
		"testuser testcontainer path/to/file")
}

func main() {
	segment_size_mb := flag.Int("segment-size", 4000,
		"Maximum size of segments in Mb used to upload data. Valid range is 10-4000.")
	journal_number := flag.String("journal-number", "",
		"Journal Number/Name specific for Findata uploads")
	original_filename := flag.String("original-file", "",
		"Filename of original unecrypted file when uploading pre-encrypted file from Findata vm")
	quiet := flag.Bool("quiet", false, "Print only errors")
	debug := flag.Bool("debug", false, "Enable debug prints")

	flag.Parse()

	if flag.NArg() != 3 {
		usage(os.Args[0])
		os.Exit(2)
	}

	username := flag.Arg(0)
	container := flag.Arg(1)
	filename := flag.Arg(2)

	if *segment_size_mb < 10 || *segment_size_mb > 4000 {
		logs.Fatal("Valid values for segment size are 10-4000")
	}

	if *debug {
		logs.SetLevel("debug")
	} else if *quiet {
		logs.SetLevel("error")
	}

	err := api.GetCommonEnvs()
	if err != nil {
		logs.Fatal(err)
	}

	err = api.InitializeClient()
	if err != nil {
		logs.Fatal(err)
	}

	if isManager, err := airlock.IsProjectManager(); err != nil {
		logs.Fatalf("Unable to determine project manager status: %s", err.Error())
	} else if !isManager {
		logs.Fatal("You cannot use Airlock as you are not the project manager")
	}

	if err = airlock.GetPublicKey(); err != nil {
		logs.Fatal(err)
	}

	password, password_from_env := os.LookupEnv("AIRLOCK_PASSWORD")

	if password_from_env {
		logs.Info("Using password from environment variable AIRLOCK_PASSWORD")
	} else {
		fmt.Println("Enter Password: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			logs.Fatalf("Could not read password: %s", err.Error())
		}
		password = string(bytePassword)
	}

	api.SetBasicToken(username, password)

	var encrypted bool
	if encrypted, err = airlock.CheckEncryption(filename); err != nil {
		logs.Fatal(err)
	} else if !encrypted {
		*original_filename = filename
		filename += ".c4gh"
	}

	err = airlock.Upload(*original_filename, filename, container, *journal_number, uint64(*segment_size_mb), !encrypted)
	if err != nil {
		logs.Fatal(err)
	}
}
