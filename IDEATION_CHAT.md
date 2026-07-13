On macOS, `top` displays **Load Avg** as three numbers representing the **system load average over the last 1, 5, and 15 minutes**.

For example:

```text
Load Avg: 2.14, 1.87, 1.42
```

means:

* **2.14** = average load during the last **1 minute**
* **1.87** = average load during the last **5 minutes**
* **1.42** = average load during the last **15 minutes**

### What is "load"?

Load average is **not CPU utilization**. It is the average number of processes that are:

* running on the CPU, or
* ready to run (waiting for CPU time), and on macOS/BSD may also include processes waiting in certain uninterruptible kernel states (such as some I/O waits).

Think of it as:

> **How many processes want CPU (or are blocked in kernel execution) at any given time?**

### How to interpret it

The meaning depends on how many CPU cores your machine has.

For example:

| CPU Cores | Load Avg | Interpretation                   |
| --------- | -------- | -------------------------------- |
| 4         | 1.0      | CPU mostly idle                  |
| 4         | 4.0      | CPUs fully utilized              |
| 4         | 8.0      | About 4 tasks running, 4 waiting |
| 10        | 8.0      | Plenty of CPU capacity remaining |
| 10        | 15.0     | CPU oversubscribed               |

For an Apple Silicon Mac:

* M1 Pro: 8 or 10 cores
* M2 Max: 12 cores
* M3 Max: up to 16 cores
* M4 Max: even more

A load of **8** on a 16-core machine is quite normal.

### Trend interpretation

Suppose you see:

```text
Load Avg: 12.3, 8.5, 4.1
```

This means load is **increasing**:

* 15 minutes ago: ~4
* 5 minutes ago: ~8.5
* now: ~12.3

Conversely:

```text
Load Avg: 2.1, 5.8, 9.3
```

means the machine was busy but is calming down.

### Comparing with CPU usage

These are related but different:

```
Load Avg: 10.5, 9.8, 8.7

CPU usage:
 75% user
 15% sys
 10% idle
```

Here, the CPU is busy but not saturated, and there are roughly 10 runnable or blocked processes on average.

### Useful companion commands

```bash
sysctl -n hw.ncpu
```

Shows the number of logical CPU cores.

```bash
top -l 1
```

Takes a single snapshot.

```bash
uptime
```

Displays just the load averages:

```text
04:21  up 7 days, 18:34, 2 users, load averages: 3.18 2.87 2.42
```

### Rule of thumb

* **Load < number of CPU cores:** generally healthy.
* **Load ≈ number of CPU cores:** CPUs are fully occupied.
* **Load > number of CPU cores:** some work is waiting for CPU time (or blocked in kernel execution), so the system may begin to feel sluggish if the condition persists.

On modern Apple Silicon systems with many cores, load averages that would have been considered "high" on older dual-core or quad-core Macs are often perfectly normal.

===

If you suddenly see many **`rg`** processes on macOS, they are almost certainly instances of **`ripgrep`**, a very fast recursive text-search tool. Seeing many at once usually means some application or agent is indexing or searching your filesystem.

Common causes include:

### 1. AI coding assistants (most likely)

Modern coding agents frequently invoke `rg` because it's much faster than walking directories manually.

Examples include:

* Claude Code
* Codex CLI
* Gemini CLI
* Cursor
* Windsurf
* Aider
* Continue.dev
* Many MCP servers

Typical searches include:

* Finding symbols
* Searching for TODOs
* Reading repository structure
* Locating files matching patterns
* Finding references before editing

If you're running multiple agents or subagents simultaneously, you may legitimately see dozens of concurrent `rg` processes.

---

### 2. VS Code or another IDE

VS Code uses `ripgrep` extensively for:

* Global search (`⌘⇧F`)
* "Go to Symbol"
* "Find in Files"
* Extensions such as GitHub Copilot
* Language servers

If you open a large monorepo, VS Code may launch several `rg` processes.

---

### 3. Indexing a large repository

Repositories with:

* millions of files
* large `node_modules`
* generated code
* vendor directories
* logs

can keep `rg` busy for quite a while.

---

### 4. Build or automation scripts

Many shell scripts use `rg` instead of `grep`.

Examples:

```bash
rg TODO
rg "type User"
rg "package main"
```

---

