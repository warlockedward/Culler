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
	if err == nil || !strings.Contains(err.Error(), "http") || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("expected a cleartext-address-class refusal, got %v", err)
	}
}

func TestRegistryAcceptsDeclaredCleartextLocalEndpoints(t *testing.T) {
	for _, block := range []string{
		"[runtimes.registry.a]\nbase_url = \"http://127.0.0.1\"\n",
		"[runtimes.registry.a]\nbase_url = \"http://192.168.1.5:8000\"\nport_allowlist = [8000]\n",
		"[runtimes.registry.a]\nbase_url = \"http://[::1]\"\n",
		"[runtimes.registry.a]\nbase_url = \"http://10.20.30.40:38011\"\nport_allowlist = [38011]\n",
	} {
		if _, err := Parse(registryBase(block)); err != nil {
			t.Fatalf("declared cleartext local endpoint refused: %v\n%s", err, block)
		}
	}
}

func TestRegistryRejectsMalformedCleartextEndpoints(t *testing.T) {
	cases := map[string]string{
		"cleartext hostname":    "[runtimes.registry.a]\nbase_url = \"http://x.example.com\"\n",
		"cleartext link-local":  "[runtimes.registry.a]\nbase_url = \"http://169.254.1.1\"\n",
		"cleartext unspecified": "[runtimes.registry.a]\nbase_url = \"http://0.0.0.0\"\n",
		"cleartext public ip":   "[runtimes.registry.a]\nbase_url = \"http://8.8.8.8\"\n",
	}
	for name, block := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(registryBase(block)); err == nil {
				t.Fatalf("cleartext mutation %q was accepted:\n%s", name, block)
			}
		})
	}
}

func TestOpenAICompatibleCredentialChain(t *testing.T) {
	valid := string(registryBase(`
[runtimes.registry.newapi]
base_url = "http://127.0.0.1:38011/v1"
port_allowlist = [38011]

[runtimes.credentials.selfhost]
kind = "openai-compatible"
registry_entry = "newapi"

[runtimes.bindings.selfhost]
harness = "codex"
model = "test-model"
credential = "selfhost"
`))
	cfg, err := Parse([]byte(valid))
	if err != nil {
		t.Fatalf("valid self-hosted chain refused: %v", err)
	}
	source := cfg.Runtimes.Credentials["selfhost"]
	if source.Delivery() != "proxy" || source.Channel() != "codex" || source.Harness() != "codex" {
		t.Fatalf("self-hosted kind routes wrong: %+v", source)
	}
	address, ok := cfg.CredentialAddress(source)
	if !ok || address != "http://127.0.0.1:38011/v1" {
		t.Fatalf("address not resolved from the registry entry: %q %v", address, ok)
	}
	for name, mutation := range map[string]string{
		"missing entry":   "registry_entry = \"\"\n",
		"undefined entry": "registry_entry = \"absent\"\n",
		"pinned url set":  "inference_url = \"https://api.openai.com/v1\"\n",
		"oauth fields":    "token_url = \"https://x\"\nclient_id = \"c\"\n",
		"catalog entry":   "catalog_entry = \"openrouter\"\n",
	} {
		block := strings.Replace(valid, "registry_entry = \"newapi\"", "registry_entry = \"newapi\"\n"+mutation, 1)
		if name == "missing entry" {
			block = strings.Replace(valid, "registry_entry = \"newapi\"", "", 1)
		}
		if _, err := Parse([]byte(block)); err == nil {
			t.Fatalf("self-hosted mutation %q was accepted", name)
		}
	}
}
