package aws

import "testing"

func TestApplyStepFunctionBindings_UsesLambdaARNMap(t *testing.T) {
	decl := stepFunctionDecl{
		Bindings: map[string]string{
			"charge": "charge-payment",
		},
	}
	definition := `{"StartAt":"Run","States":{"Run":{"Type":"Task","Resource":"${bindings.charge}","End":true}}}`
	got := applyStepFunctionBindings(definition, decl, map[string]string{
		"charge-payment": "arn:aws:lambda:us-east-1:123456789012:function:charge-payment",
	})
	want := `{"StartAt":"Run","States":{"Run":{"Type":"Task","Resource":"arn:aws:lambda:us-east-1:123456789012:function:charge-payment","End":true}}}`
	if got != want {
		t.Fatalf("unexpected substitution\nwant: %s\n got: %s", want, got)
	}
}

func TestApplyStepFunctionBindings_UsesLiteralARN(t *testing.T) {
	decl := stepFunctionDecl{
		Bindings: map[string]string{
			"handler": "arn:aws:lambda:us-east-1:123456789012:function:literal-handler",
		},
	}
	definition := `{{bindings.handler}}`
	got := applyStepFunctionBindings(definition, decl, nil)
	want := `arn:aws:lambda:us-east-1:123456789012:function:literal-handler`
	if got != want {
		t.Fatalf("unexpected literal substitution\nwant: %s\n got: %s", want, got)
	}
}

func TestApplyStepFunctionBindings_SupportsTemplateVariants(t *testing.T) {
	decl := stepFunctionDecl{
		Bindings: map[string]string{
			"notify": "notify-user",
		},
	}
	definition := `${binding.notify}|${bindings.notify}|{{binding.notify}}|{{bindings.notify}}`
	got := applyStepFunctionBindings(definition, decl, map[string]string{
		"notify-user": "arn:aws:lambda:us-east-1:123456789012:function:notify-user",
	})
	want := `arn:aws:lambda:us-east-1:123456789012:function:notify-user|arn:aws:lambda:us-east-1:123456789012:function:notify-user|arn:aws:lambda:us-east-1:123456789012:function:notify-user|arn:aws:lambda:us-east-1:123456789012:function:notify-user`
	if got != want {
		t.Fatalf("unexpected variant substitution\nwant: %s\n got: %s", want, got)
	}
}
