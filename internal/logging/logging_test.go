package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestNew_FormatJSON(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(&buf, "info", "json")
	if err != nil {
		t.Fatalf("New gagal: %v", err)
	}

	logger.Info("halo", "kunci", "nilai")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("keluaran bukan JSON valid: %v (isi: %q)", err, buf.String())
	}
	if entry["msg"] != "halo" {
		t.Errorf("msg = %v, ingin halo", entry["msg"])
	}
	if entry["level"] != "INFO" {
		t.Errorf("level = %v, ingin INFO", entry["level"])
	}
	if entry["kunci"] != "nilai" {
		t.Errorf("kunci = %v, ingin nilai", entry["kunci"])
	}
}

func TestNew_FormatText(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(&buf, "info", "text")
	if err != nil {
		t.Fatalf("New gagal: %v", err)
	}

	logger.Info("halo", "kunci", "nilai")

	out := buf.String()
	for _, ingin := range []string{"level=INFO", "msg=halo", "kunci=nilai"} {
		if !strings.Contains(out, ingin) {
			t.Errorf("keluaran %q tidak memuat %q", out, ingin)
		}
	}
}

func TestNew_Bawaan(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(&buf, "", "")
	if err != nil {
		t.Fatalf("New gagal: %v", err)
	}

	logger.Debug("rinci")
	if buf.Len() != 0 {
		t.Errorf("level bawaan harus info, tetapi debug tertulis: %q", buf.String())
	}

	logger.Info("halo")
	if !strings.Contains(buf.String(), "level=INFO") {
		t.Errorf("format bawaan harus text, keluaran = %q", buf.String())
	}
}

func TestNew_LevelMenyaring(t *testing.T) {
	tests := []struct {
		level       string
		debugMuncul bool
		infoMuncul  bool
		warnMuncul  bool
	}{
		{"debug", true, true, true},
		{"info", false, true, true},
		{"warn", false, false, true},
		{"error", false, false, false},
		{"WARN", false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			var buf bytes.Buffer
			logger, err := New(&buf, tt.level, "text")
			if err != nil {
				t.Fatalf("New gagal: %v", err)
			}

			logger.Debug("pesan-debug")
			logger.Info("pesan-info")
			logger.Warn("pesan-warn")
			logger.Error("pesan-error")

			out := buf.String()
			if got := strings.Contains(out, "pesan-debug"); got != tt.debugMuncul {
				t.Errorf("debug muncul = %v, ingin %v", got, tt.debugMuncul)
			}
			if got := strings.Contains(out, "pesan-info"); got != tt.infoMuncul {
				t.Errorf("info muncul = %v, ingin %v", got, tt.infoMuncul)
			}
			if got := strings.Contains(out, "pesan-warn"); got != tt.warnMuncul {
				t.Errorf("warn muncul = %v, ingin %v", got, tt.warnMuncul)
			}
			if !strings.Contains(out, "pesan-error") {
				t.Error("error harus selalu muncul")
			}
		})
	}
}

func TestNew_TidakValid(t *testing.T) {
	tests := []struct {
		name          string
		level, format string
	}{
		{"level tidak dikenal", "verbose", "text"},
		{"level angka", "3", "text"},
		{"format tidak dikenal", "info", "xml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := New(io.Discard, tt.level, tt.format)
			if err == nil {
				t.Error("New harus gagal untuk konfigurasi tidak valid")
			}
			if logger != nil {
				t.Error("logger harus nil saat konfigurasi tidak valid")
			}
		})
	}
}
