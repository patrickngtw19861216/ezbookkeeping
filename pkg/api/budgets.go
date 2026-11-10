package api

import (
	"fmt"

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
// GET /api/v1/budget/household.json?year=2026&month=11[&all=1]
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

	resp, err := a.budgets.GetHouseholdBudgetView(c, req.Year, req.Month, memberUids, req.All == "1")
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

// BulkSaveHandler saves all budgets at once and writes a dated snapshot file.
// POST /api/v1/budget/household/bulk-save.json
func (a *BudgetsApi) BulkSaveHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.HouseholdBudgetBulkSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[budgets.BulkSaveHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	memberUids := a.CurrentConfig().BudgetHouseholdMemberUids

	if !isHouseholdMember(uid, memberUids) {
		return nil, errs.ErrUserNotFound
	}

	for _, e := range req.Entries {
		if !isHouseholdMember(e.Uid, memberUids) {
			continue
		}
		if e.CategoryPath == "" || len(e.CategoryPath) > 255 || e.Amount < 0 {
			continue
		}
		if err := a.budgets.UpsertBudget(c, e.Uid, e.CategoryPath, req.YearMonth, e.Amount); err != nil {
			log.Errorf(c, "[budgets.BulkSaveHandler] upsert failed uid=%d cat=%s, because %s", e.Uid, e.CategoryPath, err.Error())
			return nil, errs.Or(err, errs.ErrOperationFailed)
		}
	}

	dbPath := a.CurrentConfig().DatabaseConfig.DatabasePath
	if err := a.budgets.SaveSnapshot(c, dbPath, req.YearMonth, memberUids); err != nil {
		log.Warnf(c, "[budgets.BulkSaveHandler] snapshot write failed, because %s", err.Error())
		// non-fatal
	}

	log.Infof(c, "[budgets.BulkSaveHandler] uid:%d saved %d entries for %s", uid, len(req.Entries), req.YearMonth)
	return true, nil
}

// ListSnapshotsHandler returns a list of available snapshot files.
// GET /api/v1/budget/household/snapshots.json
func (a *BudgetsApi) ListSnapshotsHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	if !isHouseholdMember(uid, a.CurrentConfig().BudgetHouseholdMemberUids) {
		return nil, errs.ErrUserNotFound
	}

	dbPath := a.CurrentConfig().DatabaseConfig.DatabasePath
	summaries, err := a.budgets.ListSnapshots(dbPath)
	if err != nil {
		log.Errorf(c, "[budgets.ListSnapshotsHandler] failed, because %s", err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}
	return summaries, nil
}

// GetSnapshotHandler reads one snapshot file by key.
// GET /api/v1/budget/household/snapshot.json?key=20261109-202611
func (a *BudgetsApi) GetSnapshotHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.HouseholdBudgetSnapshotKeyRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Warnf(c, "[budgets.GetSnapshotHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	// Validate key format: exactly {8digits}-{6digits} to prevent path traversal
	if !isValidSnapshotKey(req.Key) {
		log.Warnf(c, "[budgets.GetSnapshotHandler] invalid key format: %s", req.Key)
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(fmt.Errorf("invalid key"))
	}

	uid := c.GetCurrentUid()
	if !isHouseholdMember(uid, a.CurrentConfig().BudgetHouseholdMemberUids) {
		return nil, errs.ErrUserNotFound
	}

	dbPath := a.CurrentConfig().DatabaseConfig.DatabasePath
	snapshot, err := a.budgets.LoadSnapshot(dbPath, req.Key)
	if err != nil {
		log.Errorf(c, "[budgets.GetSnapshotHandler] failed for key=%s, because %s", req.Key, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}
	return snapshot, nil
}

func isHouseholdMember(uid int64, memberUids []int64) bool {
	for _, m := range memberUids {
		if m == uid {
			return true
		}
	}
	return false
}

// isValidSnapshotKey validates key is "{8 digits}-{6 digits}" to prevent path traversal.
func isValidSnapshotKey(key string) bool {
	if len(key) != 15 { // 8 + 1 + 6
		return false
	}
	if key[8] != '-' {
		return false
	}
	for i, ch := range key {
		if i == 8 {
			continue
		}
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
