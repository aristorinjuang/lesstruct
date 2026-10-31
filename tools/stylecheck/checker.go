// Package stylecheck enforces the Lesstruct Go file-layout contract: methods
// stay with the struct that owns them, constructors close their type, and
// package-level helpers precede public declarations.
package stylecheck

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const generatedWindow = 2048

var skipDirs = map[string]bool{
	"bin":          true,
	"node_modules": true,
	"testdata":     true,
	"vendor":       true,
	"web":          true,
}

// typeLayout records where a type and the functions that belong to it sit in
// one file.
type typeLayout struct {
	name      string
	declIndex int
	ctors     []int
	methods   []int
}

// baseTypeName unwraps pointers, generics, and parentheses down to the base
// identifier.
func baseTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return baseTypeName(t.X)
	case *ast.ParenExpr:
		return baseTypeName(t.X)
	case *ast.IndexExpr:
		return baseTypeName(t.X)
	case *ast.IndexListExpr:
		return baseTypeName(t.X)
	default:
		return ""
	}
}

// collectTypeLayouts indexes every single-spec struct declaration in file and
// the methods and constructors that belong to it. Named non-struct types
// (enums) are skipped: their const blocks sit between the type and its
// methods, and constructors return structs by definition.
func collectTypeLayouts(file *ast.File) []*typeLayout {
	layouts := make([]*typeLayout, 0)
	byName := make(map[string]*typeLayout)

	for i, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE || len(gd.Specs) != 1 {
			continue
		}
		ts, ok := gd.Specs[0].(*ast.TypeSpec)
		if !ok {
			continue
		}
		if _, isStruct := ts.Type.(*ast.StructType); !isStruct {
			continue
		}
		layout := &typeLayout{name: ts.Name.Name, declIndex: i}
		byName[layout.name] = layout
		layouts = append(layouts, layout)
	}

	for i, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fd.Recv != nil {
			if recv := baseTypeName(fd.Recv.List[0].Type); recv != "" {
				if layout, ok := byName[recv]; ok {
					layout.methods = append(layout.methods, i)
				}
			}
			continue
		}
		if fd.Type.Results == nil || len(fd.Type.Results.List) == 0 {
			continue
		}
		if result := baseTypeName(fd.Type.Results.List[0].Type); result != "" {
			if layout, ok := byName[result]; ok {
				layout.ctors = append(layout.ctors, i)
			}
		}
	}

	return layouts
}

// declName returns a human-readable name for a top-level declaration.
func declName(decl ast.Decl) string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return d.Name.Name
	case *ast.GenDecl:
		if len(d.Specs) > 0 {
			if ts, ok := d.Specs[0].(*ast.TypeSpec); ok {
				return ts.Name.Name
			}
		}
	}
	return "declaration"
}

// isWitness reports whether decl is a compile-time interface witness
// (`var _ kernel.Module = (*Module)(nil)`), which may sit beside its type.
func isWitness(decl ast.Decl) bool {
	gd, ok := decl.(*ast.GenDecl)
	if !ok || gd.Tok != token.VAR {
		return false
	}
	for _, spec := range gd.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			return false
		}
		for _, name := range vs.Names {
			if name.Name != "_" {
				return false
			}
		}
	}
	return true
}

// isPublicDecl reports whether decl starts the public section: an exported
// type or an exported package-level function. Exported consts and vars live
// in the top blocks and do not start it.
func isPublicDecl(decl ast.Decl) bool {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return d.Recv == nil && d.Name.IsExported()
	case *ast.GenDecl:
		if d.Tok != token.TYPE {
			return false
		}
		return slices.ContainsFunc(d.Specs, func(spec ast.Spec) bool {
			ts, ok := spec.(*ast.TypeSpec)
			return ok && ts.Name.IsExported()
		})
	default:
		return false
	}
}

// isGenerated reports whether src carries a code-generation header. Generated
// files are exempt from the layout contract.
func isGenerated(src []byte) bool {
	head := src
	if len(head) > generatedWindow {
		head = head[:generatedWindow]
	}
	return bytes.Contains(head, []byte("Code generated")) &&
		bytes.Contains(head, []byte("DO NOT EDIT."))
}

// shouldSkipDir reports whether the walker must not descend into a directory.
func shouldSkipDir(name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return true
	}
	return skipDirs[name]
}

