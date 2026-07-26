package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCSPConfig_NilConfig(t *testing.T) {
	_, err := config.LoadCSPConfig(nil)
	require.Error(t, err)
}

func TestLoadCSPConfig_PathTraversalRejected(t *testing.T) {
	cfg := &config.Config{ConfigFile: "../evil.toml"}
	_, err := config.LoadCSPConfig(cfg)
	require.Error(t, err)
}

func TestLoadCSPConfig_NoConfigDir(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{ConfigDir: filepath.Join(tmpDir, "nonexistent"), ConfigFile: "config.toml"}
	cspCfg, err := config.LoadCSPConfig(cfg)
	require.NoError(t, err)
	assert.False(t, cspCfg.Disable)
	assert.Empty(t, cspCfg.ScriptSrc)
}

func TestLoadCSPConfig_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{ConfigDir: tmpDir, ConfigFile: "does-not-exist.toml"}
	cspCfg, err := config.LoadCSPConfig(cfg)
	require.NoError(t, err)
	assert.False(t, cspCfg.Disable)
}

func TestLoadCSPConfig_NoSection(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte("languages = [\"en\"]\n"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{ConfigDir: tmpDir, ConfigFile: "config.toml"}
	cspCfg, err := config.LoadCSPConfig(cfg)
	require.NoError(t, err)
	assert.False(t, cspCfg.Disable)
	assert.Empty(t, cspCfg.ScriptSrc)
	assert.Empty(t, cspCfg.Policy)
}

func TestLoadCSPConfig_WithExtras(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `
[csp]
script_src = ["https://www.googletagmanager.com"]
frame_src = ["https://www.youtube-nocookie.com"]
font_src = ["data:"]
report_only = true
`
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg := &config.Config{ConfigDir: tmpDir, ConfigFile: "config.toml"}
	cspCfg, err := config.LoadCSPConfig(cfg)
	require.NoError(t, err)
	assert.True(t, cspCfg.ReportOnly)
	assert.Equal(t, []string{"https://www.googletagmanager.com"}, cspCfg.ScriptSrc)
	assert.Equal(t, []string{"https://www.youtube-nocookie.com"}, cspCfg.FrameSrc)
	assert.Equal(t, []string{"data:"}, cspCfg.FontSrc)
}

func TestLoadCSPConfig_ExtraDirectives(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `
[csp]
extra_directives = { "upgrade-insecure-requests" = "", "report-uri" = "https://example.com/csp-report" }
`
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg := &config.Config{ConfigDir: tmpDir, ConfigFile: "config.toml"}
	cspCfg, err := config.LoadCSPConfig(cfg)
	require.NoError(t, err)
	require.NotNil(t, cspCfg.ExtraDirectives)
	assert.Equal(t, "", cspCfg.ExtraDirectives["upgrade-insecure-requests"])
	assert.Equal(t, "https://example.com/csp-report", cspCfg.ExtraDirectives["report-uri"])
}

func TestLoadCSPConfig_PolicyOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `[csp]
policy = "default-src 'none'"
`
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg := &config.Config{ConfigDir: tmpDir, ConfigFile: "config.toml"}
	cspCfg, err := config.LoadCSPConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, "default-src 'none'", cspCfg.Policy)
}

func TestLoadCSPConfig_Disable(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `[csp]
disable = true
`
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg := &config.Config{ConfigDir: tmpDir, ConfigFile: "config.toml"}
	cspCfg, err := config.LoadCSPConfig(cfg)
	require.NoError(t, err)
	assert.True(t, cspCfg.Disable)
}

func TestLoadCSPConfig_RejectsSemicolonsInSources(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `[csp]
script_src = ["https://evil.com; default-src *"]
`
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg := &config.Config{ConfigDir: tmpDir, ConfigFile: "config.toml"}
	_, err = config.LoadCSPConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain semicolons")
}

func TestLoadCSPConfig_RejectsNewlinesInSources(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := "[[csp]]\nscript_src = [\"https://evil.com\n\"]\n"
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg := &config.Config{ConfigDir: tmpDir, ConfigFile: "config.toml"}
	_, err = config.LoadCSPConfig(cfg)
	require.Error(t, err)
}

func TestLoadCSPConfig_RejectsSemicolonsInExtraDirectivesValue(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `[csp]
extra_directives = { "report-uri" = "https://evil.com; script-src *" }
`
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg := &config.Config{ConfigDir: tmpDir, ConfigFile: "config.toml"}
	_, err = config.LoadCSPConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain semicolons")
}

func TestLoadCSPConfig_RejectsNewlinesInPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := "[csp]\npolicy = \"default-src 'self'\nx: evil\"\n"
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg := &config.Config{ConfigDir: tmpDir, ConfigFile: "config.toml"}
	_, err = config.LoadCSPConfig(cfg)
	require.Error(t, err)
}

// --- Build() tests ---

