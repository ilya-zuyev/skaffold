package perf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/slowjam/pkg/stacklog"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/sdk/trace"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
)

type SlowJamSpan struct {
	File     string
	SpanName string
}

const sjEnv = "SJ_PROFILE"
const pprofEnv = "PP_PROFILE"
const execEnv = "EXEC_PROFILE"

const profPathEnv = "PROF_PATH"
const otelGcpProjEnv = "OT_GCP_PROJ"

var sjEnabled = false
var execEnabled = false
var ppEnabled = false

var started = int64(0)

type execLogEntry struct {
	msg  string
	span string
}

var execLog = make(chan execLogEntry, 32)

var profPath = "."
var lastSpan = ""

func init() {
	if v, ok := os.LookupEnv(sjEnv); ok {
		if v != "0" {
			sjEnabled = true
		}
	}
	if v, ok := os.LookupEnv(pprofEnv); ok {
		if v != "0" {
			ppEnabled = true
		}
	}
	if v, ok := os.LookupEnv(execEnv); ok {
		if v != "0" {
			execEnabled = true
		}
	}

	if sjEnabled || ppEnabled || execEnabled {
		if p, ok := os.LookupEnv(profPathEnv); ok {
			profPath = p
		}
	}

	if execEnabled {
		f, err := os.Create(profileFile("exec", "log"))
		if err != nil {
			logrus.Warnf("failed to create logexec file: %v", err)
			execEnabled = false
		} else {
			go func() {
				for e := range execLog {
					fmt.Fprintf(f, "%v [%s] %s\n", fmtTime(time.Now()), e.span, e.msg)
				}
			}()
		}
	}

	if gcpProj, ok := os.LookupEnv(otelGcpProjEnv); ok {
		// start otel exporter
		gexp, err := texporter.NewExporter(texporter.WithProjectID(gcpProj))
		if err != nil {
			panic(err)
		}
		gtp := trace.NewTracerProvider(trace.WithSyncer(gexp))
		global.SetTracerProvider(gtp)
	}

	//jFlush, _ = jaeger.InstallNewPipeline(
	//	jaeger.WithCollectorEndpoint("http://localhost:14268/api/traces"),
	//	jaeger.WithProcess(jaeger.Process{
	//		ServiceName: "skaffold-perf2",
	//		Tags: []label.KeyValue{
	//			label.String("exporter", "jaaaeger"),
	//			label.String("boom", "42"),
	//		},
	//	}),
	//	jaeger.WithSDK(&trace.Config{
	//		DefaultSampler: trace.AlwaysSample(),
	//	}))
}

var jFlush = noStop

func fmtTime(t time.Time) string {
	return t.Format("Mon Jan 2 15:04:05 MST 2006")
}

type StopFunc func()

func noStop() {}

func Wd() string {
	if wd, err := os.Getwd(); err != nil {
		return "<error>"
	} else {
		pth := strings.Split(wd, string(filepath.Separator))
		return pth[len(pth)-1]
	}
}

func startSJSpan(span string) (StopFunc, error) {
	if !sjEnabled {
		return noStop, nil
	}
	s, err := stacklog.Start(stacklog.Config{
		Path: profileFile("sj", span),
	})
	if err != nil {
		return noStop, err
	}
	return func() {
		s.Stop()
	}, nil
}

func profileFile(kind, span string) string {
	return filepath.Join(profPath, fmt.Sprintf("%s_%v_%s-%s-%v",
		os.Args[0], os.Getpid(), kind, span, time.Now().Unix()))
}

func startPprofSpan(span string) (StopFunc, error) {
	if !ppEnabled {
		return noStop, nil
	}
	f, err := os.Create(profileFile("pprof", span))
	if err != nil {
		return noStop, err
	}
	err = pprof.StartCPUProfile(f)
	if err != nil {
		_ = f.Close()
		return noStop, err
	}
	return func() {
		pprof.StopCPUProfile()
	}, nil
}

// an ugly hack to keep the last used OT contextz
var LastOTCtx = context.Background()

func OTSpan(ctx context.Context, name string) (context.Context, StopFunc) {
	tr := global.Tracer("foo")
	traceCtx, stop := tr.Start(ctx, name)
	LastOTCtx = ctx
	return traceCtx, func() {
		stop.End()
	}
}

func ProfSpan(span string) (StopFunc, error) {
	if !sjEnabled && !ppEnabled {
		return noStop, nil
	}
	if atomic.SwapInt64(&started, 1) != 0 {
		// already started
		return noStop, nil
	}
	startedOk := false
	var rtErr error

	sjStop, err1 := startSJSpan(span)
	if err1 == nil {
		startedOk = true
	}
	ppStop, err2 := startPprofSpan(span)
	if err2 == nil {
		startedOk = true
	}
	if !startedOk {
		atomic.StoreInt64(&started, 0)
	} else {
		lastSpan = span
	}

	if err1 != nil || err2 != nil {
		rtErr = fmt.Errorf("failed to start some profilers: sj: [%v]; pp: [%v]", err1, err2)
	}
	return func() {
		sjStop()
		ppStop()
		if startedOk {
			atomic.StoreInt64(&started, 0)
		}
		jFlush()
		lastSpan = ""
	}, rtErr
}