// Finding is one layout violation.
type Finding struct {
	Pos     token.Position
	Message string
}

// String renders the finding as `file:line:col: message`.
func (f Finding) String() string {
	return fmt.Sprintf("%s: %s", f.Pos, f.Message)
}

// Checker holds the parser state shared by one run.
type Checker struct {
	fset *token.FileSet
}

// checkAST runs every rule against one parsed file.
func (c *Checker) checkAST(file *ast.File) []Finding {
	findings := make([]Finding, 0)
	for _, layout := range collectTypeLayouts(file) {
		findings = append(findings, c.checkMethodBlock(file, layout)...)
		findings = append(findings, c.checkConstructorLast(file, layout)...)
	}
	return append(findings, c.checkHelpersBeforePublic(file)...)
}

// checkMethodBlock requires every declaration between a type and its last
// method to be a method of that type (witnesses and constructors excepted —
// constructor placement is checkConstructorLast's concern).
func (c *Checker) checkMethodBlock(file *ast.File, layout *typeLayout) []Finding {
	if len(layout.methods) == 0 {
		return nil
	}
	lastMethod := layout.methods[len(layout.methods)-1]
	for i := layout.declIndex + 1; i <= lastMethod; i++ {
		decl := file.Decls[i]
		if isWitness(decl) || slices.Contains(layout.methods, i) || slices.Contains(layout.ctors, i) {
			continue
		}
		return []Finding{{
			Pos: c.fset.Position(decl.Pos()),
			Message: fmt.Sprintf(
				"%s breaks the method block of type %s — methods must immediately follow their struct",
				declName(decl),
				layout.name,
			),
		}}
	}
	return nil
}

// checkConstructorLast requires every method of a type to precede that type's
// first constructor.
func (c *Checker) checkConstructorLast(file *ast.File, layout *typeLayout) []Finding {
	if len(layout.methods) == 0 || len(layout.ctors) == 0 {
		return nil
	}
	firstCtor := layout.ctors[0]
	findings := make([]Finding, 0)
	for _, idx := range layout.methods {
		if idx < firstCtor {
			continue
		}
		findings = append(findings, Finding{
			Pos: c.fset.Position(file.Decls[idx].Pos()),
			Message: fmt.Sprintf(
				"method %s of type %s appears after its constructor %s — constructors go last",
				declName(file.Decls[idx]),
				layout.name,
				declName(file.Decls[firstCtor]),
			),
		})
	}
	return findings
}

// checkHelpersBeforePublic requires package-level unexported functions to
// precede the first exported type or function. init and main are lifecycle
// hooks, not helpers, and are exempt.
func (c *Checker) checkHelpersBeforePublic(file *ast.File) []Finding {
	firstPublic := slices.IndexFunc(file.Decls, isPublicDecl)
	if firstPublic < 0 {
		return nil
	}
	findings := make([]Finding, 0)
	for _, decl := range file.Decls[firstPublic+1:] {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || fd.Name.IsExported() {
			continue
		}
		if fd.Name.Name == "init" || fd.Name.Name == "main" {
			continue
		}
		findings = append(findings, Finding{
			Pos: c.fset.Position(fd.Pos()),
			Message: fmt.Sprintf(
				"package-level helper %s must precede public declarations",
				fd.Name.Name,
			),
		})
	}
	return findings
}

// CheckSource parses and checks one source blob named path.
func (c *Checker) CheckSource(path string, src []byte) ([]Finding, error) {
	if isGenerated(src) {
		return nil, nil
	}
	file, err := parser.ParseFile(c.fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return c.checkAST(file), nil
}

// CheckFile checks one .go file.
func (c *Checker) CheckFile(path string) ([]Finding, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return c.CheckSource(path, src)
}

// CheckDir checks every non-test, non-generated .go file under root.
func (c *Checker) CheckDir(root string) ([]Finding, error) {
	findings := make([]Finding, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && shouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fileFindings, err := c.CheckFile(path)
		if err != nil {
			return err
		}
		findings = append(findings, fileFindings...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return findings, nil
}

// NewChecker returns a checker with a fresh file set.
func NewChecker() *Checker {
	return &Checker{fset: token.NewFileSet()}
}
