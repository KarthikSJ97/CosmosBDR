package workflows

import (
	"time"

	"cosmosbdr/src/cadence/activities"
	"cosmosbdr/src/types"

	"go.uber.org/cadence/workflow"
)

// loadCadenceRuntimeLimits reads fan-out limits from worker config via activity (not workflow input / API).
func loadCadenceRuntimeLimits(ctx workflow.Context, jobActivities *activities.JobActivities) (types.CadenceRuntimeLimits, error) {
	limAO := workflow.ActivityOptions{
		StartToCloseTimeout:    30 * time.Second,
		ScheduleToStartTimeout: time.Minute,
	}
	limCtx := workflow.WithActivityOptions(ctx, limAO)
	var lim types.CadenceRuntimeLimits
	err := workflow.ExecuteActivity(limCtx, jobActivities.GetCadenceRuntimeLimitsActivity).Get(limCtx, &lim)
	return lim, err
}