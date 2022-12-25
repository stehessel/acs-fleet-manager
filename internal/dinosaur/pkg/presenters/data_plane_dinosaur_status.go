package presenters

import (
	"github.com/stackrox/acs-fleet-manager/pkg/api/dbapi"
	"github.com/stackrox/acs-fleet-manager/pkg/api/private"
)

// ConvertDataPlaneDinosaurStatus ...
func ConvertDataPlaneDinosaurStatus(status map[string]private.DataPlaneCentralStatus) []*dbapi.DataPlaneCentralStatus {
	res := make([]*dbapi.DataPlaneCentralStatus, 0, len(status))

	for k, v := range status {
		c := make([]dbapi.DataPlaneCentralStatusCondition, 0, len(v.Conditions))
		var routes []dbapi.DataPlaneCentralRoute
		for _, s := range v.Conditions {
			c = append(c, dbapi.DataPlaneCentralStatusCondition{
				Type:    s.Type,
				Reason:  s.Reason,
				Status:  s.Status,
				Message: s.Message,
			})
		}
		if v.Routes != nil {
			routes = make([]dbapi.DataPlaneCentralRoute, 0, len(v.Routes))
			for _, ro := range v.Routes {
				routes = append(routes, dbapi.DataPlaneCentralRoute{
					Domain: ro.Domain,
					Router: ro.Router,
				})
			}
		}
		res = append(res, &dbapi.DataPlaneCentralStatus{
			CentralClusterID:       k,
			Conditions:             c,
			Routes:                 routes,
			CentralVersion:         v.Versions.Central,
			CentralOperatorVersion: v.Versions.CentralOperator,
		})
	}

	return res
}
