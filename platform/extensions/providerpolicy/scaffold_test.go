package providerpolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestBuiltinProviderScaffoldDeclarations pins that every built-in provider
// declares a scaffolding comment (the runfabric.yml header `runfabric init`
// writes) via its ProviderScaffold, so init.go stays free of a per-provider
// switch.
func TestBuiltinProviderScaffoldDeclarations(t *testing.T) {
	for _, id := range append(append([]string{}, hostableProviders...), "kubernetes") {
		sc := ProviderScaffold(id)
		if sc.Comment == "" {
			t.Errorf("%s: no scaffold comment declared", id)
		}
	}
	// Cloudflare Workers scaffold a worker.js script rather than a Lambda-style
	// handler — its entry/file/sample overrides must be present.
	cf := ProviderScaffold("cloudflare-workers")
	if cf.Entry != "worker.fetch" || cf.EntryFile != "worker.js" || cf.Sample == "" {
		t.Errorf("cloudflare-workers scaffold missing worker overrides: %+v", cf)
	}
}

// TestBuiltinStateScaffoldDeclarations pins the backend: config block every
// non-local state backend contributes to `runfabric init`, so init.go carries no
// per-backend switch and the postgres/dynamodb/sqlite blocks are non-empty.
func TestBuiltinStateScaffoldDeclarations(t *testing.T) {
	want := map[string][]string{
		"s3":       {"s3Bucket", "s3Prefix", "lockTable"},
		"gcs":      {"gcsBucket", "gcsPrefix"},
		"azblob":   {"azblobContainer", "azblobPrefix"},
		"postgres": {"postgresConnectionStringEnv", "postgresTable"},
		"dynamodb": {"lockTable"},
		"sqlite":   {"sqlitePath"},
	}
	for kind, keys := range want {
		got := StateBackendScaffold(kind)
		if len(got) != len(keys) {
			t.Errorf("%s: got %d config lines, want %d (%v)", kind, len(got), len(keys), keys)
			continue
		}
		for i, k := range keys {
			if got[i].Key != k {
				t.Errorf("%s: config line %d key = %q, want %q", kind, i, got[i].Key, k)
			}
			if got[i].Value == "" {
				t.Errorf("%s: config line %q has empty value", kind, k)
			}
		}
	}
	if StateBackendScaffold("local") != nil {
		t.Error("local must declare no backend config (init omits the block)")
	}
	if StateBackendScaffold("unknown") != nil {
		t.Error("unknown backend must declare no config")
	}
}

// TestPluginYamlScaffoldMatchDeclarations pins plugin.yaml's scaffold: block (the
// external-plugin declaration) to the built-in ProviderScaffold so the two
// sources cannot drift. Only providers that ship a scaffold: block are checked.
func TestPluginYamlScaffoldMatchDeclarations(t *testing.T) {
	type yamlScaffold struct {
		Comment       string            `yaml:"comment"`
		Entry         string            `yaml:"entry"`
		EntryFile     string            `yaml:"entryFile"`
		Sample        string            `yaml:"sample"`
		RuntimeByLang map[string]string `yaml:"runtimeByLang"`
	}
	var manifest struct {
		Scaffold *yamlScaffold `yaml:"scaffold"`
	}
	checked := 0
	for _, id := range append(append([]string{}, hostableProviders...), "kubernetes") {
		path := filepath.Join("..", "..", "..", "extensions", "providers", id, "plugin.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: read plugin.yaml: %v", id, err)
		}
		manifest.Scaffold = nil
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			t.Fatalf("%s: parse plugin.yaml: %v", id, err)
		}
		if manifest.Scaffold == nil {
			t.Errorf("%s: plugin.yaml declares no scaffold: block", id)
			continue
		}
		checked++
		y := manifest.Scaffold
		sc := ProviderScaffold(id)
		if y.Comment != sc.Comment || y.Entry != sc.Entry || y.EntryFile != sc.EntryFile || y.Sample != sc.Sample {
			t.Errorf("%s: plugin.yaml scaffold differs from code:\n plugin.yaml=%+v\n code=%+v", id, *y, sc)
		}
	}
	if checked == 0 {
		t.Fatal("no plugin.yaml scaffold: blocks found to pin")
	}
}

// TestPluginYamlStateScaffoldMatchDeclarations pins each state backend plugin.yaml's
// scaffold.config block to StateBackendScaffold (the built-in declaration), matched
// by the plugin.yaml's own id — so the external declaration cannot drift from code.
func TestPluginYamlStateScaffoldMatchDeclarations(t *testing.T) {
	type yamlState struct {
		ID       string `yaml:"id"`
		Scaffold struct {
			Config []struct {
				Key   string `yaml:"key"`
				Value string `yaml:"value"`
			} `yaml:"config"`
		} `yaml:"scaffold"`
	}
	statesDir := filepath.Join("..", "..", "..", "extensions", "states")
	entries, err := os.ReadDir(statesDir)
	if err != nil {
		t.Fatalf("read states dir: %v", err)
	}
	checked := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(statesDir, e.Name(), "plugin.yaml")
		data, statErr := os.ReadFile(path)
		if statErr != nil {
			continue // helper packages (awsauth, cmd) are not plugins
		}
		var m yamlState
		if err := yaml.Unmarshal(data, &m); err != nil {
			t.Fatalf("%s: parse plugin.yaml: %v", path, err)
		}
		declared := StateBackendScaffold(m.ID)
		if len(m.Scaffold.Config) != len(declared) {
			t.Errorf("state %s: plugin.yaml declares %d config lines, code declares %d", m.ID, len(m.Scaffold.Config), len(declared))
			continue
		}
		for i, c := range declared {
			y := m.Scaffold.Config[i]
			if y.Key != c.Key || y.Value != c.Value {
				t.Errorf("state %s: config line %d differs: plugin.yaml={%s:%s} code={%s:%s}", m.ID, i, y.Key, y.Value, c.Key, c.Value)
			}
		}
		if len(declared) > 0 {
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no state plugin.yaml scaffold.config blocks found to pin")
	}
}

// TestPluginYamlKindScaffoldComments checks that runtime, simulator, and router
// plugin.yaml each carry a non-empty scaffold.comment. These kinds are not consumed
// by `runfabric init`, so the block is documentation / external-plugin authoring
// reference only — this is a presence + well-formed check, not a pin to Go (there
// is deliberately no built-in declaration to pin against for non-consumed kinds).
func TestPluginYamlKindScaffoldComments(t *testing.T) {
	type yc struct {
		Scaffold struct {
			Comment string `yaml:"comment"`
		} `yaml:"scaffold"`
	}
	root := func(parts ...string) string {
		return filepath.Join(append([]string{"..", "..", "..", "extensions"}, parts...)...)
	}
	check := func(path string) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var m yc
		if err := yaml.Unmarshal(data, &m); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if strings.TrimSpace(m.Scaffold.Comment) == "" {
			t.Errorf("%s: no scaffold comment declared", path)
		}
	}

	for _, d := range []string{"nodejs", "python"} {
		check(root("runtimes", d, "plugin.yaml"))
	}
	check(root("simulators", "plugin.yaml"))

	routerDir := root("routers")
	entries, err := os.ReadDir(routerDir)
	if err != nil {
		t.Fatalf("read routers dir: %v", err)
	}
	routers := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(routerDir, e.Name(), "plugin.yaml")
		if _, statErr := os.Stat(p); statErr != nil {
			continue
		}
		check(p)
		routers++
	}
	if routers == 0 {
		t.Fatal("no router plugin.yaml found to check")
	}
}
