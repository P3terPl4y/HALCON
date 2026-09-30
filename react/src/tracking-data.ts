import type {Halcon} from './api'

export function validPosition(lat:unknown,lng:unknown): boolean {
  return typeof lat==='number' && Number.isFinite(lat) && lat>=-90 && lat<=90 && typeof lng==='number' && Number.isFinite(lng) && lng>=-180 && lng<=180
}
export function validHalcon(value:unknown): value is Halcon {
  if (!value || typeof value!=='object') return false
  const h=value as Partial<Halcon>
  return typeof h.halcon_id==='number' && Number.isSafeInteger(h.halcon_id) && h.halcon_id>0 && typeof h.name==='string' && typeof h.active==='boolean' && typeof h.has_location==='boolean' && validPosition(h.lat,h.lng)
}
