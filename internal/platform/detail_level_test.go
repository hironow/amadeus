package platform_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/platform"
)

func TestInitDetailLevel_Default(t *testing.T) {
	t.Setenv("OTEL_DETAIL_LEVEL", "")
	platform.InitDetailLevel()

	if platform.OTELDetailLevel != platform.DetailBasic {
		t.Errorf("expected basic, got %q", platform.OTELDetailLevel)
	}
	if platform.IsDetailDebug() {
		t.Error("expected IsDetailDebug() == false")
	}
}

func TestInitDetailLevel_Debug(t *testing.T) {
	t.Setenv("OTEL_DETAIL_LEVEL", "debug")
	platform.InitDetailLevel()

	if platform.OTELDetailLevel != platform.DetailDebug {
		t.Errorf("expected debug, got %q", platform.OTELDetailLevel)
	}
	if !platform.IsDetailDebug() {
		t.Error("expected IsDetailDebug() == true")
	}
}

func TestInitDetailLevel_Unknown(t *testing.T) {
	t.Setenv("OTEL_DETAIL_LEVEL", "verbose")
	platform.InitDetailLevel()

	if platform.OTELDetailLevel != platform.DetailBasic {
		t.Errorf("expected basic for unknown value, got %q", platform.OTELDetailLevel)
	}
}

func TestInitDetailLevel_ResetFromDebugToBasic(t *testing.T) {
	t.Setenv("OTEL_DETAIL_LEVEL", "debug")
	platform.InitDetailLevel()
	if platform.OTELDetailLevel != platform.DetailDebug {
		t.Fatal("setup: expected debug")
	}

	t.Setenv("OTEL_DETAIL_LEVEL", "")
	platform.InitDetailLevel()
	if platform.OTELDetailLevel != platform.DetailBasic {
		t.Errorf("expected reset to basic, got %q", platform.OTELDetailLevel)
	}
}
