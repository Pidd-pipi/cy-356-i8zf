<template>
  <div class="page-card">
    <div style="display: flex; justify-content: space-between; align-items: center">
      <h3 class="page-title">菜园地块认养（GIS 分布）</h3>
      <div>
        <el-button v-if="isAdmin" type="primary" @click="openCreate">+ 新增地块</el-button>
      </div>
    </div>

    <el-card shadow="never" style="margin-bottom: 16px">
      <template #header>地块分布图（按经纬度示意）</template>
      <svg :viewBox="viewBoxStr" class="plot-map" xmlns="http://www.w3.org/2000/svg">
        <rect x="0" y="0" :width="mapW" :height="mapH" fill="#e8f5e9" stroke="#a5d6a7" />
        <line v-for="i in 4" :key="'h' + i" :x1="0" :y1="(mapH / 5) * i" :x2="mapW" :y2="(mapH / 5) * i" stroke="#c8e6c9" stroke-dasharray="4 4" />
        <line v-for="i in 4" :key="'v' + i" :x1="(mapW / 5) * i" :y1="0" :x2="(mapW / 5) * i" :y2="mapH" stroke="#c8e6c9" stroke-dasharray="4 4" />
        <g v-for="p in store.plots" :key="p.id">
          <circle :cx="mapX(p)" :cy="mapY(p)" r="10" :fill="colorOf(p.status)" stroke="#fff" stroke-width="2" />
          <text :x="mapX(p)" :y="mapY(p) - 14" text-anchor="middle" font-size="10" fill="#333">{{ p.code }}</text>
          <title>{{ p.name }}｜{{ PlotStatusMeta[p.status]?.label }}</title>
        </g>
      </svg>
    </el-card>

    <DataTable :data="store.plots" :loading="store.loading" :total="store.total" :page-size="pagination.size.value" :current-page="pagination.page.value" @update:current-page="onPage">
      <el-table-column prop="code" label="编号" width="90" />
      <el-table-column prop="name" label="地块名称" min-width="140" />
      <el-table-column label="面积" width="90">
        <template #default="{ row }">{{ formatArea(row.area) }}</template>
      </el-table-column>
      <el-table-column label="土壤" width="90">
        <template #default="{ row }">{{ SoilTypeText[row.soil_type] }}</template>
      </el-table-column>
      <el-table-column label="日照" width="90">
        <template #default="{ row }">{{ SunlightText[row.sunlight] }}</template>
      </el-table-column>
      <el-table-column label="状态" width="110">
        <template #default="{ row }"><StatusBadge :value="row.status" :meta-map="PlotStatusMeta" /></template>
      </el-table-column>
      <el-table-column label="认养人" width="120">
        <template #default="{ row }">{{ row.adopter?.nickname || row.adopter?.username || '-' }}</template>
      </el-table-column>
      <el-table-column label="共管" width="200">
        <template #default="{ row }">
          <template v-if="row.latest_custodian">
            <StatusBadge :value="row.latest_custodian.status" :meta-map="CustodianStatusMeta" />
            <div class="custodian-line">
              <span>{{ displayName(row.latest_custodian.custodian) }}</span>
              <el-button link type="primary" size="small" @click="openHistory(row)">记录</el-button>
            </div>
          </template>
          <span v-else class="muted">-</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="360">
        <template #default="{ row }">
          <el-button v-if="row.status === 'available'" type="success" size="small" @click="adopt(row)">认养</el-button>
          <el-button v-if="canRelease(row)" type="warning" size="small" @click="release(row)">释放</el-button>
          <el-button v-if="canInvite(row)" type="primary" size="small" @click="openInvite(row)">邀请共管</el-button>
          <el-button v-if="canAccept(row)" type="success" size="small" @click="acceptInvite(row)">接受共管</el-button>
          <el-button v-if="canRemove(row)" type="danger" size="small" @click="removeCustodian(row)">移除共管</el-button>
          <el-button v-if="canDecline(row)" type="info" size="small" @click="removeCustodian(row)">拒绝</el-button>
          <el-button v-if="canWithdraw(row)" type="info" size="small" @click="removeCustodian(row)">撤回邀请</el-button>
          <el-button v-if="canQuit(row)" type="warning" size="small" @click="removeCustodian(row)">退出共管</el-button>
        </template>
      </el-table-column>
    </DataTable>

    <el-dialog v-model="inviteVisible" :title="`邀请共管（${inviteTarget?.code || ''} ${inviteTarget?.name || ''}）`" width="480px">
      <el-form label-width="90px">
        <el-form-item label="邻居用户名">
          <el-input v-model="inviteUsername" placeholder="输入已注册用户的用户名，如 citizen" />
        </el-form-item>
        <el-form-item label="共管权限">
          <span class="muted">接受后可帮该地块下的种植计划写日记；不能认养、释放或转交地块。</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="inviteVisible = false">取消</el-button>
        <el-button type="primary" :loading="custodianLoading" @click="submitInvite">发送邀请</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="historyVisible" :title="`共管记录（${historyTarget?.code || ''}）`" width="620px">
      <el-timeline v-if="historyList.length">
        <el-timeline-item
          v-for="c in historyList"
          :key="c.id"
          :timestamp="custodianTimestamp(c)"
          :type="timelineType(c.status)"
        >
          <div>
            <StatusBadge :value="c.status" :meta-map="CustodianStatusMeta" />
            <span style="margin-left: 8px">共管人：{{ displayName(c.custodian) }}</span>
          </div>
          <div class="muted" style="font-size: 12px">邀请人：{{ displayName(c.inviter) }}</div>
        </el-timeline-item>
      </el-timeline>
      <EmptyState v-else description="暂无共管邀请记录" />
    </el-dialog>

    <el-dialog v-model="createVisible" title="新增地块（管理员）" width="520px">
      <el-form :model="createForm" label-width="90px">
        <el-form-item label="地块名称"><el-input v-model="createForm.name" /></el-form-item>
        <el-form-item label="地块编号"><el-input v-model="createForm.code" placeholder="如 P-007" /></el-form-item>
        <el-form-item label="面积(m²)"><el-input-number v-model="createForm.area" :min="1" /></el-form-item>
        <el-form-item label="土壤类型">
          <el-select v-model="createForm.soil_type">
            <el-option v-for="(t, k) in SoilTypeText" :key="k" :label="t" :value="k" />
          </el-select>
        </el-form-item>
        <el-form-item label="日照条件">
          <el-select v-model="createForm.sunlight">
            <el-option v-for="(t, k) in SunlightText" :key="k" :label="t" :value="k" />
          </el-select>
        </el-form-item>
        <el-form-item label="纬度"><el-input-number v-model="createForm.latitude" :precision="4" :step="0.001" /></el-form-item>
        <el-form-item label="经度"><el-input-number v-model="createForm.longitude" :precision="4" :step="0.001" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="createForm.description" type="textarea" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="submitCreate">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { usePlotStore } from '@/stores/plot'
