package model

import "testing"

func TestProviderTypeValid(t *testing.T) {
	valid := []ProviderType{ProviderOllama, ProviderOpenAI, ProviderAnthropic, ProviderCustom}
	for _, p := range valid {
		if !p.Valid() {
			t.Errorf("expected %q to be valid", p)
		}
	}
}

func TestProviderTypeInvalid(t *testing.T) {
	if ProviderType("invalid").Valid() {
		t.Error("expected invalid provider type to be invalid")
	}
}

func TestProviderConfigDefaults(t *testing.T) {
	pc := ProviderConfig{
		Provider:    "ollama",
		Enabled:     true,
		TimeoutMS:   60000,
		MaxRetries:  3,
	}
	if pc.Provider != "ollama" {
		t.Errorf("expected ollama, got %s", pc.Provider)
	}
	if !pc.Enabled {
		t.Error("expected enabled to be true")
	}
}
