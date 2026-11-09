package api

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// BudgetsApi handles household budget endpoints
type BudgetsApi struct {
	ApiUsingConfig
	budgets *services.HouseholdBudgetService
}

var Budgets = &BudgetsApi{
	ApiUsingConfig: ApiUsingConfig{
		container: settings.Container,
	},
	budgets: services.HouseholdBudgets,
}

// HouseholdViewHandler returns the household budget view for a year/month.
// GET /api/v1/budget/household.json?year=2026&month=11
func (a *BudgetsApi) HouseholdViewHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.HouseholdBudgetQueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Warnf(c, "[budgets.HouseholdViewHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	memberUids := a.CurrentConfig().BudgetHouseholdMemberUids

	if !isHouseholdMember(uid, memberUids) {
		return nil, errs.ErrUserNotFound
	}

	resp, err := a.budgets.GetHouseholdBudgetView(c, req.Year, req.Month, memberUids)
	if err != nil {
		log.Errorf(c, "[budgets.HouseholdViewHandler] failed to get household budget view for year=%d month=%d, because %s", req.Year, req.Month, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return resp, nil
}

// UpdateBudgetHandler upserts a budget entry for a household member.
// PUT /api/v1/budget/household/update.json
func (a *BudgetsApi) UpdateBudgetHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.HouseholdBudgetUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[budgets.UpdateBudgetHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	memberUids := a.CurrentConfig().BudgetHouseholdMemberUids

	if !isHouseholdMember(uid, memberUids) {
		return nil, errs.ErrUserNotFound
	}
	if !isHouseholdMember(req.TargetUid, memberUids) {
		return nil, errs.ErrUserNotFound
	}

	err := a.budgets.UpsertBudget(c, req.TargetUid, req.CategoryPath, req.YearMonth, req.Amount)
	if err != nil {
		log.Errorf(c, "[budgets.UpdateBudgetHandler] failed to upsert budget for uid=%d, because %s", req.TargetUid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[budgets.UpdateBudgetHandler] uid:%d updated budget for targetUid:%d category:%s yearMonth:%s amount:%d",
		uid, req.TargetUid, req.CategoryPath, req.YearMonth, req.Amount)

	return true, nil
}

func isHouseholdMember(uid int64, memberUids []int64) bool {
	for _, m := range memberUids {
		if m == uid {
			return true
		}
	}
	return false
}
