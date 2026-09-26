import { defineStore } from 'pinia'
import {
  acceptCustodian,
  inviteCustodian,
  listPlotCustodians,
  removeCustodian,
  type PlotCustodian
} from '@/api/plotCustodian'

// 地块共管 store：负责发起邀请、接受、移除与历史查询（地块列表页复用）。
interface PlotCustodianState {
  history: Record<number, PlotCustodian[]>
  loading: boolean
}

export const usePlotCustodianStore = defineStore('plotCustodian', {
  state: (): PlotCustodianState => ({ history: {}, loading: false }),
  actions: {
    async invite(plotId: number, username: string) {
      const rec = await inviteCustodian(plotId, username)
      await this.fetchHistory(plotId)
      return rec
    },
    async accept(custodianId: number, plotId: number) {
      await acceptCustodian(custodianId)
      await this.fetchHistory(plotId)
    },
    async remove(custodianId: number, plotId: number) {
      await removeCustodian(custodianId)
      await this.fetchHistory(plotId)
    },
    async fetchHistory(plotId: number) {
      this.loading = true
      try {
        this.history[plotId] = await listPlotCustodians(plotId)
      } finally {
        this.loading = false
      }
    }
  }
})
