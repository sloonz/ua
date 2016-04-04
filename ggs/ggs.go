package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
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
	disabled bool
}

const CONFIG_WRAPPER = `
workers=5
default_timeout=0
commands=$(jq -n '[]')

command() {
    delay=$1; shift
    commands=$(echo "$commands" | \
        jq --arg delay "$delay" --arg cmd "$*" \
           --arg timeout "${timeout:-$default_timeout}" \
           '. + [{Timeout: ($timeout|tonumber), Delay: ($delay|tonumber), Command: $cmd}]')
    timeout=
}

. %s

echo "$commands" | jq --arg workers "$workers" '{Workers: ($workers|tonumber), Commands: .}'
`

func readConfig() (cfg *Config, err error) {
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
		return nil, err
	}

	cfg = new(Config)
	err = json.Unmarshal(out, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
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
		return
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
}

func reload(oldConfig *Config) (config *Config, err error) {
	var wg sync.WaitGroup
	var once sync.Once

	config, err = readConfig()
	if err != nil {
		return nil, err
	}

	ch := make(chan *Command, len(config.Commands))

	for i := 0; i < config.Workers; i++ {
		go func() {
			wg.Add(1)

			for !config.disabled {
				cmd := <-ch
				process(cmd)
				wg.Add(1)
				time.AfterFunc(time.Duration(cmd.Delay)*time.Second, func() {
					ch <- cmd
					wg.Done()
				})
			}

			wg.Done()
			wg.Wait()
			once.Do(func() { close(ch) })
		}()
	}

	for _, cmd := range config.Commands {
		ch <- cmd
	}

	if oldConfig != nil {
		oldConfig.disabled = true
	}

	return config, nil
}

func main() {
	config, err := reload(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while reading configuration: %s", err)
		os.Exit(1)
	}

	// wait for signals (interrupt, reload)
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGUSR1)
	for sig := range sigChan {
		switch sig {
		case syscall.SIGINT:
			return
		case syscall.SIGUSR1:
			config, err = reload(config)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error while reloading configuration: %s", err)
			}
		}
	}
}
