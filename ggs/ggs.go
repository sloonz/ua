package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/google/go-jsonnet"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Command struct {
	Delay   int
	Timeout int
	Command string
	Title   string
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
           --arg title "${title:-}" \
           '. + [{Timeout: ($timeout|tonumber), Delay: ($delay|tonumber), Command: $cmd, Title: $title}]')
    timeout=
    title=
}

. %s

echo "$commands" | jq --arg workers "$workers" '{Workers: ($workers|tonumber), Commands: .}'
`

type loggerWriter struct {
	log   *log.Logger
	cmd   *exec.Cmd
	buf   []byte
	title string
}

func (w *loggerWriter) Write(data []byte) (int, error) {
	sz := len(data)
	data = append(w.buf, data...)
	lines := bytes.Split(data, []byte("\n"))
	if len(lines[len(lines)-1]) == 0 {
		w.buf = nil
	} else {
		w.buf = lines[len(lines)-1]
	}
	lines = lines[:len(lines)-1]
	for _, line := range lines {
		w.log.Printf("%s %s", formatLabel(w.cmd.Process.Pid, w.title), string(line))
	}
	return sz, nil
}

func (w *loggerWriter) Close() {
	if w.buf != nil {
		w.log.Printf("%s %s", formatLabel(w.cmd.Process.Pid, w.title), string(w.buf))
		w.buf = nil
	}
}

func formatLabel(pid int, title string) string {
	if title == "" {
		return fmt.Sprintf("[%d]", pid)
	}
	return fmt.Sprintf("[%d %s]", pid, title)
}

func readConfig(cfgFile string) (cfg *Config, err error) {
	var out []byte
	if strings.HasSuffix(cfgFile, ".jsonnet") {
		vm := jsonnet.MakeVM()
		var jsonText string
		jsonText, err = vm.EvaluateFile(cfgFile)
		if err != nil {
			return nil, err
		}
		out = []byte(jsonText)
	} else {
		sp := exec.Command("sh")
		sp.Stderr = os.Stderr
		sp.Stdin = bytes.NewBuffer([]byte(fmt.Sprintf(CONFIG_WRAPPER, cfgFile)))
		out, err = sp.Output()
		if err != nil {
			return nil, err
		}
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

	sp := exec.Command("sh", "-c", cmd.Command)
	stdout := &loggerWriter{log: log.Default(), cmd: sp, title: cmd.Title}
	stderr := &loggerWriter{log: log.Default(), cmd: sp, title: cmd.Title}
	sp.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	sp.Stdout = stdout
	sp.Stderr = stderr

	if err = sp.Start(); err != nil {
		log.Printf("%s failed: %s", cmd.Command, err.Error())
		return
	}
	log.Printf("%s %s", formatLabel(sp.Process.Pid, cmd.Title), cmd.Command)

	if cmd.Timeout > 0 {
		timer = time.AfterFunc(time.Duration(cmd.Timeout)*time.Second, func() {
			if sp.ProcessState == nil {
				syscall.Kill(-sp.Process.Pid, syscall.SIGTERM)
			}
		})
	}

	err = sp.Wait()
	stdout.Close()
	stderr.Close()

	if err != nil {
		log.Printf("%s %s failed: %s", formatLabel(sp.Process.Pid, cmd.Title), cmd.Command, err.Error())
	} else {
		log.Printf("%s done", formatLabel(sp.Process.Pid, cmd.Title))
	}

	timer.Stop()
}

func reload(cfgFile string, oldConfig *Config, runOnce bool) (config *Config, err error) {
	// loopGroup is the number of (pending) writers on the command channel.
	// After disabling a configuration, we have to wait for it to fall to 0 before
	// closing the channel (otherwise, they will write to the closed channel).
	//
	// onceGroup is the number of unprocessed commands in the initial batch.
	var loopGroup, onceGroup sync.WaitGroup

	var closeChannel sync.Once

	config, err = readConfig(cfgFile)
	if err != nil {
		return nil, err
	}

	ch := make(chan *Command, len(config.Commands))

	for i := 0; i < config.Workers; i++ {
		go func() {
			for !config.disabled {
				var cmd *Command
				if cmd = <-ch; cmd == nil {
					continue
				}

				process(cmd)

				if runOnce {
					onceGroup.Done()
				} else {
					loopGroup.Add(1)
					time.AfterFunc(time.Duration(cmd.Delay)*time.Second, func() {
						if !config.disabled {
							ch <- cmd
						}
						loopGroup.Done()
					})
				}
			}

			loopGroup.Wait()
			closeChannel.Do(func() { close(ch) })
		}()
	}

	for _, cmd := range config.Commands {
		ch <- cmd
		if runOnce {
			onceGroup.Add(1)
		}
	}

	if runOnce {
		onceGroup.Wait()
		os.Exit(0)
	}

	if oldConfig != nil {
		oldConfig.disabled = true
	}

	return config, nil
}

func main() {
	var runOnce bool
	var cfgFile string

	flag.BoolVar(&runOnce, "once", false, "Process commands once, and then exit")
	flag.Parse()

	if cfgFile = flag.Arg(0); cfgFile == "" {
		jsonnetPath := os.ExpandEnv("$HOME/.config/ggs.jsonnet")
		if _, statErr := os.Stat(jsonnetPath); statErr == nil {
			cfgFile = jsonnetPath
		} else {
			cfgFile = os.ExpandEnv("$HOME/.config/ggsrc")
		}
	}

	config, err := reload(cfgFile, nil, runOnce)
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
			config, err = reload(cfgFile, config, runOnce)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error while reloading configuration: %s", err)
			}
		}
	}
}
