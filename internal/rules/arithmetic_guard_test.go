package rules

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ARCHITECTURE_ESSENTIALS.md section 8: the signer and threshold rules call
// internal/threshold and never restate the arithmetic, and a weight sum, a 20 cap
// or a zero-weight filter written inside a rule is the bug. Output tests cannot
// keep that true, because an inlined copy passes every fixture until the day it
// drifts from the arithmetic task 0.7 verified. This guard parses the package's
// own source and fails the build on the three shapes instead.
//
// It is syntactic on purpose: go/parser and go/ast from the standard library, no
// type checker and no production linter. That means it cannot tell an observed
// signer's Weight from a declared one, which is why the filter allowlist below
// names functions with a reason each rather than whole files.

const thresholdImportPath = "github.com/posture-sec/guage/internal/threshold"

// weightFilterAllowlist names the functions allowed to compare a Weight with 0.
// Each entry must still exist and still contain such a comparison, so an entry
// cannot outlive the code it excuses.
var weightFilterAllowlist = map[string]string{
	// The one standing-control filter in this package. sigNoSigningWeight and
	// sigSingleKeyHigh use it to list which entries hold control, not to decide
	// reachability; reachability always goes through internal/threshold.
	"participating": "lists the signers that hold standing control; reachability is still threshold.Reachable and threshold.MinSigners",
	// A declared weight of 0 is an intent to remove (ARCHITECTURE_ESSENTIALS.md
	// section 8), so a signer the chain still shows above 0 is extra. This reads
	// one observed weight against one declaration; nothing is summed or capped.
	"sigUnexpectedSigner": "compares one observed weight with a declared remove-intent of 0; no sum, no cap",
	// expected.Weight is the policy's declared weight, not the chain's. A declared
	// signer at 0 promises nothing, so it cannot be missing.
	"sigMissingExpected": "reads the declared weight from policy to skip remove-intents; no sum, no cap",
}

// requiredDelegations names the call sites that must keep asking internal/threshold.
// If one disappears or stops calling the function named, the arithmetic has moved
// somewhere this guard has not been told about, and the test fails.
var requiredDelegations = map[string][]string{
	"sigSingleKeyHigh":        {"MinSigners", "Reachable"},
	"sigThresholdUnreachable": {"Reachable"},
	"reachability":            {"MinSigners"},
}

type arithmeticViolation struct {
	function string // enclosing top-level function, or "" at package level
	kind     string // "weight-sum", "cap-20" or "weight-filter"
	position string
	snippet  string
}

func (v arithmeticViolation) String() string {
	where := v.function
	if where == "" {
		where = "package level"
	}
	return v.position + " in " + where + ": " + v.kind + " `" + v.snippet + "`"
}

// fileScan is what one parsed file says about restated arithmetic and delegation.
type fileScan struct {
	violations []arithmeticViolation
	// calls maps a top-level function name to the threshold functions it calls.
	calls map[string]map[string]bool
	// functions is every top-level function and method declared in the file.
	functions map[string]bool
}

