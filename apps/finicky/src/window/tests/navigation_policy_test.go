package windowtests

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// Exercise the production Objective-C policy with Foundation's actual URL
// parser, including origin confusion and navigation from untrusted frames.
func TestNativeSettingsNavigationPolicy(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "navigation-policy-tests")
	compile := exec.Command("/usr/bin/clang", "-fobjc-arc", "-framework", "Foundation", "../navigation_policy.m", "testdata/navigation_policy_test.m", "-o", binary)
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("compile native policy tests: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("native settings policy: %v\n%s", err, output)
	}
}
