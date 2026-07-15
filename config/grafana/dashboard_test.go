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

package grafana_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const dashboardFile = "mcp-lifecycle-operator.json"

// knownMetrics lists metric selectors the dashboard is allowed to reference.
// Keep in sync with internal/controller/metrics.go and controller-runtime defaults.
var knownMetrics = []string{
	"mcpserver_condition_info",
	"mcpserver_validation_failures_total",
	"mcpserver_deployment_failures_total",
	"mcpserver_service_failures_total",
	"mcpserver_networkpolicy_failures_total",
	"mcpserver_reconcile_phase_duration_seconds",
	"mcpserver_reconcile_phase_duration_seconds_bucket",
	"controller_runtime_reconcile_total",
	"controller_runtime_reconcile_errors_total",
	"controller_runtime_reconcile_time_seconds_bucket",
	"controller_runtime_active_workers",
	"process_cpu_seconds_total",
	"process_resident_memory_bytes",
	"rest_client_requests_total",
}

func TestDashboardJSONIsValid(t *testing.T) {
	path := filepath.Join(".", dashboardFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dashboard: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("dashboard is not valid JSON: %v", err)
	}

	for _, key := range []string{"title", "uid", "panels", "tags"} {
		if _, ok := root[key]; !ok {
			t.Errorf("dashboard missing required field %q", key)
		}
	}

	if root["uid"] != "mcp-lifecycle-operator" {
		t.Errorf("expected uid mcp-lifecycle-operator, got %v", root["uid"])
	}

	panels, ok := root["panels"].([]any)
	if !ok || len(panels) == 0 {
		t.Fatal("dashboard must contain panels")
	}
}

func TestDashboardQueriesReferenceKnownMetrics(t *testing.T) {
	path := filepath.Join(".", dashboardFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dashboard: %v", err)
	}
	content := string(data)

	exprs := extractExprs(data)
	if len(exprs) == 0 {
		t.Fatal("no PromQL expressions found in dashboard")
	}

	required := []string{
		"mcpserver_condition_info",
		"mcpserver_validation_failures_total",
		"mcpserver_deployment_failures_total",
		"mcpserver_service_failures_total",
		"mcpserver_networkpolicy_failures_total",
		"mcpserver_reconcile_phase_duration_seconds",
		"controller_runtime_reconcile_total",
		"controller_runtime_reconcile_errors_total",
		"controller_runtime_reconcile_time_seconds",
	}
	for _, metric := range required {
		if !strings.Contains(content, metric) {
			t.Errorf("dashboard missing required metric reference %q", metric)
		}
	}

	unimplemented := []string{
		"referenced_resources_count",
		"mcpserver_configmap",
		"mcpserver_secret",
	}
	for _, metric := range unimplemented {
		if strings.Contains(content, metric) {
			t.Errorf("dashboard references unimplemented metric %q", metric)
		}
	}

	selectorRE := regexp.MustCompile(`([a-zA-Z_:][a-zA-Z0-9_:]*)\s*[\{\[]`)
	seen := map[string]struct{}{}
	for _, expr := range exprs {
		for _, match := range selectorRE.FindAllStringSubmatch(expr, -1) {
			seen[match[1]] = struct{}{}
		}
	}

	allowed := make(map[string]struct{}, len(knownMetrics))
	for _, m := range knownMetrics {
		allowed[m] = struct{}{}
	}
	for name := range seen {
		if _, ok := allowed[name]; !ok {
			t.Errorf("expression uses unexpected metric selector %q", name)
		}
	}
}

func TestDashboardKustomizationBuilds(t *testing.T) {
	path := filepath.Join(".", dashboardFile)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("dashboard file missing for kustomize: %v", err)
	}
}

func extractExprs(data []byte) []string {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	var exprs []string
	walkJSON(root, func(key string, value string) {
		if key == "expr" {
			if strings.TrimSpace(value) != "" {
				exprs = append(exprs, value)
			}
		}
	})
	return exprs
}

func walkJSON(node any, fn func(key, value string)) {
	switch v := node.(type) {
	case map[string]any:
		for k, child := range v {
			switch c := child.(type) {
			case string:
				fn(k, c)
			default:
				walkJSON(c, fn)
			}
		}
	case []any:
		for _, child := range v {
			walkJSON(child, fn)
		}
	}
}
