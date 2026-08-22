import type { BoardListItem } from './protocol'
import { logger } from './logger'

export type ApiError = { code: string; message: string }

async function parse<T>(res: Response): Promise<T> {
  if (res.status === 204) return undefined as T
  const text = await res.text()
  let body: unknown = null
  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      body = { message: text }
    }
  }
  if (!res.ok) {
    const err = (body ?? {}) as ApiError
    throw Object.assign(new Error(err.message || res.statusText), {
      code: err.code || 'http',
      status: res.status,
    })
  }
  return body as T
}

export const api = {
  async listBoards(): Promise<BoardListItem[]> {
    const data = await parse<{ items: BoardListItem[] }>(await fetch('/api/v1/boards'))
    return data.items ?? []
  },
  async createBoard(title: string, passcode?: string): Promise<BoardListItem> {
    return parse(
      await fetch('/api/v1/boards', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title, passcode: passcode || undefined }),
      }),
    )
  },
  async getBoard(id: string): Promise<BoardListItem> {
    return parse(await fetch(`/api/v1/boards/${id}`))
  },
  async patchBoard(
    id: string,
    body: { title?: string; passcode?: string; clearPass?: boolean; thumbnail?: string },
  ): Promise<BoardListItem> {
    return parse(
      await fetch(`/api/v1/boards/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      }),
    )
  },
  async deleteBoard(id: string): Promise<void> {
    await parse(await fetch(`/api/v1/boards/${id}`, { method: 'DELETE' }))
  },
  async unlock(id: string, passcode: string): Promise<boolean> {
    await parse(await fetch(`/api/v1/boards/${id}/unlock`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ passcode }),
    }))
    return true
  },
  async upload(file: File): Promise<{ hash: string; url: string; mime: string; bytes: number }> {
    const fd = new FormData()
    fd.append('file', file)
    return parse(await fetch('/api/v1/uploads', { method: 'POST', body: fd }))
  },
}

export function reportApiError(e: unknown, fallback: string): string {
  const msg = e instanceof Error ? e.message : fallback
  logger.warn('api', msg)
  return msg
}
