import {useEffect, useRef, useState} from 'react'
import type {Halcon} from './api'
import {validHalcon,validPosition} from './tracking-data'

export function useTracking(userID: number, onExpired: () => void) {
  const [halcones, setHalcones] = useState<Halcon[]>([])
  const [connection, setConnection] = useState('Conectando…')
  const expire = useRef(onExpired); expire.current = onExpired
  useEffect(() => {
    let disposed = false, retry: ReturnType<typeof setTimeout> | undefined, socket: WebSocket
    let delay = 1000
    const connect = () => {
      if (disposed) return
      socket = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/dashboard/${userID}/`)
      socket.onopen = () => { delay = 1000; setConnection('Conectado') }
      socket.onmessage = (event) => {
        let data: any
        try { data = JSON.parse(event.data) } catch { return }
        if (!data || typeof data!=='object') return
        if (data.type === 'snapshot' && Array.isArray(data.halcones) && data.halcones.every(validHalcon)) setHalcones(data.halcones)
        else if (validPosition(data.latitude,data.longitude)) setHalcones(items => items.map(h => h.halcon_id === data.halcon_id ? {...h,lat:data.latitude,lng:data.longitude,active:true,has_location:true} : h))
      }
      socket.onclose = async () => {
        if (disposed) return
        setHalcones([]); setConnection('Reconectando…')
        try {
          const response = await fetch('/api/session', {credentials:'same-origin',cache:'no-store'})
          if (!response.ok) throw new Error('session temporarily unavailable')
          const session = await response.json()
          if (session.user===null) { expire.current(); return }
        } catch { /* Network failure: retry with bounded backoff. */ }
        if (!disposed) { retry = setTimeout(connect,delay); delay = Math.min(delay*2,15000) }
      }
    }
    connect()
    return () => { disposed = true; clearTimeout(retry); socket?.close() }
  }, [userID])
  return {halcones,connection}
}

export function useGPS() {
  const [enabled, setEnabled] = useState(false)
  const [status,setStatus] = useState('Ubicación desactivada. Tú decides cuándo transmitir.')
  const stopRef = useRef<() => void>(() => {})
  useEffect(() => {
    if (!enabled) return
    let socket: WebSocket | null = null, retry: ReturnType<typeof setTimeout> | undefined, watch: number | undefined
    let stopped = false, last: GeolocationPosition | undefined
    const send = () => {
      if (socket?.readyState === WebSocket.OPEN && last && Date.now()-last.timestamp < 45000) {
        socket.send(JSON.stringify({latitude:last.coords.latitude,longitude:last.coords.longitude})); setStatus('Transmitiendo. Mantén esta página abierta.')
      }
    }
    const connect = () => {
      if (stopped) return
      socket = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/location`)
      socket.onopen = send
      socket.onclose = (event) => {
        if (stopped) return
        if (event.code === 4009 || event.code === 1008) {
          setEnabled(false); setStatus(event.code === 4009 ? 'Otra pestaña está transmitiendo tu halcón.' : 'La sesión terminó. Inicia sesión de nuevo.'); return
        }
        setStatus('Reconectando transmisión…'); retry = setTimeout(connect,3000)
      }
    }
    const cleanup = () => { stopped = true; clearTimeout(retry); clearInterval(heartbeat); if(watch!==undefined)navigator.geolocation.clearWatch(watch); socket?.close() }
    const heartbeat = setInterval(send,15000)
    stopRef.current = cleanup
    if (!navigator.geolocation || !window.isSecureContext) { cleanup(); setEnabled(false); setStatus('GPS requiere HTTPS o localhost y permiso de ubicación.'); return }
    setStatus('Esperando permiso y una posición GPS…'); connect()
    watch = navigator.geolocation.watchPosition(p => {last=p;send()}, error => {
      if(error.code===1){setEnabled(false);setStatus('Permiso denegado. Puedes habilitarlo desde el navegador.')}
      else setStatus('GPS no disponible. Esperando una posición nueva…')
    },{enableHighAccuracy:true,maximumAge:10000,timeout:20000})
    const pagehide = () => { cleanup(); setEnabled(false) }
    window.addEventListener('pagehide',pagehide)
    return () => { cleanup(); window.removeEventListener('pagehide',pagehide) }
  },[enabled])
  return {enabled,status,start:()=>setEnabled(true),stop:()=>{stopRef.current();setEnabled(false);setStatus('GPS detenido. Retira el acceso si no quieres compartir la última posición.')}}
}
