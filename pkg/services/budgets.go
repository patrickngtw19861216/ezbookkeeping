package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// CATEGORY_ORDER is the canonical display order for household categories.
// Categories not in this list appear after, sorted alphabetically.
var budgetCategoryOrder = []string{
	"年度儲備金/家庭旅行", "年度儲備金/利是", "年度儲備金/工人續約",
	"番工開支/交通", "番工開支/手提電話", "番工開支/飲食",
	"普通儲蓄",
	"家庭開支/水費", "家庭開支/電費", "家庭開支/煤氣費",
	"家庭開支/買餸 - 日常", "家庭開支/買餸 - 宴客",
	"家庭開支/雜費", "家庭開支/出街食飯",
	"必要開支/家用", "必要開支/保險", "必要開支/十一奉獻",
	"必要開支/強積金", "必要開支/管理費", "必要開支/工人薪金", "必要開支/屋企上網",
	"訂閱服務/iCloud", "訂閱服務/Backblaze", "訂閱服務/Disney+", "訂閱服務/Netflix",
	"訂閱服務/Fantastical", "訂閱服務/Scan", "訂閱服務/AI - Claude", "訂閱服務/AI - ChatGPT",
	"分期付款/Other 1 - 2028-02", "分期付款/Other2 - 2029-02",
	"分期付款/Mac - 2028-08", "分期付款/Sony - 2029-03",
	"零用",
	"囝囝開支/包包 - 英文補習", "囝囝開支/包包 - 鋼琴", "囝囝開支/包包 - 樂團",
	"囝囝開支/包包 - 敲擊", "囝囝開支/包包 - 校巴 and 新通", "囝囝開支/蛋蛋",
}

// HouseholdBudgetService handles household budget views and upserts
type HouseholdBudgetService struct {
	ServiceUsingDB
	ServiceUsingUuid
}

var HouseholdBudgets = &HouseholdBudgetService{
	ServiceUsingDB: ServiceUsingDB{
		container: datastore.Container,
	},
	ServiceUsingUuid: ServiceUsingUuid{
		container: uuid.Container,
	},
}