import { createPlot, releasePlot, type Plot } from '@/api/plot'
import { usePlotCustodianStore } from '@/stores/plotCustodian'
import type { PlotCustodian } from '@/api/plotCustodian'
import { useAuth } from '@/hooks/useAuth'
import { usePagination } from '@/hooks/usePagination'
import DataTable from '@/components/DataTable.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import EmptyState from '@/components/EmptyState.vue'
import { PlotStatusMeta, CustodianStatusMeta, SoilTypeText, SunlightText } from '@/constants'
import { formatArea, formatDateTime, clamp } from '@/utils/format'

const store = usePlotStore()
const custodianStore = usePlotCustodianStore()
const pagination = usePagination()
const { user, role, isAdmin } = useAuth()

const createVisible = ref(false)
const creating = ref(false)
const createForm = reactive({ name: '', code: '', area: 10, soil_type: 'loam', sunlight: 'full', latitude: 31.2304, longitude: 121.4737, description: '' })

// 地块共管：邀请弹窗与历史弹窗状态
const inviteVisible = ref(false)
const inviteTarget = ref<Plot | null>(null)
const inviteUsername = ref('')
const custodianLoading = ref(false)
const historyVisible = ref(false)
const historyTarget = ref<Plot | null>(null)
const historyList = ref<PlotCustodian[]>([])

const mapW = 600
const mapH = 360
const viewBoxStr = `0 0 ${mapW} ${mapH}`

function mapX(p: Plot) {
  return clamp(((p.longitude - 121.47) / 0.01) * 1000 + mapW / 2, 20, mapW - 20)
}
function mapY(p: Plot) {
  return clamp(((31.24 - p.latitude) / 0.02) * 1000 + mapH / 2, 20, mapH - 20)
}
function colorOf(status: string) {
  if (status === 'available') return '#67c23a'
  if (status === 'adopted') return '#e6a23c'
  return '#909399'
}

function displayName(u: { username?: string; nickname?: string } | null | undefined): string {
  if (!u) return '-'
  return u.nickname || u.username || '-'
}

function custodianTimestamp(c: PlotCustodian): string {
  if (c.status === 'accepted' && c.accepted_at) return `接受于 ${formatDateTime(c.accepted_at)}`
  if (c.status === 'removed' && c.removed_at) return `移除/退出于 ${formatDateTime(c.removed_at)}`
  return `邀请于 ${formatDateTime(c.invited_at)}`
}

function timelineType(status: string): 'success' | 'warning' | 'info' {
  if (status === 'accepted') return 'success'
  if (status === 'pending') return 'warning'
  return 'info'
}