## Why so many?

`ripgrep` itself is multithreaded, but applications often launch **multiple independent searches**.

For example an AI agent might simultaneously ask:

* find all Go files
* locate ROADMAP.md
* search for TODO
* find OpenAPI specs
* locate package.json
* search for "AgentProvider"

instead of doing one huge search.

---

## How to determine who launched them

The most useful command is:

```bash
ps -Ao pid,ppid,command | grep " rg"
```

Then inspect the parent process:

```bash
ps -p <PPID> -o pid,command
```

Example:

```
12345 rg ...
12300 claude
```

or

```
12345 rg ...
11002 Cursor Helper
```

or

```
12345 rg ...
9876 Code Helper
```

The parent process usually tells you exactly which application is responsible.

---

## Activity Monitor

In Activity Monitor:

1. Add the **Parent Process** column if available.
2. Sort by Process Name.
3. Look for all `rg` entries.
4. See which application owns them.

---

## If CPU usage is high

Run:

```bash
top -o cpu
```

or

```bash
htop
```

(if installed)

You can also inspect one process:

```bash
sample <pid> 5
```

or

```bash
spindump <pid>
```

to see what it's doing.

---

## Given your recent work

Based on what we've been discussing recently—large Go repositories, multiple MCP servers, Codex CLI, Claude Code, and spec-driven development—I would first suspect **multiple AI coding agents**. They rely heavily on `ripgrep` to understand repository contents before making changes, and if several agents are running in parallel or traversing multiple repositories, it's normal to see many concurrent `rg` processes.

If you paste the output of:

```bash
ps -Ao pid,ppid,%cpu,command | grep " rg"
```

or

```bash
ps -Ao pid,ppid,command | grep rg
```

I can help identify exactly which application is spawning them and whether the behavior looks expected.

===

Yes. At **load averages of 400–700**, you likely have a runaway process-spawning loop—not merely a few expensive searches. The best automation should detect excessive `rg` activity directly, record which parent launched it, terminate the searches, and optionally terminate the parent if it keeps respawning them.

Do **not** trigger solely on load average: load decays slowly, and another workload could be responsible. Use a combination of:

* Number of `rg` processes
* Aggregate `rg` CPU usage
* System load
* Repeated triggering within a short interval

## Recommended guard script

Create:

```bash
mkdir -p "$HOME/bin" "$HOME/Library/Logs"
nano "$HOME/bin/rg-guard.sh"
```

Paste:

```bash
#!/bin/zsh

# Adjust these based on normal behavior on your Mac.
MAX_RG_COUNT=30
MAX_LOAD1=150
COOLDOWN_SECONDS=60

LOG="$HOME/Library/Logs/rg-guard.log"
STATE="/tmp/rg-guard-${UID}.last-trigger"

timestamp() {
    date '+%Y-%m-%d %H:%M:%S'
}

# Prevent overlapping guard executions.
LOCK="/tmp/rg-guard-${UID}.lock"

if ! mkdir "$LOCK" 2>/dev/null; then
    exit 0
fi

trap 'rmdir "$LOCK"' EXIT

RG_PIDS=("${(@f)$(pgrep -x rg 2>/dev/null)}")
RG_COUNT=${#RG_PIDS[@]}

# zsh gives an array containing one empty value when command output is empty.
if [[ -z "${RG_PIDS[1]:-}" ]]; then
    RG_COUNT=0
fi

(( RG_COUNT == 0 )) && exit 0

LOAD1=$(sysctl -n vm.loadavg | awk '{print $2}')
LOAD1_INT=${LOAD1%.*}

# Trigger when rg is clearly multiplying, or when elevated rg count and
# extreme system load occur together.
SHOULD_TRIGGER=0

if (( RG_COUNT >= MAX_RG_COUNT )); then
    SHOULD_TRIGGER=1
elif (( RG_COUNT >= 10 && LOAD1_INT >= MAX_LOAD1 )); then
    SHOULD_TRIGGER=1
fi

(( SHOULD_TRIGGER == 0 )) && exit 0

NOW=$(date +%s)
LAST_TRIGGER=0

if [[ -f "$STATE" ]]; then
    read -r LAST_TRIGGER < "$STATE"
fi

{
    echo
    echo "[$(timestamp)] runaway rg detected"
    echo "rg_count=$RG_COUNT load1=$LOAD1"
    echo "Processes:"
    ps -o user,pid,ppid,%cpu,%mem,etime,command -p "${(j:,:)RG_PIDS}"

    echo "Parent processes:"
    for pid in "${RG_PIDS[@]}"; do
        ppid=$(ps -p "$pid" -o ppid= 2>/dev/null | tr -d ' ')
        [[ -n "$ppid" ]] &&
            ps -p "$ppid" -o user,pid,ppid,%cpu,%mem,etime,command 2>/dev/null
    done | sort -u
} >> "$LOG" 2>&1

# First ask rg to terminate cleanly.
kill -TERM "${RG_PIDS[@]}" 2>/dev/null

sleep 3

# Force-kill only surviving rg processes.
SURVIVORS=("${(@f)$(pgrep -x rg 2>/dev/null)}")

if [[ -n "${SURVIVORS[1]:-}" ]]; then
    kill -KILL "${SURVIVORS[@]}" 2>/dev/null
    echo "[$(timestamp)] force-killed ${#SURVIVORS[@]} surviving rg processes" \
        >> "$LOG"
fi

echo "$NOW" > "$STATE"

# Warn when this happened again during the cooldown period. This usually means
# the parent application is immediately respawning searches.
if (( NOW - LAST_TRIGGER < COOLDOWN_SECONDS )); then
    /usr/bin/osascript -e \
        'display notification "rg processes are repeatedly respawning. Check ~/Library/Logs/rg-guard.log for the parent process." with title "Runaway ripgrep activity"'
else
    /usr/bin/osascript -e \
        'display notification "Excessive rg processes were terminated." with title "rg guard activated"'
fi
```

