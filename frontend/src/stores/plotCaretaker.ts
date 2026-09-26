import { defineStore } from 'pinia'
import {
  acceptCaretaker,
  inviteCaretaker,
  listMyCaretakerInvitations,
  removeCaretaker,
  type CaretakerInvitation
} from '@/api/plotCaretaker'
import { usePlotStore } from './plot'

// 地块共管 store：邀请/接受/移除后统一刷新地块列表，保证地块列表里能看到状态变化。
export const useCaretakerStore = defineStore('plotCaretaker', {
  state: () => ({
    invitations: [] as CaretakerInvitation[],
    loadingInvitations: false
  }),
  actions: {
    async fetchInvitations() {
      this.loadingInvitations = true
      try {
        this.invitations = await listMyCaretakerInvitations()
      } finally {
        this.loadingInvitations = false
      }
    },
    async invite(plotId: number, username: string) {
      const res = await inviteCaretaker(plotId, username)
      await usePlotStore().fetchPlots()
      await this.fetchInvitations()
      return res
    },
    async accept(plotId: number) {
      const res = await acceptCaretaker(plotId)
      await usePlotStore().fetchPlots()
      await this.fetchInvitations()
      return res
    },
    async remove(plotId: number) {
      const res = await removeCaretaker(plotId)
      await usePlotStore().fetchPlots()
      return res
    }
  }
})