function onPage(page: number) {
  pagination.page.value = page
  fetch()
}

async function fetch() {
  await store.fetchPlots({ page: pagination.page.value, page_size: pagination.size.value })
}

function canRelease(row: Plot) {
  return row.status === 'harvested' && (role.value === 'admin' || row.adopter_id === user.value?.id)
}

// 共管按钮显隐：仅当前认养人可邀请/移除；被邀请人可接受/拒绝；accepted 共管人可退出
function canInvite(row: Plot) {
  return row.status === 'adopted' && row.adopter_id === user.value?.id
}
function canAccept(row: Plot) {
  const c = row.latest_custodian
  return !!c && c.status === 'pending' && c.custodian_id === user.value?.id
}
function canDecline(row: Plot) {
  const c = row.latest_custodian
  // 被邀请人拒绝邀请
  return !!c && c.status === 'pending' && c.custodian_id === user.value?.id
}
function canWithdraw(row: Plot) {
  const c = row.latest_custodian
  // 认养人撤回尚未接受的邀请
  return !!c && c.status === 'pending' && c.inviter_id === user.value?.id && c.custodian_id !== user.value?.id
}
function canRemove(row: Plot) {
  const c = row.latest_custodian
  return !!c && c.status === 'accepted' && c.inviter_id === user.value?.id
}
function canQuit(row: Plot) {
  const c = row.latest_custodian
  return !!c && c.status === 'accepted' && c.custodian_id === user.value?.id
}

async function adopt(row: Plot) {
  try {
    await ElMessageBox.confirm(`确认认养地块 ${row.name}（${row.code}）吗？`, '认养确认', { type: 'success' })
  } catch {
    return
  }
  await store.adopt(row.id)
  ElMessage.success('认养成功，开始你的都市农夫之旅')
}

async function release(row: Plot) {
  try {
    await ElMessageBox.confirm(`确认释放地块 ${row.name} 吗？释放后将重新回到共享池，现有共管关系也会终止。`, '释放确认', { type: 'warning' })
  } catch {
    return
  }
  await releasePlot(row.id)
  ElMessage.success('地块已释放')
  await fetch()
}

function openInvite(row: Plot) {
  inviteTarget.value = row
  inviteUsername.value = ''
  inviteVisible.value = true
}

async function submitInvite() {
  if (!inviteTarget.value || !inviteUsername.value.trim()) {
    ElMessage.warning('请输入要邀请的注册用户名')
    return
  }
  custodianLoading.value = true
  try {
    await custodianStore.invite(inviteTarget.value.id, inviteUsername.value.trim())
    ElMessage.success('共管邀请已发出，等待对方接受')
    inviteVisible.value = false
    await fetch()
  } finally {
    custodianLoading.value = false
  }
}

async function acceptInvite(row: Plot) {
  const c = row.latest_custodian
  if (!c) return
  await custodianStore.accept(c.id, row.id)
  ElMessage.success('已接受共管邀请，可以帮忙写种植日记')
  await fetch()
}

async function removeCustodian(row: Plot) {
  const c = row.latest_custodian
  if (!c) return
  const isOwnerAction = c.inviter_id === user.value?.id
  const verb = c.status === 'pending' ? (isOwnerAction ? '撤回' : '拒绝') : (isOwnerAction ? '移除' : '退出')
  try {
    await ElMessageBox.confirm(
      isOwnerAction && c.status === 'accepted'
        ? `确认移除共管人 ${displayName(c.custodian)} 吗？移除后对方将无法再写入新日记，已写内容仍保留。`
        : `确认${verb}该地块共管吗？`,
      '共管确认',
      { type: 'warning' }
    )
  } catch {
    return
  }
  await custodianStore.remove(c.id, row.id)
  ElMessage.success(`已${verb}共管`)
  await fetch()
  if (historyVisible.value && historyTarget.value?.id === row.id) {
    historyList.value = custodianStore.history[row.id] || []
  }
}

async function openHistory(row: Plot) {
  historyTarget.value = row
  historyVisible.value = true
  await custodianStore.fetchHistory(row.id)
  historyList.value = custodianStore.history[row.id] || []
}

function openCreate() {
  createVisible.value = true
}

async function submitCreate() {
  creating.value = true
  try {
    await createPlot({ ...createForm })
    ElMessage.success('地块创建成功')
    createVisible.value = false
    await fetch()
  } finally {
    creating.value = false
  }
}

onMounted(fetch)
</script>

<style scoped>
.plot-map { width: 100%; height: 360px; border-radius: 8px; }
.custodian-line { font-size: 12px; color: #606266; margin-top: 2px; display: flex; justify-content: space-between; align-items: center; }
</style>
