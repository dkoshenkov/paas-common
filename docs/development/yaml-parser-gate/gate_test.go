package gate

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4/plugin/aliasdata"
	"go.yaml.in/yaml/v4/plugin/loaderlimits"
	"go.yaml.in/yaml/v4/plugin/mergekey"

	yaml "go.yaml.in/yaml/v4"
)

func depth(n *yaml.Node) int {
	m := 0
	for _, c := range n.Content {
		if d := depth(c); d > m {
			m = d
		}
	}
	if n.Kind == yaml.MappingNode || n.Kind == yaml.SequenceNode {
		m++
	}
	return m
}

func nodes(n *yaml.Node) int {
	count := 1
	for _, c := range n.Content {
		count += nodes(c)
	}
	return count
}

func block(n int, leaf string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%sk:\n", strings.Repeat("  ", i))
	}
	b.WriteString(strings.Repeat("  ", n))
	b.WriteString(leaf)
	b.WriteByte('\n')
	return b.String()
}
func flow(n int) string { return strings.Repeat("[", n) + "0" + strings.Repeat("]", n) }
func indentless(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%sk:\n%s-\n", strings.Repeat("  ", i), strings.Repeat("  ", i))
	}
	b.WriteString(strings.Repeat("  ", n) + "0\n")
	return b.String()
}

func load(input string, extra ...yaml.Option) (yaml.Node, error) {
	opts := []yaml.Option{yaml.WithPlugin(loaderlimits.New(loaderlimits.DepthValue(64)))}
	opts = append(opts, extra...)
	l, e := yaml.NewLoader(strings.NewReader(input), opts...)
	if e != nil {
		return yaml.Node{}, e
	}
	var n yaml.Node
	e = l.Load(&n)
	return n, e
}

func TestDepthCounterexamples(t *testing.T) {
	cases := []struct {
		name, input string
		wantDepth   int
		wantErr     bool
	}{
		{"flow64", flow(64), 64, false},
		{"flow65", flow(65), 0, true},
		{"block64", block(64, "0"), 64, false},
		{"block65", block(65, "0"), 0, true},
		{"mixed65", block(32, flow(33)), 65, false},
		{"mixed128", block(64, flow(64)), 128, false},
		{"indentless64", indentless(32), 64, false},
		{"indentless66", indentless(33), 66, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, e := load(c.input)
			if (e != nil) != c.wantErr {
				t.Fatalf("unexpected outcome depth=%d err=%v", depth(&n), e)
			}
			if e == nil && depth(&n) != c.wantDepth {
				t.Fatalf("depth=%d want=%d", depth(&n), c.wantDepth)
			}
			t.Logf("bytes=%d produced_tree_depth=%d err=%v", len(c.input), depth(&n), e)
		})
	}
}

func TestAliasesAndMerge(t *testing.T) {
	deny, e := aliasdata.New(aliasdata.WithDisable(true))
	if e != nil {
		t.Fatal(e)
	}
	_, e = load("a: &a [1]\nb: *a\n", yaml.WithPlugin(deny))
	if e == nil {
		t.Fatal("alias unexpectedly accepted")
	}
	t.Log("alias rejected:", e)
	n, e := load("a:\n  <<: {b: 1}\n", yaml.WithPlugin(deny), yaml.WithPlugin(mergekey.New(mergekey.WithDisable(true))))
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("MERGE COUNTEREXAMPLE: disable-merge accepted tree, nodes=%d; explicit reject hook still needed", nodes(&n))
}

var errDocuments = errors.New("document budget exceeded")

type docPolicy struct{ count int }

func (p *docPolicy) NewAliasDataContext(yaml.AliasDataServices) (yaml.AliasDataContext, error) {
	return p, nil
}

func (p *docPolicy) BeginDocument() error {
	p.count++
	if p.count > 128 {
		return errDocuments
	}
	return nil
}
func (p *docPolicy) DefineAnchor(string, *yaml.Node) error { return nil }
func (p *docPolicy) ResolveAlias(string) (*yaml.Node, bool, error) {
	return nil, false, errors.New("aliases forbidden")
}
func (p *docPolicy) EndDocument() error { return nil }
func TestDocumentBudget(t *testing.T) {
	p := &docPolicy{}
	for file, input := range []string{"", strings.Repeat("---\n", 64), strings.Repeat("---\n", 64), "---\n" + flow(1000)} {
		l, e := yaml.NewLoader(strings.NewReader(input), yaml.WithPlugin(p), yaml.WithPlugin(loaderlimits.New(loaderlimits.DepthValue(64))))
		if e != nil {
			t.Fatal(e)
		}
		for {
			var n yaml.Node
			e = l.Load(&n)
			if e == io.EOF {
				break
			}
			if file == 3 {
				if !errors.Is(e, errDocuments) {
					t.Fatalf("129th document: %v", e)
				}
				t.Logf("rejected129 before deeply nested body: count=%d node_kind=%v", p.count, n.Kind)
				return
			}
			if e != nil {
				t.Fatal(e)
			}
		}
	}
	t.Fatal("129th document not rejected")
}

func TestNoNodeBudget(t *testing.T) {
	calls := 0
	limits := loaderlimits.New(loaderlimits.DepthFunc(func(d int, _ *yaml.DepthContext) error {
		calls++
		if d > 64 {
			return errors.New("depth")
		}
		return nil
	}))
	n, e := load("["+strings.Repeat("0,", 300000)+"0]", yaml.WithPlugin(limits))
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("wide input tree nodes=%d depth_callbacks=%d; depth API cannot limit width", nodes(&n), calls)
	if nodes(&n) < 300000 {
		t.Fatal("missing width counterexample")
	}
}
