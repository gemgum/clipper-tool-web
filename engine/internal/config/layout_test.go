package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// makeCheckout membuat penanda pohon sumber (engine/go.mod).
func makeCheckout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "engine", "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// clearEnv mengosongkan env yang bisa menimpa layout, supaya uji tidak ikut
// terpengaruh setelan mesin yang menjalankannya.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"CLIPPER_DATA_DIR", "CLIPPER_MODELS_DIR", "CLIPPER_TOOLS_DIR",
		"CLIPPER_FONTS_DIR", "CLIPPER_ENV_FILE",
	} {
		t.Setenv(k, "")
	}
}

// Dari checkout, semuanya tetap di dalam repo. Ini yang menjaga model 466 MB &
// cache transkrip milik pengembang tidak mendadak dianggap belum ada.
func TestCheckoutKeepsEverythingInTheRepo(t *testing.T) {
	clearEnv(t)
	root := makeCheckout(t)

	l := Locate(root)

	if !l.Dev {
		t.Error("checkout tidak dikenali sebagai pohon sumber")
	}
	for _, c := range []struct{ name, got, want string }{
		{"DataDir", l.DataDir, filepath.Join(root, "data")},
		{"ModelsDir", l.ModelsDir, filepath.Join(root, "models")},
		{"ToolsDir", l.ToolsDir, filepath.Join(root, "bin")},
		{"EnvFile", l.EnvFile, filepath.Join(root, ".env")},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, mau %q", c.name, c.got, c.want)
		}
	}
}

// Di luar checkout, tidak boleh ada satu pun folder tulis yang menempel ke
// biner: "Program Files" dan "/Applications" hanya-baca.
func TestInstalledWritesNothingNextToTheBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uji ini memakai HOME/XDG ala Unix")
	}
	clearEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	root := t.TempDir() // tanpa engine/go.mod = seolah terpasang

	l := Locate(root)

	if l.Dev {
		t.Fatal("folder tanpa engine/go.mod dikira pohon sumber")
	}
	want := filepath.Join(home, ".local", "share", "clipper")
	for _, c := range []struct{ name, got string }{
		{"DataDir", l.DataDir},
		{"ModelsDir", l.ModelsDir},
		{"ToolsDir", l.ToolsDir},
		{"EnvFile", l.EnvFile},
	} {
		if !strings.HasPrefix(c.got, want) {
			t.Errorf("%s = %q, mau di dalam %q", c.name, c.got, want)
		}
		if strings.HasPrefix(c.got, root) {
			t.Errorf("%s = %q — masih menempel ke folder aplikasi", c.name, c.got)
		}
	}
}

func TestXDGDataHomeIsHonoured(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG hanya berlaku di Unix")
	}
	clearEnv(t)
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)

	l := Locate(t.TempDir())

	if want := filepath.Join(xdg, "clipper", "data"); l.DataDir != want {
		t.Errorf("DataDir = %q, mau %q", l.DataDir, want)
	}
}

// Env menimpa keduanya — jalan keluar bila model harus ditaruh di disk lain.
func TestEnvOverridesEveryFolder(t *testing.T) {
	clearEnv(t)
	t.Setenv("CLIPPER_DATA_DIR", "/x/data")
	t.Setenv("CLIPPER_MODELS_DIR", "/x/models")
	t.Setenv("CLIPPER_TOOLS_DIR", "/x/bin")
	t.Setenv("CLIPPER_ENV_FILE", "/x/env")

	l := Locate(makeCheckout(t))

	if l.DataDir != "/x/data" || l.ModelsDir != "/x/models" || l.ToolsDir != "/x/bin" || l.EnvFile != "/x/env" {
		t.Errorf("env tidak dipakai: %+v", l)
	}
}

// Model & biner dicari di folder layout, bukan di sebelah biner.
func TestResolvePathsFollowsTheLayout(t *testing.T) {
	clearEnv(t)
	for _, k := range []string{"CLIPPER_WHISPER_MODEL", "CLIPPER_WHISPER_BIN", "CLIPPER_FFMPEG_BIN", "CLIPPER_FFPROBE_BIN"} {
		t.Setenv(k, "")
	}
	l := Layout{DataDir: "/d", ModelsDir: "/m", ToolsDir: "/t"}
	o := DefaultOptions()

	p := ResolvePaths(l, o)

	if want := filepath.Join("/m", "ggml-small.bin"); p.Model != want {
		t.Errorf("Model = %q, mau %q", p.Model, want)
	}
	if !strings.HasPrefix(p.Whisper, "/t") && p.Whisper != exeName("whisper-cli") {
		t.Errorf("Whisper = %q, mau di /t atau nama polos dari PATH", p.Whisper)
	}
	if p.DataDir != "/d" || p.ModelsDir != "/m" || p.ToolsDir != "/t" {
		t.Errorf("folder tidak diteruskan ke Paths: %+v", p)
	}
}

// ffmpeg yang dipasang aplikasi menang atas yang ada di PATH: di mesin pengguna
// desktop, versi yang pasti cocok hanyalah yang diunduh aplikasi sendiri.
func TestBundledFFmpegWinsOverPATH(t *testing.T) {
	clearEnv(t)
	t.Setenv("CLIPPER_FFMPEG_BIN", "")
	tools := t.TempDir()
	bundled := filepath.Join(tools, exeName("ffmpeg"))
	if err := os.WriteFile(bundled, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	p := ResolvePaths(Layout{ToolsDir: tools}, DefaultOptions())

	if p.FFmpeg != bundled {
		t.Errorf("FFmpeg = %q, mau %q", p.FFmpeg, bundled)
	}
}

// Tanpa biner unduhan, engine tetap memakai yang ada di PATH — itu jalur
// pengembang hari ini dan tidak boleh ikut mati.
func TestFFmpegFallsBackToPATH(t *testing.T) {
	clearEnv(t)
	t.Setenv("CLIPPER_FFMPEG_BIN", "")

	p := ResolvePaths(Layout{ToolsDir: t.TempDir()}, DefaultOptions())

	if p.FFmpeg != exeName("ffmpeg") {
		t.Errorf("FFmpeg = %q, mau %q", p.FFmpeg, exeName("ffmpeg"))
	}
}
