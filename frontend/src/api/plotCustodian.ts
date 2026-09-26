import { get, post } from '@/utils/request'
import type { UserInfo } from './auth'

// 地块共管邀请记录（与后端 PlotCustodianOutDTO 对应）
export interface PlotCustodian {
  id: number
  plot_id: number
  custodian_id: number
  custodian: UserInfo | null
  inviter_id: number
  inviter: UserInfo | null
  status: 'pending' | 'accepted' | 'removed'
  invited_at: string
  accepted_at: string | null
  removed_at: string | null
  created_at: string
}

// 认养人按用户名邀请一位注册用户共管地块
export function inviteCustodian(plotId: number, username: string): Promise<PlotCustodian> {
  return post(`/plots/${plotId}/custodians/invite`, { username })
}

// 被邀请人接受共管邀请
export function acceptCustodian(custodianId: number): Promise<PlotCustodian> {
  return post(`/plot-custodians/${custodianId}/accept`)
}

// 认养人移除共管人 / 被邀请人拒绝或共管人退出
export function removeCustodian(custodianId: number): Promise<PlotCustodian> {
  return post(`/plot-custodians/${custodianId}/remove`)
}

// 查询某地块的全部共管记录（邀请、接受、移除历史）
export function listPlotCustodians(plotId: number): Promise<PlotCustodian[]> {
  return get(`/plots/${plotId}/custodians`)
}
