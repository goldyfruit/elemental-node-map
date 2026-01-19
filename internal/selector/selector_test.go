package selector

import (
	"testing"

	"k8s.io/apimachinery/pkg/labels"
)

func TestParseSelector(t *testing.T) {
	selector, err := Parse("env=prod,node-role.kubernetes.io/worker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	set := labels.Set{"env": "prod", "node-role.kubernetes.io/worker": ""}
	if !selector.Matches(set) {
		t.Fatalf("expected selector to match labels")
	}
}

func TestParseSelectorNotEquals(t *testing.T) {
	selector, err := Parse("env!=prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	set := labels.Set{"env": "dev"}
	if !selector.Matches(set) {
		t.Fatalf("expected selector to match env!=prod")
	}
}

func TestParseSelectorExistsNotExists(t *testing.T) {
	selector, err := Parse("zone,!gpu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	set := labels.Set{"zone": "us-east-1"}
	if !selector.Matches(set) {
		t.Fatalf("expected selector to match zone and !gpu")
	}
}

func TestParseSelectorEmpty(t *testing.T) {
	selector, err := Parse("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !selector.Matches(labels.Set{"any": "value"}) {
		t.Fatalf("expected empty selector to match everything")
	}
}
