// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"go.miloapis.com/instrumentation/buildinfo"
)

func newTestLogger(t *testing.T, args ...string) (logr.Logger, *bytes.Buffer) {
	t.Helper()
	opts := zap.Options{}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	opts.BindFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	buf := &bytes.Buffer{}
	opts.DestWriter = buf
	return New(&opts), buf
}

func jsonLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("line is not JSON: %s", line)
		}
		entries = append(entries, entry)
	}
	return entries
}

func TestDefaultsLogInfoAsJSONWithStacktraceOnlyOnError(t *testing.T) {
	logger, buf := newTestLogger(t)

	logger.V(1).Info("routine detail")
	logger.Info("something happened", "key", "value")
	logger.Error(errors.New("boom"), "something failed")

	entries := jsonLines(t, buf)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2: %v", len(entries), entries)
	}

	info, failure := entries[0], entries[1]
	for k, want := range map[string]any{"level": "info", "msg": "something happened", "key": "value"} {
		if info[k] != want {
			t.Errorf("info entry %s = %v, want %v", k, info[k], want)
		}
	}
	if _, ok := info["stacktrace"]; ok {
		t.Error("info entry has a stacktrace")
	}
	for k, want := range map[string]any{"level": "error", "msg": "something failed", "error": "boom"} {
		if failure[k] != want {
			t.Errorf("error entry %s = %v, want %v", k, failure[k], want)
		}
	}
	if _, ok := failure["stacktrace"]; !ok {
		t.Error("error entry has no stacktrace")
	}
}

func TestEveryLineCarriesBuildVersions(t *testing.T) {
	logger, buf := newTestLogger(t)

	logger.Info("first")
	logger.WithName("child").WithValues("version", "v1beta1").Error(errors.New("boom"), "second")

	entries := jsonLines(t, buf)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	for _, e := range entries {
		if e["build_version"] != buildinfo.Version() {
			t.Errorf("%v: build_version = %v, want %s", e["msg"], e["build_version"], buildinfo.Version())
		}
		if e["instrumentation_version"] != buildinfo.InstrumentationVersion() {
			t.Errorf("%v: instrumentation_version = %v, want %s", e["msg"], e["instrumentation_version"], buildinfo.InstrumentationVersion())
		}
	}
}

func TestApplicationVersionKeyDoesNotCollide(t *testing.T) {
	logger, buf := newTestLogger(t)

	logger.Info("reconciled", "version", "v1beta1")

	if n := strings.Count(buf.String(), `"version":`); n != 1 {
		t.Errorf("line has %d version keys, want 1: %s", n, buf.String())
	}
}

func TestTimestampsAreUTCWithNanoseconds(t *testing.T) {
	logger, buf := newTestLogger(t)

	logger.Info("now")

	ts, _ := jsonLines(t, buf)[0]["ts"].(string)
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t.Fatalf("ts %q is not RFC 3339: %v", ts, err)
	}
	if !strings.HasSuffix(ts, "Z") || parsed.Nanosecond() == 0 && !strings.Contains(ts, ".") {
		t.Errorf("ts %q is not UTC with sub-second precision", ts)
	}
}

func TestNilOptionsUseDefaults(t *testing.T) {
	if NewRaw(nil) == nil {
		t.Fatal("NewRaw(nil) returned nil")
	}
}

func TestDefaultsDoNotSample(t *testing.T) {
	logger, buf := newTestLogger(t)

	const n = 200
	for range n {
		logger.Info("identical line")
	}

	if got := len(jsonLines(t, buf)); got != n {
		t.Errorf("got %d lines, want %d", got, n)
	}
}

func TestDefaultsLogKubernetesObjectsByReference(t *testing.T) {
	logger, buf := newTestLogger(t)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "creds"},
		Data:       map[string][]byte{"password": []byte("hunter2")},
	}
	logger.Info("saw object", "object", secret)

	for _, leaked := range []string{"hunter2", "aHVudGVyMg=="} {
		if strings.Contains(buf.String(), leaked) {
			t.Errorf("output contains secret data %q", leaked)
		}
	}
	entries := jsonLines(t, buf)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	want := map[string]any{"namespace": "ns", "name": "creds"}
	if !reflect.DeepEqual(entries[0]["object"], want) {
		t.Errorf("object = %v, want %v", entries[0]["object"], want)
	}
}

func TestFlagsOverrideDefaults(t *testing.T) {
	t.Run("log level", func(t *testing.T) {
		logger, buf := newTestLogger(t, "--zap-log-level=debug")

		logger.V(1).Info("routine detail")

		entries := jsonLines(t, buf)
		if len(entries) != 1 || entries[0]["level"] != "debug" {
			t.Errorf("entries = %v, want one debug entry", entries)
		}
	})

	t.Run("console encoder", func(t *testing.T) {
		logger, buf := newTestLogger(t, "--zap-encoder=console")

		logger.Info("something happened")

		if !strings.Contains(buf.String(), "\tINFO\tsomething happened") {
			t.Errorf("output %q is not console encoded", buf.String())
		}
		if json.Valid(bytes.TrimSpace(buf.Bytes())) {
			t.Error("output is JSON")
		}
	})

	t.Run("stacktrace level", func(t *testing.T) {
		logger, buf := newTestLogger(t, "--zap-stacktrace-level=info")

		logger.Info("something happened")

		entries := jsonLines(t, buf)
		if len(entries) != 1 {
			t.Fatalf("got %d entries, want 1", len(entries))
		}
		if _, ok := entries[0]["stacktrace"]; !ok {
			t.Error("info entry has no stacktrace")
		}
	})

	t.Run("development mode", func(t *testing.T) {
		logger, buf := newTestLogger(t, "--zap-devel=true")

		logger.V(1).Info("routine detail")

		if !strings.Contains(buf.String(), "\tDEBUG\troutine detail") {
			t.Errorf("output %q is not a console debug line", buf.String())
		}
		if json.Valid(bytes.TrimSpace(buf.Bytes())) {
			t.Error("output is JSON")
		}
	})
}
