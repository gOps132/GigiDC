package agent

import "context"

type MultiPlanner []Planner

func (p MultiPlanner) Plan(ctx context.Context, request Request, specs []ToolSpec) (Plan, bool, error) {
	var lastTrace Plan
	for _, planner := range p {
		if planner == nil {
			continue
		}
		plan, ok, err := planner.Plan(ctx, request, specs)
		if err != nil || ok {
			return plan, ok, err
		}
		if len(plan.Trace) > 0 {
			lastTrace = plan
		}
	}
	return lastTrace, false, nil
}
