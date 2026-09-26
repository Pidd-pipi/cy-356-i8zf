<template>
  <div class="page-card">
    <div style="display: flex; justify-content: space-between; align-items: center">
      <h3 class="page-title">菜园地块认养（GIS 分布）</h3>
      <div>
        <el-badge :hidden="!caretakerStore.invitations.length" :value="caretakerStore.invitations.length" style="margin-right: 12px">
          <el-button @click="scrollToPlots">共管邀请</el-button>
        </el-badge>
        <el-button v-if="isAdmin" type="primary" @click="openCreate">+ 新增地块</el-button>
      </div>
    </div>

    <el-alert
      v-if="caretakerStore.invitations.length"
      type="warning"
      :closable="false"
      show-icon
      style="margin: 12px 0"
      title="你有地块共管邀请"
      description="接受后可以帮同一地块下的种植计划写种植日记（不能认养、释放或转交地块）"
    >
      <div style="margin-top: 8px; display: flex; flex-wrap: wrap; gap: 8px">
        <el-tag v-for="inv in caretakerStore.invitations" :key="inv.id" type="warning">
          {{ inv.plot_code }} {{ inv.plot_name }}（邀请人：{{ inv.inviter?.nickname || inv.inviter?.username }}）
          <el-button type="success" size="small" style="margin-left: 6px" @click="acceptInvite(inv)">接受</el-button>
        </el-tag>
      </div>
    </el-alert>

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

    <div id="plots-table">
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
      <el-table-column label="共管人" min-width="230">
        <template #default="{ row }">
          <div v-if="row.caretaker" class="caretaker-cell">
            <StatusBadge :value="row.caretaker.status" :meta-map="CaretakerStatusMeta" />
            <span>{{ caretakerName(row.caretaker) }}</span>
            <el-button
              v-if="canAccept(row)"
              type="success" size="small" @click="acceptInviteByPlot(row)"
            >接受</el-button>
            <el-button
              v-if="canManageCaretaker(row)"
              type="danger" size="small" @click="removeCaretaker(row)"
            >{{ row.caretaker.status === 'invited' ? '撤销邀请' : '移除' }}</el-button>
          </div>
          <el-button
            v-else-if="canInvite(row)"
            type="primary" link size="small" @click="openInvite(row)"
          >+ 邀请共管</el-button>
          <span v-else class="muted">-</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="220">
        <template #default="{ row }">
          <el-button v-if="row.status === 'available'" type="success" size="small" @click="adopt(row)">认养</el-button>
          <el-button v-if="canRelease(row)" type="warning" size="small" @click="release(row)">释放</el-button>
        </template>
      </el-table-column>
    </DataTable>
    </div>

    <el-dialog v-model="inviteVisible" :title="`邀请共管人 · ${invitePlot?.code || ''} ${invitePlot?.name || ''}`" width="460px">
      <el-form label-width="100px">
        <el-form-item label="对方用户名">
          <el-input v-model="inviteUsername" placeholder="输入已注册用户的用户名" />
        </el-form-item>
        <el-alert type="info" :closable="false" show-icon
          title="对方接受后可为该地块的种植计划写日记，但不能认养、释放或转交地块；移除后其新日记无法写入，已写内容仍保留。" />
      </el-form>
      <template #footer>
        <el-button @click="inviteVisible = false">取消</el-button>
        <el-button type="primary" :loading="inviting" @click="submitInvite">发送邀请</el-button>
      </template>
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
import { useCaretakerStore } from '@/stores/plotCaretaker'
import { createPlot, type Plot } from '@/api/plot'
import type { PlotCaretaker, CaretakerInvitation } from '@/api/plotCaretaker'
import { useAuth } from '@/hooks/useAuth'
import { usePagination } from '@/hooks/usePagination'
import DataTable from '@/components/DataTable.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import { PlotStatusMeta, CaretakerStatusMeta, SoilTypeText, SunlightText } from '@/constants'
import { formatArea, clamp } from '@/utils/format'