Then make it executable:

```bash
chmod 755 "$HOME/bin/rg-guard.sh"
```

Test it manually:

```bash
"$HOME/bin/rg-guard.sh"
```

## Run it automatically with `launchd`

macOS uses `launchd` to manage scheduled scripts and per-user agents. Apple documents `launchd` as the standard mechanism for running scripts as agents or daemons. ([Apple Support][1])

Create:

```bash
nano "$HOME/Library/LaunchAgents/com.johnwang.rg-guard.plist"
```

Paste:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">

<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.johnwang.rg-guard</string>

    <key>ProgramArguments</key>
    <array>
        <string>/bin/zsh</string>
        <string>/Users/YOUR_USERNAME/bin/rg-guard.sh</string>
    </array>

    <key>StartInterval</key>
    <integer>10</integer>

    <key>RunAtLoad</key>
    <true/>

    <key>StandardOutPath</key>
    <string>/Users/YOUR_USERNAME/Library/Logs/rg-guard-launchd.log</string>

    <key>StandardErrorPath</key>
    <string>/Users/YOUR_USERNAME/Library/Logs/rg-guard-launchd-error.log</string>

    <key>ProcessType</key>
    <string>Background</string>
</dict>
</plist>
```

Replace `YOUR_USERNAME` automatically with:

```bash
sed -i '' "s/YOUR_USERNAME/$USER/g" \
  "$HOME/Library/LaunchAgents/com.johnwang.rg-guard.plist"
```

Validate and load it:

```bash
plutil -lint "$HOME/Library/LaunchAgents/com.johnwang.rg-guard.plist"

launchctl bootstrap "gui/$UID" \
  "$HOME/Library/LaunchAgents/com.johnwang.rg-guard.plist"
```

Check its status:

```bash
launchctl print "gui/$UID/com.johnwang.rg-guard"
```

To reload after editing:

```bash
launchctl bootout "gui/$UID/com.johnwang.rg-guard" 2>/dev/null

launchctl bootstrap "gui/$UID" \
  "$HOME/Library/LaunchAgents/com.johnwang.rg-guard.plist"
```

## Find the real cause

The guard restores usability, but it does not fix the application spawning hundreds of searches. Examine:

```bash
tail -200 "$HOME/Library/Logs/rg-guard.log"
```

The **Parent processes** section should identify something such as:

```text
claude
codex
Code Helper
Cursor Helper
node ...mcp...
```

Also run this while the incident is underway:

```bash
for pid in $(pgrep -x rg); do
    ps -p "$pid" -o pid=,ppid=,etime=,%cpu=,command=
