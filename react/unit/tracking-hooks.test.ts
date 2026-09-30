import {act,cleanup,renderHook} from '@testing-library/react'
import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest'
import {useGPS,useTracking} from '../src/useTracking'

class FakeSocket {
  static OPEN=1;static all:FakeSocket[]=[];readyState=0;sent:string[]=[];closed=false
  onopen:(()=>void)|null=null;onmessage:((event:{data:string})=>void)|null=null;onclose:((event:{code:number})=>void)|null=null
  constructor(public url:string){FakeSocket.all.push(this)}
  open(){this.readyState=1;this.onopen?.()}
  send(value:string){this.sent.push(value)}
  close(){this.closed=true;this.readyState=3;this.onclose?.({code:1000})}
  disconnect(code:number){this.readyState=3;this.onclose?.({code})}
  receive(data:unknown){this.onmessage?.({data:JSON.stringify(data)})}
}
let success:PositionCallback,error:PositionErrorCallback
const watch=vi.fn(),clear=vi.fn()
beforeEach(()=>{vi.useFakeTimers();FakeSocket.all=[];watch.mockReset();clear.mockReset();watch.mockImplementation((ok:PositionCallback,fail:PositionErrorCallback)=>{success=ok;error=fail;return 7});Object.defineProperty(navigator,'geolocation',{configurable:true,value:{watchPosition:watch,clearWatch:clear}});Object.defineProperty(window,'isSecureContext',{configurable:true,value:true});vi.stubGlobal('WebSocket',FakeSocket)})
afterEach(()=>{cleanup();vi.useRealTimers();vi.unstubAllGlobals()})
const point=(timestamp=Date.now())=>({timestamp,coords:{latitude:0,longitude:0}} as GeolocationPosition)
describe('GPS lifecycle',()=>{
  it('requires explicit start, transmits zero, and clears resources on stop',()=>{
    const {result}=renderHook(()=>useGPS());expect(watch).not.toHaveBeenCalled();act(()=>result.current.start());expect(result.current.enabled).toBe(true)
    act(()=>{success(point());FakeSocket.all[0].open()});expect(JSON.parse(FakeSocket.all[0].sent[0])).toEqual({latitude:0,longitude:0})
    act(()=>result.current.stop());expect(clear).toHaveBeenCalledWith(7);expect(FakeSocket.all[0].closed).toBe(true);expect(result.current.enabled).toBe(false)
  })
  it('does not retransmit stale positions',async()=>{const {result}=renderHook(()=>useGPS());act(()=>result.current.start());act(()=>{FakeSocket.all[0].open();success(point(Date.now()-60000))});await act(async()=>{await vi.advanceTimersByTimeAsync(15000)});expect(FakeSocket.all[0].sent).toHaveLength(0)})
  it('handles GPS permission denial',()=>{const {result}=renderHook(()=>useGPS());act(()=>result.current.start());act(()=>error({code:1} as GeolocationPositionError));expect(result.current.enabled).toBe(false);expect(result.current.status).toContain('Permiso denegado');expect(clear).toHaveBeenCalledWith(7)})
  it.each([4009,1008])('stops reconnection on close code %d',async(code)=>{const {result}=renderHook(()=>useGPS());act(()=>result.current.start());act(()=>FakeSocket.all[0].disconnect(code));await act(async()=>{await vi.advanceTimersByTimeAsync(5000)});expect(result.current.enabled).toBe(false);expect(FakeSocket.all).toHaveLength(1)})
  it('reconnects after network loss and stops when leaving the page',async()=>{const {result}=renderHook(()=>useGPS());act(()=>result.current.start());act(()=>FakeSocket.all[0].disconnect(1006));await act(async()=>{await vi.advanceTimersByTimeAsync(3000)});expect(FakeSocket.all).toHaveLength(2);act(()=>window.dispatchEvent(new Event('pagehide')));expect(result.current.enabled).toBe(false);expect(clear).toHaveBeenCalledWith(7)})
  it('requires a secure context',()=>{Object.defineProperty(window,'isSecureContext',{configurable:true,value:false});const {result}=renderHook(()=>useGPS());act(()=>result.current.start());expect(watch).not.toHaveBeenCalled();expect(FakeSocket.all).toHaveLength(0);expect(result.current.status).toContain('HTTPS')})
})
describe('dashboard connection and privacy',()=>{
  it('reconciles snapshots, zero coordinates, revocation and malformed frames',()=>{
    const {result,unmount}=renderHook(()=>useTracking(1,vi.fn()));const socket=FakeSocket.all[0];const h={halcon_id:1,name:'Propio',lat:1,lng:1,has_location:true,active:true}
    act(()=>{socket.open();socket.receive({type:'snapshot',halcones:[h]});socket.receive({halcon_id:1,latitude:0,longitude:0})});expect(result.current.halcones[0].lat).toBe(0)
    act(()=>{socket.receive(null);socket.receive({type:'snapshot'});socket.receive({type:'snapshot',halcones:[{}]});socket.receive({halcon_id:1,latitude:999,longitude:0})});expect(result.current.halcones[0].lat).toBe(0)
    act(()=>socket.receive({type:'snapshot',halcones:[]}));expect(result.current.halcones).toEqual([]);unmount();expect(socket.closed).toBe(true)
  })
  it('drops location data on disconnection and reports an expired session',async()=>{const expired=vi.fn();vi.stubGlobal('fetch',vi.fn().mockResolvedValue(new Response(JSON.stringify({user:null}))));const {result}=renderHook(()=>useTracking(1,expired));await act(async()=>{FakeSocket.all[0].disconnect(1006);await vi.advanceTimersByTimeAsync(1000)});expect(expired).toHaveBeenCalledOnce();expect(result.current.halcones).toEqual([]);expect(FakeSocket.all).toHaveLength(1)})
  it('retries temporary server errors without ending the session',async()=>{const expired=vi.fn();vi.stubGlobal('fetch',vi.fn().mockResolvedValue(new Response(JSON.stringify({error:'temporary'}),{status:500})));renderHook(()=>useTracking(1,expired));await act(async()=>{FakeSocket.all[0].disconnect(1006);await vi.advanceTimersByTimeAsync(1000)});expect(expired).not.toHaveBeenCalled();expect(FakeSocket.all).toHaveLength(2)})
})