// GetHouseholdBudgetView returns the combined household budget view for a given year/month.
// memberUids is the ordered list of household member UIDs.
// When showAll is true, all canonical categories are included even if budget and spent are zero.
func (s *HouseholdBudgetService) GetHouseholdBudgetView(c core.Context, year int32, month int32, memberUids []int64, showAll bool) (*models.HouseholdBudgetResponse, error) {
	minTxTime, maxTxTime, err := utils.GetTransactionTimeRangeByYearMonth(year, month)
	if err != nil {
		return nil, errs.ErrSystemError
	}

	ymStr := fmt.Sprintf("%d-%02d", year, month)

	// --- Per-member data ---
	type memberData struct {
		user     *models.User
		catPaths map[int64]string // categoryId → "parent/sub" path
		budgets  map[string]int64 // categoryPath → budgeted amount
		spent    map[string]int64 // categoryPath → spent amount
	}

	members := make([]*memberData, 0, len(memberUids))

	for _, uid := range memberUids {
		md := &memberData{
			catPaths: make(map[int64]string),
			budgets:  make(map[string]int64),
			spent:    make(map[string]int64),
		}

		// Get user info (for nickname)
		user, err := Users.GetUserById(c, uid)
		if err != nil {
			return nil, err
		}
		md.user = user

		// Build categoryId → path map
		allCats, err := TransactionCategories.GetAllCategoriesByUid(c, uid, models.CATEGORY_TYPE_EXPENSE, -1)
		if err != nil {
			return nil, err
		}
		parentNames := make(map[int64]string)
		for _, cat := range allCats {
			if cat.ParentCategoryId == 0 {
				parentNames[cat.CategoryId] = cat.Name
			}
		}
		for _, cat := range allCats {
			if cat.ParentCategoryId == 0 {
				md.catPaths[cat.CategoryId] = cat.Name
			} else {
				parentName := parentNames[cat.ParentCategoryId]
				md.catPaths[cat.CategoryId] = parentName + "/" + cat.Name
			}
		}

		// Load budget amounts from DB
		var budgetRows []*models.Budget
		err = s.UserDataDB(uid).NewSession(c).
			Where("uid=? AND year_month=?", uid, ymStr).
			Find(&budgetRows)
		if err != nil {
			return nil, err
		}
		for _, b := range budgetRows {
			md.budgets[b.CategoryPath] = b.Amount
		}

		// Sum spending from transactions in the month
		var txs []*models.Transaction
		err = s.UserDataDB(uid).NewSession(c).
			Where("uid=? AND deleted=? AND type=? AND transaction_time>=? AND transaction_time<=?",
				uid, false, models.TRANSACTION_DB_TYPE_EXPENSE, minTxTime, maxTxTime).
			Find(&txs)
		if err != nil {
			return nil, err
		}
		for _, tx := range txs {
			path, ok := md.catPaths[tx.CategoryId]
			if !ok {
				continue
			}
			md.spent[path] += tx.Amount
		}

		members = append(members, md)
	}

	// --- Collect all category paths across all members ---
	allPaths := make(map[string]struct{})
	for _, cat := range budgetCategoryOrder {
		allPaths[cat] = struct{}{}
	}
	for _, md := range members {
		for p := range md.budgets {
			allPaths[p] = struct{}{}
		}
		for p := range md.spent {
			allPaths[p] = struct{}{}
		}
	}

	// Sort by canonical order, then alphabetically
	sortedPaths := make([]string, 0, len(allPaths))
	for p := range allPaths {
		sortedPaths = append(sortedPaths, p)
	}
	orderIdx := make(map[string]int, len(budgetCategoryOrder))
	for i, p := range budgetCategoryOrder {
		orderIdx[p] = i
	}
	sort.Slice(sortedPaths, func(i, j int) bool {
		ia, oka := orderIdx[sortedPaths[i]]
		ib, okb := orderIdx[sortedPaths[j]]
		if oka && okb {
			return ia < ib
		}
		if oka {
			return true
		}
		if okb {
			return false
		}
		return sortedPaths[i] < sortedPaths[j]
	})

	// --- Build rows, optionally skipping paths where everyone has 0 budget and 0 spent ---
	rows := make([]*models.HouseholdBudgetRow, 0, len(sortedPaths))
	for _, path := range sortedPaths {
		anyNonZero := false
		memberDataSlice := make([]*models.HouseholdBudgetMemberData, 0, len(members))
		for i, md := range members {
			budgeted := md.budgets[path]
			spent := md.spent[path]
			if budgeted != 0 || spent != 0 {
				anyNonZero = true
			}
			memberDataSlice = append(memberDataSlice, &models.HouseholdBudgetMemberData{
				Uid:      memberUids[i],
				Budgeted: budgeted,
				Spent:    spent,
				Balance:  budgeted - spent,
			})
		}
		if !anyNonZero && !showAll {
			continue
		}

		isLeaf := !strings.Contains(path, "/")
		subcat := path
		if !isLeaf {
			parts := strings.SplitN(path, "/", 2)
			subcat = parts[1]
		}
		rows = append(rows, &models.HouseholdBudgetRow{
			CategoryPath: path,
			Subcat:       subcat,
			IsLeaf:       isLeaf,
			MemberData:   memberDataSlice,
		})
	}

	// --- Group rows by parent ---
	groups := groupBudgetRows(rows)

	// --- Build member list ---
	memberList := make([]*models.HouseholdBudgetMember, 0, len(members))
	for i, md := range members {
		memberList = append(memberList, &models.HouseholdBudgetMember{
			Uid:      memberUids[i],
			Nickname: md.user.Nickname,
		})
	}

	return &models.HouseholdBudgetResponse{
		Year:    year,
		Month:   month,
		Members: memberList,
		Groups:  groups,
	}, nil
}

// UpsertBudget creates or updates a budget entry for a user/category/month.
func (s *HouseholdBudgetService) UpsertBudget(c core.Context, uid int64, categoryPath string, yearMonth string, amount int64) error {
	now := time.Now().Unix()

	// Try to find existing
	existing := &models.Budget{}
	has, err := s.UserDataDB(uid).NewSession(c).
		Where("uid=? AND category_path=? AND year_month=?", uid, categoryPath, yearMonth).
		Get(existing)
	if err != nil {
		return err
	}

	if has {
		existing.Amount = amount
		existing.UpdatedUnixTime = now
		_, err = s.UserDataDB(uid).NewSession(c).ID(existing.BudgetId).
			Cols("amount", "updated_unix_time").
			Update(existing)
		return err
	}

	budget := &models.Budget{
		BudgetId:        s.GenerateUuid(uuid.UUID_TYPE_BUDGET),
		Uid:             uid,
		CategoryPath:    categoryPath,
		YearMonth:       yearMonth,
		Amount:          amount,
		CreatedUnixTime: now,
		UpdatedUnixTime: now,
	}
	_, err = s.UserDataDB(uid).NewSession(c).Insert(budget)
	return err
}

