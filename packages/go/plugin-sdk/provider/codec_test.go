package provider

import "testing"

func TestDecodeConfig(t *testing.T) {
	type KubeProvider struct {
		ServiceType string `json:"serviceType,omitempty"`
		Namespace   string `json:"namespace,omitempty"`
		Image       string `json:"image,omitempty"`
	}

	cfg := Config{
		"service": "todo-api",
		"provider": map[string]any{
			"name":        "kubernetes",
			"runtime":     "nodejs",
			"serviceType": "LoadBalancer",
			"namespace":   "prod",
		},
	}

	var kp KubeProvider
	if err := DecodeConfig(cfg, &kp); err != nil {
		t.Fatalf("DecodeConfig: %v", err)
	}
	if kp.ServiceType != "LoadBalancer" {
		t.Errorf("ServiceType=%q want LoadBalancer", kp.ServiceType)
	}
	if kp.Namespace != "prod" {
		t.Errorf("Namespace=%q want prod", kp.Namespace)
	}
	if kp.Image != "" {
		t.Errorf("Image=%q want empty", kp.Image)
	}
}

func TestDecodeSection(t *testing.T) {
	type DeploySection struct {
		Strategy string `json:"strategy,omitempty"`
	}

	cfg := Config{
		"deploy": map[string]any{
			"strategy": "per-function",
		},
	}

	var d DeploySection
	if err := DecodeSection(cfg, "deploy", &d); err != nil {
		t.Fatalf("DecodeSection: %v", err)
	}
	if d.Strategy != "per-function" {
		t.Errorf("Strategy=%q want per-function", d.Strategy)
	}
}

func TestDecodeConfig_Empty(t *testing.T) {
	type KubeProvider struct {
		ServiceType string `json:"serviceType,omitempty"`
	}
	var kp KubeProvider
	if err := DecodeConfig(Config{}, &kp); err != nil {
		t.Fatalf("empty config should not error: %v", err)
	}
	if kp.ServiceType != "" {
		t.Errorf("expected empty struct, got %+v", kp)
	}
}
