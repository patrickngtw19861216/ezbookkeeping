<template>
    <main-page-layout>
        <template #nav-items>
            <div class="px-3 py-2">
                <div class="text-caption text-medium-emphasis mb-1">{{ tt('Family Budget') }}</div>
                <div class="d-flex align-center gap-2">
                    <v-btn icon density="compact" variant="text" @click="navigate(-1)" :disabled="loading || setLoading">
                        <v-icon :icon="mdiChevronLeft" />
                    </v-btn>
                    <span class="font-weight-medium text-body-2">{{ ymLabel }}</span>
                    <v-btn icon density="compact" variant="text" @click="navigate(1)" :disabled="loading || setLoading">
                        <v-icon :icon="mdiChevronRight" />
                    </v-btn>
                </div>
            </div>
        </template>

        <template #content>
            <div class="budget-page-wrapper pa-3">
                <v-tabs v-model="tab" density="compact" class="mb-3">
                    <v-tab value="view">{{ tt('View') }}</v-tab>
                    <v-tab value="set">{{ tt('Set Budget') }}</v-tab>
                </v-tabs>

                <!-- VIEW TAB -->
                <v-window v-model="tab">
                    <v-window-item value="view">
                        <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-3" />
                        <v-alert v-if="error" type="error" class="mb-3" closable @click:close="error = ''">{{ error }}</v-alert>

                        <div v-if="data" class="budget-table-wrap">
                            <table class="budget-table">
                                <thead>
                                    <tr class="group-header">
                                        <th class="cat-col" rowspan="2">{{ tt('Category') }}</th>
                                        <th class="member-hdr" colspan="3" v-for="m in data.members" :key="String(m.uid)">{{ m.nickname }}</th>
                                    </tr>
                                    <tr class="col-header">
                                        <template v-for="m in data.members" :key="String(m.uid)">
                                            <th class="num-col">{{ tt('Budget') }}</th>
                                            <th class="num-col">{{ tt('Spent') }}</th>
                                            <th class="num-col bal-col">{{ tt('Balance') }}</th>
                                        </template>
                                    </tr>
                                </thead>
                                <tbody>
                                    <template v-for="grp in data.groups" :key="grp.group">
                                        <tr v-if="!isSingleLeafGroup(grp)" class="group-row">
                                            <td :colspan="1 + data.members.length * 3">{{ grp.group }}</td>
                                        </tr>
                                        <tr v-for="row in grp.rows" :key="row.categoryPath" class="data-row">
                                            <td class="cat-col" :class="{ 'leaf-row': row.isLeaf }">
                                                <strong v-if="row.isLeaf">{{ row.subcat }}</strong>
                                                <span v-else>{{ row.subcat }}</span>
                                            </td>
                                            <template v-for="(md, idx) in row.memberData" :key="String(md.uid)">
                                                <td class="num-col editable" :class="{ 'first-member': idx === 0 }"
                                                    @click="startEdit(row.categoryPath, md.uid, md.budgeted)">
                                                    <template v-if="editKey === editId(row.categoryPath, md.uid)">
                                                        <input
                                                            class="budget-input"
                                                            type="number"
                                                            min="0"
                                                            step="1"
                                                            v-model.number="editValue"
                                                            @blur="saveEdit(row, md)"
                                                            @keydown.enter="($event.target as HTMLInputElement).blur()"
                                                            @keydown.escape="cancelEdit"
                                                            ref="editInput"
                                                        />
                                                    </template>
                                                    <template v-else>
                                                        {{ fmtAmt(md.budgeted) }}
                                                    </template>
                                                </td>
                                                <td class="num-col">{{ fmtAmt(md.spent) }}</td>
                                                <td class="num-col" :class="md.balance >= 0 ? 'bal-pos' : 'bal-neg'">{{ fmtAmt(md.balance) }}</td>
                                            </template>
                                        </tr>
                                    </template>

                                    <!-- Totals row -->
                                    <tr class="totals-row">
                                        <td>{{ tt('Total') }}</td>
                                        <template v-for="(col, idx) in totals" :key="idx">
                                            <td class="num-col" :class="{ 'first-member': idx === 0 }">{{ fmtAmt(col.budgeted) }}</td>
                                            <td class="num-col">{{ fmtAmt(col.spent) }}</td>
                                            <td class="num-col" :class="col.balance >= 0 ? 'bal-pos' : 'bal-neg'">{{ fmtAmt(col.balance) }}</td>
                                        </template>
                                    </tr>
                                </tbody>
                            </table>
                        </div>
                    </v-window-item>

                    <!-- SET BUDGET TAB -->
                    <v-window-item value="set">
                        <v-progress-linear v-if="setLoading" indeterminate color="primary" class="mb-3" />
                        <v-alert v-if="setError" type="error" class="mb-3" closable @click:close="setError = ''">{{ setError }}</v-alert>

                        <div v-if="setBudgetData">
                            <div class="d-flex justify-end mb-2">
                                <v-btn variant="flat" color="primary" size="small" :loading="savingSet" @click="saveSetBudget">
                                    {{ tt('Save') }}
                                </v-btn>
                            </div>
                            <div class="budget-table-wrap">
                                <table class="budget-table">
                                    <thead>
                                        <tr>
                                            <th class="cat-col">{{ tt('Category') }}</th>
                                            <th class="num-col" v-for="m in setBudgetData.members" :key="String(m.uid)">{{ m.nickname }}</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        <template v-for="grp in setBudgetData.groups" :key="grp.group">
                                            <tr v-if="!isSingleLeafGroup(grp)" class="group-row">
                                                <td :colspan="1 + setBudgetData.members.length">{{ grp.group }}</td>
                                            </tr>
                                            <tr v-for="row in grp.rows" :key="row.categoryPath" class="data-row">
                                                <td class="cat-col">{{ row.subcat }}</td>
                                                <td class="num-col set-input-cell" v-for="md in row.memberData" :key="String(md.uid)">
                                                    <input
                                                        type="number"
                                                        min="0"
                                                        step="1"
                                                        class="set-input"
                                                        :value="getSetAmount(row.categoryPath, String(md.uid))"
                                                        @change="onSetAmountChange(row.categoryPath, String(md.uid), $event)"
                                                    />
                                                </td>
                                            </tr>
                                        </template>
                                    </tbody>
                                </table>
                            </div>
                        </div>

                        <!-- History section -->
                        <div class="history-section mt-5">
                            <div class="text-caption text-medium-emphasis mb-2">{{ tt('Past Saves') }}</div>
                            <div v-if="snapshots.length === 0" class="text-caption" style="opacity:0.4">{{ tt('No saves yet') }}</div>
                            <div class="d-flex flex-wrap gap-2">
                                <v-chip
                                    v-for="s in snapshots"
                                    :key="s.key"
                                    size="small"
                                    variant="outlined"
                                    @click="viewSnapshot(s.key)"
                                    style="cursor:pointer"
                                >{{ s.date }} ({{ s.yearMonth }})</v-chip>
                            </div>
                        </div>
                    </v-window-item>
                </v-window>

                <!-- Snapshot viewer dialog -->
                <v-dialog v-model="showSnapshotDialog" max-width="640" scrollable>
                    <v-card v-if="currentSnapshot">
                        <v-card-title class="text-body-1">{{ currentSnapshot.savedAt.substring(0, 10) }} — {{ currentSnapshot.yearMonth }}</v-card-title>
                        <v-card-text>
                            <table class="budget-table" style="min-width:300px">
                                <thead>
                                    <tr>
                                        <th class="cat-col">{{ tt('Category') }}</th>
                                        <th class="num-col" v-for="m in currentSnapshot.members" :key="String(m.uid)">{{ m.nickname }}</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    <tr v-for="row in snapshotRows" :key="row.categoryPath" class="data-row">
                                        <td class="cat-col">{{ row.categoryPath }}</td>
                                        <td class="num-col" v-for="uid in snapshotMemberUids" :key="uid">{{ fmtAmt(row.amounts[uid] ?? 0) }}</td>
                                    </tr>
                                </tbody>
                            </table>
                        </v-card-text>
                        <v-card-actions>
                            <v-spacer />
                            <v-btn variant="text" size="small" @click="showSnapshotDialog = false">{{ tt('Close') }}</v-btn>
                        </v-card-actions>
                    </v-card>
                </v-dialog>
            </div>
        </template>
    </main-page-layout>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { mdiChevronLeft, mdiChevronRight } from '@mdi/js';
