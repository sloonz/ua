# GGS

`GGS` (Grey Goo Spawner) is a simple software that runs jobs
periodically. It is similar with cron, but with some differences :

* Whereas `cron` launches jobs at specific times, `ggs` is mainly
interested in intervals. It will run all jobs at its startup and then
will re-run each job after a certain delay has passed.

* `ggs` has a system of `workers`, similar to many servers (like nginx
or Apache with MPM Workers) to limit ressource concurrency between your
jobs .

* You can define a timeout for your jobs, too.

## Usage

`ggs [configuration file]`

If no configuration file is provided, `ggs` will use `~/.config/ggsrc`
by default.

## Installation

`go build ggs.go && cp ggs /usr/local/bin`

## Configuration

Configuration file is a shell script, so same rule as `sh` applies.

You create a job with the `command` function, which takes two arguments:
the delay between launches, and the command to run. You can specify a
timeout (in seconds) by setting the `timeout` environnement variable
(optional, default: 0 no timeout).

	timeout=30 command 300 "uptime | mail admin@example.com"
	command 5 'ping -c 1 github.com || sudo halt -p'

You can also set the number of workers (maximum number of jobs that can
run simultaneously):

	workers=5 #Warning: dont do "workers = 5", spaces matters here !

## Advanced configuration

The configuration file is just a shell script which produces a JSON
document which maches the structure of the `Config` structure. You can do
`exec my_script` to produce the same JSON with a script in your favorite
language. You can also use variables, functions, execute external
commands, and so on...