func scanRulesFile(fset *token.FileSet, file *ast.File) fileScan {
	scan := fileScan{calls: map[string]map[string]bool{}, functions: map[string]bool{}}
	thresholdName := ""
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != thresholdImportPath {
			continue
		}
		thresholdName = "threshold"
		if imp.Name != nil {
			thresholdName = imp.Name.Name
		}
	}

	for _, decl := range file.Decls {
		name := ""
		var root ast.Node = decl
		if fn, ok := decl.(*ast.FuncDecl); ok {
			name = fn.Name.Name
			scan.functions[name] = true
			if fn.Body == nil {
				continue
			}
			root = fn.Body
		}
		ast.Inspect(root, func(n ast.Node) bool {
			add := func(kind string, node ast.Node) {
				scan.violations = append(scan.violations, arithmeticViolation{
					function: name,
					kind:     kind,
					position: fset.Position(node.Pos()).String(),
					snippet:  exprString(node),
				})
			}
			switch node := n.(type) {
			case *ast.AssignStmt:
				if node.Tok == token.ADD_ASSIGN {
					for _, rhs := range node.Rhs {
						if mentionsWeight(rhs) {
							add("weight-sum", node)
						}
					}
				}
			case *ast.BinaryExpr:
				if node.Op == token.ADD && (mentionsWeight(node.X) || mentionsWeight(node.Y)) {
					add("weight-sum", node)
				}
				if isWeightFilter(node) {
					add("weight-filter", node)
				}
			case *ast.BasicLit:
				if node.Kind == token.INT {
					if v, err := strconv.ParseInt(strings.ReplaceAll(node.Value, "_", ""), 0, 64); err == nil && v == 20 {
						add("cap-20", node)
					}
				}
			case *ast.CallExpr:
				if sel, ok := node.Fun.(*ast.SelectorExpr); ok && thresholdName != "" && name != "" {
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == thresholdName {
						if scan.calls[name] == nil {
							scan.calls[name] = map[string]bool{}
						}
						scan.calls[name][sel.Sel.Name] = true
					}
				}
			}
			return true
		})
	}
	return scan
}

// mentionsWeight reports whether an expression reads a field named Weight.
func mentionsWeight(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "Weight" {
			found = true
		}
		return !found
	})
	return found
}

// isWeightFilter matches a Weight field compared with the literal 0, in either
// operand order and with any comparison operator, plus `>= 1` and `< 1`, which
// are the same filter spelled differently.
func isWeightFilter(b *ast.BinaryExpr) bool {
	switch b.Op {
	case token.GTR, token.GEQ, token.LSS, token.LEQ, token.EQL, token.NEQ:
	default:
		return false
	}
	weight, literal := b.X, b.Y
	op := b.Op
	if !isWeightSelector(weight) {
		weight, literal = b.Y, b.X
		op = mirror(op)
	}
	if !isWeightSelector(weight) {
		return false
	}
	lit, ok := literal.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return false
	}
	switch lit.Value {
	case "0":
		return true
	case "1":
		return op == token.GEQ || op == token.LSS
	}
	return false
}

func isWeightSelector(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Weight"
}

func mirror(op token.Token) token.Token {
	switch op {
	case token.GTR:
		return token.LSS
	case token.LSS:
		return token.GTR
	case token.GEQ:
		return token.LEQ
	case token.LEQ:
		return token.GEQ
	}
	return op
}

// exprString renders a node compactly for a failure message without needing the
// original source bytes.
func exprString(n ast.Node) string {
	switch node := n.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.BasicLit:
		return node.Value
	case *ast.SelectorExpr:
		return exprString(node.X) + "." + node.Sel.Name
	case *ast.BinaryExpr:
		return exprString(node.X) + " " + node.Op.String() + " " + exprString(node.Y)
	case *ast.AssignStmt:
		lhs := make([]string, 0, len(node.Lhs))
		for _, e := range node.Lhs {
			lhs = append(lhs, exprString(e))
		}
		rhs := make([]string, 0, len(node.Rhs))
		for _, e := range node.Rhs {
			rhs = append(rhs, exprString(e))
		}
		return strings.Join(lhs, ", ") + " " + node.Tok.String() + " " + strings.Join(rhs, ", ")
	case *ast.CallExpr:
		return exprString(node.Fun) + "(...)"
	case *ast.ParenExpr:
		return "(" + exprString(node.X) + ")"
	case *ast.IndexExpr:
		return exprString(node.X) + "[" + exprString(node.Index) + "]"
	}
	return "..."
}

