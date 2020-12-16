package perf

import (
	"context"
	"flag"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/pkg/ioutils"

	sc "github.com/GoogleContainerTools/skaffold/pkg/skaffold/config"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/initializer"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/initializer/config"
)

var testProj = flag.String("target", "examples/getting-started", "The target skaffold project dir")
var skDir = flag.String("dir", ".", "Skaffold root dir")
var skaffoldBinary = flag.String("binary", "skaffold", "Skaffold binary to run")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func BenchmarkRender(b *testing.B) {
	skRoot, err := filepath.Abs(*skDir)
	if err != nil {
		b.Fatalf("failed to process path: %v", err)
	}

	for i := 0; i < b.N; i++ {
		cmd := exec.Command(filepath.Join(skRoot, *skaffoldBinary), "render")

		cmd.Dir = filepath.Join(skRoot, *testProj)
		cmd.Stdout = &ioutils.NopWriter{}
		cmd.Stderr = os.Stderr

		time.Sleep(3000 * time.Millisecond)
		err := cmd.Run()
		if err != nil {
			b.Errorf("failed to run skaffold: %v", err)
		}
	}
}

func BenchmarkBuild(b *testing.B) {
	skRoot, err := filepath.Abs(*skDir)
	if err != nil {
		b.Fatalf("failed to process path: %v", err)
	}

	for i := 0; i < b.N; i++ {
		cmd := exec.Command(filepath.Join(skRoot, *skaffoldBinary), "build")

		cmd.Dir = filepath.Join(skRoot, *testProj)
		cmd.Stdout = &ioutils.NopWriter{}
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err != nil {
			b.Errorf("failed to run skaffold: %v", err)
		}
	}
}

func BenchmarkDeploy(b *testing.B) {
	skRoot, err := filepath.Abs(*skDir)
	if err != nil {
		b.Fatalf("failed to process path: %v", err)
	}

	for i := 0; i < b.N; i++ {
		cmd := exec.Command(filepath.Join(skRoot, *skaffoldBinary), "deploy", "-t", "foo")

		cmd.Dir = filepath.Join(skRoot, *testProj)
		cmd.Stdout = &ioutils.NopWriter{}
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err != nil {
			b.Errorf("failed to run skaffold: %v", err)
		}
	}
}

func BenchmarkFoo(b *testing.B) {
	skRoot, err := filepath.Abs(*skDir)
	if err != nil {
		b.Fatalf("failed to process path: %v", err)
	}
	err = os.Chdir(filepath.Join(skRoot, *testProj))
	if err != nil {
		b.Fatalf("failed to chdir: %v", err)
	}

	tmpCfg, err := ioutil.TempFile("", "")
	if err != nil {
		b.Fatalf("failed to create temp file: %v", err)
	}

	for i := 0; i < b.N; i++ {
		cfg := config.Config{
			ComposeFile:            "",
			CliArtifacts:           []string{},
			CliKubernetesManifests: []string{},
			SkipBuild:              false,
			SkipDeploy:             false,
			Force:                  true,
			Analyze:                false,
			EnableJibInit:          false,
			EnableJibGradleInit:    false,
			EnableBuildpacksInit:   false,
			EnableNewInitFormat:    false,

			BuildpacksBuilder: "gcr.io/buildpacks/builder:v1",
			Opts: sc.SkaffoldOptions{
				Force:             true,
				ConfigurationFile: tmpCfg.Name(),
			},
		}
		err := initializer.DoInit(context.Background(), &ioutils.NopWriter{}, cfg)
		if err != nil {
			b.Errorf("failed to init skaffold: %v", err)
		}
	}
}