done
```

Then summarize parents:

```bash
for pid in $(pgrep -x rg); do
    ps -p "$pid" -o ppid=
done |
tr -d ' ' |
sort |
uniq -c |
sort -nr |
while read count ppid; do
    ps -p "$ppid" -o pid=,ppid=,command= |
        awk -v count="$count" '{ print count, $0 }'
done
```

That will produce something like:

```text
423 78120 78050 claude
91  79214 78050 node /path/to/mcp-server
```

At that point, the better fix is usually to terminate or configure the offending **parent**, because otherwise it may immediately recreate all the `rg` children.

## Safer threshold strategy

For your observed failure mode, I would start with:

```bash
MAX_RG_COUNT=30
MAX_LOAD1=150
```

The process-count threshold will usually fire well before load reaches 400. If parallel coding agents legitimately create more than 30 searches, raise it to 50–75.

Avoid automatically running `killall rg` whenever load exceeds a fixed value. `killall` targets every process with that name, while PID-based termination lets the script log and act on the exact processes it inspected. ([SS64][2])

[1]: https://support.apple.com/guide/terminal/script-management-with-launchd-apdc6c1077b-5d5d-4d35-9c19-60f2397b2369/mac?utm_source=chatgpt.com "Script management with launchd in Terminal on Mac"
[2]: https://ss64.com/mac/killall.html?utm_source=chatgpt.com "KILLALL Command: Kill processes by name in macOS"

===

A Go implementation is cleaner and safer than `killall rg`. It can inspect exact PIDs, capture parent processes, send `SIGTERM`, wait, and use `SIGKILL` only for survivors.

Below is a single-file, standard-library implementation for macOS.

```go
// rg-guard monitors macOS system load and ripgrep process counts.
//
// When configured thresholds are exceeded, it:
//   1. Logs each rg process and its parent.
//   2. Sends SIGTERM to the exact rg processes it inspected.
//   3. Waits for a grace period.
//   4. Sends SIGKILL only to surviving processes.
//
// It does not use `killall`, so newly created or unrelated processes are less
// likely to be terminated accidentally.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Config struct {
	Interval          time.Duration
	GracePeriod       time.Duration
	Cooldown          time.Duration
	MaxRGCount        int
	ElevatedRGCount   int
	MaxLoad1          float64
	RepeatedThreshold int
	LogPath           string
	DryRun            bool
}

type Process struct {
	PID     int
	PPID    int
	User    string
	CPU     float64
	Memory  float64
	Elapsed string
	Command string
}

type Guard struct {
	config Config
	logger *log.Logger

	mu               sync.Mutex
	lastTrigger      time.Time
	consecutiveTrips int
}

func main() {
	config := parseFlags()

	if err := os.MkdirAll(filepath.Dir(config.LogPath), 0o755); err != nil {
		log.Fatalf("create log directory: %v", err)
	}

	logFile, err := os.OpenFile(
		config.LogPath,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		log.Fatalf("open log file: %v", err)
	}
	defer logFile.Close()

	logger := log.New(logFile, "", log.LstdFlags|log.Lmicroseconds)

	guard := &Guard{
		config: config,
		logger: logger,
	}

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	logger.Printf(
		"starting rg-guard interval=%s max_rg_count=%d elevated_rg_count=%d max_load1=%.2f dry_run=%t",
		config.Interval,
		config.MaxRGCount,
		config.ElevatedRGCount,
		config.MaxLoad1,
		config.DryRun,
	)

	if err := guard.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatalf("guard stopped: %v", err)
	}
}