// guardReport applies the allowlist and the delegation list to a set of scans and
// returns every problem as a sentence. It is shared by the tree test and the
// synthetic tests, so the latter prove the same code that guards the tree.
func guardReport(scans []fileScan, allow map[string]string, delegations map[string][]string) []string {
	var problems []string
	functions := map[string]bool{}
	calls := map[string]map[string]bool{}
	filterHits := map[string]bool{}
	for _, scan := range scans {
		for fn := range scan.functions {
			functions[fn] = true
		}
		for fn, called := range scan.calls {
			if calls[fn] == nil {
				calls[fn] = map[string]bool{}
			}
			for c := range called {
				calls[fn][c] = true
			}
		}
		for _, v := range scan.violations {
			if v.kind == "weight-filter" {
				if _, ok := allow[v.function]; ok && v.function != "" {
					filterHits[v.function] = true
					continue
				}
			}
			problems = append(problems, "restated threshold arithmetic: "+v.String()+
				"; call internal/threshold instead (ARCHITECTURE_ESSENTIALS.md section 8)")
		}
	}
	for fn := range allow {
		switch {
		case !functions[fn]:
			problems = append(problems, "allowlisted function "+fn+" no longer exists; remove its entry or restore it")
		case !filterHits[fn]:
			problems = append(problems, "allowlisted function "+fn+" no longer compares a Weight with 0; remove its entry")
		}
	}
	for fn, want := range delegations {
		if !functions[fn] {
			problems = append(problems, "delegating call site "+fn+" no longer exists; the threshold arithmetic it asked for has moved")
			continue
		}
		for _, callee := range want {
			if !calls[fn][callee] {
				problems = append(problems, "delegating call site "+fn+" no longer calls threshold."+callee)
			}
		}
	}
	sort.Strings(problems)
	return problems
}

func TestNoRestatedArithmetic(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package directory: %v", err)
	}
	fset := token.NewFileSet()
	var scans []fileScan
	parsed := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		scans = append(scans, scanRulesFile(fset, file))
		parsed++
	}
	// rules.go, signers.go, thresholds.go and facts.go at least; a guard that
	// parsed nothing would pass trivially.
	if parsed < 4 {
		t.Fatalf("parsed %d non-test files, want at least 4", parsed)
	}
	for _, problem := range guardReport(scans, weightFilterAllowlist, requiredDelegations) {
		t.Error(problem)
	}
}

// The synthetic cases run the same scanner and report over source written here,
// so the guard is shown to fail on each shape without editing the tree.

func scanSource(t *testing.T, src string) fileScan {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "synthetic.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parsing synthetic source: %v", err)
	}
	return scanRulesFile(fset, file)
}

const syntheticHeader = `package rules

import (
	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/threshold"
)
`

func TestNoRestatedArithmeticCatchesEachShape(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "accumulating range",
			body: `func inlined(signers []model.Signer, need int) bool {
	total := 0
	for _, s := range signers {
		total += s.Weight
	}
	return total >= need
}`,
			want: "weight-sum `total += s.Weight`",
		},
		{
			name: "sum by plain addition",
			body: `func inlined(a, b model.Signer) int { return a.Weight + b.Weight }`,
			want: "weight-sum `a.Weight + b.Weight`",
		},
		{
			name: "converted weight still counts",
			body: `func inlined(signers []model.Signer) int {
	total := 0
	for _, s := range signers {
		total += int(s.Weight)
	}
	return total
}`,
			want: "weight-sum",
		},
		{
			name: "signature cap literal",
			body: `func capped(signers []model.Signer) []model.Signer {
	if len(signers) > 20 {
		return signers[:20]
	}
	return signers
}`,
			want: "cap-20 `20`",
		},
		{
			name: "cap as a package constant",
			body: `const maxSigs = 0x14`,
			want: "package level: cap-20",
		},
		{
			name: "zero-weight filter outside the allowlist",
			body: `func rogue(signers []model.Signer) (out []model.Signer) {
	for _, s := range signers {
		if s.Weight > 0 {
			out = append(out, s)
		}
	}
	return out
}`,
			want: "rogue: weight-filter `s.Weight > 0`",
		},
		{
			name: "filter with operands swapped",
			body: `func rogue(s model.Signer) bool { return 0 < s.Weight }`,
			want: "weight-filter `0 < s.Weight`",
		},
		{
			name: "filter spelled as at least one",
			body: `func rogue(s model.Signer) bool { return s.Weight >= 1 }`,
			want: "weight-filter `s.Weight >= 1`",
		},
		{
			name: "skip spelled as at most zero",
			body: `func rogue(s model.Signer) bool { return s.Weight <= 0 }`,
			want: "weight-filter `s.Weight <= 0`",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			scan := scanSource(t, syntheticHeader+"\nvar _ = threshold.Reachable\n\n"+tt.body+"\n")
			problems := guardReport([]fileScan{scan}, nil, nil)
			if len(problems) == 0 {
				t.Fatalf("guard passed source it must refuse:\n%s", tt.body)
			}
			joined := strings.Join(problems, "\n")
			if !strings.Contains(joined, tt.want) {
				t.Errorf("guard report does not mention %q:\n%s", tt.want, joined)
			}
		})
	}
}

