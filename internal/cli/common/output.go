package common

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/runfabric/runfabric/platform/core/model/protocol"
	"golang.org/x/term"
)

// ANSI 4-bit colors (only when stderr is a TTY)
const (
	ansiReset = "\033[0m"
	ansiDim   = "\033[2m"
	ansiBold  = "\033[1m"
	ansiRed   = "\033[31m"
	ansiGreen = "\033[32m"
	ansiCyan  = "\033[36m"
)

func stderrIsTTY() bool {
	fd := int(os.Stderr.Fd())
	return term.IsTerminal(fd)
}

func stdoutIsTTY() bool {
	fd := int(os.Stdout.Fd())
	return term.IsTerminal(fd)
}

// initWrote prints "Wrote <path>" with optional color (green path) for init.
func InitWrote(path string) {
	if stdoutIsTTY() {
		fmt.Printf("  %sWrote%s %s%s%s\n", ansiDim, ansiReset, ansiGreen, path, ansiReset)
	} else {
		fmt.Printf("Wrote %s\n", path)
	}
}

// initReady prints the "Project ready" block with optional color for init.
func InitReady(projectDirLabel, provider, trigger, lang, state string) {
	if stdoutIsTTY() {
		fmt.Printf("\n  %s%sProject ready%s in %s%s%s\n", ansiBold, ansiGreen, ansiReset, ansiCyan, projectDirLabel, ansiReset)
		fmt.Printf("  %sprovider: %s  trigger: %s  lang: %s  state: %s%s\n", ansiDim, provider, trigger, lang, state, ansiReset)
	} else {
		fmt.Printf("\nProject ready in %s\n", projectDirLabel)
		fmt.Printf("  provider: %s  trigger: %s  lang: %s  state: %s\n", provider, trigger, lang, state)
	}
}

// initNext prints the "Next:" line with optional color for init.
func InitNext(cmd string) {
	if stdoutIsTTY() {
		fmt.Printf("  %sNext:%s %s%s%s\n", ansiDim, ansiReset, ansiCyan, cmd, ansiReset)
	} else {
		fmt.Printf("  Next: %s\n", cmd)
	}
}

// statusRunning prints a "running" line to stderr when json is false (human-readable mode).
func StatusRunning(jsonOutput bool, message string) {
	if jsonOutput {
		return
	}
	if stderrIsTTY() {
		fmt.Fprintf(os.Stderr, "%s%s→%s %s%s%s\n", ansiCyan, ansiBold, ansiReset, ansiDim, message, ansiReset)
	} else {
		fmt.Fprintln(os.Stderr, "→", message)
	}
	os.Stderr.Sync()
}

// statusDone prints a success line to stderr when json is false.
func StatusDone(jsonOutput bool, message string) {
	if jsonOutput {
		return
	}
	if stderrIsTTY() {
		fmt.Fprintf(os.Stderr, "%s%s✔%s %s%s%s\n", ansiGreen, ansiBold, ansiReset, ansiDim, message, ansiReset)
	} else {
		fmt.Fprintln(os.Stderr, "✔", message)
	}
	os.Stderr.Sync()
}

// statusFail prints a failure line to stderr when json is false.
func StatusFail(jsonOutput bool, message string) {
	if jsonOutput {
		return
	}
	if stderrIsTTY() {
		fmt.Fprintf(os.Stderr, "%s%s✖%s %s%s%s\n", ansiRed, ansiBold, ansiReset, ansiDim, message, ansiReset)
	} else {
		fmt.Fprintln(os.Stderr, "✖", message)
	}
	os.Stderr.Sync()
}

func PrintSuccess(command string, data any) error {
	resp := protocol.Success(command, data)
	return printJSON(resp)
}

func PrintFailure(command string, err error) error {
	resp := protocol.Failure(command, protocol.FromError(err))
	if marshalErr := printJSON(resp); marshalErr != nil {
		return marshalErr
	}
	return err
}

func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal output: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
