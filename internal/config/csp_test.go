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

func TestLoadCSPConfig_RejectsNoneMixedInFrameAncestors(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `[csp]
frame_ancestors = ["'none'", "https://embedder.example.com"]
`
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg := &config.Config{ConfigDir: tmpDir, ConfigFile: "config.toml"}
	_, err = config.LoadCSPConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'none' cannot be combined")
}

func TestLoadCSPConfig_RejectsWhitespaceInFrameAncestors(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `[csp]
frame_ancestors = ["https://a.example.com https://b.example.com"]
`
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg := &config.Config{ConfigDir: tmpDir, ConfigFile: "config.toml"}
	_, err = config.LoadCSPConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain whitespace")
}

func TestLoadCSPConfig_RejectsLeadingWhitespaceInFrameAncestors(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `[csp]
frame_ancestors = [" https://embedder.example.com"]
`
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg := &config.Config{ConfigDir: tmpDir, ConfigFile: "config.toml"}
	_, err = config.LoadCSPConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain whitespace")
}

func TestLoadCSPConfig_RejectsSemicolonsInFrameAncestors(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `[csp]
frame_ancestors = ["https://evil.com; default-src *"]
`
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg := &config.Config{ConfigDir: tmpDir, ConfigFile: "config.toml"}
	_, err = config.LoadCSPConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain semicolons")
}

func TestLoadCSPConfig_AcceptsHostListFrameAncestors(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `[csp]
frame_ancestors = ["'self'", "https://embedder.example.com"]
`
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg := &config.Config{ConfigDir: tmpDir, ConfigFile: "config.toml"}
	cspCfg, err := config.LoadCSPConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, []string{"'self'", "https://embedder.example.com"}, cspCfg.FrameAncestors)
}

func TestLoadCSPConfig_AcceptsSelfFrameAncestors(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `[csp]
frame_ancestors = ["'self'"]
`
	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg := &config.Config{ConfigDir: tmpDir, ConfigFile: "config.toml"}
	cspCfg, err := config.LoadCSPConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, []string{"'self'"}, cspCfg.FrameAncestors)
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

func TestBuild_FrameAncestorsReplaces(t *testing.T) {
	tests := []struct {
		name      string
		cfg       config.CSPConfig
		want      string
	}{
		{
			name: "default none",
			cfg:  config.CSPConfig{},
			want: "frame-ancestors 'none'",
		},
		{
			name: "same origin replaces",
			cfg:  config.CSPConfig{FrameAncestors: []string{"'self'"}},
			want: "frame-ancestors 'self'",
		},
		{
			name: "host list replaces",
			cfg:  config.CSPConfig{FrameAncestors: []string{"https://embedder.example.com"}},
			want: "frame-ancestors https://embedder.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, headerValue := tt.cfg.Build()
			assert.Contains(t, headerValue, tt.want)
			if tt.cfg.FrameAncestors != nil {
				assert.NotContains(t, headerValue, "frame-ancestors 'none'")
			}
		})
	}
}

func TestXFrameOptions(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.CSPConfig
		want string
	}{
		{
			name: "default deny",
			cfg:  config.CSPConfig{},
			want: "DENY",
		},
		{
			name: "same origin from knob",
			cfg:  config.CSPConfig{FrameAncestors: []string{"'self'"}},
			want: "SAMEORIGIN",
		},
		{
			name: "duplicate self still same origin",
			cfg:  config.CSPConfig{FrameAncestors: []string{"'self'", "'self'"}},
			want: "SAMEORIGIN",
		},
		{
			name: "case-insensitive self keyword",
			cfg:  config.CSPConfig{FrameAncestors: []string{"'SELF'"}},
			want: "SAMEORIGIN",
		},
		{
			name: "none maps to deny",
			cfg:  config.CSPConfig{FrameAncestors: []string{"'none'"}},
			want: "DENY",
		},
		{
			name: "mixed none with hosts denies",
			cfg:  config.CSPConfig{FrameAncestors: []string{"'none'", "https://embedder.example.com"}},
			want: "DENY",
		},
		{
			name: "self plus hosts omits header",
			cfg:  config.CSPConfig{FrameAncestors: []string{"'self'", "https://embedder.example.com"}},
			want: "",
		},
		{
			name: "host list omits header",
			cfg:  config.CSPConfig{FrameAncestors: []string{"https://embedder.example.com"}},
			want: "",
		},
		{
			name: "policy wins over knob",
			cfg: config.CSPConfig{
				FrameAncestors: []string{"'self'"},
				Policy:         "frame-ancestors 'none'",
			},
			want: "DENY",
		},
		{
			name: "policy same origin",
			cfg:  config.CSPConfig{Policy: "default-src 'self'; frame-ancestors 'self'"},
			want: "SAMEORIGIN",
		},
		{
			name: "policy host list omits header",
			cfg:  config.CSPConfig{Policy: "frame-ancestors https://embedder.example.com"},
			want: "",
		},
		{
			name: "policy mixed none denies",
			cfg:  config.CSPConfig{Policy: "frame-ancestors 'none' https://embedder.example.com"},
			want: "DENY",
		},
		{
			name: "policy without frame-ancestors denies",
			cfg:  config.CSPConfig{Policy: "default-src 'none'"},
			want: "DENY",
		},
		{
			name: "disable keeps default deny floor",
			cfg:  config.CSPConfig{Disable: true},
			want: "DENY",
		},
		{
			name: "disable with same-origin knob",
			cfg:  config.CSPConfig{Disable: true, FrameAncestors: []string{"'self'"}},
			want: "SAMEORIGIN",
		},
		{
			name: "disable with host list keeps omission",
			cfg:  config.CSPConfig{Disable: true, FrameAncestors: []string{"https://embedder.example.com"}},
			want: "",
		},
		{
			name: "report only keeps deny floor",
			cfg:  config.CSPConfig{ReportOnly: true},
			want: "DENY",
		},
		{
			name: "report only with host list floors to deny",
			cfg:  config.CSPConfig{ReportOnly: true, FrameAncestors: []string{"https://embedder.example.com"}},
			want: "DENY",
		},
		{
			name: "report only with same origin",
			cfg:  config.CSPConfig{ReportOnly: true, FrameAncestors: []string{"'self'"}},
			want: "SAMEORIGIN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cfg.XFrameOptions())
		})
	}
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

