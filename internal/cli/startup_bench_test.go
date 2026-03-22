package cli

import (
	"io"
	"testing"
)

func BenchmarkCommandStartupPlanHelp(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cmd := NewRootCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"plan", "--help"})
		if err := cmd.Execute(); err != nil {
			b.Fatalf("execute plan --help: %v", err)
		}
	}
}
