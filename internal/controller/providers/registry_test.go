/*
Copyright 2026 The Kubernetes Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package providers

import (
	"fmt"
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"
)

func TestRegisterAndSetupAll(t *testing.T) {
	saved := registry
	t.Cleanup(func() { registry = saved })

	registry = nil

	var calls []string
	Register("alpha", func(mgr ctrl.Manager) error {
		calls = append(calls, "alpha")
		return nil
	})
	Register("beta", func(mgr ctrl.Manager) error {
		calls = append(calls, "beta")
		return nil
	})

	if len(registry) != 2 {
		t.Fatalf("expected 2 registered factories, got %d", len(registry))
	}

	if err := SetupAll(nil); err != nil {
		t.Fatalf("SetupAll() unexpected error: %v", err)
	}
	if len(calls) != 2 || calls[0] != "alpha" || calls[1] != "beta" {
		t.Fatalf("expected calls [alpha, beta], got %v", calls)
	}
}

func TestSetupAllError(t *testing.T) {
	saved := registry
	t.Cleanup(func() { registry = saved })

	registry = nil

	Register("failing", func(mgr ctrl.Manager) error {
		return fmt.Errorf("setup failed")
	})

	err := SetupAll(nil)
	if err == nil {
		t.Fatal("expected error from SetupAll")
	}
	if got := err.Error(); got != "provider failing: setup failed" {
		t.Fatalf("unexpected error message: %s", got)
	}
}
