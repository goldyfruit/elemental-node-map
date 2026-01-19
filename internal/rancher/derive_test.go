package rancher

import "testing"

func TestInventoryURLFromServer(t *testing.T) {
	url, err := InventoryURLFromServer("https://rancher.example.com/k8s/clusters/local")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "https://rancher.example.com" + DefaultInventoryPath
	if url != expected {
		t.Fatalf("expected %s, got %s", expected, url)
	}
}

func TestBaseURLFromServerSubpath(t *testing.T) {
	base, err := BaseURLFromServer("https://rancher.example.com/rancher/k8s/clusters/c-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "https://rancher.example.com/rancher"
	if base != expected {
		t.Fatalf("expected %s, got %s", expected, base)
	}
}
