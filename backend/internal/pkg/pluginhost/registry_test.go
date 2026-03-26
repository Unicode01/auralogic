package pluginhost

import "testing"

func TestSharedActionDefinitionsRemainUnique(t *testing.T) {
	defs := ListSharedActionDefinitions()
	if len(defs) == 0 {
		t.Fatal("expected shared action definitions")
	}

	actions := map[string]struct{}{}
	jsPaths := map[string]struct{}{}
	for _, def := range defs {
		if def.Action == "" {
			t.Fatal("expected action key")
		}
		if len(def.PluginPermissions) == 0 {
			t.Fatalf("expected plugin permissions for %s", def.Action)
		}
		if len(def.OperatorPermissions) == 0 {
			t.Fatalf("expected operator permissions for %s", def.Action)
		}
		if len(def.JSImportPath) < 2 {
			t.Fatalf("expected JSImportPath for %s", def.Action)
		}

		if _, exists := actions[def.Action]; exists {
			t.Fatalf("duplicate action definition %s", def.Action)
		}
		actions[def.Action] = struct{}{}

		jsPath := joinPath(def.JSImportPath)
		if _, exists := jsPaths[jsPath]; exists {
			t.Fatalf("duplicate JSImportPath %s", jsPath)
		}
		jsPaths[jsPath] = struct{}{}

		lookup, ok := LookupSharedActionDefinition(def.Action)
		if !ok {
			t.Fatalf("expected lookup for %s", def.Action)
		}
		if lookup.Action != def.Action {
			t.Fatalf("expected lookup action %s, got %s", def.Action, lookup.Action)
		}
	}
}

func joinPath(parts []string) string {
	result := ""
	for index, part := range parts {
		if index > 0 {
			result += "."
		}
		result += part
	}
	return result
}