// SaveSnapshot writes the current DB budgets for yearMonth to a dated JSON file.
// Files are named {YYYYMMDD}-{YYYYMM}.json (e.g. 20261109-202611.json) in a
// budget_snapshots/ directory next to the SQLite database file.
func (s *HouseholdBudgetService) SaveSnapshot(c core.Context, dbPath string, yearMonth string, memberUids []int64) error {
	hkt := time.FixedZone("HKT", 8*3600)
	now := time.Now().In(hkt)
	saveDate := now.Format("20060102")
	ymKey := strings.ReplaceAll(yearMonth, "-", "") // "2026-11" → "202611"

	// Gather member info
	members := make([]*models.HouseholdBudgetMember, 0, len(memberUids))
	for _, uid := range memberUids {
		user, err := Users.GetUserById(c, uid)
		if err != nil {
			return err
		}
		members = append(members, &models.HouseholdBudgetMember{Uid: uid, Nickname: user.Nickname})
	}

	// Gather all budget rows from DB
	entries := make([]*models.BudgetSnapshotEntry, 0)
	for _, uid := range memberUids {
		var budgetRows []*models.Budget
		err := s.UserDataDB(uid).NewSession(c).
			Where("uid=? AND year_month=?", uid, yearMonth).
			Find(&budgetRows)
		if err != nil {
			return err
		}
		for _, b := range budgetRows {
			entries = append(entries, &models.BudgetSnapshotEntry{
				Uid:          uid,
				CategoryPath: b.CategoryPath,
				Amount:       b.Amount,
			})
		}
	}

	snapshot := &models.BudgetSnapshot{
		SavedAt:   now.Format(time.RFC3339),
		YearMonth: yearMonth,
		Members:   members,
		Budgets:   entries,
	}

	snapshotDir := filepath.Join(filepath.Dir(dbPath), "budget_snapshots")
	if err := os.MkdirAll(snapshotDir, 0750); err != nil {
		return err
	}
	filename := saveDate + "-" + ymKey + ".json"
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(snapshotDir, filename), data, 0640)
}

// ListSnapshots returns all available snapshot summaries sorted newest-first.
func (s *HouseholdBudgetService) ListSnapshots(dbPath string) ([]*models.BudgetSnapshotSummary, error) {
	snapshotDir := filepath.Join(filepath.Dir(dbPath), "budget_snapshots")
	dirEntries, err := os.ReadDir(snapshotDir)
	if os.IsNotExist(err) {
		return []*models.BudgetSnapshotSummary{}, nil
	}
	if err != nil {
		return nil, err
	}

	result := make([]*models.BudgetSnapshotSummary, 0, len(dirEntries))
	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".json") {
			continue
		}
		key := strings.TrimSuffix(de.Name(), ".json") // "20261109-202611"
		parts := strings.SplitN(key, "-", 2)
		if len(parts) != 2 || len(parts[0]) != 8 || len(parts[1]) != 6 {
			continue
		}
		saveDate := parts[0]
		ymKey := parts[1]
		ymDisplay := ymKey[:4] + "-" + ymKey[4:] // "202611" → "2026-11"
		result = append(result, &models.BudgetSnapshotSummary{
			Key:       key,
			Date:      saveDate,
			YearMonth: ymDisplay,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key > result[j].Key })
	return result, nil
}

// LoadSnapshot reads one snapshot file by key (filename without .json).
// Returns nil, nil if the file does not exist.
func (s *HouseholdBudgetService) LoadSnapshot(dbPath string, key string) (*models.BudgetSnapshot, error) {
	snapshotDir := filepath.Join(filepath.Dir(dbPath), "budget_snapshots")
	filePath := filepath.Join(snapshotDir, key+".json")
	raw, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var snap models.BudgetSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func groupBudgetRows(rows []*models.HouseholdBudgetRow) []*models.HouseholdBudgetGroup {
	groups := make([]*models.HouseholdBudgetGroup, 0)
	groupMap := make(map[string]*models.HouseholdBudgetGroup)

	for _, row := range rows {
		groupName := row.CategoryPath
		if strings.Contains(row.CategoryPath, "/") {
			groupName = strings.SplitN(row.CategoryPath, "/", 2)[0]
		}
		g, ok := groupMap[groupName]
		if !ok {
			g = &models.HouseholdBudgetGroup{
				Group: groupName,
				Rows:  make([]*models.HouseholdBudgetRow, 0),
			}
			groups = append(groups, g)
			groupMap[groupName] = g
		}
		g.Rows = append(g.Rows, row)
	}

	return groups
}
