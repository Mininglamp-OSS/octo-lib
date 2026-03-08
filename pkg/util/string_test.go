package util

import (
	"testing"
)

func TestGetRandomName(t *testing.T) {
	// Test that all names can be selected (including the last one)
	// Run multiple iterations to statistically verify the last element is reachable
	lastElement := names[len(names)-1]
	foundLast := false

	// Run enough iterations to have high probability of hitting the last element
	for i := 0; i < 10000; i++ {
		name := GetRandomName()
		if name == lastElement {
			foundLast = true
			break
		}
	}

	if !foundLast {
		t.Errorf("GetRandomName() never returned the last element %q after 10000 iterations", lastElement)
	}
}

func TestGetRandomNameReturnsValidName(t *testing.T) {
	for i := 0; i < 100; i++ {
		name := GetRandomName()
		found := false
		for _, n := range names {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GetRandomName() returned invalid name: %q", name)
		}
	}
}
