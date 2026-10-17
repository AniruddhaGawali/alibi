package utils

import "testing"

func TestLoadEnvReportsMissingVariable(t *testing.T) {
	t.Setenv("ALIBI_TEST_PRESENT", "x")
	t.Setenv("ALIBI_TEST_MISSING", "")

	if err := LoadEnv([]string{"ALIBI_TEST_PRESENT"}, "does-not-exist.env"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := LoadEnv([]string{"ALIBI_TEST_PRESENT", "ALIBI_TEST_MISSING"}, "does-not-exist.env"); err == nil {
		t.Fatal("expected error for missing variable")
	}
}
