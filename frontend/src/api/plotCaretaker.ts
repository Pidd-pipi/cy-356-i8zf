import { del, get, post } from '@/utils/request'
import type { UserInfo } from './auth'

// 地块共管状态：invited 待接受 / active 共管中 / ended 已移除
export interface PlotCaretaker {
  id: number
  plot_id: number
  caretaker_id: number
  caretaker: UserInfo | null
  inviter_id: number
  inviter: UserInfo | null
  status: 'invited' | 'active' | 'ended' | string
  status_text: string
  invited_at: string
  accepted_at: string
  removed_at: string
}

export interface CaretakerInvitation {
  id: number
  plot_id: number
  plot_code: string
  plot_name: string
  inviter_id: number
  inviter: UserInfo | null
  status: string
  status_text: string
  invited_at: string
}

export function inviteCaretaker(plotId: number, username: string): Promise<{ plot_id: number; caretaker_id: number; status: string; message: string }> {
  return post(`/plots/${plotId}/caretaker/invite`, { username })
}

export function acceptCaretaker(plotId: number): Promise<{ plot_id: number; caretaker_id: number; status: string; message: string }> {
  return post(`/plots/${plotId}/caretaker/accept`)
}

export function removeCaretaker(plotId: number): Promise<{ plot_id: number; caretaker_id: number; status: string; message: string }> {
  return del(`/plots/${plotId}/caretaker`)
}

export function listMyCaretakerInvitations(): Promise<CaretakerInvitation[]> {
  return get('/caretaker/invitations')
}
