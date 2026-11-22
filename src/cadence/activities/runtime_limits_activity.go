package activities

import (
	"context"

	"cosmosbdr/src/config"
	"cosmosbdr/src/types"
)

func (ja *JobActivities) GetCadenceRuntimeLimitsActivity(ctx context.Context) (types.CadenceRuntimeLimits, error) {
	_ = ctx
	w := ja.CadenceMaxConcurrentWorkflows
	cap := ja.CadenceWorkerMaxConcurrentActivities
	if w <= 0 {
		w = config.DefaultCadenceMaxConcurrentWorkflows
	}
	if cap <= 0 {
		cap = config.DefaultCadenceWorkerMaxConcurrentActivities
	}
	return types.CadenceRuntimeLimits{
		MaxConcurrentWorkflows:          w,
		WorkerMaxConcurrentActivities: cap,
	}, nil
}