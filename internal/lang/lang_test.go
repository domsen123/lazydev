package lang

import (
	"reflect"
	"slices"
	"testing"
)

func TestParse(t *testing.T) {
	for in, want := range map[string]Lang{"": EN, "en": EN, "de": DE} {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %q, %v, want %q", in, got, err, want)
		}
	}
	if _, err := Parse("fr"); err == nil || err.Error() != `language must be en or de, not "fr"` {
		t.Errorf("Parse(fr) = %v", err)
	}
}

func TestVocabComplete(t *testing.T) {
	if len(All()) != 2 {
		t.Fatalf("All() has %d vocabularies", len(All()))
	}
	for _, v := range All() {
		rv := reflect.ValueOf(v)
		for i := range rv.NumField() {
			if rv.Field(i).String() == "" {
				t.Errorf("%s: field %s is empty", v.Lang, rv.Type().Field(i).Name)
			}
		}
		if For(v.Lang) != v {
			t.Errorf("For(%s) differs from All()", v.Lang)
		}
	}
	if For("fr") != For(Default) {
		t.Error("For of an unknown language is not Default")
	}
	if All()[0].Lang != Default {
		t.Error("All() does not start with Default")
	}
}

func TestUnique(t *testing.T) {
	de := Unique(DE)
	for _, w := range []string{"geplant", "Nach:", "keine", "Schritt:", "Kosten:"} {
		if !slices.Contains(de, w) {
			t.Errorf("Unique(DE) lacks %q", w)
		}
	}
	for _, w := range []string{"## Blocker", "de", "en"} {
		if slices.Contains(de, w) {
			t.Errorf("Unique(DE) has %q", w)
		}
	}
	if !slices.Contains(Unique(EN), "planned") {
		t.Error("Unique(EN) lacks planned")
	}
}
