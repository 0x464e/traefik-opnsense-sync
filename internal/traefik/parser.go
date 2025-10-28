package traefik

// Base treeBuilder and parser -related code copied and adapted from the Traefik source code

import (
	"strings"

	"github.com/vulcand/predicate"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	and = "and"
	or  = "or"
)

var httpFuncs = []string{
	"ClientIP",
	"Method",
	"Host",
	"HostRegexp",
	"Path",
	"PathRegexp",
	"PathPrefix",
	"Header",
	"HeaderRegexp",
	"Query",
	"QueryRegexp",
	"Headers",
	"HeadersRegexp",
}

type treeBuilder func() *tree

type tree struct {
	Matcher   string
	Not       bool
	Value     []string
	RuleLeft  *tree
	RuleRight *tree
}

func newParser(matchers []string) (predicate.Parser, error) {
	parserFuncs := make(map[string]interface{})

	for _, matcherName := range matchers {
		fn := func(value ...string) treeBuilder {
			return func() *tree {
				return &tree{
					Matcher: matcherName,
					Value:   value,
				}
			}
		}
		parserFuncs[matcherName] = fn
		parserFuncs[strings.ToLower(matcherName)] = fn
		parserFuncs[strings.ToUpper(matcherName)] = fn
		parserFuncs[cases.Title(language.Und).String(strings.ToLower(matcherName))] = fn
	}

	return predicate.NewParser(predicate.Def{
		Operators: predicate.Operators{
			AND: andFunc,
			OR:  orFunc,
			NOT: notFunc,
		},
		Functions: parserFuncs,
	})
}

func andFunc(left, right treeBuilder) treeBuilder {
	return func() *tree {
		return &tree{
			Matcher:   and,
			RuleLeft:  left(),
			RuleRight: right(),
		}
	}
}

func orFunc(left, right treeBuilder) treeBuilder {
	return func() *tree {
		return &tree{
			Matcher:   or,
			RuleLeft:  left(),
			RuleRight: right(),
		}
	}
}

func invert(t *tree) *tree {
	switch t.Matcher {
	case or:
		t.Matcher = and
		t.RuleLeft = invert(t.RuleLeft)
		t.RuleRight = invert(t.RuleRight)
	case and:
		t.Matcher = or
		t.RuleLeft = invert(t.RuleLeft)
		t.RuleRight = invert(t.RuleRight)
	default:
		t.Not = !t.Not
	}

	return t
}

func notFunc(elem treeBuilder) treeBuilder {
	return func() *tree {
		return invert(elem())
	}
}

func (tree *tree) parseMatchers(matchers []string) []string {
	switch tree.Matcher {
	case and, or:
		return append(tree.RuleLeft.parseMatchers(matchers), tree.RuleRight.parseMatchers(matchers)...)
	default:
		for _, matcher := range matchers {
			if tree.Matcher == matcher {
				return lower(tree.Value)
			}
		}

		return nil
	}
}

func lower(slice []string) []string {
	var lowerStrings []string
	for _, value := range slice {
		lowerStrings = append(lowerStrings, strings.ToLower(value))
	}

	return lowerStrings
}

func (tree *tree) collectHostMatches(matchers []string) (out []DomainMatch, neg []string) {
	switch tree.Matcher {
	case and, or:
		lOut, lNeg := tree.RuleLeft.collectHostMatches(matchers)
		rOut, rNeg := tree.RuleRight.collectHostMatches(matchers)
		return append(lOut, rOut...), append(lNeg, rNeg...)
	default:
		for _, m := range matchers {
			if tree.Matcher == m {
				kind := DomainLiteral
				if m == "HostRegexp" {
					kind = DomainRegex
				}
				vals := lower(tree.Value)
				if tree.Not {
					neg = append(neg, vals...)
				} else {
					for _, v := range vals {
						out = append(out, DomainMatch{Value: v, Kind: kind})
					}
				}
				break
			}
		}
		return
	}
}

// returns a deduped order-preserving slice.
func unique(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

func ParseDomains(rule string) ([]DomainMatch, error) {
	parser, _ := newParser(httpFuncs)
	parse, _ := parser.Parse(rule)
	buildTree, _ := parse.(treeBuilder)

	matches, neg := buildTree().collectHostMatches([]string{"Host", "HostRegexp"})

	// build a set of negatives for quick filtering
	negSet := make(map[string]struct{}, len(neg))
	for _, n := range unique(neg) {
		negSet[n] = struct{}{}
	}

	// de-dupe positives by value while preserving first-seen Kind
	seen := make(map[string]DomainKind, len(matches))
	order := make([]string, 0, len(matches))

	for _, m := range matches {
		if _, blocked := negSet[m.Value]; blocked {
			continue
		}
		if prevKind, ok := seen[m.Value]; ok {
			if prevKind == DomainLiteral && m.Kind == DomainRegex {
				seen[m.Value] = DomainRegex
			}
			continue
		}
		seen[m.Value] = m.Kind
		order = append(order, m.Value)
	}

	out := make([]DomainMatch, 0, len(order))
	for _, v := range order {
		out = append(out, DomainMatch{Value: v, Kind: seen[v]})
	}
	return out, nil
}