// --- IFrameHosts() tests ---

func TestIFrameHosts(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.CSPConfig
		want []string
	}{
		{
			name: "defaults",
			cfg:  config.CSPConfig{},
			want: []string{"www.youtube.com", "www.youtube-nocookie.com"},
		},
		{
			name: "with appends",
			cfg: config.CSPConfig{
				FrameSrc: []string{
					"https://aristorinjuang.disqus.com",
					"https://*.disquscdn.com",
				},
			},
			want: []string{
				"www.youtube.com",
				"www.youtube-nocookie.com",
				"aristorinjuang.disqus.com",
				"*.disquscdn.com",
			},
		},
		{
			name: "skips non-host sources",
			cfg: config.CSPConfig{
				FrameSrc: []string{
					"'self'",
					"'none'",
					"https:",
					"data:",
					"blob:",
				},
			},
			want: []string{"www.youtube.com", "www.youtube-nocookie.com"},
		},
		{
			name: "keeps port and path",
			cfg: config.CSPConfig{
				FrameSrc: []string{
					"https://localhost:8080",
					"http://media.example.com/path/embed",
				},
			},
			want: []string{
				"www.youtube.com",
				"www.youtube-nocookie.com",
				"localhost:8080",
				"media.example.com/path/embed",
			},
		},
		{
			name: "dedup",
			cfg:  config.CSPConfig{FrameSrc: []string{"https://www.youtube.com"}},
			want: []string{"www.youtube.com", "www.youtube-nocookie.com"},
		},
		{
			name: "skips userinfo, ipv6, bare and mid-host wildcards",
			cfg: config.CSPConfig{
				FrameSrc: []string{
					"https://user:pw@host.com/x",
					"https://[::1]:8080/",
					"https://*",
					"https://www.*.com",
					"https://*.disqus.com",
				},
			},
			want: []string{"www.youtube.com", "www.youtube-nocookie.com", "*.disqus.com"},
		},
		{
			name: "lowercases hosts",
			cfg:  config.CSPConfig{FrameSrc: []string{"https://WWW.VIMEO.com"}},
			want: []string{"www.youtube.com", "www.youtube-nocookie.com", "www.vimeo.com"},
		},
		{
			name: "policy override uses its frame-src only",
			cfg: config.CSPConfig{
				Policy: "default-src 'self'; frame-src 'self' https://vimeo.com https://www.youtube-nocookie.com",
			},
			want: []string{"vimeo.com", "www.youtube-nocookie.com"},
		},
		{
			name: "policy override without frame-src leaves allowlist empty",
			cfg:  config.CSPConfig{Policy: "default-src 'none'"},
			want: []string{},
		},
		{
			name: "disable keeps default allowlist as safety net",
			cfg:  config.CSPConfig{Disable: true, FrameSrc: []string{"https://vimeo.com"}},
			want: []string{"www.youtube.com", "www.youtube-nocookie.com", "vimeo.com"},
		},
		{
			name: "report only keeps default allowlist as safety net",
			cfg:  config.CSPConfig{ReportOnly: true, FrameSrc: []string{"https://vimeo.com"}},
			want: []string{"www.youtube.com", "www.youtube-nocookie.com", "vimeo.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.IFrameHosts()
			assert.Equal(t, tt.want, got)
		})
	}
}
