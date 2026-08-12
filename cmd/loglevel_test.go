package main

import (
	"context"
	"testing"

	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	crzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input   string
		want    zapcore.Level
		wantErr bool
	}{
		{"debug", uzap.DebugLevel, false},
		{"DEBUG", uzap.DebugLevel, false},
		{"info", uzap.InfoLevel, false},
		{"error", uzap.ErrorLevel, false},
		{"panic", uzap.PanicLevel, false},
		{"1", zapcore.Level(-1), false},
		{"3", zapcore.Level(-3), false},
		{"", 0, true},
		{"invalid", 0, true},
		{"0", 0, true},
		{"-1", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseLogLevel(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseLogLevel(%q) expected error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseLogLevel(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractAtomicLevel_Default(t *testing.T) {
	opts := &crzap.Options{}
	level := extractAtomicLevel(opts)
	if level.Level() != uzap.InfoLevel {
		t.Fatalf("default level = %v, want info", level.Level())
	}
}

func TestExtractAtomicLevel_FromFlagValue(t *testing.T) {
	flagLevel := uzap.NewAtomicLevelAt(uzap.DebugLevel)
	opts := &crzap.Options{Level: flagLevel}

	level := extractAtomicLevel(opts)
	if level.Level() != uzap.DebugLevel {
		t.Fatalf("level = %v, want debug", level.Level())
	}
}

func TestExtractAtomicLevel_FromPointer(t *testing.T) {
	flagLevel := uzap.NewAtomicLevelAt(uzap.ErrorLevel)
	opts := &crzap.Options{Level: &flagLevel}

	level := extractAtomicLevel(opts)
	if level.Level() != uzap.ErrorLevel {
		t.Fatalf("level = %v, want error", level.Level())
	}
}

func TestRuntimeLogLevelChange(t *testing.T) {
	atomicLevel := uzap.NewAtomicLevelAt(uzap.ErrorLevel)
	opts := crzap.Options{Level: &atomicLevel}
	logger := crzap.New(crzap.UseFlagOptions(&opts)).WithName("test")

	if logger.V(1).Enabled() {
		t.Fatal("expected verbosity 1 to be disabled at error level")
	}

	atomicLevel.SetLevel(uzap.DebugLevel)
	if !logger.V(1).Enabled() {
		t.Fatal("expected verbosity 1 to be enabled after switching to debug level")
	}
}

func TestDetectOperatorNamespace(t *testing.T) {
	t.Setenv(podNamespaceEnvVar, "mcp-lifecycle-operator-system")
	if got := detectOperatorNamespace(); got != "mcp-lifecycle-operator-system" {
		t.Fatalf("detectOperatorNamespace() = %q, want mcp-lifecycle-operator-system", got)
	}
}

func TestLogLevelReconciler_UpdatesLevel(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 scheme: %v", err)
	}

	atomicLevel := uzap.NewAtomicLevelAt(uzap.InfoLevel)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcp-lifecycle-operator-config",
			Namespace: "system",
		},
		Data: map[string]string{
			"log-level": "debug",
		},
	}

	reconciler := &logLevelReconciler{
		Client:      fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build(),
		atomicLevel: atomicLevel,
		key:         "log-level",
	}

	if _, err := reconciler.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "system",
			Name:      "mcp-lifecycle-operator-config",
		},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if atomicLevel.Level() != uzap.DebugLevel {
		t.Fatalf("atomic level = %v, want debug", atomicLevel.Level())
	}
}

func TestLogLevelReconciler_InvalidLevelIsIgnored(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 scheme: %v", err)
	}

	atomicLevel := uzap.NewAtomicLevelAt(uzap.InfoLevel)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcp-lifecycle-operator-config",
			Namespace: "system",
		},
		Data: map[string]string{
			"log-level": "not-a-level",
		},
	}

	reconciler := &logLevelReconciler{
		Client:      fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build(),
		atomicLevel: atomicLevel,
		key:         "log-level",
	}

	if _, err := reconciler.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "system",
			Name:      "mcp-lifecycle-operator-config",
		},
	}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if atomicLevel.Level() != uzap.InfoLevel {
		t.Fatalf("atomic level = %v, want info to be preserved", atomicLevel.Level())
	}
}

func TestLogLevelReconciler_Idempotent(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 scheme: %v", err)
	}

	atomicLevel := uzap.NewAtomicLevelAt(uzap.DebugLevel)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcp-lifecycle-operator-config",
			Namespace: "system",
		},
		Data: map[string]string{
			"log-level": "debug",
		},
	}

	reconciler := &logLevelReconciler{
		Client:      fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build(),
		atomicLevel: atomicLevel,
		key:         "log-level",
	}

	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "system", Name: "mcp-lifecycle-operator-config"}}
	if _, err := reconciler.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("first Reconcile() error = %v", err)
	}
	if _, err := reconciler.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}

	if atomicLevel.Level() != uzap.DebugLevel {
		t.Fatalf("atomic level = %v, want debug", atomicLevel.Level())
	}
}
