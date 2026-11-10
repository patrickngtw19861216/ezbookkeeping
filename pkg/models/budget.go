package models

// Budget represents a household budget entry for one user, one category path, one month
type Budget struct {
	BudgetId        int64  `xorm:"pk"`
	Uid             int64  `xorm:"UNIQUE(UQE_budget_uid_cat_ym) INDEX NOT NULL"`
	CategoryPath    string `xorm:"UNIQUE(UQE_budget_uid_cat_ym) VARCHAR(255) NOT NULL"`
	YearMonth       string `xorm:"UNIQUE(UQE_budget_uid_cat_ym) VARCHAR(7) NOT NULL"`
	Amount          int64  `xorm:"NOT NULL DEFAULT 0"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
}

// HouseholdBudgetMemberData holds budget + spent for one member in one category
type HouseholdBudgetMemberData struct {
	Uid      int64 `json:"uid,string"`
	Budgeted int64 `json:"budgeted"`
	Spent    int64 `json:"spent"`
	Balance  int64 `json:"balance"`
}

// HouseholdBudgetRow is one row in the household budget view
type HouseholdBudgetRow struct {
	CategoryPath string                       `json:"categoryPath"`
	Subcat       string                       `json:"subcat"`
	IsLeaf       bool                         `json:"isLeaf"`
	MemberData   []*HouseholdBudgetMemberData `json:"memberData"`
}

// HouseholdBudgetGroup is a group of rows sharing the same parent category name
type HouseholdBudgetGroup struct {
	Group string                `json:"group"`
	Rows  []*HouseholdBudgetRow `json:"rows"`
}

// HouseholdBudgetMember identifies a member in the response
type HouseholdBudgetMember struct {
	Uid      int64  `json:"uid,string"`
	Nickname string `json:"nickname"`
}

// HouseholdBudgetResponse is the top-level response for GET /api/v1/budget/household.json
type HouseholdBudgetResponse struct {
	Year    int32                    `json:"year"`
	Month   int32                    `json:"month"`
	Members []*HouseholdBudgetMember `json:"members"`
	Groups  []*HouseholdBudgetGroup  `json:"groups"`
}

// HouseholdBudgetUpdateRequest is the body for PUT /api/v1/budget/household/update.json
type HouseholdBudgetUpdateRequest struct {
	TargetUid    int64  `json:"targetUid,string" binding:"required,min=1"`
	CategoryPath string `json:"categoryPath" binding:"required,notBlank,max=255"`
	YearMonth    string `json:"yearMonth" binding:"required,len=7"`
	Amount       int64  `json:"amount" binding:"min=0"`
}

// HouseholdBudgetQueryRequest is the query params for GET /api/v1/budget/household.json
type HouseholdBudgetQueryRequest struct {
	Year  int32  `form:"year" binding:"required,min=2000,max=9999"`
	Month int32  `form:"month" binding:"required,min=1,max=12"`
	All   string `form:"all"` // "1" to include zero-data rows (for Set Budget tab)
}

// --- Snapshot types ---

// BudgetSnapshotEntry is one budget record inside a snapshot file.
type BudgetSnapshotEntry struct {
	Uid          int64  `json:"uid,string"`
	CategoryPath string `json:"categoryPath"`
	Amount       int64  `json:"amount"`
}

// BudgetSnapshot is the structure written to each snapshot JSON file.
type BudgetSnapshot struct {
	SavedAt   string                   `json:"savedAt"`   // "2026-11-09T15:04:05+08:00"
	YearMonth string                   `json:"yearMonth"` // "2026-11"
	Members   []*HouseholdBudgetMember `json:"members"`
	Budgets   []*BudgetSnapshotEntry   `json:"budgets"`
}

// BudgetSnapshotSummary is one entry in the list-snapshots response.
type BudgetSnapshotSummary struct {
	Key       string `json:"key"`       // "20261109-202611" (filename without .json)
	Date      string `json:"date"`      // "20261109"
	YearMonth string `json:"yearMonth"` // "2026-11"
}

// HouseholdBudgetBulkSaveRequest is the POST body for /budget/household/bulk-save.json
type HouseholdBudgetBulkSaveRequest struct {
	YearMonth string               `json:"yearMonth" binding:"required,len=7"`
	Entries   []*BudgetSnapshotEntry `json:"entries"`
}

// HouseholdBudgetSnapshotKeyRequest is the query param for GET /budget/household/snapshot.json
type HouseholdBudgetSnapshotKeyRequest struct {
	Key string `form:"key" binding:"required"`
}
