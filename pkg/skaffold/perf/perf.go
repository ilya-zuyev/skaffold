package perf

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sync/atomic"
	"time"

	"github.com/google/slowjam/pkg/stacklog"
)

type SlowJamSpan struct {
	File     string
	SpanName string
}

const sjEnv = "SJ_PROFILE"
const pprofEnv = "PP_PROFILE"
const profPathEnv = "PROF_PATH"

var sjEnabled = false
var ppEnabled = false

var started = int64(0)

var profPath = "."

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

	if sjEnabled || ppEnabled {
		if p, ok := os.LookupEnv(profPathEnv); ok {
			profPath = p
		}
	}
}

type StopFunc func()

func noStop() {}

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

func Span(span string) (StopFunc, error) {
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
	}, rtErr
}
