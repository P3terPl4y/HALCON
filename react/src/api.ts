export type User = {id: number; name: string; email: string; role: string}
export type Session = {user: User | null; csrf_token: string}
export type Halcon = {halcon_id: number; name: string; lat: number; lng: number; active: boolean; has_location: boolean}
export type Tracking = {personal_id: number; recipient: User | null; csrf_token: string}
export class APIError extends Error { constructor(message: string, public status: number) { super(message) } }
export async function request<T>(path: string, values?: Record<string, string>, token?: string): Promise<T> {
  const body = values ? new URLSearchParams({...values, _csrf: token || ''}) : undefined
  let response: Response
  try { response = await fetch(path, {method: body ? 'POST' : 'GET', body, credentials: 'same-origin', cache: 'no-store', headers: {Accept: 'application/json'}}) }
  catch { throw new APIError('Sin conexión. Conservamos tus datos para que puedas reintentar.', 0) }
  if (response.status === 204) return undefined as T
  const text = await response.text()
  let data: any
  let valid = true
  try { data = text ? JSON.parse(text) : {}; if(!text || data===null || typeof data!=='object' || Array.isArray(data)) { valid=false; data={} } } catch { data = {}; valid = false }
  if (!response.ok) throw new APIError(data.error || (response.status === 403 ? 'El formulario caducó. Recarga la página.' : response.status === 429 ? 'Demasiados intentos. Espera un minuto.' : 'No se pudo completar la acción.'), response.status)
  if (!valid) throw new APIError('El servidor envió una respuesta inválida. Inténtalo de nuevo.', response.status)
  return data as T
}
