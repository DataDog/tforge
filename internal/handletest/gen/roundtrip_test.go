//go:build integration

// Package gen hosts a tiny generated SDK used to exercise the generator end
// to end: a synthetic schema (../schema.json) is fed through `tforge generate`
// at build time, and these tests call the resulting constructors to verify
// the marshal/unmarshal round-trip preserves field values.
//
// The *_gen.go files are not committed; `make handletest` (or
// `make update-snapshots`) runs `go generate` first to produce them and then
// `go test -tags integration` to run the tests defined here. Without the
// integration build tag this file is skipped, so `go test ./...` on a fresh
// checkout (where *_gen.go does not exist) still passes — the gen package
// is empty in that case.
package gen

import (
	"testing"

	"github.com/DataDog/tforge/config"
)

// TestResourceConstructor_RequiredString verifies that a required string
// attribute round-trips through the constructor into Resource.Attributes.
func TestResourceConstructor_RequiredString(t *testing.T) {
	stack := config.NewStack("rt")
	if err := NewWidget(stack, "basic", Widget_SetName("hello")); err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	if len(stack.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(stack.Resources))
	}
	r := stack.Resources[0]
	if r.Type != "test_widget" {
		t.Errorf("type = %q, want test_widget", r.Type)
	}
	if r.Name != "basic" {
		t.Errorf("name = %q, want basic", r.Name)
	}
	if r.Attributes["name"] != "hello" {
		t.Errorf("config[name] = %v, want hello", r.Attributes["name"])
	}
}

// TestResourceConstructor_OptionalBoolFalse pins the pointer-wrapping behavior
// for optional bools: Widget_SetEnabled(false) produces &false, which is NOT
// omitted by `json:",omitempty"` — Terraform "enabled = false" is meaningful
// and must round-trip distinctly from unset.
func TestResourceConstructor_OptionalBoolFalse(t *testing.T) {
	stack := config.NewStack("rt")
	if err := NewWidget(stack, "booltest",
		Widget_SetName("x"),
		Widget_SetEnabled(false),
	); err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	r := stack.Resources[0]
	if _, exists := r.Attributes["enabled"]; !exists {
		t.Error("config[enabled] should be present when explicitly set to false")
	} else if r.Attributes["enabled"] != false {
		t.Errorf("config[enabled] = %v, want false", r.Attributes["enabled"])
	}
}

// TestResourceConstructor_OptionalNumberZero is the float64 sibling of
// OptionalBoolFalse: count=0 differs from unset and must appear in the output.
func TestResourceConstructor_OptionalNumberZero(t *testing.T) {
	stack := config.NewStack("rt")
	if err := NewWidget(stack, "zerocount",
		Widget_SetName("x"),
		Widget_SetCount(0),
	); err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	r := stack.Resources[0]
	if _, exists := r.Attributes["count"]; !exists {
		t.Error("config[count] should be present when explicitly set to 0")
	} else if r.Attributes["count"] != float64(0) {
		t.Errorf("config[count] = %v, want 0", r.Attributes["count"])
	}
}

// TestResourceConstructor_OptionalBoolTrue is the positive case for OptionalBoolFalse.
func TestResourceConstructor_OptionalBoolTrue(t *testing.T) {
	stack := config.NewStack("rt")
	if err := NewWidget(stack, "truebool",
		Widget_SetName("y"),
		Widget_SetEnabled(true),
	); err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	r := stack.Resources[0]
	if r.Attributes["enabled"] != true {
		t.Errorf("config[enabled] = %v, want true", r.Attributes["enabled"])
	}
}

// TestResourceConstructor_NestedBlock verifies that a list-nesting nested
// block populated via NewWidgetSetting + Widget_SetSetting round-trips
// through Attributes as []any of map[string]any.
func TestResourceConstructor_NestedBlock(t *testing.T) {
	stack := config.NewStack("rt")
	if err := NewWidget(stack, "nested",
		Widget_SetName("z"),
		Widget_SetSetting(NewWidgetSetting(
			WidgetSetting_SetKey("k1"),
			WidgetSetting_SetValue("v1"),
		)),
	); err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	r := stack.Resources[0]
	settingRaw, ok := r.Attributes["setting"]
	if !ok {
		t.Fatal("config[setting] missing")
	}
	settingSlice, ok := settingRaw.([]any)
	if !ok {
		t.Fatalf("config[setting] is %T, want []any", settingRaw)
	}
	if len(settingSlice) != 1 {
		t.Fatalf("len(setting) = %d, want 1", len(settingSlice))
	}
	settingMap, ok := settingSlice[0].(map[string]any)
	if !ok {
		t.Fatalf("setting[0] is %T, want map[string]any", settingSlice[0])
	}
	if settingMap["key"] != "k1" {
		t.Errorf("setting[0].key = %v, want k1", settingMap["key"])
	}
	if settingMap["value"] != "v1" {
		t.Errorf("setting[0].value = %v, want v1", settingMap["value"])
	}
}

// TestResourceConstructor_OptionalUnset confirms that unset optionals do not
// leak into the rendered config — the `omitempty` tags pull their weight.
func TestResourceConstructor_OptionalUnset(t *testing.T) {
	stack := config.NewStack("rt")
	if err := NewWidget(stack, "minimal", Widget_SetName("only-name")); err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	r := stack.Resources[0]
	for _, key := range []string{"tags", "count", "enabled", "setting"} {
		if _, exists := r.Attributes[key]; exists {
			t.Errorf("config[%s] should be absent for unset optional, got %v", key, r.Attributes[key])
		}
	}
}

// TestDataSourceConstructor verifies the data-source constructor path
// (which differs from the resource path in the generator).
func TestDataSourceConstructor(t *testing.T) {
	stack := config.NewStack("ds")
	if err := NewLookupData(stack, "myds", LookupData_SetFilter("active")); err != nil {
		t.Fatalf("NewLookupData: %v", err)
	}
	if len(stack.DataSources) != 1 {
		t.Fatalf("expected 1 data source, got %d", len(stack.DataSources))
	}
	ds := stack.DataSources[0]
	if ds.Type != "test_lookup" {
		t.Errorf("type = %q, want test_lookup", ds.Type)
	}
	if ds.Name != "myds" {
		t.Errorf("name = %q, want myds", ds.Name)
	}
	if ds.Attributes["filter"] != "active" {
		t.Errorf("config[filter] = %v, want active", ds.Attributes["filter"])
	}
}

// TestFromBridge verifies the FromWidget(cfg) bridge: a populated Widget value
// is applied as an option, copying all set fields into the constructed resource.
func TestFromBridge(t *testing.T) {
	stack := config.NewStack("bridge")
	cfg := Widget{
		Name:    "direct",
		Enabled: Ptr(true),
		Tags:    []string{"a", "b"},
	}
	if err := NewWidget(stack, "bridged", FromWidget(cfg)); err != nil {
		t.Fatalf("NewWidget with FromWidget: %v", err)
	}
	r := stack.Resources[0]
	if r.Attributes["name"] != "direct" {
		t.Errorf("config[name] = %v, want direct", r.Attributes["name"])
	}
	if r.Attributes["enabled"] != true {
		t.Errorf("config[enabled] = %v, want true", r.Attributes["enabled"])
	}
	tagsRaw, ok := r.Attributes["tags"].([]any)
	if !ok {
		t.Fatalf("config[tags] is %T, want []any", r.Attributes["tags"])
	}
	if len(tagsRaw) != 2 || tagsRaw[0] != "a" || tagsRaw[1] != "b" {
		t.Errorf("config[tags] = %v, want [a b]", tagsRaw)
	}
}