func parseFlags() Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}

	defaultLogPath := filepath.Join(home, "Library", "Logs", "rg-guard.log")

	var config Config

	flag.DurationVar(
		&config.Interval,
		"interval",
		10*time.Second,
		"monitoring interval",
	)
	flag.DurationVar(
		&config.GracePeriod,
		"grace-period",
		3*time.Second,
		"time to wait after SIGTERM before SIGKILL",
	)
	flag.DurationVar(
		&config.Cooldown,
		"cooldown",
		60*time.Second,
		"minimum interval between normal trigger notifications",
	)
	flag.IntVar(
		&config.MaxRGCount,
		"max-rg-count",
		30,
		"terminate when the rg process count reaches this threshold",
	)
	flag.IntVar(
		&config.ElevatedRGCount,
		"elevated-rg-count",
		10,
		"minimum rg count used with the load threshold",
	)
	flag.Float64Var(
		&config.MaxLoad1,
		"max-load1",
		150,
		"one-minute load threshold",
	)
	flag.IntVar(
		&config.RepeatedThreshold,
		"repeated-threshold",
		2,
		"number of consecutive trigger cycles before reporting repeated respawning",
	)
	flag.StringVar(
		&config.LogPath,
		"log",
		defaultLogPath,
		"log file path",
	)
	flag.BoolVar(
		&config.DryRun,
		"dry-run",
		false,
		"log actions without sending signals",
	)

	flag.Parse()

	return config
}

func (g *Guard) Run(ctx context.Context) error {
	// Run immediately rather than waiting for the first ticker interval.
	g.check(ctx)

	ticker := time.NewTicker(g.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			g.logger.Printf("stopping rg-guard")
			return ctx.Err()

		case <-ticker.C:
			g.check(ctx)
		}
	}
}

func (g *Guard) check(ctx context.Context) {
	load1, err := getLoad1(ctx)
	if err != nil {
		g.logger.Printf("read load average: %v", err)
		return
	}

	processes, err := listProcesses(ctx)
	if err != nil {
		g.logger.Printf("list processes: %v", err)
		return
	}

	rgProcesses := filterRGProcesses(processes)
	rgCount := len(rgProcesses)

	shouldTrigger := rgCount >= g.config.MaxRGCount ||
		(rgCount >= g.config.ElevatedRGCount && load1 >= g.config.MaxLoad1)

	if !shouldTrigger {
		g.mu.Lock()
		g.consecutiveTrips = 0
		g.mu.Unlock()
		return
	}

	g.mu.Lock()
	g.consecutiveTrips++
	consecutiveTrips := g.consecutiveTrips
	lastTrigger := g.lastTrigger
	g.lastTrigger = time.Now()
	g.mu.Unlock()

	g.logger.Printf(
		"runaway rg detected load1=%.2f rg_count=%d consecutive_trips=%d",
		load1,
		rgCount,
		consecutiveTrips,
	)

	g.logProcessTree(rgProcesses, processes)

	if g.config.DryRun {
		g.logger.Printf("dry-run enabled; no processes terminated")
		return
	}

	targetPIDs := make([]int, 0, len(rgProcesses))
	for _, process := range rgProcesses {
		targetPIDs = append(targetPIDs, process.PID)
	}

	terminated := signalProcesses(targetPIDs, syscall.SIGTERM, g.logger)
	g.logger.Printf("sent SIGTERM to %d rg processes", terminated)

	if !sleepContext(ctx, g.config.GracePeriod) {
		return
	}

	survivors := findSurvivingPIDs(targetPIDs)
	if len(survivors) > 0 {
		killed := signalProcesses(survivors, syscall.SIGKILL, g.logger)
		g.logger.Printf("sent SIGKILL to %d surviving rg processes", killed)
	}

	repeated := consecutiveTrips >= g.config.RepeatedThreshold ||
		(!lastTrigger.IsZero() && time.Since(lastTrigger) < g.config.Cooldown)

	if repeated {
		g.notify(
			ctx,
			"Runaway ripgrep activity",
			"rg processes are repeatedly respawning. Check the rg-guard log for the parent process.",
		)
	} else {
		g.notify(
			ctx,
			"rg guard activated",
			fmt.Sprintf("Terminated %d excessive rg processes.", len(targetPIDs)),
		)
	}
}

func getLoad1(ctx context.Context) (float64, error) {
	output, err := exec.CommandContext(ctx, "/usr/sbin/sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return 0, fmt.Errorf("sysctl vm.loadavg: %w", err)
	}

	// Typical macOS output:
	// { 2.31 2.58 2.42 }
	fields := strings.Fields(strings.TrimSpace(string(output)))
	if len(fields) < 2 {
		return 0, fmt.Errorf("unexpected vm.loadavg output: %q", output)
	}

	loadText := strings.Trim(fields[1], "{}")
	load1, err := strconv.ParseFloat(loadText, 64)
	if err != nil {
		return 0, fmt.Errorf("parse one-minute load %q: %w", loadText, err)
	}

	return load1, nil
}

