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
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println(" ", self_path, "[-segment-size=size_in_mb] "+
		"[-journal-number=journal_number] [-original-file=unecrypted_filename] "+
		"[-password=password] [-force] |-quiet] "+
		"username container filename")
	fmt.Println("Examples:")
	fmt.Println(" ", self_path, "testuser testcontainer path/to/file")
	fmt.Println(" ", self_path, "-segment-size=100 testuser testcontainer path/to/file")
	fmt.Println(" ", self_path, "-segment-size=100 "+
		"-original-file=/path/to/original/unecrypted/file -journal-number=example124"+
		"testuser testcontainer path/to/file")
	fmt.Println(" ", self_path, "-password=my_very_secure_password -force "+
		"testuser testcontainer path/to/file")
	fmt.Println("")
	fmt.Println(" segment-size: Option to define in how large segments the upload occures. " +
		"Default size is 4000Mb. Valid range is 10-4000")
	fmt.Println(" journal-number: Journal number associated with findata uploads.")
	fmt.Println(" original-file: When uploading pre-encrypted file from findata vm, use " +
		"this option to point to the original un-encrypted file.")
	fmt.Println(" password: Option to provide password on command line. Do not prompt to " +
		"ask password.")
	fmt.Println(" force: Do not prompt to ask if encrypted file exists. Overwrite file if " +
		"it exists.")
	fmt.Println(" quiet: Print errors only")
	fmt.Println("")
}

func main() {
	if len(os.Args) == 1 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		usage(os.Args[0])
		os.Exit(0)
	}

	segment_size_mb := flag.Int("segment-size", 4000, "Maximum segments size in Mb")
	journal_number := flag.String("journal-number", "",
		"Journal Number/Name specific for Findata")
	original_filename := flag.String("original-file", "",
		"Filename of original unecrypted file")
	force := flag.Bool("force", false, "Do not prompt questions")
	quiet := flag.Bool("quiet", false, "Print only errors")
	debug := flag.Bool("debug", false, "Enable debug prints")

	flag.Parse()

	if *segment_size_mb < 10 || *segment_size_mb > 4000 {
		logs.Fatal("Valid values for segment size are 10-4000")
	}

	username := flag.Arg(0)
	container := flag.Arg(1)
	filename := flag.Arg(2)

	if *debug {
		logs.SetLevel("debug")
	} else if *quiet {
		logs.SetLevel("error")
	}

	if username == "" || container == "" || filename == "" {
		logs.Info("Username, container or filename not set!")
		usage(os.Args[0])
		os.Exit(2)
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

	fmt.Println("Enter Password: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		logs.Fatalf("Could not read password: %s", err.Error())
	}
	password := string(bytePassword)

	api.SetBasicToken(username, password)
	if err = airlock.GetPublicKey(); err != nil {
		logs.Fatal(err)
	}

	logs.Info("\n### UPLOAD ###")
	err = airlock.Upload(*original_filename, filename, container, *journal_number, uint64(*segment_size_mb), *force)
	if err != nil {
		logs.Fatal(err)
	}
}
