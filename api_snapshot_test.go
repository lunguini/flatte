package flatte

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// TestPublicAPISnapshot is a type-checked API test. Package discovery comes
// from `go list`, while declarations, generic constraints, evaluated
// constants, fields, and methods come from go/types. That combination keeps
// the snapshot tied to the build-selected public packages and to the API the
// compiler sees, rather than to a hand-maintained package list or source
// spelling alone.
func TestPublicAPISnapshot(t *testing.T) {
	got := strings.Join(collectPublicAPI(t), "\n") + "\n"
	if os.Getenv("FLAT_REGEN_API_SNAPSHOT") != "" {
		if err := os.WriteFile(filepath.Join(moduleRoot(t), "testdata", "public-api.golden"), []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("regenerated testdata/public-api.golden")
		return
	}
	want, err := os.ReadFile(filepath.Join(moduleRoot(t), "testdata", "public-api.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal([]byte(got), want) {
		t.Fatalf("public API snapshot mismatch\n\nGot:\n%s", got)
	}
}

// TestPublicAPISnapshotIncludesContracts protects the protection itself. In
// particular, these assertions make generic constraints, evaluated values,
// declaration order, and sealed interface methods visible in review when the
// snapshot implementation is changed.
func TestPublicAPISnapshotIncludesContracts(t *testing.T) {
	items := strings.Join(collectPublicAPI(t), "\n")
	for _, want := range []string{
		"func Named[S any](name string, apply func(*S)) StateUpdate[S]",
		"const KeyUp Key = 1",
		"field KeyEvent.Rune rune [order=1]",
		"method Mod.Contains(mods Mod) bool",
		"interface Event method isEvent()",
	} {
		if !strings.Contains(items, want) {
			t.Errorf("API snapshot does not include %q", want)
		}
	}
}

type apiPackage struct {
	path  string
	dir   string
	files []string

	fset     *token.FileSet
	filesAST []*ast.File
	types    *types.Package
	exports  map[string]string
}

type goListPackage struct {
	ImportPath      string
	Dir             string
	Name            string
	Export          string
	GoFiles         []string
	CgoFiles        []string
	CompiledGoFiles []string
}

type apiItem struct {
	key  string
	text string
}

func collectPublicAPI(t *testing.T) []string {
	t.Helper()
	packages := discoverPackages(t)
	var out []string
	for _, pkg := range packages {
		loadAndCheckPackage(t, pkg)
		items := exportedItems(pkg)
		sort.Slice(items, func(i, j int) bool {
			if items[i].key != items[j].key {
				return items[i].key < items[j].key
			}
			return items[i].text < items[j].text
		})
		label := strings.TrimPrefix(pkg.path, modulePath(t))
		label = strings.TrimPrefix(label, "/")
		if label == "" {
			label = "flatte"
		}
		out = append(out, label)
		for _, item := range items {
			out = append(out, "  "+item.text)
		}
	}
	return out
}

func discoverPackages(t *testing.T) []*apiPackage {
	t.Helper()
	root := moduleRoot(t)
	cmd := exec.Command("go", "list", "-json", "-export", "-deps", "./...")
	cmd.Dir = root
	data, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("go list: %v\n%s", err, exitErr.Stderr)
		}
		t.Fatal(err)
	}
	module := modulePath(t)
	decoder := json.NewDecoder(bytes.NewReader(data))
	exports := make(map[string]string)
	var listedPackages []goListPackage
	var packages []*apiPackage
	for {
		var listed goListPackage
		if err := decoder.Decode(&listed); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode go list output: %v", err)
		}
		listedPackages = append(listedPackages, listed)
		if listed.Export != "" {
			exports[listed.ImportPath] = listed.Export
		}
	}
	for _, listed := range listedPackages {
		if listed.Name == "main" || listed.ImportPath != module && !strings.HasPrefix(listed.ImportPath, module+"/") {
			continue
		}
		if strings.Contains(listed.ImportPath, "/internal/") || strings.HasSuffix(listed.ImportPath, "/internal") {
			continue
		}
		files := listed.CompiledGoFiles
		if len(files) == 0 {
			files = append(append([]string{}, listed.GoFiles...), listed.CgoFiles...)
		}
		if len(files) == 0 {
			continue
		}
		packages = append(packages, &apiPackage{path: listed.ImportPath, dir: listed.Dir, files: files, exports: exports})
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].path < packages[j].path })
	if len(packages) == 0 {
		t.Fatal("go list discovered no public packages")
	}
	return packages
}