import { useI18n } from '@/locales/helpers.ts';
import type { ErrorResponse } from '@/core/api.ts';
import services from '@/lib/services.ts';

const { tt } = useI18n();
const route = useRoute();
const router = useRouter();

const AMOUNT_FACTOR = 100;

interface MemberData { uid: string; budgeted: number; spent: number; balance: number; }
interface BudgetRow { categoryPath: string; subcat: string; isLeaf: boolean; memberData: MemberData[]; }
interface BudgetGroup { group: string; rows: BudgetRow[]; }
interface Member { uid: string; nickname: string; }
interface BudgetData { year: number; month: number; members: Member[]; groups: BudgetGroup[]; }

interface SnapshotSummary { key: string; date: string; yearMonth: string; }
interface SnapshotEntry { uid: string; categoryPath: string; amount: number; }
interface SnapshotMember { uid: string; nickname: string; }
interface Snapshot { savedAt: string; yearMonth: string; members: SnapshotMember[]; budgets: SnapshotEntry[]; }

// --- Shared state ---
const tab = ref<'view' | 'set'>('view');

function getYM(): { year: number; month: number } {
    const hash = route.hash.replace('#', '');
    const m = hash.match(/^(\d{4})-(\d{2})$/);
    if (m) return { year: +(m[1] ?? '0'), month: +(m[2] ?? '0') };
    const now = new Date();
    return { year: now.getFullYear(), month: now.getMonth() + 1 };
}

