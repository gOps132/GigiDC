package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type ToolKind string

const (
	ToolKindRead  ToolKind = "read"
	ToolKindWrite ToolKind = "write"
)

type ToolSpec struct {
	Name        string
	Description string
	Kind        ToolKind
	Capability  string
}

type ToolCall struct {
	Name string
	Args map[string]string
}

type ToolResult struct {
	Name    string
	Summary string
	Data    map[string]string
}

type Tool interface {
	Spec() ToolSpec
	Execute(context.Context, Request, ToolCall) (ToolResult, error)
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry(tools ...Tool) Registry {
	registry := Registry{tools: map[string]Tool{}}
	for _, tool := range tools {
		registry.Register(tool)
	}
	return registry
}

func (r *Registry) Register(tool Tool) {
	if tool == nil {
		return
	}
	spec := NormalizeToolSpec(tool.Spec())
	if spec.Name == "" {
		return
	}
	if r.tools == nil {
		r.tools = map[string]Tool{}
	}
	r.tools[spec.Name] = tool
}

func (r Registry) Specs() []ToolSpec {
	specs := make([]ToolSpec, 0, len(r.tools))
	for _, tool := range r.tools {
		specs = append(specs, NormalizeToolSpec(tool.Spec()))
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Name < specs[j].Name
	})
	return specs
}

func (r Registry) Lookup(name string) (Tool, ToolSpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ToolSpec{}, fmt.Errorf("tool name is required")
	}
	tool, ok := r.tools[name]
	if !ok {
		return nil, ToolSpec{}, fmt.Errorf("unknown tool %q", name)
	}
	return tool, NormalizeToolSpec(tool.Spec()), nil
}

func (r Registry) Execute(ctx context.Context, request Request, call ToolCall) (ToolResult, error) {
	tool, spec, err := r.Lookup(call.Name)
	if err != nil {
		return ToolResult{}, err
	}
	call.Name = spec.Name
	return tool.Execute(ctx, request, call)
}

func NormalizeToolSpec(spec ToolSpec) ToolSpec {
	spec.Name = strings.TrimSpace(spec.Name)
	spec.Description = strings.TrimSpace(spec.Description)
	spec.Kind = ToolKind(strings.TrimSpace(string(spec.Kind)))
	spec.Capability = strings.TrimSpace(spec.Capability)
	switch spec.Kind {
	case "":
		spec.Kind = ToolKindRead
	case ToolKindRead, ToolKindWrite:
	default:
		spec.Kind = ToolKindRead
	}
	return spec
}

type Plan struct {
	Intent               string
	ContextScopes        []string
	ToolCalls            []ToolCall
	ClarifyingQuestion   string
	RequiresConfirmation bool
}

type Planner interface {
	Plan(context.Context, Request, []ToolSpec) (Plan, bool, error)
}
