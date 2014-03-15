package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

type Command struct {
	Delay   int
	Timeout int
	Command string
}

type Config struct {
	Workers  int
	Commands []*Command
}

var config Config
var ch chan *Command

const CONFIG_WRAPPER = `
workers=5
default_timeout=0

commands=

jsonString() {
	perl -pe 's/\\/\\\\/g;s/"/\\"/g'
}

command() {
	if [ "$commands" != "" ] ; then
		commands="$commands,"
	fi
	delay=$1;shift
	timeout=${timeout:-$default_timeout}
	commands=$commands'{"Delay":'$delay',"Command":"'$(echo $@|jsonString)'","Timeout":'$timeout'}'
	timeout=
}

source %s

echo '{"Workers":'$workers',"Commands":['$commands']}'
`

func readConfig() error {
	var cfgFile string

	if len(os.Args) > 1 {
		cfgFile = os.Args[1]
	} else {
		cfgFile = os.ExpandEnv("$HOME/.config/ggsrc")
	}

	sp := exec.Command("sh")
	sp.Stderr = os.Stderr
	sp.Stdin = bytes.NewBuffer([]byte(fmt.Sprintf(CONFIG_WRAPPER, cfgFile)))
	out, err := sp.Output()
	if err != nil {
		return err
	}

	err = json.Unmarshal(out, &config)
	if err != nil {
		return err
	}

	return nil
}

func process(cmd *Command) {
	var timer *time.Timer
	var err error

	log.Print(cmd.Command)
	sp := exec.Command("sh", "-c", cmd.Command)
	sp.Stdout = os.Stdout
	sp.Stderr = os.Stderr
	if err = sp.Start(); err != nil {
		log.Printf("%s failed: %s", err.Error(), cmd.Command)
		goto scheduleNextLaunch
	}
	if cmd.Timeout > 0 {
		timer = time.AfterFunc(time.Duration(cmd.Timeout)*time.Second, func() {
			timer = nil
			if sp.ProcessState == nil {
				sp.Process.Kill()
			}
		})
	}
	err = sp.Wait()
	if timer != nil {
		timer.Stop()
	}
	if err != nil {
		log.Printf("%s failed: %s", err.Error(), cmd.Command)
	}

scheduleNextLaunch:
	time.AfterFunc(time.Duration(cmd.Delay)*time.Second, func() {
		ch <- cmd
	})
}

func worker() {
	for {
		process(<-ch)
	}
}

func main() {
	err := readConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while reading configuration: %s", err)
		os.Exit(1)
	}

	ch = make(chan *Command, len(config.Commands))

	for i := 0; i < config.Workers; i++ {
		go worker()
	}

	for _, cmd := range config.Commands {
		ch <- cmd
	}

	// wait for SIGINT
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGINT)
	<-sigChan
}
