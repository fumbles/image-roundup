// Central API client — all requests go through here.
import axios from 'axios'
import type { ImageRecord, Summary, ScanStatus, RegistryInfo, Settings, ImagesQuery } from './types'

const client = axios.create({ baseURL: '/api/v1' })

export async function getSummary(): Promise<Summary> {
  const { data } = await client.get<Summary>('/summary')
  return data
}

export async function getImages(q: ImagesQuery = {}): Promise<ImageRecord[]> {
  const { data } = await client.get<ImageRecord[]>('/images', { params: q })
  return data
}

export async function getImage(id: string): Promise<ImageRecord> {
  const { data } = await client.get<ImageRecord>(`/images/${encodeURIComponent(id)}`)
  return data
}

export async function getRegistries(): Promise<RegistryInfo[]> {
  const { data } = await client.get<RegistryInfo[]>('/registries')
  return data
}

export async function getScan(): Promise<ScanStatus> {
  const { data } = await client.get<ScanStatus>('/scan')
  return data
}

export async function triggerScan(): Promise<void> {
  await client.post('/scan')
}

export async function getSettings(): Promise<Settings> {
  const { data } = await client.get<Settings>('/settings')
  return data
}

export async function putSettings(settings: Settings): Promise<Settings> {
  const { data } = await client.put<Settings>('/settings', settings)
  return data
}
