import {afterEach,describe,expect,it,vi} from 'vitest'
import {APIError,request} from '../src/api'
import {validHalcon,validPosition} from '../src/tracking-data'

afterEach(()=>vi.unstubAllGlobals())
describe('HTTP session and CSRF client',()=>{
  it('posts URL-encoded CSRF and uses same-origin cookies without caching',async()=>{
    const fetcher=vi.fn().mockResolvedValue(new Response(JSON.stringify({user:{id:1}}),{status:200}));vi.stubGlobal('fetch',fetcher)
    const result=await request<{user:{id:number}}>('/api/auth/login',{email:'ana@example.test',password:'Test123456!'},'csrf-token')
    expect(result.user.id).toBe(1);const options=fetcher.mock.calls[0][1];expect(options.method).toBe('POST');expect(options.credentials).toBe('same-origin');expect(options.cache).toBe('no-store');expect(options.body.get('_csrf')).toBe('csrf-token')
  })
  it('gets session and accepts empty logout replies',async()=>{
    const fetcher=vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({user:null,csrf_token:'token'}))).mockResolvedValueOnce(new Response(null,{status:204}));vi.stubGlobal('fetch',fetcher)
    expect((await request<{user:null}>('/api/session')).user).toBeNull();expect(fetcher.mock.calls[0][1].method).toBe('GET');expect(await request('/api/auth/logout',{},'token')).toBeUndefined()
  })
  it.each([401,403,422,429,500])('reports HTTP %d without exposing an HTML error body',async(status)=>{
    vi.stubGlobal('fetch',vi.fn().mockResolvedValue(new Response('<html>private SQL detail</html>',{status})))
    try{await request('/api/tracking');throw new Error('unexpected success')}catch(error){expect(error).toBeInstanceOf(APIError);expect((error as APIError).status).toBe(status);expect((error as Error).message).not.toContain('SQL')}
  })
  it('uses server validation messages and translates network failures',async()=>{
    const fetcher=vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({error:'Correo inválido'}),{status:422})).mockRejectedValueOnce(new TypeError('Failed to fetch'));vi.stubGlobal('fetch',fetcher)
    await expect(request('/api/tracking/recipient',{})).rejects.toThrow('Correo inválido');await expect(request('/api/tracking')).rejects.toThrow('Sin conexión')
  })
  it.each(['invalid JSON','null','[]','42',''])('rejects malformed successful reply %s',async(body)=>{vi.stubGlobal('fetch',vi.fn().mockResolvedValue(new Response(body)));await expect(request('/api/session')).rejects.toThrow('respuesta inválida')})
})
describe('untrusted real-time payloads',()=>{
  it('accepts zero and geographic bounds',()=>{expect(validPosition(0,0)).toBe(true);expect(validPosition(-90,180)).toBe(true)})
  it.each([[NaN,0],[Infinity,0],[91,0],[0,-181],['0',0],[null,0]])('rejects %s, %s',(lat,lng)=>expect(validPosition(lat,lng)).toBe(false))
  it('checks snapshot structure and rejects malformed ids',()=>{
    const halcon={halcon_id:1,name:'Halcón',active:false,has_location:false,lat:0,lng:0};expect(validHalcon(halcon)).toBe(true)
    for(const value of [null,{}, {...halcon,halcon_id:0},{...halcon,halcon_id:1.2},{...halcon,name:null},{...halcon,lat:999}])expect(validHalcon(value)).toBe(false)
  })
})
