package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"

	"sda-filesystem/internal/airlock"
	"sda-filesystem/internal/api"
	"sda-filesystem/internal/logs"

	"golang.org/x/term"
)

func askOverwrite(filename string, message string) {
	if _, err := os.Stat(filename); err == nil {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print(message, " [y/N]?")

		response, err := reader.ReadString('\n')
		if err != nil {
			logs.Fatalf("Could not read response: %s", err.Error())
		}
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			logs.Info("Not overwriting. Exiting.")
			os.Exit(0)
		}
	}
}

func usage(self_path string) {
	fmt.Println("Usage:")
	fmt.Println(" ", self_path, "[-segment-size=size_in_mb] "+
		"[-journal-number=journal_number] [-original-file=unecrypted_filename] "+
		"[-password=password] [-force] [-quiet] "+
		"username container filename")
	fmt.Println("Examples:")
	fmt.Println(" ", self_path, "testuser testcontainer path/to/file")
	fmt.Println(" ", self_path, "-segment-size=100 testuser testcontainer path/to/file")
	fmt.Println(" ", self_path, "-segment-size=100 "+
		"-original-file=/path/to/original/unecrypted/file -journal-number=example124"+
		"testuser testcontainer path/to/file")
	fmt.Println(" ", self_path, "-password=my_very_secure_password -force "+
		"testuser testcontainer path/to/file")
}

func main() {
	segment_size_mb := flag.Int("segment-size", 4000,
		"Maximum size of segments in Mb used to upload data. Valid range is 10-4000.")
	journal_number := flag.String("journal-number", "",
		"Journal Number/Name specific for Findata uploads")
	original_filename := flag.String("original-file", "",
		"Filename of original unecrypted file when uploading pre-encrypted file from Findata vm")
	force := flag.Bool("force", false, "Do not prompt questions and overwrite encrypted file if it already exists")
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

	if err = airlock.GetPublicKey(); err != nil {
		logs.Fatal(err)
	}

	fmt.Println("Enter Password: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		logs.Fatalf("Could not read password: %s", err.Error())
	}
	password := string(bytePassword)

	api.SetBasicToken(username, password)

	var encrypted bool
	if encrypted, err = airlock.CheckEncryption(filename); err != nil {
		logs.Fatal(err)
	} else if encrypted {
		logs.Info("File ", filename, " is already encrypted. Skipping encryption.")
	} else {
		*original_filename = filename
		filename = filename + ".c4gh"
	}

	// Ask user confirmation if output file exists
	if !*force {
		askOverwrite(filename, "File "+filename+" exists. Overwrite file")
	}

	err = airlock.Upload(*original_filename, filename, container, *journal_number, uint64(*segment_size_mb), !encrypted)
	if err != nil {
		logs.Fatal(err)
	}
}
