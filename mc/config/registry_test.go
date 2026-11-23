package config

import (
	"strings"
	"testing"
)

// registryBase composes a valid config_version 4 config plus a caller-supplied
// registry block, so each test varies only the registry under scrutiny.
func registryBase(block string) []byte {
	return []byte(runtimesExample + "\n" + block + "\n")
}

func TestRegistryAcceptsAControlledEndpoint(t *testing.T) {
	cfg, err := Parse(registryBase(`
[runtimes.registry.vllm]
base_url = "https://inference.example.com"
`))
	if err != nil {
		t.Fatalf("a valid registry entry was refused: %v", err)
	}
	entry, ok := cfg.Runtimes.Registry["vllm"]
	if !ok || entry.BaseURL != "https://inference.example.com" {
		t.Fatalf("registry entry not stored: %+v", cfg.Runtimes.Registry)
	}
}

func TestRegistryRejectsMalformedEndpoints(t *testing.T) {
	cases := map[string]string{
		"http scheme":         "[runtimes.registry.a]\nbase_url = \"http://x.example.com\"\n",
		"userinfo":            "[runtimes.registry.a]\nbase_url = \"https://u:p@x.example.com\"\n",
		"missing base_url":    "[runtimes.registry.a]\npooling = \"off\"\n",
		"loopback ip":         "[runtimes.registry.a]\nbase_url = \"https://127.0.0.1\"\n",
		"private ip":          "[runtimes.registry.a]\nbase_url = \"https://10.1.2.3\"\n",
		"link-local ip":       "[runtimes.registry.a]\nbase_url = \"https://169.254.1.1\"\n",
		"unspecified ip":      "[runtimes.registry.a]\nbase_url = \"https://0.0.0.0\"\n",
		"port off allowlist":  "[runtimes.registry.a]\nbase_url = \"https://x.example.com:8443\"\n",
		"bad pooling":         "[runtimes.registry.a]\nbase_url = \"https://x.example.com\"\npooling = \"always\"\n",
		"insecure key":        "[runtimes.registry.a]\nbase_url = \"https://x.example.com\"\ninsecure_ok = true\n",
		"insecure_ok v01 key": "[runtimes.registry.a]\nbase_url = \"https://x.example.com\"\ninsecure = true\n",
		"invalid entry id":    "[runtimes.registry.\"bad id\"]\nbase_url = \"https://x.example.com\"\n",
	}
	for name, block := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(registryBase(block)); err == nil {
				t.Fatalf("registry mutation %q was accepted:\n%s", name, block)
			}
		})
	}
}

func TestRegistryPortAllowedWhenListed(t *testing.T) {
	cfg, err := Parse(registryBase(`
[runtimes.registry.a]
base_url = "https://x.example.com:8443"
port_allowlist = [8443]
`))
	if err != nil {
		t.Fatalf("an explicitly allowed port was refused: %v", err)
	}
	if _, ok := cfg.Runtimes.Registry["a"]; !ok {
		t.Fatal("entry missing after accept")
	}
}

func TestRegistryEmptyIsValid(t *testing.T) {
	// A config without any registry block must be unchanged in behavior.
	if _, err := Parse([]byte(runtimesExample)); err != nil {
		t.Fatalf("empty registry should be valid: %v", err)
	}
}

func TestRegistryRejectReasonIsSpecific(t *testing.T) {
	_, err := Parse(registryBase("[runtimes.registry.a]\nbase_url = \"http://x.example.com\"\n"))
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("expected an https-specific refusal, got %v", err)
	}
}