func listProcesses(ctx context.Context) ([]Process, error) {
	// The "=" suffix removes column headers.
	command := exec.CommandContext(
		ctx,
		"/bin/ps",
		"-axo",
		"user=,pid=,ppid=,%cpu=,%mem=,etime=,command=",
	)

	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("ps: %w", err)
	}

	var processes []Process
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	// Process command lines may be long.
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)

	for scanner.Scan() {
		process, ok := parseProcessLine(scanner.Text())
		if ok {
			processes = append(processes, process)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan ps output: %w", err)
	}

	return processes, nil
}

func parseProcessLine(line string) (Process, bool) {
	// Split into at most seven fields so the final field keeps the full command.
	fields := strings.Fields(line)
	if len(fields) < 7 {
		return Process{}, false
	}

	pid, err := strconv.Atoi(fields[1])
	if err != nil {
		return Process{}, false
	}

	ppid, err := strconv.Atoi(fields[2])
	if err != nil {
		return Process{}, false
	}

	cpu, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		return Process{}, false
	}

	memory, err := strconv.ParseFloat(fields[4], 64)
	if err != nil {
		return Process{}, false
	}

	command := strings.Join(fields[6:], " ")

	return Process{
		User:    fields[0],
		PID:     pid,
		PPID:    ppid,
		CPU:     cpu,
		Memory:  memory,
		Elapsed: fields[5],
		Command: command,
	}, true
}

func filterRGProcesses(processes []Process) []Process {
	result := make([]Process, 0)

	for _, process := range processes {
		if isRGCommand(process.Command) {
			result = append(result, process)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].PID < result[j].PID
	})

	return result
}

func isRGCommand(command string) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}

	executable := filepath.Base(fields[0])
	return executable == "rg"
}

func (g *Guard) logProcessTree(rgProcesses, allProcesses []Process) {
	processByPID := make(map[int]Process, len(allProcesses))
	for _, process := range allProcesses {
		processByPID[process.PID] = process
	}

	parentCounts := make(map[int]int)

	for _, process := range rgProcesses {
		parentCounts[process.PPID]++

		g.logger.Printf(
			"rg pid=%d ppid=%d user=%s cpu=%.1f mem=%.1f elapsed=%s command=%q",
			process.PID,
			process.PPID,
			process.User,
			process.CPU,
			process.Memory,
			process.Elapsed,
			process.Command,
		)
	}

	parentPIDs := make([]int, 0, len(parentCounts))
	for pid := range parentCounts {
		parentPIDs = append(parentPIDs, pid)
	}
	sort.Ints(parentPIDs)

	for _, pid := range parentPIDs {
		parent, found := processByPID[pid]
		if !found {
			g.logger.Printf(
				"parent pid=%d rg_children=%d process=no-longer-present",
				pid,
				parentCounts[pid],
			)
			continue
		}

		g.logger.Printf(
			"parent pid=%d ppid=%d rg_children=%d user=%s cpu=%.1f mem=%.1f elapsed=%s command=%q",
			parent.PID,
			parent.PPID,
			parentCounts[pid],
			parent.User,
			parent.CPU,
			parent.Memory,
			parent.Elapsed,
			parent.Command,
		)
	}
}

func signalProcesses(pids []int, signal syscall.Signal, logger *log.Logger) int {
	count := 0

	for _, pid := range pids {
		process, err := os.FindProcess(pid)
		if err != nil {
			logger.Printf("find pid=%d: %v", pid, err)
			continue
		}

		if err := process.Signal(signal); err != nil {
			if errors.Is(err, os.ErrProcessDone) {
				continue
			}

			// ESRCH means the process exited between inspection and signaling.
			if errors.Is(err, syscall.ESRCH) {
				continue
			}

			logger.Printf("signal pid=%d signal=%s: %v", pid, signal, err)
			continue
		}

		count++
	}

	return count
}

func findSurvivingPIDs(pids []int) []int {
	survivors := make([]int, 0)

	for _, pid := range pids {
		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}

		// Signal 0 checks whether the process still exists.
		err = process.Signal(syscall.Signal(0))
		if err == nil || errors.Is(err, syscall.EPERM) {
			survivors = append(survivors, pid)
		}
	}

	return survivors
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (g *Guard) notify(ctx context.Context, title, message string) {
	script := fmt.Sprintf(
		`display notification %s with title %s`,
		appleScriptString(message),
		appleScriptString(title),
	)

	command := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script)
	if err := command.Run(); err != nil {
		g.logger.Printf("display notification: %v", err)
	}
}

