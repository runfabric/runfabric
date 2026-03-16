package app

import (
	"bufio"
	"context"
	"os"
	"path/filepath"

	deployapi "github.com/runfabric/runfabric/engine/internal/deploy/api"
	"github.com/runfabric/runfabric/engine/internal/lifecycle"
	"github.com/runfabric/runfabric/engine/internal/providers"
	"github.com/runfabric/runfabric/engine/internal/state"
)

const defaultLogsPath = ".runfabric/logs"

// Logs returns logs for one function or, when function is "", for all functions (aggregated by service/stage).
// Unified source: provider logs (e.g. CloudWatch) plus optional local log files from config.logs.path (default .runfabric/logs).
func Logs(configPath, stage, function, providerOverride string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}
	if function != "" {
		return logsSingle(ctx, function)
	}
	// --all: aggregate logs for all functions in this service/stage
	return logsAll(ctx)
}

func logsSingle(ctx *AppContext, function string) (any, error) {
	provider := ctx.Config.Provider.Name
	var receipt *state.Receipt
	if ctx.Backends != nil && ctx.Backends.Receipts != nil {
		receipt, _ = ctx.Backends.Receipts.Load(ctx.Stage)
	}
	var result *providers.LogsResult
	if deployapi.HasLogger(provider) {
		r, err := deployapi.Logs(context.Background(), provider, ctx.Config, ctx.Stage, function, ctx.RootDir, receipt)
		if err != nil {
			return nil, err
		}
		result = r
	} else {
		r, err := lifecycle.Logs(ctx.Registry, ctx.Config, ctx.Stage, function)
		if err != nil {
			return nil, err
		}
		result = r
	}
	mergeLocalLogs(ctx, result, function)
	return result, nil
}

func logsAll(ctx *AppContext) (any, error) {
	provider := ctx.Config.Provider.Name
	byFunction := make(map[string]any)
	var receipt *state.Receipt
	if ctx.Backends != nil && ctx.Backends.Receipts != nil {
		receipt, _ = ctx.Backends.Receipts.Load(ctx.Stage)
	}
	for name := range ctx.Config.Functions {
		var result *providers.LogsResult
		var err error
		if deployapi.HasLogger(provider) {
			result, err = deployapi.Logs(context.Background(), provider, ctx.Config, ctx.Stage, name, ctx.RootDir, receipt)
		} else {
			result, err = lifecycle.Logs(ctx.Registry, ctx.Config, ctx.Stage, name)
		}
		if err != nil {
			byFunction[name] = map[string]string{"error": err.Error()}
			continue
		}
		mergeLocalLogs(ctx, result, name)
		byFunction[name] = result
	}
	return map[string]any{
		"service":        ctx.Config.Service,
		"stage":          ctx.Stage,
		"provider":       provider,
		"logsByFunction": byFunction,
	}, nil
}

// mergeLocalLogs appends lines from local log files (config.logs.path or .runfabric/logs) to result.Lines.
func mergeLocalLogs(ctx *AppContext, result *providers.LogsResult, function string) {
	if result == nil {
		return
	}
	path := defaultLogsPath
	if ctx.Config.Logs != nil && ctx.Config.Logs.Path != "" {
		path = ctx.Config.Logs.Path
	}
	dir := filepath.Join(ctx.RootDir, path)
	// Read <stage>.log (all functions) and <function>_<stage>.log
	files := []string{filepath.Join(dir, ctx.Stage+".log")}
	if function != "" {
		files = append(files, filepath.Join(dir, function+"_"+ctx.Stage+".log"))
	}
	for _, f := range files {
		lines := readLogFileLines(f)
		result.Lines = append(result.Lines, lines...)
	}
}

func readLogFileLines(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}