func TestNoRestatedArithmeticAcceptsDelegationAndReads(t *testing.T) {
	t.Parallel()
	// Reading one weight for display or comparing it with a threshold is not
	// restating the arithmetic, and the allowlisted filter is accepted only under
	// its own name.
	src := syntheticHeader + `
func participating(signers []model.Signer) []model.Signer {
	out := make([]model.Signer, 0, len(signers))
	for _, s := range signers {
		if s.Weight > 0 {
			out = append(out, s)
		}
	}
	return out
}

func delegate(signers []model.Signer, high int) bool {
	count, ok := threshold.MinSigners(signers, high)
	return ok && count == 1 && threshold.Reachable(signers, high)
}

func reads(s model.Signer, medium int) bool {
	return medium > 0 && s.Weight >= medium && s.Weight != 7
}
`
	scan := scanSource(t, src)
	problems := guardReport([]fileScan{scan},
		map[string]string{"participating": "test"},
		map[string][]string{"delegate": {"MinSigners", "Reachable"}})
	if len(problems) != 0 {
		t.Errorf("guard refused clean source:\n%s", strings.Join(problems, "\n"))
	}
}

func TestNoRestatedArithmeticFailsOnAStaleAllowlist(t *testing.T) {
	t.Parallel()
	src := syntheticHeader + `
var _ = threshold.Reachable

func participating(signers []model.Signer) []model.Signer { return signers }
`
	scan := scanSource(t, src)
	for _, tt := range []struct {
		name        string
		allow       map[string]string
		delegations map[string][]string
		want        string
	}{
		{"allowlisted function removed", map[string]string{"gone": "test"}, nil,
			"allowlisted function gone no longer exists"},
		{"allowlisted function lost its filter", map[string]string{"participating": "test"}, nil,
			"allowlisted function participating no longer compares a Weight with 0"},
		{"delegating call site removed", nil, map[string][]string{"sigSingleKeyHigh": {"MinSigners"}},
			"delegating call site sigSingleKeyHigh no longer exists"},
		{"delegating call site stopped delegating", nil, map[string][]string{"participating": {"Reachable"}},
			"delegating call site participating no longer calls threshold.Reachable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			problems := guardReport([]fileScan{scan}, tt.allow, tt.delegations)
			joined := strings.Join(problems, "\n")
			if !strings.Contains(joined, tt.want) {
				t.Errorf("guard report does not mention %q:\n%s", tt.want, joined)
			}
		})
	}
}

func TestNoRestatedArithmeticHonoursAnImportAlias(t *testing.T) {
	t.Parallel()
	src := `package rules

import (
	"github.com/posture-sec/guage/internal/model"
	th "github.com/posture-sec/guage/internal/threshold"
)

func delegate(signers []model.Signer) bool { return th.Reachable(signers, 1) }
`
	scan := scanSource(t, src)
	if problems := guardReport([]fileScan{scan}, nil, map[string][]string{"delegate": {"Reachable"}}); len(problems) != 0 {
		t.Errorf("aliased threshold import not recognised:\n%s", strings.Join(problems, "\n"))
	}
}