func appleScriptString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}
```

## Build and install

Assuming the source is stored as `cmd/rg-guard/main.go`:

```bash
go build -o rg-guard ./cmd/rg-guard
mkdir -p "$HOME/bin"
mv rg-guard "$HOME/bin/rg-guard"
chmod 755 "$HOME/bin/rg-guard"
```

Test it in dry-run mode:

```bash
"$HOME/bin/rg-guard" \
  -dry-run \
  -interval 10s \
  -max-rg-count 30 \
  -elevated-rg-count 10 \
  -max-load1 150
```

Watch the log:

```bash
tail -f "$HOME/Library/Logs/rg-guard.log"
```

Then test actual enforcement:

```bash
"$HOME/bin/rg-guard" \
  -interval 10s \
  -max-rg-count 30 \
  -elevated-rg-count 10 \
  -max-load1 150
```

## `launchd` configuration

Create:

```text
~/Library/LaunchAgents/com.johnwang.rg-guard.plist
```

With:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">

<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.johnwang.rg-guard</string>

    <key>ProgramArguments</key>
    <array>
        <string>/Users/YOUR_USERNAME/bin/rg-guard</string>

        <string>-interval</string>
        <string>10s</string>

        <string>-max-rg-count</string>
        <string>30</string>

        <string>-elevated-rg-count</string>
        <string>10</string>

        <string>-max-load1</string>
        <string>150</string>

        <string>-grace-period</string>
        <string>3s</string>

        <string>-cooldown</string>
        <string>60s</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <key>ProcessType</key>
    <string>Background</string>

    <key>ThrottleInterval</key>
    <integer>10</integer>

    <key>StandardOutPath</key>
    <string>/Users/YOUR_USERNAME/Library/Logs/rg-guard-stdout.log</string>

    <key>StandardErrorPath</key>
    <string>/Users/YOUR_USERNAME/Library/Logs/rg-guard-stderr.log</string>
</dict>
</plist>
```

Replace the username:

```bash
sed -i '' "s/YOUR_USERNAME/$USER/g" \
  "$HOME/Library/LaunchAgents/com.johnwang.rg-guard.plist"
```

Validate and start it:

```bash
plutil -lint "$HOME/Library/LaunchAgents/com.johnwang.rg-guard.plist"

launchctl bootstrap "gui/$UID" \
  "$HOME/Library/LaunchAgents/com.johnwang.rg-guard.plist"
```

Inspect it:

```bash
launchctl print "gui/$UID/com.johnwang.rg-guard"
```

Reload after changing configuration:

```bash
launchctl bootout \
  "gui/$UID/com.johnwang.rg-guard" 2>/dev/null

launchctl bootstrap \
  "gui/$UID" \
  "$HOME/Library/LaunchAgents/com.johnwang.rg-guard.plist"
```

## Important design choice

I would initially terminate only `rg`, not the parent process. The log will show entries such as:

```text
parent pid=12345 rg_children=287 command="claude ..."
```

or:

```text
parent pid=12345 rg_children=287 command="node /path/to/mcp-server ..."
```

Once the offending parent is known, the next enhancement should be a configurable parent policy, for example:

```yaml
parent_policy:
  allowed:
    - "Visual Studio Code"
  terminate_when_respawning:
    - "claude"
    - "codex"
    - "some-mcp-server"
```

I would also consider renaming this from `rg-guard` to something more generic such as **`process-guard`** or **`workload-guard`**, with `rg` defined as its first process policy. That would let the same daemon protect against runaway `git`, `node`, language-server, or agent subprocess storms.

===

Yes. Based on the kinds of tooling you're building, I would make this a **general system health daemon** rather than an `rg`-specific utility. `rg` is just the first policy.

This also aligns well with your Go-first, reusable platform approach.

## I would avoid "LoadGuard"

The term **LoadGuard** suggests it only monitors CPU load. In reality, you want to detect **runaway workloads**, not just high load.

For example:

