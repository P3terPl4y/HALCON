import {useEffect,useRef} from 'react'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import type {Halcon} from './api'

export default function Map({halcones,selected}:{halcones:Halcon[];selected:number|null}) {
  const element=useRef<HTMLDivElement>(null), map=useRef<L.Map|null>(null), markers=useRef(new globalThis.Map<number,L.CircleMarker>())
  useEffect(()=>{
    if(!element.current)return
    const instance=L.map(element.current).setView([21.5218,-77.7812],7); map.current=instance
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',{attribution:'© OpenStreetMap contributors',maxZoom:19}).addTo(instance)
    const observer=new ResizeObserver(()=>instance.invalidateSize());observer.observe(element.current)
    return()=>{observer.disconnect();instance.remove();map.current=null;markers.current.clear()}
  },[])
  useEffect(()=>{
    const instance=map.current;if(!instance)return
    const allowed=new Set(halcones.filter(h=>h.has_location).map(h=>h.halcon_id))
    markers.current.forEach((marker,id)=>{if(!allowed.has(id)){marker.remove();markers.current.delete(id)}})
    for(const h of halcones.filter(h=>h.has_location)) {
      let marker=markers.current.get(h.halcon_id)
      const color=h.active?'#3a2478':'#64748b'
      if(!marker){marker=L.circleMarker([h.lat,h.lng],{radius:9,color,fillColor:color,fillOpacity:.85}).addTo(instance);markers.current.set(h.halcon_id,marker)}
      marker.setLatLng([h.lat,h.lng]).setStyle({color,fillColor:color})
      const label=document.createElement('div');label.textContent=`${h.name} · ${h.lat.toFixed(5)}, ${h.lng.toFixed(5)}`;marker.bindPopup(label)
    }
  },[halcones])
  useEffect(()=>{const marker=selected===null?null:markers.current.get(selected);if(marker&&map.current){map.current.setView(marker.getLatLng(),15);marker.openPopup()}},[selected])
  return <div ref={element} className="map" role="region" aria-label="Mapa de ubicaciones" />
}
