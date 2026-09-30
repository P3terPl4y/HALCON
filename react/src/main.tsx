import {StrictMode,useCallback,useEffect,useState} from 'react'
import {createRoot} from 'react-dom/client'
import {APIError,request,type Session,type Tracking,type User} from './api'
import {useGPS,useTracking} from './useTracking'
import Map from './Map'
import './style.css'

function Login({session,onLogin}:{session:Session;onLogin:(s:Session)=>void}) {
  const [busy,setBusy]=useState(false),[error,setError]=useState('')
  async function submit(event:React.FormEvent<HTMLFormElement>){
    event.preventDefault();if(busy)return;setBusy(true);setError('')
    const data=new FormData(event.currentTarget)
    try{onLogin(await request<Session>('/api/auth/login',{email:String(data.get('email')),password:String(data.get('password'))},session.csrf_token))}
    catch(e){setError(e instanceof Error?e.message:'No se pudo iniciar sesión.');try{onLogin(await request<Session>('/api/session'))}catch{/* Keep the error and allow retry. */}}
    finally{setBusy(false)}
  }
  return <main className="login"><div className="login-intro"><span className="eyebrow">HALCON</span><h1>Tu recorrido.<br/>Tu tranquilidad.</h1><p>Comparte tu ubicación con quien elijas y sigue tus halcones en tiempo real.</p></div><form className="card" onSubmit={submit} aria-busy={busy}><h2>Inicia sesión</h2><p>Usa tu cuenta de HALCON.</p><label htmlFor="email">Correo electrónico</label><input id="email" name="email" type="email" autoComplete="username" required/><label htmlFor="password">Contraseña</label><input id="password" name="password" type="password" autoComplete="current-password" required/><p role="alert" className="feedback">{error}</p><button disabled={busy}>{busy?'Entrando…':'Iniciar sesión'}</button><a href={`${import.meta.env.VITE_BACKEND_PUBLIC_URL||'http://localhost:3300'}/register`}>Crear una cuenta</a></form></main>
}
function Dashboard({user,token,onExpired,onLogout}:{user:User;token:string;onExpired:()=>void;onLogout:()=>void}){
  const [tracking,setTracking]=useState<Tracking|null>(null),[email,setEmail]=useState(''),[busy,setBusy]=useState(false),[feedback,setFeedback]=useState(''),[selected,setSelected]=useState<number|null>(null),[loadingError,setLoadingError]=useState('')
  const {halcones,connection}=useTracking(user.id,onExpired),gps=useGPS()
  const load=useCallback(async()=>{try{const data=await request<Tracking>('/api/tracking');setTracking(data);setEmail(data.recipient?.email||'');setLoadingError('')}catch(e){if(e instanceof APIError&&e.status===401)onExpired();else setLoadingError('No pudimos cargar tu cuenta. Reintenta cuando tengas conexión.')}},[onExpired])
  useEffect(()=>{void load()},[load])
  async function save(remove=false){
    if(busy||!tracking)return;setBusy(true);setFeedback(remove?'Retirando acceso…':'Guardando…')
    try{const data=await request<Tracking>('/api/tracking/recipient',remove?{recipient_id:'0'}:{recipient_email:email},tracking.csrf_token);setTracking(data);setEmail(data.recipient?.email||'');setFeedback(remove?'Acceso retirado.':'Destinatario guardado. Puedes activar el GPS.')}
    catch(e){if(e instanceof APIError&&e.status===401)onExpired();else setFeedback(e instanceof Error?e.message:'No se pudo guardar.')}
    finally{setBusy(false)}
  }
  async function logout(){if(busy)return;setBusy(true);try{await request('/api/auth/logout',{},tracking?.csrf_token||token);gps.stop();onLogout()}catch(e){setFeedback(e instanceof Error?e.message:'No se pudo cerrar sesión.');setBusy(false)}}
  const backend=import.meta.env.VITE_BACKEND_PUBLIC_URL||'http://localhost:3300'
  return <><header className="topbar"><a href="/" className="brand">HALCON<span>Ubicación en tiempo real</span></a><nav aria-label="Tu cuenta">{user.role!=='user'&&<a href={`${backend}/${user.role==='admin'?'admin/users':'moderator/halcones'}`}>Administrar</a>}<span className="account-name">{user.name}</span><button className="secondary" onClick={logout} disabled={busy}>Cerrar sesión</button></nav></header><main className="dashboard" id="main"><div className="page-heading"><div><span className="eyebrow">TU UBICACIÓN, BAJO TU CONTROL</span><h1>Comparte tu recorrido</h1><p>Elige quién puede verte. Tú decides cuándo activar el GPS.</p></div><span className="badge" role="status">{connection}</span></div>{loadingError?<section className="card"><p role="alert">{loadingError}</p><button onClick={load}>Reintentar</button></section>:<section className="card control-grid" aria-label="Controles de ubicación"><form onSubmit={e=>{e.preventDefault();void save()}} aria-busy={busy}><label htmlFor="recipient">¿Con quién quieres compartir?</label><div className="field-row"><input id="recipient" type="email" autoComplete="email" required value={email} onChange={e=>setEmail(e.target.value)} placeholder="persona@correo.com" aria-describedby="recipient-help"/><button disabled={busy||!tracking}>Guardar</button></div><p className="help" id="recipient-help">Usa el correo de una cuenta activa de HALCON.</p><p className="current">{tracking?.recipient?`Compartiendo con ${tracking.recipient.name}`:'Tu ubicación aún no se comparte.'}</p><button className="text-button" type="button" onClick={()=>save(true)} disabled={busy||!tracking?.recipient}>Retirar acceso</button><p className="feedback" role="status">{feedback}</p></form><div><h2>Transmisión GPS</h2><p>Tu última posición se conserva hasta que retires el acceso.</p><div className="actions"><button onClick={gps.start} disabled={gps.enabled||!tracking}>Activar ubicación</button><button className="secondary" onClick={gps.stop} disabled={!gps.enabled}>Detener transmisión</button></div><p role="status" className="current">{gps.status}</p><p className="help">Necesitas permiso de ubicación y mantener esta página abierta.</p></div></section>}<section className="map-card" aria-label="Seguimiento de halcones"><div className="map-heading"><h2>Ubicaciones disponibles <span className="badge">{halcones.length}</span></h2><span className="help">{halcones.filter(h=>h.active).length} en vivo</span></div><div className="map-grid"><Map halcones={halcones} selected={selected}/><aside aria-label="Lista de halcones">{halcones.length===0?<p className="empty">Aún no hay ubicaciones recibidas. Las verás aquí cuando alguien comparta contigo.</p>:halcones.map(h=><button className={`halcon-row ${selected===h.halcon_id?'selected':''}`} key={h.halcon_id} onClick={()=>setSelected(h.halcon_id)} aria-pressed={selected===h.halcon_id}><span>{h.name}<small>#{h.halcon_id} · {h.has_location?`${h.lat.toFixed(5)}, ${h.lng.toFixed(5)}`:'Esperando primera posición'}</small></span><span className="badge">{h.active?'En vivo':'En espera'}</span></button>)}</aside></div></section></main></>
}
function App(){
  const [session,setSession]=useState<Session|null>(null),[error,setError]=useState('')
  const load=useCallback(async()=>{try{setSession(await request<Session>('/api/session'));setError('')}catch{setError('No pudimos conectar con HALCON. Comprueba que el servidor esté activo.')}},[])
  useEffect(()=>{void load()},[load])
  const expired=useCallback(()=>{setSession(null);void load()},[load])
  if(!session)return <main className="loading" role="status"><h1>HALCON</h1><p>{error||'Conectando con tu cuenta…'}</p>{error&&<button onClick={load}>Reintentar</button>}</main>
  return session.user?<Dashboard user={session.user} token={session.csrf_token} onExpired={expired} onLogout={expired}/>:<Login session={session} onLogin={setSession}/>
}
createRoot(document.getElementById('root')!).render(<StrictMode><App/></StrictMode>)
