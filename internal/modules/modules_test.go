package modules

import (
	"testing"
	"github.com/COMPANYNAMEHERE/PerSSH/internal/common"
)

func TestRegistry(t *testing.T) {
	if len(Registry) == 0 {
		t.Fatal("Registry is empty")
	}

	for _, mod := range Registry {
		if mod.Name() == "" {
			t.Errorf("Module %v has empty name", mod)
		}
		
		defaults := mod.GetDefaults()
		if defaults.Type != mod.Type() {
			t.Errorf("Module %s type mismatch: got %v, want %v", mod.Name(), defaults.Type, mod.Type())
		}
		
		if mod.Type() == common.EnvTypeMinecraft {
			if defaults.Image != "itzg/minecraft-server" {
				t.Errorf("Minecraft default image incorrect: %s", defaults.Image)
			}
		}
	}
}
