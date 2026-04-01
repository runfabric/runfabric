package app

import (
	"bufio"
	"context"
	"os"
	"path/filepath"

	providers "github.com/runfabric/runfabric/internal/provider/contracts"
	statetypes "github.com/runfabric/runfabric/internal/state/types"
	deployapi "github.com/runfabric/runfabric/platform/deploy/core/api"
	"github.com/runfabric/runfabric/platform/state/receiptconv"
	"github.com/runfabric/runfabric/platform/workflow/lifecycle"
)

const defaultLogsPath = ".runfabric/logs"

// Logs returns logs for one function or, when function is "", for all functions (aggregated by service/stage).
// Unified source: provider logs (e.g. CloudWatch) plus optional local log files from config.logs.path (default .runfabric/logs).
func Logs(configPath, stage, function, providerOverride, service string) (any, error) {
	ctx, err := Bootstrap(configPath, stage, providerOverride)
	if err != nil {
		return nil, err
	}
	if err := validateServiceScope(ctx.Config.Service, service); err != nil {
		return nil, err
	}
	if function != "" {
		return logsSingle(ctx, function)
	}
	// --all: aggregate logs for all functions in this service/stage
	return logsAll(ctx)
}

func logsSingle(ctx *AppContext, function string) (any, error) {
	provider, err := resolveProvider(ctx)
	if err != nil {
		return nil, err
	}
	var receipt *statetypes.Receipt
	if ctx.Backends != nil && ctx.Backends.Receipts != nil {
		receipt, _ = ctx.Backends.Receipts.Load(ctx.Stage)
	}
	var result *providers.LogsResult
	if provider.mode == dispatchAPI {
		r, err := deployapi.Logs(context.Background(), provider.name, ctx.Config, ctx.Stage, function, ctx.RootDir, receiptconv.ToCoreReceipt(receipt))
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
	provider, err := resolveProvider(ctx)
	if err != nil {
		return nil, err
	}
	byFunction := make(map[string]any)
	var receipt *statetypes.Receipt
	if ctx.Backends != nil && ctx.Backends.Receipts != nil {
		receipt, _ = ctx.Backends.Receipts.Load(ctx.Stage)
	}
	for name := range ctx.Config.Functions {
		var result *providers.LogsResult
		var err error
		if provider.mode == dispatchAPI {
			result, err = deployapi.Logs(context.Background(), provider.name, ctx.Config, ctx.Stage, name, ctx.RootDir, receiptconv.ToCoreReceipt(receipt))
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
		"provider":       provider.name,
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
