// SPDX-License-Identifier: Apache-2.0

// Package logging builds the logger every service uses.
//
// Defaults, each overridable with controller-runtime's zap flags:
//   - JSON at info level, with no sampling, in production.
//   - Console encoding at debug level in development (--zap-devel).
//   - Stack traces at error level in production, warn level in development.
//   - RFC 3339 timestamps in UTC, to the nanosecond.
//   - Kubernetes objects formatted as controller-runtime formats them.
//   - Every line carries build_version and instrumentation_version. Call
//     buildinfo.SetVersion before building the logger if you use it.
//
// This package does not redact. Don't log secrets.
//
// klog is not redirected, so client-go's own lines, such as leader election,
// keep klog's format. Pass the logger to klog.SetLogger to redirect them.
package logging

import (
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	uberzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"go.miloapis.com/instrumentation/buildinfo"
	"go.miloapis.com/instrumentation/labels"
)

// Option configures New and NewRaw. None are defined yet; the parameter
// exists so options can be added without breaking callers.
type Option func(*options)

type options struct{}

// New returns a logr.Logger built from opts with the package defaults applied.
// Bind opts to flags with opts.BindFlags before parsing them. A nil opts uses
// the defaults.
func New(opts *zap.Options, o ...Option) logr.Logger {
	return zapr.NewLogger(NewRaw(opts, o...))
}

// NewRaw returns the underlying zap logger that New wraps.
func NewRaw(opts *zap.Options, _ ...Option) *uberzap.Logger {
	if opts == nil {
		opts = &zap.Options{}
	}
	o := *opts

	if o.DestWriter == nil {
		o.DestWriter = os.Stderr
	}

	zapOpts := append([]uberzap.Option{}, o.ZapOpts...)
	if o.Development {
		if o.NewEncoder == nil {
			o.NewEncoder = consoleEncoder
		}
		if o.Level == nil {
			o.Level = uberzap.NewAtomicLevelAt(zapcore.DebugLevel)
		}
		if o.StacktraceLevel == nil {
			o.StacktraceLevel = uberzap.NewAtomicLevelAt(zapcore.WarnLevel)
		}
		zapOpts = append(zapOpts, uberzap.Development())
	} else {
		if o.NewEncoder == nil {
			o.NewEncoder = jsonEncoder
		}
		if o.Level == nil {
			o.Level = uberzap.NewAtomicLevelAt(zapcore.InfoLevel)
		}
		if o.StacktraceLevel == nil {
			o.StacktraceLevel = uberzap.NewAtomicLevelAt(zapcore.ErrorLevel)
		}
	}

	if o.TimeEncoder == nil {
		o.TimeEncoder = utcRFC3339Nano
	}
	encoderOpts := append([]zap.EncoderConfigOption{func(c *zapcore.EncoderConfig) {
		c.EncodeTime = o.TimeEncoder
	}}, o.EncoderConfigOptions...)

	encoder := o.Encoder
	if encoder == nil {
		encoder = o.NewEncoder(encoderOpts...)
	}

	sink := zapcore.AddSync(o.DestWriter)
	zapOpts = append(zapOpts, uberzap.AddStacktrace(o.StacktraceLevel), uberzap.ErrorOutput(sink))

	core := zapcore.NewCore(&zap.KubeAwareEncoder{Encoder: encoder, Verbose: o.Development}, sink, o.Level)
	return uberzap.New(core, zapOpts...).With(
		uberzap.String(labels.BuildVersion, buildinfo.Version()),
		uberzap.String(labels.InstrumentationVersion, buildinfo.InstrumentationVersion()),
	)
}

func utcRFC3339Nano(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.UTC().Format(time.RFC3339Nano))
}

func jsonEncoder(opts ...zap.EncoderConfigOption) zapcore.Encoder {
	c := uberzap.NewProductionEncoderConfig()
	for _, opt := range opts {
		opt(&c)
	}
	return zapcore.NewJSONEncoder(c)
}

func consoleEncoder(opts ...zap.EncoderConfigOption) zapcore.Encoder {
	c := uberzap.NewDevelopmentEncoderConfig()
	for _, opt := range opts {
		opt(&c)
	}
	return zapcore.NewConsoleEncoder(c)
}
