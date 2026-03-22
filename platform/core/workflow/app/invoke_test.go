package app

import "testing"

func TestParseOrchestrationTarget(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "sfn:hello", want: "hello", ok: true},
		{input: "stepfunction:hello", want: "hello", ok: true},
		{input: "cwf:order-flow", want: "order-flow", ok: true},
		{input: "cloudworkflow:order-flow", want: "order-flow", ok: true},
		{input: "durable:process-order", want: "process-order", ok: true},
		{input: "hello", want: "", ok: false},
		{input: "durable:", want: "", ok: false},
	}

	for _, tc := range tests {
		got, ok := parseOrchestrationTarget(tc.input)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("parseOrchestrationTarget(%q) = (%q,%v), want (%q,%v)", tc.input, got, ok, tc.want, tc.ok)
		}
	}
}
