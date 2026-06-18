package flat

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestPublicAPISnapshot(t *testing.T) {
	got := strings.Join(collectPublicAPI(t), "\n") + "\n"
	want, err := os.ReadFile(filepath.Join("testdata", "public-api.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal([]byte(got), want) {
		t.Fatalf("public API snapshot mismatch\n\nGot:\n%s", got)
	}
}

func collectPublicAPI(t *testing.T) []string {
	t.Helper()
	packages := []struct {
		label string
		dir   string
	}{
		{"flat", "."},
		{"flatui", "flatui"},
		{"flatest", "flatest"},
	}
	var out []string
	for _, pkg := range packages {
		out = append(out, pkg.label)
		for _, item := range exportedItems(t, pkg.dir) {
			out = append(out, "  "+item)
		}
	}
	return out
}

func exportedItems(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(info os.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("%s: parsed %d packages, want 1", dir, len(pkgs))
	}
	var items []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				items = append(items, exportedDeclItems(decl)...)
			}
		}
	}
	sort.Strings(items)
	return items
}

func exportedDeclItems(decl ast.Decl) []string {
	switch d := decl.(type) {
	case *ast.GenDecl:
		return exportedGenDeclItems(d)
	case *ast.FuncDecl:
		if !d.Name.IsExported() {
			return nil
		}
		if d.Recv == nil {
			return []string{"func " + d.Name.Name}
		}
		receiver := receiverTypeName(d.Recv)
		if receiver == "" || !ast.IsExported(receiver) {
			return nil
		}
		return []string{"method " + receiver + "." + d.Name.Name}
	default:
		return nil
	}
}

func exportedGenDeclItems(decl *ast.GenDecl) []string {
	var items []string
	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if s.Name.IsExported() {
				items = append(items, "type "+s.Name.Name)
			}
		case *ast.ValueSpec:
			kind := strings.ToLower(decl.Tok.String())
			for _, name := range s.Names {
				if name.IsExported() {
					items = append(items, kind+" "+name.Name)
				}
			}
		}
	}
	return items
}

func receiverTypeName(fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}
	expr := fields.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	if indexed, ok := expr.(*ast.IndexExpr); ok {
		if ident, ok := indexed.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	if indexed, ok := expr.(*ast.IndexListExpr); ok {
		if ident, ok := indexed.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}
