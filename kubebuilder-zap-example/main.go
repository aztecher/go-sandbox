package main

import (
	"flag"
	"os"
	"io"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	uzap "go.uber.org/zap"
	uzapcore "go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)


func zapDefaultOpts(opts ...zap.Opts) *zap.Options {
	newJSONEncoder := func(opts ...zap.EncoderConfigOption) uzapcore.Encoder {
		encoderConfig := uzap.NewProductionEncoderConfig()
		for _, opt := range opts {
			opt(&encoderConfig)
		}
		return uzapcore.NewJSONEncoder(encoderConfig)
	}

	addDefaults := func(o *zap.Options) {
		if o.DestWriter == nil {
			o.DestWriter = os.Stderr
		}

		if o.NewEncoder == nil {
			o.NewEncoder = newJSONEncoder
		}
		if o.Level == nil {
			lvl := uzap.NewAtomicLevelAt(uzap.InfoLevel)
			o.Level = &lvl
		}
		if o.StacktraceLevel == nil {
			lvl := uzap.NewAtomicLevelAt(uzap.ErrorLevel)
			o.StacktraceLevel = &lvl
		}
		// Disable sampling for increased Debug levels. Otherwise, this will
		// cause index out of bounds errors in the sampling code.
		if !o.Level.Enabled(uzapcore.Level(-2)) {
			o.ZapOpts = append(o.ZapOpts,
				uzap.WrapCore(func(core uzapcore.Core) uzapcore.Core {
					return uzapcore.NewSamplerWithOptions(core, time.Second, 100, 100)
				}))
		}

		if o.TimeEncoder == nil {
			o.TimeEncoder = uzapcore.RFC3339TimeEncoder
		}
		f := func(ecfg *uzapcore.EncoderConfig) {
			ecfg.EncodeTime = o.TimeEncoder
		}
		// prepend instead of append it in case someone adds a time encoder option in it
		o.EncoderConfigOptions = append([]zap.EncoderConfigOption{f}, o.EncoderConfigOptions...)

		if o.Encoder == nil {
			o.Encoder = o.NewEncoder(o.EncoderConfigOptions...)
		}
		o.ZapOpts = append(o.ZapOpts, uzap.AddStacktrace(o.StacktraceLevel))
	}

	o := &zap.Options{}
	for _, opt := range opts {
		opt(o)
	}
	addDefaults(o)
	return o
}

func defaultLevel() uzapcore.LevelEnabler {
	lvl := uzap.NewAtomicLevelAt(uzap.InfoLevel)
	return &lvl
}

func defaultStacktraceLevel() uzapcore.LevelEnabler {
	lvl := uzap.NewAtomicLevelAt(uzap.ErrorLevel)
	return &lvl
}

func zapConsoleOpts(level, stacktraceLevel uzapcore.LevelEnabler) *zap.Options {
	opts := zap.Options{
		DestWriter: os.Stderr,
		Level: level,
		StacktraceLevel: stacktraceLevel,
	}
	return zapDefaultOpts(zap.UseFlagOptions(&opts))
}

func zapFileOpts(w io.Writer, level, stacktraceLevel uzapcore.LevelEnabler) *zap.Options {
	opts := zap.Options {
		DestWriter: w,
		Level: level,
		StacktraceLevel: stacktraceLevel,
	}
	return zapDefaultOpts(zap.UseFlagOptions(&opts))
}

func zapOptsDefault(level, stacktraceLevel uzapcore.LevelEnabler) []uzap.Option {
	var opts []uzap.Option
	if !level.Enabled(uzapcore.Level(-2)) {
		opts = append(opts, uzap.WrapCore(func(core uzapcore.Core) uzapcore.Core {
			return uzapcore.NewSamplerWithOptions(core, time.Second, 100, 100)
		}))
	}
	opts = append(opts, uzap.AddStacktrace(stacktraceLevel))
	return opts
}

func zapNew(w io.Writer) logr.Logger {
	zapOpts := zapOptsDefault(defaultLevel(), defaultStacktraceLevel())
	consoleCore := NewRawCore(zapConsoleOpts(defaultLevel(), defaultStacktraceLevel()))
	fileCore := NewRawCore(zapFileOpts(w, defaultLevel(), defaultStacktraceLevel()))
	log := uzap.New(uzapcore.NewTee(consoleCore, fileCore))
	log = log.WithOptions(zapOpts...)
	return zapr.NewLogger(log)
}

func NewRawCore(o *zap.Options) uzapcore.Core {
	sink := uzapcore.AddSync(o.DestWriter)
	o.ZapOpts = append(o.ZapOpts, uzap.ErrorOutput(sink))
	return uzapcore.NewCore(&zap.KubeAwareEncoder{Encoder: o.Encoder, Verbose: o.Development}, sink, o.Level)
}

func NewRaw(o *zap.Options) *uzap.Logger {
	sink := uzapcore.AddSync(o.DestWriter)
	o.ZapOpts = append(o.ZapOpts, uzap.ErrorOutput(sink))
	log := uzap.New(uzapcore.NewTee(
		uzapcore.NewCore(&zap.KubeAwareEncoder{Encoder: o.Encoder, Verbose: o.Development}, sink, o.Level),
	))
	log = log.WithOptions(o.ZapOpts...)
	return log
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var namespace string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&namespace, "namespace", "", "The namespace that the manager targeted.")
	logfile, _ := os.OpenFile("zap.log", os.O_WRONLY|os.O_CREATE, 0666)
	defer logfile.Close()
	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zapNew(logfile))
	// ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	// ctrl.SetLogger(zapNew(zap.UseFlagOptions(&opts)))

	ctx := ctrl.SetupSignalHandler()
	logger := log.FromContext(ctx)
	logger.Info("sample", "key", "value")
}
