package agent

import "context"

type MultiPlanner []Planner

func (p MultiPlanner) Plan(ctx context.Context, request Request, specs []ToolSpec) (Plan, bool, error) {
	for _, planner := range p {
		if planner == nil {
			continue
		}
		plan, ok, err := planner.Plan(ctx, request, specs)
		if err != nil || ok {
			return plan, ok, err
		}
	}
	return Plan{}, false, nil
}
