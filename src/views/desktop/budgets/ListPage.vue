<template>
    <main-page-layout>
        <template #nav-items>
            <div class="px-3 py-2">
                <div class="text-caption text-medium-emphasis mb-1">{{ tt('Family Budget') }}</div>
                <div class="d-flex align-center gap-2">
                    <v-btn icon density="compact" variant="text" @click="navigate(-1)" :disabled="loading">
                        <v-icon :icon="mdiChevronLeft" />
                    </v-btn>
                    <span class="font-weight-medium text-body-2">{{ ymLabel }}</span>
                    <v-btn icon density="compact" variant="text" @click="navigate(1)" :disabled="loading">
                        <v-icon :icon="mdiChevronRight" />
                    </v-btn>
                </div>
            </div>
        </template>

        <template #content>
            <div class="budget-page-wrapper pa-3">
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

const loading = ref(false);
const error = ref('');
const data = ref<BudgetData | null>(null);
const editKey = ref('');
const editValue = ref(0);
const editInput = ref<HTMLInputElement[]>([]);

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
});

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

function editId(categoryPath: string, uid: string): string {
    return `${categoryPath}::${uid}`;
}

function startEdit(categoryPath: string, uid: string, currentCents: number) {
    editKey.value = editId(categoryPath, uid);
    editValue.value = Math.round(currentCents / AMOUNT_FACTOR);
    nextTick(() => {
        const inp = editInput.value?.[0];
        if (inp) {
            inp.focus();
            inp.select();
        }
    });
}

function cancelEdit() {
    editKey.value = '';
}

async function saveEdit(row: BudgetRow, md: MemberData) {
    const newDollars = editValue.value;
    if (isNaN(newDollars) || newDollars < 0) {
        cancelEdit();
        return;
    }
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

onMounted(() => loadData());
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
    min-width: 600px;
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
.editable {
    cursor: pointer;
}
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
</style>