func loadAndCheckPackage(t *testing.T, pkg *apiPackage) {
	t.Helper()
	pkg.fset = token.NewFileSet()
	for _, name := range pkg.files {
		if !filepath.IsAbs(name) {
			name = filepath.Join(pkg.dir, name)
		}
		file, err := parser.ParseFile(pkg.fset, name, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("%s: parse %s: %v", pkg.path, name, err)
		}
		pkg.filesAST = append(pkg.filesAST, file)
	}
	info := &types.Info{
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
		Types: make(map[ast.Expr]types.TypeAndValue),
	}
	lookup := func(path string) (io.ReadCloser, error) {
		name := pkg.exports[path]
		if name == "" {
			return nil, fmt.Errorf("no export data for %q", path)
		}
		return os.Open(name)
	}
	conf := types.Config{Importer: importer.ForCompiler(pkg.fset, "gc", lookup)}
	typed, err := conf.Check(pkg.path, pkg.fset, pkg.filesAST, info)
	if err != nil {
		t.Fatalf("%s: type-check: %v", pkg.path, err)
	}
	pkg.types = typed
}

func exportedItems(pkg *apiPackage) []apiItem {
	var specs = make(map[string]*ast.TypeSpec)
	for _, file := range pkg.filesAST {
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.TYPE {
				for _, spec := range gen.Specs {
					typeSpec := spec.(*ast.TypeSpec)
					specs[typeSpec.Name.Name] = typeSpec
				}
			}
		}
	}
	var items []apiItem
	scope := pkg.types.Scope()
	for _, name := range scope.Names() {
		if !token.IsExported(name) {
			continue
		}
		switch obj := scope.Lookup(name).(type) {
		case *types.Const:
			items = append(items, apiItem{"const:" + name, "const " + name + " " + formatType(obj.Type(), pkg) + " = " + obj.Val().String()})
		case *types.Var:
			items = append(items, apiItem{"var:" + name, "var " + name + " " + formatType(obj.Type(), pkg)})
		case *types.Func:
			items = append(items, apiItem{"func:" + name, "func " + name + signatureText(obj.Type().(*types.Signature), pkg)})
		case *types.TypeName:
			items = append(items, typeItems(pkg, obj, specs[name])...)
		}
	}
	return items
}

