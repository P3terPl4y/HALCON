export type User = {id: number; name: string; email: string; role: string}
export type Session = {user: User | null; csrf_token: string}
export type Halcon = {halcon_id: number; name: string; lat: number; lng: number; active: boolean; has_location: boolean}
export type Tracking = {personal_id: number; recipient: User | null; csrf_token: string}
export class APIError extends Error { constructor(message: string, public status: number) { super(message) } }
export async function request<T>(path: string, values?: Record<string, string>, token?: string): Promise<T> {
  const body = values ? new URLSearchParams({...values, _csrf: token || ''}) : undefined
  const response = await fetch(path, {method: body ? 'POST' : 'GET', body, credentials: 'same-origin', cache: 'no-store', headers: {Accept: 'application/json'}})
  if (response.status === 204) return undefined as T
  const text = await response.text()
  let data: any
  try { data = text ? JSON.parse(text) : {} } catch { data = {} }
  if (!response.ok) throw new APIError(data.error || (response.status === 403 ? 'El formulario caducó. Recarga la página.' : response.status === 429 ? 'Demasiados intentos. Espera un minuto.' : 'No se pudo completar la acción.'), response.status)
  return data as T
}
