import {readFileSync} from 'node:fs'
import {dirname, resolve} from 'node:path'
import {fileURLToPath} from 'node:url'
import {JSDOM} from 'jsdom'
import {afterEach,expect,it,vi} from 'vitest'

const windows:JSDOM[]=[]
afterEach(()=>{for(const dom of windows)dom.window.close();windows.length=0})
async function createDOM(html:string,script:string){const dom=new JSDOM(html,{url:'https://halcon.example',runScripts:'outside-only'});windows.push(dom);await new Promise<void>(resolve=>dom.window.document.addEventListener('DOMContentLoaded',()=>resolve(),{once:true}));const fetcher=vi.fn();dom.window.fetch=fetcher;dom.window.eval(readFileSync(resolve(dirname(fileURLToPath(import.meta.url)),'../../public/js',script),'utf8'));dom.window.document.dispatchEvent(new dom.window.Event('DOMContentLoaded'));return {dom,document:dom.window.document,fetcher}}
const formHTML='<form id="recipient-form"><input name="_csrf" value="token"><input id="recipient-email" name="recipient_email" value="ana@example.test"><button id="recipient-save">Guardar</button><button id="recipient-remove" name="recipient_id" value="0" disabled>Retirar</button></form><p id="recipient-feedback"></p><p id="recipient-current"></p>'
it('authentication controls toggle theme and password and update strength and confirmation',async()=>{
  const {dom,document}=await createDOM('<button id="authThemeToggle"></button><input id="password" type="password"><button data-pw-toggle="password"></button><div id="pwStrength"></div><span id="pwStrengthLabel"></span><input id="password_confirm"><span id="pwMatchMsg"></span>','auth.js')
  const password=document.getElementById('password') as HTMLInputElement
  const toggle=document.querySelector('[data-pw-toggle]') as HTMLButtonElement
  toggle.click();expect(password.type).toBe('text');expect(toggle.getAttribute('aria-label')).toBe('Ocultar contraseña')
  toggle.click();expect(password.type).toBe('password');expect(toggle.getAttribute('aria-pressed')).toBe('false')
  password.value='StrongPassword123!';password.dispatchEvent(new dom.window.Event('input'))
  expect(document.getElementById('pwStrength')!.getAttribute('data-score')).toBe('4')
  const confirm=document.getElementById('password_confirm') as HTMLInputElement
  confirm.value='wrong';confirm.dispatchEvent(new dom.window.Event('input'));expect(document.getElementById('pwMatchMsg')!.textContent).toBe('Las contraseñas no coinciden')
  confirm.value=password.value;confirm.dispatchEvent(new dom.window.Event('input'));expect(document.getElementById('pwMatchMsg')!.textContent).toContain('coinciden')
  const initial=document.documentElement.getAttribute('data-theme');(document.getElementById('authThemeToggle') as HTMLButtonElement).click();expect(document.documentElement.getAttribute('data-theme')).not.toBe(initial);expect(dom.window.localStorage.getItem('halcon-theme')).toBe(document.documentElement.getAttribute('data-theme'))
})
it('recipient form prevents duplicate requests, updates CSRF and uses text for unsafe names',async()=>{
  const {dom,document,fetcher}=await createDOM(formHTML,'recipient-form.js');let resolve!:(value:unknown)=>void;fetcher.mockReturnValue(new Promise(r=>{resolve=r}))
  const form=document.querySelector('form')!,save=document.getElementById('recipient-save') as HTMLButtonElement
  form.dispatchEvent(new dom.window.SubmitEvent('submit',{cancelable:true,submitter:save}));form.dispatchEvent(new dom.window.SubmitEvent('submit',{cancelable:true,submitter:save}));expect(fetcher).toHaveBeenCalledOnce();expect(save.disabled).toBe(true)
  resolve({ok:true,status:200,json:async()=>({recipient:{name:'<img onerror=x>',email:'ana@example.test'},csrf_token:'new-token'})})
  await vi.waitFor(()=>expect(document.getElementById('recipient-feedback')!.textContent).toContain('guardado'))
  expect(document.getElementById('recipient-current')!.querySelector('img')).toBeNull();expect((document.querySelector('[name=_csrf]') as HTMLInputElement).value).toBe('new-token');expect(save.disabled).toBe(false)
  fetcher.mockResolvedValue({ok:true,status:200,json:async()=>({recipient:null,csrf_token:'next'})});const remove=document.getElementById('recipient-remove') as HTMLButtonElement;form.dispatchEvent(new dom.window.SubmitEvent('submit',{cancelable:true,submitter:remove}));await vi.waitFor(()=>expect(document.getElementById('recipient-feedback')!.textContent).toContain('Acceso retirado'));expect(fetcher.mock.calls[1][1].body.get('recipient_id')).toBe('0');expect(remove.disabled).toBe(true)
})
it.each([401,403,422,500])('recipient HTTP %d preserves input and restores controls',async(status)=>{
  const {dom,document,fetcher}=await createDOM(formHTML,'recipient-form.js');fetcher.mockResolvedValue({ok:false,status,json:async()=>{if(status===500)throw new SyntaxError('HTML error');return {error:'Correo inválido'}}});const form=document.querySelector('form')!,save=document.getElementById('recipient-save') as HTMLButtonElement;form.dispatchEvent(new dom.window.SubmitEvent('submit',{cancelable:true,submitter:save}));await vi.waitFor(()=>expect(save.disabled).toBe(false));expect((document.getElementById('recipient-email') as HTMLInputElement).value).toBe('ana@example.test');expect(document.getElementById('recipient-feedback')!.textContent).not.toContain('HTML error')
})
it('user search finds later users, preserves selected value and ignores stale responses',async()=>{
  const {dom,document,fetcher}=await createDOM('<select name="user_id"><option value="">Elige</option><option value="1" selected>Ana</option></select>','user-selector.js');const input=document.querySelector('input[type=search]') as HTMLInputElement;let oldResolve!:(value:unknown)=>void
  fetcher.mockReturnValueOnce(new Promise(r=>{oldResolve=r})).mockResolvedValueOnce({ok:true,redirected:false,json:async()=>({users:[{id:1000,name:'Último usuario',email:'last@example.test'}],total:1})})
  input.value='old';input.dispatchEvent(new dom.window.Event('input'));await vi.waitFor(()=>expect(fetcher).toHaveBeenCalledOnce())
  input.value='last@example.test';input.dispatchEvent(new dom.window.Event('input'));await vi.waitFor(()=>expect(document.querySelector('option[value="1000"]')).not.toBeNull())
  oldResolve({ok:true,redirected:false,json:async()=>({users:[],total:0})});await new Promise(r=>setTimeout(r,20));expect(document.querySelector('option[value="1000"]')).not.toBeNull();expect((document.querySelector('select') as HTMLSelectElement).value).toBe('1');expect(document.querySelector('p')!.textContent).toContain('1 usuarios')
})
