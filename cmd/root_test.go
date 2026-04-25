package cmd

import "testing"

func TestValidateConcurrencyRejectsNonPositiveValues(t *testing.T) {
	for _, value := range []int{0, -1} {
		t.Run("value", func(t *testing.T) {
			if err := validateConcurrency(value); err == nil {
				t.Fatalf("validateConcurrency(%d) expected error", value)
			}
		})
	}
}

func TestValidateConcurrencyAcceptsPositiveValue(t *testing.T) {
	if err := validateConcurrency(1); err != nil {
		t.Fatalf("validateConcurrency(1) error: %v", err)
	}
}
