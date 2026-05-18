//go:build integration

// Package gen hosts a tiny generated SDK used to exercise the generator end
// to end: a synthetic schema (../schema.json) is fed through `tforge generate`
// at build time, and these tests call the resulting constructors to verify
// the handle/Configure API preserves field values.
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

// widgetAttrs fetches the typed Widget payload from the first resource in the
// stack. Centralizes the AsResource + type-assert boilerplate.
func widgetAttrs(t *testing.T, stack *config.Stack) *Widget {
	t.Helper()
	if len(stack.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(stack.Resources))
	}
	r := stack.Resources[0].AsResource()
	w, ok := r.TypedAttributes.(*Widget)
	if !ok {
		t.Fatalf("TypedAttributes is %T, want *Widget", r.TypedAttributes)
	}
	return w
}

// TestResourceConstructor_RequiredString verifies that a required string
// attribute set via Configure round-trips through the typed payload.
func TestResourceConstructor_RequiredString(t *testing.T) {
	stack := config.NewStack("rt")
	w, err := NewWidget(stack, "basic")
	if err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	w.Configure(Widget_SetName("hello"))

	r := stack.Resources[0].AsResource()
	if r.Type != "test_widget" {
		t.Errorf("Type = %q, want test_widget", r.Type)
	}
	if r.Name != "basic" {
		t.Errorf("Name = %q, want basic", r.Name)
	}
	if got := widgetAttrs(t, stack).Name; got != "hello" {
		t.Errorf("Widget.Name = %q, want hello", got)
	}
}

// TestResourceConstructor_OptionalBoolFalse pins the pointer-wrapping behavior
// for optional bools: Widget_SetEnabled(false) produces &false, which is NOT
// omitted by `json:",omitempty"` — Terraform "enabled = false" is meaningful
// and must round-trip distinctly from unset.
func TestResourceConstructor_OptionalBoolFalse(t *testing.T) {
	stack := config.NewStack("rt")
	w, err := NewWidget(stack, "booltest")
	if err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	w.Configure(Widget_SetName("x"), Widget_SetEnabled(false))

	got := widgetAttrs(t, stack).Enabled
	if got == nil {
		t.Fatal("Widget.Enabled is nil, want pointer to false")
	}
	if *got != false {
		t.Errorf("Widget.Enabled = %v, want false", *got)
	}
}

// TestResourceConstructor_OptionalNumberZero is the float64 sibling of
// OptionalBoolFalse: count=0 differs from unset and must survive as &0.
func TestResourceConstructor_OptionalNumberZero(t *testing.T) {
	stack := config.NewStack("rt")
	w, err := NewWidget(stack, "zerocount")
	if err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	w.Configure(Widget_SetName("x"), Widget_SetCount(0))

	got := widgetAttrs(t, stack).Count
	if got == nil {
		t.Fatal("Widget.Count is nil, want pointer to 0")
	}
	if *got != 0 {
		t.Errorf("Widget.Count = %v, want 0", *got)
	}
}

// TestResourceConstructor_OptionalBoolTrue is the positive case for OptionalBoolFalse.
func TestResourceConstructor_OptionalBoolTrue(t *testing.T) {
	stack := config.NewStack("rt")
	w, err := NewWidget(stack, "truebool")
	if err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	w.Configure(Widget_SetName("y"), Widget_SetEnabled(true))

	got := widgetAttrs(t, stack).Enabled
	if got == nil {
		t.Fatal("Widget.Enabled is nil, want pointer to true")
	}
	if *got != true {
		t.Errorf("Widget.Enabled = %v, want true", *got)
	}
}

// TestResourceConstructor_NestedBlock verifies that a list-nesting nested
// block populated via NewWidgetSetting + Widget_SetSetting lands on the
// typed payload as a properly populated slice.
func TestResourceConstructor_NestedBlock(t *testing.T) {
	stack := config.NewStack("rt")
	w, err := NewWidget(stack, "nested")
	if err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	w.Configure(
		Widget_SetName("z"),
		Widget_SetSetting(NewWidgetSetting(
			WidgetSetting_SetKey("k1"),
			WidgetSetting_SetValue("v1"),
		)),
	)

	settings := widgetAttrs(t, stack).Setting
	if len(settings) != 1 {
		t.Fatalf("len(Setting) = %d, want 1", len(settings))
	}
	if settings[0].Key != "k1" {
		t.Errorf("Setting[0].Key = %q, want k1", settings[0].Key)
	}
	if settings[0].Value != "v1" {
		t.Errorf("Setting[0].Value = %q, want v1", settings[0].Value)
	}
}

// TestResourceConstructor_OptionalUnset confirms that unset optionals stay
// at their zero value on the typed payload — the renderer's omitempty tags
// drop them at marshal time.
func TestResourceConstructor_OptionalUnset(t *testing.T) {
	stack := config.NewStack("rt")
	w, err := NewWidget(stack, "minimal")
	if err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	w.Configure(Widget_SetName("only-name"))

	got := widgetAttrs(t, stack)
	if got.Enabled != nil {
		t.Errorf("Widget.Enabled = %v, want nil", *got.Enabled)
	}
	if got.Count != nil {
		t.Errorf("Widget.Count = %v, want nil", *got.Count)
	}
	if got.Tags != nil {
		t.Errorf("Widget.Tags = %v, want nil", got.Tags)
	}
	if got.Setting != nil {
		t.Errorf("Widget.Setting = %v, want nil", got.Setting)
	}
}

// TestDataSourceConstructor verifies the data-source constructor path
// (which differs from the resource path in the generator).
func TestDataSourceConstructor(t *testing.T) {
	stack := config.NewStack("ds")
	ld, err := NewLookupData(stack, "myds")
	if err != nil {
		t.Fatalf("NewLookupData: %v", err)
	}
	ld.Configure(LookupData_SetFilter("active"))

	if len(stack.DataSources) != 1 {
		t.Fatalf("expected 1 data source, got %d", len(stack.DataSources))
	}
	d := stack.DataSources[0].AsDataSource()
	if d.Type != "test_lookup" {
		t.Errorf("Type = %q, want test_lookup", d.Type)
	}
	if d.Name != "myds" {
		t.Errorf("Name = %q, want myds", d.Name)
	}
	got, ok := d.TypedAttributes.(*LookupData)
	if !ok {
		t.Fatalf("TypedAttributes is %T, want *LookupData", d.TypedAttributes)
	}
	if got.Filter != "active" {
		t.Errorf("Filter = %q, want active", got.Filter)
	}
}

// TestFromBridge verifies the FromWidget(cfg) bridge: a populated Widget value
// is applied as an option, copying all set fields onto the typed payload while
// preserving the back-pointer to the stack-owned Resource.
func TestFromBridge(t *testing.T) {
	stack := config.NewStack("bridge")
	w, err := NewWidget(stack, "bridged")
	if err != nil {
		t.Fatalf("NewWidget: %v", err)
	}
	cfg := Widget{
		Name:    "direct",
		Enabled: Ptr(true),
		Tags:    []string{"a", "b"},
	}
	w.Configure(FromWidget(cfg))

	got := widgetAttrs(t, stack)
	if got.Name != "direct" {
		t.Errorf("Widget.Name = %q, want direct", got.Name)
	}
	if got.Enabled == nil || *got.Enabled != true {
		t.Errorf("Widget.Enabled = %v, want pointer to true", got.Enabled)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "a" || got.Tags[1] != "b" {
		t.Errorf("Widget.Tags = %v, want [a b]", got.Tags)
	}
}
