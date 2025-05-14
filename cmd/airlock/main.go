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

func usage(selfPath string) {
	fmt.Println("Usage:")
	fmt.Println(" ", selfPath, "[-quiet] [-debug] [-override] bucket filename")
	fmt.Println("Examples:")
	fmt.Println(" ", selfPath, "testbucket path/to/file")
	fmt.Println(" ", selfPath, "testbucket/subfolder path/to/another/file")
}

func readPassword() (string, error) {
	password, passwordFromEnv := os.LookupEnv("CSC_PASSWORD")

	if passwordFromEnv {
		logs.Info("Using password from environment variable CSC_PASSWORD")
	} else {
		fmt.Println("Enter Password: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return "", fmt.Errorf("could not read password: %s", err.Error())
		}
		password = string(bytePassword)
	}

	return password, nil
}

func main() {
	quiet := flag.Bool("quiet", false, "Print only errors")
	debug := flag.Bool("debug", false, "Enable debug prints")
	override := flag.Bool("override", false, "Forcibly override data in SD Connect")

	flag.Parse()

	if flag.NArg() != 2 {
		usage(os.Args[0])
		os.Exit(2)
	}

	bucket := flag.Arg(0)
	filename := flag.Arg(1)

	if *debug {
		logs.SetLevel("debug")
	} else if *quiet {
		logs.SetLevel("error")
	}

	if err := api.Setup(); err != nil {
		logs.Fatal(err)
	}

	access, err := api.GetProfile()
	if err != nil {
		logs.Fatal(err)
	}
	if ok := airlock.ExportPossible(); !ok {
		logs.Fatal("You are not allowed to use Airlock")
	}
	if !access {
		logs.Info("Your session has expired")
		password, err := readPassword()
		if err != nil {
			logs.Fatal(err)
		}
		if err := api.Authenticate(password); err != nil {
			logs.Fatal(err)
		}
	}

	if err := airlock.Upload(filename, bucket, *override); err != nil {
		logs.Fatal(err)
	}
}