const { year, month } = getYM();
const curYear = ref(year);
const curMonth = ref(month);

const ymLabel = computed(() => `${curYear.value}-${String(curMonth.value).padStart(2, '0')}`);

function navigate(delta: number) {
    let y = curYear.value;
    let m = curMonth.value + delta;
    if (m > 12) { m = 1; y++; }
    if (m < 1) { m = 12; y--; }
    router.replace({ hash: `#${y}-${String(m).padStart(2, '0')}` });
}

watch(() => route.hash, () => {
    const { year: y, month: m } = getYM();
    curYear.value = y;
    curMonth.value = m;
    loadData();
    if (tab.value === 'set') loadSetBudgetData();
});

watch(tab, (newTab) => {
    if (newTab === 'set' && !setBudgetData.value) {
        loadSetBudgetData();
        loadSnapshots();
    }
});

// --- VIEW TAB ---
const loading = ref(false);
const error = ref('');
const data = ref<BudgetData | null>(null);
const editKey = ref('');
const editValue = ref(0);
const editInput = ref<HTMLInputElement[]>([]);

async function loadData() {
    loading.value = true;
    error.value = '';
    try {
        const resp = await services.getHouseholdBudget(curYear.value, curMonth.value);
        if (resp.data.success) {
            data.value = resp.data.result as BudgetData;
        } else {
            error.value = String((resp.data as unknown as ErrorResponse).errorMessage || 'Error');
        }
    } catch (e: unknown) {
        error.value = (e instanceof Error ? e.message : null) || 'Network error';
    } finally {
        loading.value = false;
    }
}

const totals = computed(() => {
    if (!data.value) return [];
    const cols = data.value.members.map(() => ({ budgeted: 0, spent: 0, balance: 0 }));
    for (const grp of data.value.groups) {
        for (const row of grp.rows) {
            row.memberData.forEach((md, i) => {
                const col = cols[i];
                if (col) {
                    col.budgeted += md.budgeted;
                    col.spent += md.spent;
                    col.balance += md.balance;
                }
            });
        }
    }
    return cols;
});

function isSingleLeafGroup(grp: BudgetGroup) {
    return grp.rows.length === 1 && (grp.rows[0]?.isLeaf ?? false);
}

function fmtAmt(cents: number): string {
    return '$' + Math.abs(Math.round(cents / AMOUNT_FACTOR)).toLocaleString('en-HK');
}

function editId(categoryPath: string, uid: string | number): string {
    return `${categoryPath}::${uid}`;
}

