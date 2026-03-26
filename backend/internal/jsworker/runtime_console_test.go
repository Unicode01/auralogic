package jsworker

import (
	"strings"
	"testing"

	"github.com/dop251/goja"
)

func TestRewriteRuntimeConsoleHelperCallsOnlyRewritesBareHelperCalls(t *testing.T) {
	input := `obj.help(); help(); ahelp(); globalThis.help(); const output = inspect(value, 2);`
	rewritten := rewriteRuntimeConsoleHelperCalls(input)

	if !strings.Contains(rewritten, `obj.help();`) {
		t.Fatalf("expected member access to remain untouched, got %q", rewritten)
	}
	if !strings.Contains(rewritten, runtimeConsoleHelperHelpName+`();`) {
		t.Fatalf("expected bare help() call to be rewritten, got %q", rewritten)
	}
	if strings.Contains(rewritten, `__auralogicConsoleahelp`) || !strings.Contains(rewritten, `ahelp();`) {
		t.Fatalf("expected identifier substring ahelp() to remain untouched, got %q", rewritten)
	}
	if !strings.Contains(rewritten, `globalThis.help();`) {
		t.Fatalf("expected globalThis.help() member access to remain untouched, got %q", rewritten)
	}
	if !strings.Contains(rewritten, runtimeConsoleHelperInspectName+`(value, 2)`) {
		t.Fatalf("expected inspect() call to be rewritten, got %q", rewritten)
	}
}

func TestBuildRuntimeConsolePreviewSupportsSiblingReuseAndCircularRefs(t *testing.T) {
	vm := goja.New()
	value, err := vm.RunString(`(function () {
		var shared = { value: 1 };
		var root = { left: shared, right: shared };
		root.self = root;
		return root;
	})()`)
	if err != nil {
		t.Fatalf("build preview fixture failed: %v", err)
	}

	preview := buildRuntimeConsolePreview(vm, value, 2)
	if preview.Type != "object" {
		t.Fatalf("expected root object preview, got %#v", preview)
	}

	entries := map[string]runtimeConsolePreview{}
	for _, entry := range preview.Entries {
		entries[entry.Key] = entry.Value
	}
	if entries["left"].Type != "object" {
		t.Fatalf("expected left sibling to stay expandable, got %#v", entries["left"])
	}
	if entries["right"].Type != "object" {
		t.Fatalf("expected right sibling to stay expandable, got %#v", entries["right"])
	}
	if entries["self"].Type != "circular" {
		t.Fatalf("expected self reference to be circular, got %#v", entries["self"])
	}
}

func TestBuildRuntimeConsolePreviewUsesEllipsisForDepthLimitedObjects(t *testing.T) {
	vm := goja.New()
	value, err := vm.RunString(`({ items: [{ id: 1, order_no: "ORD-1" }, { id: 2, order_no: "ORD-2" }] })`)
	if err != nil {
		t.Fatalf("build preview fixture failed: %v", err)
	}

	preview := buildRuntimeConsolePreview(vm, value, 2)
	if !strings.Contains(preview.Summary, `items: [{...}, {...}]`) {
		t.Fatalf("expected array item objects to use ellipsis summary, got %q", preview.Summary)
	}
}

func TestBuildRuntimeConsolePreviewKeepsEmptyObjectsDistinctFromCollapsedObjects(t *testing.T) {
	vm := goja.New()
	value, err := vm.RunString(`({ empty: {}, nested: { id: 1 } })`)
	if err != nil {
		t.Fatalf("build preview fixture failed: %v", err)
	}

	preview := buildRuntimeConsolePreview(vm, value, 1)
	if !strings.Contains(preview.Summary, `empty: {}`) {
		t.Fatalf("expected empty objects to stay {}, got %q", preview.Summary)
	}
	if !strings.Contains(preview.Summary, `nested: {...}`) {
		t.Fatalf("expected non-empty depth-limited objects to become {...}, got %q", preview.Summary)
	}
}

func TestFormatRuntimeConsoleLogOutputUsesCheapGojaStringFormatting(t *testing.T) {
	vm := goja.New()
	values := []goja.Value{
		goja.Undefined(),
		goja.Null(),
		vm.ToValue("alpha"),
		vm.ToValue(42),
	}
	objectValue, err := vm.RunString(`({ beta: true })`)
	if err != nil {
		t.Fatalf("build object fixture failed: %v", err)
	}
	values = append(values, objectValue)

	if got := formatRuntimeConsoleLogOutput(values); got != "undefined null alpha 42 [object Object]" {
		t.Fatalf("unexpected console log fallback output: %q", got)
	}
}