const store = usePlotStore()
const caretakerStore = useCaretakerStore()
const pagination = usePagination()
const { user, role, isAdmin } = useAuth()

const createVisible = ref(false)
const creating = ref(false)
const createForm = reactive({ name: '', code: '', area: 10, soil_type: 'loam', sunlight: 'full', latitude: 31.2304, longitude: 121.4737, description: '' })

// 地块共管：邀请弹窗状态
const inviteVisible = ref(false)
const inviting = ref(false)
const invitePlot = ref<Plot | null>(null)
const inviteUsername = ref('')

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

function onPage(page: number) {
  pagination.page.value = page
  fetch()
}

async function fetch() {
  await store.fetchPlots({ page: pagination.page.value, page_size: pagination.size.value })
}

function canRelease(row: Plot) {
  // 共管人不能释放地块：仅认养人本人或管理员可释放
  return row.status === 'harvested' && (role.value === 'admin' || row.adopter_id === user.value?.id)
}

function caretakerName(c: PlotCaretaker) {
  return c.caretaker?.nickname || c.caretaker?.username || `用户#${c.caretaker_id}`
}

// 认养人且地块处于已认养状态：可以邀请共管（每位认养人仅能邀请一位，已有邀请/共管人时按钮消失）
function canInvite(row: Plot) {
  return !!user.value && row.status === 'adopted' && row.adopter_id === user.value.id && !row.caretaker
}

// 认养人可以撤销待接受邀请或移除共管人
function canManageCaretaker(row: Plot) {
  return !!user.value && row.adopter_id === user.value.id
}

// 被邀请的当前登录用户可以接受邀请
function canAccept(row: Plot) {
  return !!user.value && row.caretaker?.status === 'invited' && row.caretaker.caretaker_id === user.value.id
}

function scrollToPlots() {
  document.getElementById('plots-table')?.scrollIntoView({ behavior: 'smooth' })
}

function openInvite(row: Plot) {
  invitePlot.value = row
  inviteUsername.value = ''
  inviteVisible.value = true
}

async function submitInvite() {
  if (!invitePlot.value || !inviteUsername.value.trim()) {
    ElMessage.warning('请输入对方的注册用户名')
    return
  }
  inviting.value = true
  try {
    const res = await caretakerStore.invite(invitePlot.value.id, inviteUsername.value.trim())
    ElMessage.success(res.message || '共管邀请已发出')
    inviteVisible.value = false
  } finally {
    inviting.value = false
  }
}

async function acceptInvite(inv: CaretakerInvitation) {
  const res = await caretakerStore.accept(inv.plot_id)
  ElMessage.success(res.message || '已接受共管邀请')
}

async function acceptInviteByPlot(row: Plot) {
  const res = await caretakerStore.accept(row.id)
  ElMessage.success(res.message || '已接受共管邀请')
}

async function removeCaretaker(row: Plot) {
  if (!row.caretaker) return
  const isPending = row.caretaker.status === 'invited'
  try {
    await ElMessageBox.confirm(
      isPending
        ? `确认撤销发给 ${caretakerName(row.caretaker)} 的共管邀请吗？`
        : `确认移除共管人 ${caretakerName(row.caretaker)} 吗？移除后对方将无法为该地块写新日记（历史日记保留）。`,
      isPending ? '撤销邀请' : '移除共管人',
      { type: 'warning' }
    )
  } catch {
    return
  }
  const res = await caretakerStore.remove(row.id)
  ElMessage.success(res.message || '共管人已移除')
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
    await ElMessageBox.confirm(`确认释放地块 ${row.name} 吗？释放后将重新回到共享池。`, '释放确认', { type: 'warning' })
  } catch {
    return
  }
  const { releasePlot } = await import('@/api/plot')
  await releasePlot(row.id)
  ElMessage.success('地块已释放')
  await fetch()
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

onMounted(() => {
  fetch()
  if (user.value) {
    caretakerStore.fetchInvitations()
  }
})
</script>

<style scoped>
.plot-map { width: 100%; height: 360px; border-radius: 8px; }
.caretaker-cell { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
</style>
