import {createElement} from 'react'
import {cleanup,render} from '@testing-library/react'
import {afterEach,expect,it,vi} from 'vitest'

const mocks=vi.hoisted(()=>{
  const map={setView:vi.fn().mockReturnThis(),invalidateSize:vi.fn(),remove:vi.fn()}
  const markers:Array<{remove:ReturnType<typeof vi.fn>;bindPopup:ReturnType<typeof vi.fn>;setLatLng:ReturnType<typeof vi.fn>;setStyle:ReturnType<typeof vi.fn>;openPopup:ReturnType<typeof vi.fn>;getLatLng:ReturnType<typeof vi.fn>}> = []
  return {map,markers,circleMarker:vi.fn((point:number[])=>{const marker={remove:vi.fn(),bindPopup:vi.fn(),setLatLng:vi.fn().mockReturnThis(),setStyle:vi.fn(),openPopup:vi.fn(),getLatLng:vi.fn(()=>point),addTo:vi.fn().mockReturnThis()};markers.push(marker);return marker})}
})
vi.mock('leaflet',()=>({default:{map:()=>mocks.map,tileLayer:()=>({addTo:vi.fn()}),circleMarker:mocks.circleMarker}}))
import Map from '../src/Map'
afterEach(()=>{cleanup();vi.unstubAllGlobals()})

it('shows located halcones, escapes names, removes revoked markers and cleans up',()=>{
  const disconnect=vi.fn();vi.stubGlobal('ResizeObserver',class {observe(){}disconnect=disconnect})
  const located={halcon_id:1,name:'<img src=x onerror=alert(1)>',lat:10,lng:20,has_location:true,active:true}
  const pending={...located,halcon_id:2,has_location:false}
  const {rerender,unmount}=render(createElement(Map,{halcones:[located,pending],selected:1}))
  expect(mocks.circleMarker).toHaveBeenCalledOnce();const popup=mocks.markers[0].bindPopup.mock.calls[0][0] as HTMLElement;expect(popup.querySelector('img')).toBeNull();expect(popup.textContent).toContain('<img');expect(mocks.markers[0].openPopup).toHaveBeenCalled()
  rerender(createElement(Map,{halcones:[],selected:null}));expect(mocks.markers[0].remove).toHaveBeenCalledOnce();unmount();expect(mocks.map.remove).toHaveBeenCalledOnce();expect(disconnect).toHaveBeenCalledOnce()
})
