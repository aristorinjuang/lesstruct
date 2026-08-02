package hugo_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/content/hugo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseContentFile(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		content  string
		want     *hugo.HugoItem
		wantErr  bool
	}{
		{
			name:     "success - multi-line frontmatter and body",
			fileName: "post.html",
			content: `---
title: "My Post"
description: "A test post"
tags:
  - go
  - testing
---
<body>
  <h1>Hello World</h1>
  <p>This is a test.</p>
</body>`,
			want: &hugo.HugoItem{
				Title:        "My Post",
				Description:  "A test post",
				Tags:         []string{"go", "testing"},
				Body:         "<body>\n  <h1>Hello World</h1>\n  <p>This is a test.</p>\n</body>",
				Language:     "en",
				OriginalBody: "<body>\n  <h1>Hello World</h1>\n  <p>This is a test.</p>\n</body>",
			},
			wantErr: false,
		},
		{
			name:     "success - language detection from .id.html suffix",
			fileName: "post.id.html",
			content: `---
title: "Post Indonesia"
---
<body>Konten</body>`,
			want: &hugo.HugoItem{
				Title:        "Post Indonesia",
				Body:         "<body>Konten</body>",
				Language:     "id",
				OriginalBody: "<body>Konten</body>",
			},
			wantErr: false,
		},
		{
			name:     "success - language from YAML frontmatter",
			fileName: "post.html",
			content: `---
title: "English Post"
language: en
---
<body>Content</body>`,
			want: &hugo.HugoItem{
				Title:        "English Post",
				Body:         "<body>Content</body>",
				Language:     "en",
				OriginalBody: "<body>Content</body>",
			},
			wantErr: false,
		},
		{
			name:     "success - slug from URL",
			fileName: "post.html",
			content: `---
title: "My Post"
url: "/posts/my-post.html"
---
<body>Content</body>`,
			want: &hugo.HugoItem{
				Title:        "My Post",
				Body:         "<body>Content</body>",
				URL:          "posts/my-post",
				Language:     "en",
				Aliases:      []string{"posts/my-post.html"},
				OriginalBody: "<body>Content</body>",
			},
			wantErr: false,
		},
		{
			name:     "success - old HTML URL added as alias",
			fileName: "post.html",
			content: `---
title: "Aliased Post"
url: "/old-path/post.html"
---
<body>Content</body>`,
			want: &hugo.HugoItem{
				Title:        "Aliased Post",
				Body:         "<body>Content</body>",
				URL:          "old-path/post",
				Language:     "en",
				Aliases:      []string{"old-path/post.html"},
				OriginalBody: "<body>Content</body>",
			},
			wantErr: false,
		},
		{
			name:     "success - explicit aliases from frontmatter",
			fileName: "post.html",
			content: `---
title: "Aliased Post"
aliases:
  - /old-path
  - /another-old-path
---
<body>Content</body>`,
			want: &hugo.HugoItem{
				Title:        "Aliased Post",
				Body:         "<body>Content</body>",
				Language:     "en",
				Aliases:      []string{"old-path", "another-old-path"},
				OriginalBody: "<body>Content</body>",
			},
			wantErr: false,
		},
		{
			name:     "success - draft flag",
			fileName: "post.html",
			content: `---
title: "Draft Post"
draft: true
---
<body>Content</body>`,
			want: &hugo.HugoItem{
				Title:        "Draft Post",
				Body:         "<body>Content</body>",
				Language:     "en",
				IsDraft:      true,
				OriginalBody: "<body>Content</body>",
			},
			wantErr: false,
		},
		{
			name:     "success - boolean custom fields",
			fileName: "post.html",
			content: `---
title: "Feature Rich"
hasMath: true
hasChart: true
hasDiagrams: true
hideMobileImages: true
---
<body>Content</body>`,
			want: &hugo.HugoItem{
				Title:            "Feature Rich",
				Body:             "<body>Content</body>",
				Language:         "en",
				HasMath:          true,
				HasChart:         true,
				HasDiagrams:      true,
				HideMobileImages: true,
				OriginalBody:     "<body>Content</body>",
			},
			wantErr: false,
		},
		{
			name:     "error - no frontmatter",
			fileName: "post.html",
			content:  "<body>no frontmatter here</body>",
			wantErr:  true,
		},
		{
			name:     "error - malformed YAML",
			fileName: "post.html",
			content: `---
title: [unclosed bracket
---
<body>Content</body>`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(t.TempDir(), tt.fileName)
			err := os.WriteFile(filePath, []byte(tt.content), 0644)
			require.NoError(t, err)

			got, err := hugo.ParseContentFile(filePath)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)

			tt.want.FilePath = filePath
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWalkContentTree(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		wantFn  func(t *testing.T, root string) *hugo.HugoSite
		wantErr bool
	}{
		{
			name: "success - walks directory with valid files",
			files: map[string]string{
				"posts/post1.html": `---
title: "Post 1"
---
<body>1</body>`,
				"posts/post2.md": `---
title: "Post 2"
---
<body>2</body>`,
			},
			wantFn: func(t *testing.T, root string) *hugo.HugoSite {
				return &hugo.HugoSite{
					SourcePath: root,
					Items: []*hugo.HugoItem{
						{
							Title:        "Post 1",
							Body:         "<body>1</body>",
							OriginalBody: "<body>1</body>",
							Language:     "en",
							FilePath:     filepath.Join(root, "posts/post1.html"),
						},
						{
							Title:        "Post 2",
							Body:         "<body>2</body>",
							OriginalBody: "<body>2</body>",
							Language:     "en",
							FilePath:     filepath.Join(root, "posts/post2.md"),
						},
					},
				}
			},
			wantErr: false,
		},
		{
			name: "success - skips hidden directories and non-content files",
			files: map[string]string{
				".hidden/skip.html": `---
title: "Hidden"
---
<body>skip</body>`,
				"visible/post.html": `---
title: "Visible"
---
<body>visible</body>`,
				"visible/notes.txt": "not content",
			},
			wantFn: func(t *testing.T, root string) *hugo.HugoSite {
				return &hugo.HugoSite{
					SourcePath: root,
					Items: []*hugo.HugoItem{
						{
							Title:        "Visible",
							Body:         "<body>visible</body>",
							OriginalBody: "<body>visible</body>",
							Language:     "en",
							FilePath:     filepath.Join(root, "visible/post.html"),
						},
					},
				}
			},
			wantErr: false,
		},
		{
			name: "error - file with malformed YAML",
			files: map[string]string{
				"bad.html": `---
bad: [unclosed
---
<body></body>`,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for relPath, content := range tt.files {
				fullPath := filepath.Join(root, relPath)
				err := os.MkdirAll(filepath.Dir(fullPath), 0755)
				require.NoError(t, err)
				err = os.WriteFile(fullPath, []byte(content), 0644)
				require.NoError(t, err)
			}

			got, err := hugo.WalkContentTree(root)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantFn(t, root), got)
		})
	}
}
