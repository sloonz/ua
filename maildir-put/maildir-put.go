package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"
	"strings"

	"github.com/sloonz/go-maildir"
	"github.com/sloonz/ua/uamessage"
)

func main() {
	var rootDir, folder string
	var err error

	flag.StringVar(&rootDir, "root", os.ExpandEnv("$HOME/Maildir"), "path to maildir")
	flag.StringVar(&folder, "folder", "", "maildir folder name to put email (empty for inbox)")

	if flag.Parse(); !flag.Parsed() {
		flag.PrintDefaults()
		os.Exit(1)
	}

	md, err := maildir.New(rootDir, true)
	if err != nil {
		log.Fatalf("Can't open maildir: %s", err.Error())
	}

	for _, subfolder := range strings.Split(folder, "/") {
		if subfolder != "" {
			md, err = md.Child(subfolder, true)
			if err != nil {
				log.Fatalf("Can't open maildir: %s", err.Error())
			}
		}
	}

	dec := json.NewDecoder(os.Stdin)
	for {
		msg := new(uamessage.Message)
		err = dec.Decode(msg)
		if err == nil {
			mailMsg, _, buildErr := uamessage.Build(msg, uamessage.BuildOptions{EOL: "\n"})
			if buildErr != nil {
				err = buildErr
			} else {
				md.CreateMail(mailMsg)
			}
		}

		if err == io.EOF {
			break
		} else if err != nil {
			log.Printf("Cannot read input message: %s", err.Error())
		}
	}
}