* excessive `rg`
* thousands of `git` processes
* runaway `node`
* Claude Code spawning hundreds of workers
* Codex CLI loops
* MCP server explosions
* excessive file descriptors
* memory exhaustion
* swap storms
* disk I/O saturation

Load is just one signal.

---

## Architecture

Instead of:

```
LoadGuard
    └── rg
```

I'd structure it more like:

```
Guard Daemon
    ├── Load Policy
    ├── Process Count Policy
    ├── Parent Process Policy
    ├── Memory Policy
    ├── IO Policy
    ├── Open Files Policy
    ├── Network Policy
    └── Notification Policy
```

where **rg** is simply one process rule.

---

## Example configuration

```yaml
policies:

  - name: runaway-rg
    process: rg
    maxCount: 30
    action:
      - terminate

  - name: runaway-node
    process: node
    maxCount: 150
    action:
      - notify

  - name: runaway-git
    process: git
    maxCount: 50
    action:
      - terminate

  - name: system-load
    load1: 150
    action:
      - investigate

  - name: memory
    freeMemoryMB: 512
    action:
      - notify
```

---

## Even better: policy engine

Given your recent work on policy systems (for example, using Cedar for automation decisions), I'd separate **measurement** from **policy**.

```
Collector
    ↓

Snapshot

{
    loadAverage: ...
    processes: ...
    memory: ...
    io: ...
}

    ↓

Policy Engine

    ↓

Actions

• log
• notify
• terminate
• kill parent
• suspend
• collect diagnostics
```

That makes it much easier to extend over time.

---

## Diagnostic actions

One feature I'd definitely add is **automatic evidence collection** before taking action.

When a rule fires, capture things like:

```text
top
ps
parent process tree
open files
stack samples
spindump
```

Store them in a timestamped directory:

```
~/Library/Logs/loadguard/

    2026-07-13T04-18-22/

        top.txt
        ps.txt
        parents.txt
        rg.txt
        sample-claude.txt
```

This gives you the information you need to identify why the runaway behavior happened.

---

## Possible names

Some options that fit a broader mission:

| Name              | Comments                                                |
| ----------------- | ------------------------------------------------------- |
| **LoadGuard**     | Good, but implies CPU load only.                        |
| **ProcessGuard**  | Focuses on runaway processes.                           |
| **WorkloadGuard** | Covers processes, memory, I/O, and system load.         |
| **ResourceGuard** | Broad and enterprise-friendly.                          |
| **HealthGuard**   | More about overall system health.                       |
| **SystemGuard**   | Generic, but likely to conflict with existing products. |
| **RuntimeGuard**  | Nice fit if you're protecting developer runtimes.       |
| **Watchdog**      | Classic term, but heavily overloaded.                   |

## Given your ecosystem

Since you're already building a cohesive suite of Go infrastructure components under PlexusOne, I'd lean toward **WorkloadGuard**.

It naturally accommodates future policies such as:

* `rg` storms
* agent loops
* runaway MCP servers
* excessive memory usage
* I/O bottlenecks
* GPU saturation (for local inference)
* thermal throttling
* battery drain
* zombie processes
* orphaned child processes

In other words, it evolves from "kill runaway `rg`" into a reusable **developer workstation protection service** that can keep AI-heavy development environments responsive while also producing the diagnostics needed to fix the underlying cause.

===

Here's the selected content formatted as a reusable writing block.

### Naming Recommendations

For a general-purpose daemon that protects a developer workstation from runaway workloads, the following names are strong candidates:

* **WorkloadGuard** — Covers processes, memory, I/O, and system load. This is the most comprehensive and future-proof name, as it describes the overall workload on the system rather than a single metric.

* **ResourceGuard** — Broad and enterprise-friendly. Suitable if the project may eventually monitor and manage CPU, memory, disk, network, GPU, and other system resources.

* **HealthGuard** — Emphasizes overall system health and stability rather than enforcement. A good fit if the daemon will primarily detect, diagnose, and report issues.

* **SystemGuard** — A simple, generic name that conveys protection of the entire system. While descriptive, it may be more likely to conflict with existing products and projects.

Among these, **WorkloadGuard** is the strongest long-term choice because it naturally encompasses process monitoring, system load, memory pressure, I/O bottlenecks, and future AI/agent-specific workload policies.

===