func typeItems(pkg *apiPackage, obj *types.TypeName, spec *ast.TypeSpec) []apiItem {
	named, _ := obj.Type().(*types.Named)
	decl := "type " + obj.Name()
	if named != nil {
		decl += typeParamsText(named.TypeParams(), pkg)
	}
	if spec != nil && spec.Assign.IsValid() {
		decl += " = " + formatNode(pkg.fset, spec.Type)
	} else if spec != nil {
		switch spec.Type.(type) {
		case *ast.StructType:
			decl += " struct"
		case *ast.InterfaceType:
			decl += " interface"
		default:
			decl += " " + formatNode(pkg.fset, spec.Type)
		}
	} else {
		decl += " " + formatType(obj.Type().Underlying(), pkg)
	}
	items := []apiItem{{"type:" + obj.Name(), decl}}
	if named == nil {
		return items
	}
	if st, ok := named.Underlying().(*types.Struct); ok {
		for i := 0; i < st.NumFields(); i++ {
			field := st.Field(i)
			if !field.Exported() {
				continue
			}
			text := "field " + obj.Name() + "." + field.Name() + " " + formatType(field.Type(), pkg) + " [order=" + fmt.Sprint(i) + "]"
			if tag := st.Tag(i); tag != "" {
				text += " `" + tag + "`"
			}
			items = append(items, apiItem{fmt.Sprintf("type:%s:field:%06d", obj.Name(), i), text})
		}
	}
	if iface, ok := named.Underlying().(*types.Interface); ok {
		iface.Complete()
		for i := 0; i < iface.NumExplicitMethods(); i++ {
			method := iface.ExplicitMethod(i)
			items = append(items, apiItem{"type:" + obj.Name() + fmt.Sprintf(":interface:%06d", i), "interface " + obj.Name() + " method " + method.Name() + signatureText(method.Type().(*types.Signature), pkg)})
		}
		for i := 0; i < iface.NumEmbeddeds(); i++ {
			items = append(items, apiItem{"type:" + obj.Name() + fmt.Sprintf(":embed:%06d", i), "interface " + obj.Name() + " embeds " + formatType(iface.EmbeddedType(i), pkg)})
		}
	}
	if named != nil {
		items = append(items, methodItems(pkg, named)...)
	}
	return items
}

func methodItems(pkg *apiPackage, named *types.Named) []apiItem {
	var items []apiItem
	direct := make(map[*types.Func]bool)
	for i := 0; i < named.NumMethods(); i++ {
		method := named.Method(i)
		if !method.Exported() {
			continue
		}
		direct[method] = true
		sig := method.Type().(*types.Signature)
		receiver := formatType(sig.Recv().Type(), pkg)
		items = append(items, apiItem{"method:" + receiver + "." + method.Name(), "method " + receiver + "." + method.Name() + signatureText(sig, pkg)})
	}
	if _, ok := named.Underlying().(*types.Interface); ok {
		return items
	}
	// Record both value and pointer method sets. The second pass catches
	// promoted methods and pointer-only methods that a declaration walk misses.
	for _, typ := range []types.Type{named, types.NewPointer(named)} {
		set := types.NewMethodSet(typ)
		receiver := methodSetReceiver(typ, pkg)
		for i := 0; i < set.Len(); i++ {
			method, ok := set.At(i).Obj().(*types.Func)
			if !ok {
				continue
			}
			if !method.Exported() || direct[method] {
				continue
			}
			sig := method.Type().(*types.Signature)
			items = append(items, apiItem{"methodset:" + receiver + "." + method.Name(), "methodset " + receiver + "." + method.Name() + signatureText(sig, pkg)})
		}
	}
	return items
}

func methodSetReceiver(typ types.Type, pkg *apiPackage) string {
	if ptr, ok := typ.(*types.Pointer); ok {
		return "*" + methodSetReceiver(ptr.Elem(), pkg)
	}
	if named, ok := typ.(*types.Named); ok {
		name := named.Obj().Name()
		if params := named.TypeParams(); params != nil && params.Len() > 0 {
			args := make([]string, params.Len())
			for i := range args {
				args[i] = params.At(i).Obj().Name()
			}
			name += "[" + strings.Join(args, ", ") + "]"
		}
		if named.Obj().Pkg() != nil && named.Obj().Pkg().Path() != pkg.path {
			name = named.Obj().Pkg().Name() + "." + name
		}
		return name
	}
	return formatType(typ, pkg)
}

func typeParamsText(list *types.TypeParamList, pkg *apiPackage) string {
	if list == nil || list.Len() == 0 {
		return ""
	}
	params := make([]string, list.Len())
	for i := 0; i < list.Len(); i++ {
		param := list.At(i)
		params[i] = param.Obj().Name() + " " + formatType(param.Constraint(), pkg)
	}
	return "[" + strings.Join(params, ", ") + "]"
}