function startEdit(categoryPath: string, uid: string | number, currentCents: number) {
    editKey.value = editId(categoryPath, uid);
    editValue.value = Math.round(currentCents / AMOUNT_FACTOR);
    nextTick(() => {
        const inp = editInput.value?.[0];
        if (inp) { inp.focus(); inp.select(); }
    });
}

function cancelEdit() { editKey.value = ''; }

async function saveEdit(row: BudgetRow, md: MemberData) {
    const newDollars = editValue.value;
    if (isNaN(newDollars) || newDollars < 0) { cancelEdit(); return; }
    const newCents = Math.round(newDollars * AMOUNT_FACTOR);
    editKey.value = '';
    try {
        const resp = await services.updateHouseholdBudget({
            targetUid: md.uid,
            categoryPath: row.categoryPath,
            yearMonth: ymLabel.value,
            amount: newCents
        });
        if (resp.data.success) {
            md.budgeted = newCents;
            md.balance = newCents - md.spent;
        } else {
            error.value = String((resp.data as unknown as ErrorResponse).errorMessage || 'Save failed');
        }
    } catch (e: unknown) {
        error.value = (e instanceof Error ? e.message : null) || 'Network error';
    }
}

// --- SET BUDGET TAB ---
const setLoading = ref(false);
const setError = ref('');
const setBudgetData = ref<BudgetData | null>(null);
const savingSet = ref(false);

// categoryPath → uid → amount in HKD dollars
const setBudgetAmounts = ref<Record<string, Record<string, number>>>({});

function getSetAmount(catPath: string, uid: string): number {
    return setBudgetAmounts.value[catPath]?.[uid] ?? 0;
}

function onSetAmountChange(catPath: string, uid: string, event: Event) {
    const val = (event.target as HTMLInputElement).valueAsNumber;
    if (!setBudgetAmounts.value[catPath]) setBudgetAmounts.value[catPath] = {};
    (setBudgetAmounts.value[catPath] as Record<string, number>)[uid] = isNaN(val) ? 0 : Math.max(0, Math.round(val));
}

async function loadSetBudgetData() {
    setLoading.value = true;
    setError.value = '';
    try {
        const resp = await services.getHouseholdBudget(curYear.value, curMonth.value, true);
        if (resp.data.success) {
            const d = resp.data.result as BudgetData;
            setBudgetData.value = d;
            // Initialise amount map from loaded data
            const amounts: Record<string, Record<string, number>> = {};
            for (const grp of d.groups) {
                for (const row of grp.rows) {
                    if (!amounts[row.categoryPath]) amounts[row.categoryPath] = {};
                    for (const md of row.memberData) {
                        (amounts[row.categoryPath] as Record<string, number>)[md.uid] = Math.round(md.budgeted / AMOUNT_FACTOR);
                    }
                }
            }
            setBudgetAmounts.value = amounts;
        } else {
            setError.value = String((resp.data as unknown as ErrorResponse).errorMessage || 'Error');
        }
    } catch (e: unknown) {
        setError.value = (e instanceof Error ? e.message : null) || 'Network error';
    } finally {
        setLoading.value = false;
    }
}

async function saveSetBudget() {
    savingSet.value = true;
    setError.value = '';
    try {
        const entries: { uid: string; categoryPath: string; amount: number }[] = [];
        for (const [catPath, memberAmounts] of Object.entries(setBudgetAmounts.value)) {
            for (const [uid, amountHKD] of Object.entries(memberAmounts)) {
                entries.push({
                    uid,
                    categoryPath: catPath,
                    amount: Math.round((amountHKD as number) * AMOUNT_FACTOR)
                });
            }
        }
        const resp = await services.bulkSaveHouseholdBudget({ yearMonth: ymLabel.value, entries });
        if (resp.data.success) {
            await loadSnapshots();
            // Refresh view data too if already loaded
            if (data.value) await loadData();
        } else {
            setError.value = String((resp.data as unknown as ErrorResponse).errorMessage || 'Save failed');
        }
    } catch (e: unknown) {
        setError.value = (e instanceof Error ? e.message : null) || 'Network error';
    } finally {
        savingSet.value = false;
    }
}