func TestBuild_Defaults(t *testing.T) {
	cspCfg := config.CSPConfig{}
	headerName, headerValue := cspCfg.Build()
	assert.Equal(t, "Content-Security-Policy", headerName)

	assert.Contains(t, headerValue, "default-src 'self'")
	assert.Contains(t, headerValue, "script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net")
	assert.Contains(t, headerValue, "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdn.jsdelivr.net")
	assert.Contains(t, headerValue, "img-src 'self' data: https:")
	assert.Contains(t, headerValue, "font-src 'self' https://fonts.gstatic.com https://cdn.jsdelivr.net")
	assert.Contains(t, headerValue, "connect-src 'self'")
	assert.Contains(t, headerValue, "frame-src 'self' https://www.youtube.com")
	assert.Contains(t, headerValue, "frame-src 'self' https://www.youtube.com https://www.youtube-nocookie.com")
	assert.Contains(t, headerValue, "frame-ancestors 'none'")
	assert.Contains(t, headerValue, "base-uri 'self'")
	assert.Contains(t, headerValue, "form-action 'self'")
}

func TestBuild_WithAppends(t *testing.T) {
	cspCfg := config.CSPConfig{
		ScriptSrc: []string{"https://www.googletagmanager.com"},
		FontSrc:   []string{"data:"},
	}
	_, headerValue := cspCfg.Build()
	assert.Contains(t, headerValue, "script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://www.googletagmanager.com")
	assert.Contains(t, headerValue, "font-src 'self' https://fonts.gstatic.com https://cdn.jsdelivr.net data:")
}

func TestBuild_ReportOnly(t *testing.T) {
	cspCfg := config.CSPConfig{ReportOnly: true}
	headerName, headerValue := cspCfg.Build()
	assert.Equal(t, "Content-Security-Policy-Report-Only", headerName)
	assert.NotEmpty(t, headerValue)
}

func TestBuild_Disabled(t *testing.T) {
	cspCfg := config.CSPConfig{Disable: true}
	headerName, headerValue := cspCfg.Build()
	assert.Empty(t, headerName)
	assert.Empty(t, headerValue)
}

func TestBuild_DisabledOverridesPolicy(t *testing.T) {
	cspCfg := config.CSPConfig{Disable: true, Policy: "default-src 'none'"}
	headerName, headerValue := cspCfg.Build()
	assert.Empty(t, headerName, "Disable=true should win over Policy")
	assert.Empty(t, headerValue, "Disable=true should win over Policy")
}

func TestBuild_PolicyOverride(t *testing.T) {
	cspCfg := config.CSPConfig{Policy: "default-src 'none'"}
	headerName, headerValue := cspCfg.Build()
	assert.Equal(t, "Content-Security-Policy", headerName)
	assert.Equal(t, "default-src 'none'", headerValue)
}

func TestBuild_PolicyOverrideWithReportOnly(t *testing.T) {
	cspCfg := config.CSPConfig{Policy: "default-src 'self'", ReportOnly: true}
	headerName, headerValue := cspCfg.Build()
	assert.Equal(t, "Content-Security-Policy-Report-Only", headerName)
	assert.Equal(t, "default-src 'self'", headerValue)
}

func TestBuild_ExtraDirectives(t *testing.T) {
	cspCfg := config.CSPConfig{
		ExtraDirectives: map[string]string{
			"upgrade-insecure-requests": "",
			"report-uri":               "https://example.com/csp-report",
		},
	}
	_, headerValue := cspCfg.Build()
	assert.Contains(t, headerValue, "upgrade-insecure-requests")
	assert.Contains(t, headerValue, "report-uri https://example.com/csp-report")
}

func TestBuild_ExtraDirectivesOrderDeterministic(t *testing.T) {
	// Two configs with different map insertion order must produce the same header.
	cspA := config.CSPConfig{
		ExtraDirectives: map[string]string{
			"a": "1",
			"b": "2",
		},
	}
	cspB := config.CSPConfig{
		ExtraDirectives: map[string]string{
			"b": "2",
			"a": "1",
		},
	}
	_, valA := cspA.Build()
	_, valB := cspB.Build()
	assert.Equal(t, valA, valB, "extra_directives order must be deterministic")
}

func TestBuild_Dedup(t *testing.T) {
	cspCfg := config.CSPConfig{
		ScriptSrc: []string{"'self'"},
	}
	_, headerValue := cspCfg.Build()
	// 'self' should appear exactly once in script-src after dedup
	assert.Contains(t, headerValue, "script-src 'self' 'unsafe-inline'")
	// Ensure 'self' is NOT repeated
	assert.NotContains(t, headerValue, "'self' 'self'")
}

func TestBuild_AllAppendableDirectives(t *testing.T) {
	cspCfg := config.CSPConfig{
		ScriptSrc:  []string{"a"},
		StyleSrc:   []string{"b"},
		ImgSrc:     []string{"c"},
		FontSrc:    []string{"d"},
		ConnectSrc: []string{"e"},
		FrameSrc:   []string{"f"},
		MediaSrc:   []string{"g"},
		ObjectSrc:  []string{"h"},
		WorkerSrc:  []string{"i"},
	}
	_, headerValue := cspCfg.Build()
	assert.Contains(t, headerValue, "script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net a")
	assert.Contains(t, headerValue, "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdn.jsdelivr.net b")
	assert.Contains(t, headerValue, "img-src 'self' data: https: c")
	assert.Contains(t, headerValue, "font-src 'self' https://fonts.gstatic.com https://cdn.jsdelivr.net d")
	assert.Contains(t, headerValue, "connect-src 'self' e")
	assert.Contains(t, headerValue, "frame-src 'self' https://www.youtube.com https://www.youtube-nocookie.com f")
	assert.Contains(t, headerValue, "media-src g")
	assert.Contains(t, headerValue, "object-src h")
	assert.Contains(t, headerValue, "worker-src i")
}