func signatureText(sig *types.Signature, pkg *apiPackage) string {
	var b strings.Builder
	b.WriteString(typeParamsText(sig.TypeParams(), pkg))
	b.WriteString(tupleText(sig.Params(), sig.Variadic(), pkg))
	if sig.Results().Len() > 0 {
		b.WriteByte(' ')
		b.WriteString(resultText(sig.Results(), pkg))
	}
	return canonicalTokens(b.String())
}

func tupleText(tuple *types.Tuple, variadic bool, pkg *apiPackage) string {
	parts := make([]string, tuple.Len())
	for i := 0; i < tuple.Len(); i++ {
		param := tuple.At(i)
		typ := param.Type()
		if variadic && i == tuple.Len()-1 {
			typ = typ.(*types.Slice).Elem()
			parts[i] = param.Name() + " ..." + formatType(typ, pkg)
		} else {
			parts[i] = param.Name() + " " + formatType(typ, pkg)
		}
		parts[i] = strings.TrimSpace(parts[i])
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func resultText(tuple *types.Tuple, pkg *apiPackage) string {
	if tuple.Len() == 1 && tuple.At(0).Name() == "" {
		return formatType(tuple.At(0).Type(), pkg)
	}
	return tupleText(tuple, false, pkg)
}

func formatType(typ types.Type, pkg *apiPackage) string {
	return canonicalTokens(types.TypeString(typ, func(other *types.Package) string {
		if other == nil {
			return ""
		}
		if other.Path() == pkg.path {
			return ""
		}
		return other.Name()
	}))
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

func modulePath(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Path}}")
	cmd.Dir = moduleRoot(t)
	data, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list module: %v", err)
	}
	return strings.TrimSpace(string(data))
}

func formatNode(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		panic(err)
	}
	return canonicalTokens(buf.String())
}

// canonicalTokens removes source-layout decisions while retaining literal and
// tag spelling. Tokenizing instead of strings.Fields keeps quoted constants
// and raw tags byte-for-byte part of the contract.
func canonicalTokens(src string) string {
	var scan scanner.Scanner
	var fset token.FileSet
	file := fset.AddFile("api", -1, len(src))
	scan.Init(file, []byte(src), nil, scanner.ScanComments)
	var out strings.Builder
	var prev, prevPrev token.Token
	for {
		_, tok, lit := scan.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.COMMENT || tok == token.SEMICOLON {
			continue
		}
		if out.Len() > 0 && needsTokenSpace(prevPrev, prev, tok) {
			out.WriteByte(' ')
		}
		if lit == "" {
			lit = tok.String()
		}
		out.WriteString(lit)
		prevPrev, prev = prev, tok
	}
	return out.String()
}

func needsTokenSpace(prevPrev, prev, curr token.Token) bool {
	if prev == token.COMMA || prev == token.SEMICOLON {
		return true
	}
	if curr == token.RPAREN || curr == token.RBRACK || curr == token.RBRACE || curr == token.COMMA || curr == token.SEMICOLON || curr == token.PERIOD {
		return false
	}
	if prev == token.LPAREN || prev == token.LBRACK || prev == token.LBRACE || prev == token.PERIOD {
		return false
	}
	if curr == token.LPAREN || curr == token.LBRACK || curr == token.PERIOD {
		return false
	}
	if prev == token.RBRACK && prevPrev == token.LBRACK {
		return false
	}
	if prev == token.ASSIGN || curr == token.ASSIGN {
		return true
	}
	if prev == token.ELLIPSIS || curr == token.ELLIPSIS {
		return true
	}
	if prev == token.RPAREN || prev == token.RBRACK || tokenWord(prev) && tokenWord(curr) {
		return true
	}
	return false
}

func tokenWord(tok token.Token) bool {
	return tok == token.IDENT || tok >= token.BREAK && tok <= token.VAR || tok >= token.INT && tok <= token.STRING
}