// --- SNAPSHOTS ---
const snapshots = ref<SnapshotSummary[]>([]);
const showSnapshotDialog = ref(false);
const currentSnapshot = ref<Snapshot | null>(null);

const snapshotMemberUids = computed(() => currentSnapshot.value?.members.map(m => String(m.uid)) ?? []);

const snapshotRows = computed(() => {
    const snap = currentSnapshot.value;
    if (!snap) return [];
    const rowMap: Record<string, Record<string, number>> = {};
    for (const e of snap.budgets) {
        const uid = String(e.uid);
        if (!rowMap[e.categoryPath]) rowMap[e.categoryPath] = {};
        (rowMap[e.categoryPath] as Record<string, number>)[uid] = e.amount;
    }
    return Object.entries(rowMap)
        .map(([categoryPath, amounts]) => ({ categoryPath, amounts }))
        .sort((a, b) => a.categoryPath.localeCompare(b.categoryPath));
});

async function loadSnapshots() {
    try {
        const resp = await services.listBudgetSnapshots();
        if (resp.data.success) {
            snapshots.value = (resp.data.result as SnapshotSummary[]) ?? [];
        }
    } catch {
        // non-critical, ignore
    }
}

async function viewSnapshot(key: string) {
    try {
        const resp = await services.getBudgetSnapshot(key);
        if (resp.data.success && resp.data.result) {
            currentSnapshot.value = resp.data.result as Snapshot;
            showSnapshotDialog.value = true;
        }
    } catch {
        // ignore
    }
}

onMounted(() => {
    loadData();
});
</script>

<style scoped>
.budget-page-wrapper {
    max-width: 100%;
    overflow-x: auto;
}
.budget-table-wrap {
    overflow-x: auto;
}
.budget-table {
    width: 100%;
    border-collapse: collapse;
    min-width: 500px;
    font-size: 13px;
}
.budget-table thead th {
    padding: 6px 8px;
    font-size: 11px;
    font-weight: 700;
    border-bottom: 2px solid rgba(255,255,255,0.12);
    white-space: nowrap;
}
.member-hdr {
    text-align: center;
    border-left: 1px solid rgba(255,255,255,0.12);
}
.cat-col {
    text-align: left;
    min-width: 120px;
    padding: 5px 8px;
}
.num-col {
    text-align: right;
    min-width: 70px;
    padding: 5px 8px;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
}
.first-member {
    border-left: 1px solid rgba(255,255,255,0.12);
}
.bal-col {
    border-right: 1px solid rgba(255,255,255,0.08);
}
.group-row td {
    padding: 6px 8px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: rgba(255,255,255,0.5);
    border-top: 1px solid rgba(255,255,255,0.08);
}
.data-row td {
    border-bottom: 1px solid rgba(255,255,255,0.05);
}
.data-row:hover td {
    background: rgba(255,255,255,0.03);
}
.leaf-row {
    padding-left: 8px !important;
}
.totals-row td {
    padding: 7px 8px;
    border-top: 2px solid rgba(255,255,255,0.2);
    font-weight: 700;
    font-size: 12px;
}
.bal-pos { color: #4caf50; }
.bal-neg { color: #f44336; }
.editable { cursor: pointer; }
.editable:hover {
    outline: 1px solid rgba(255,255,255,0.3);
    background: rgba(255,255,255,0.04) !important;
}
.budget-input {
    width: 100%;
    background: rgba(0,0,0,0.3);
    border: 1px solid #4caf50;
    color: inherit;
    text-align: right;
    padding: 2px 4px;
    font-size: 13px;
    font-family: inherit;
    border-radius: 2px;
    outline: none;
}
.set-input-cell { padding: 3px 6px; }
.set-input {
    width: 90px;
    background: rgba(255,255,255,0.05);
    border: 1px solid rgba(255,255,255,0.15);
    color: inherit;
    text-align: right;
    padding: 3px 5px;
    font-size: 13px;
    font-family: inherit;
    border-radius: 2px;
    outline: none;
}
.set-input:focus {
    border-color: #4caf50;
    background: rgba(0,0,0,0.3);
}
.history-section {
    border-top: 1px solid rgba(255,255,255,0.08);
    padding-top: 12px;
}
</style>
