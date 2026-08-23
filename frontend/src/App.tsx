import { useState, useEffect, useCallback } from 'react'
import {
  BrowserRouter, Routes, Route, Navigate,
  useNavigate, useParams, useSearchParams,
} from 'react-router-dom'

// Prefijo del backend. Lo resuelve el proxy: vite.config.ts en desarrollo
// y frontend/nginx.conf en el contenedor. Nunca se llama al puerto 8080
// directamente, así se evita CORS.
const API = '/api'

// La sesión se guarda para que una URL como /jobs/:id siga funcionando
// tras recargar. Sin esto la vista de detalle no sería direccionable:
// pegar el enlace devolvería al login.
const SESSION_KEY = 'okf.session'
type Session = { user: User; token: string }

function loadSession(): Session | null {
  try {
    const raw = localStorage.getItem(SESSION_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    return parsed?.token && parsed?.user ? parsed as Session : null
  } catch { return null }   // navegación privada, almacenamiento bloqueado
}

function saveSession(session: Session | null) {
  try {
    if (session) localStorage.setItem(SESSION_KEY, JSON.stringify(session))
    else localStorage.removeItem(SESSION_KEY)
  } catch { /* la sesión dura lo que la pestaña */ }
}

// El backend acepta .md y .txt. Declarar el tipo aquí evita depender de
// lo que el sistema operativo tenga registrado para cada extensión.
function mimeForFile(filename: string) {
  return /\.(md|markdown)$/i.test(filename) ? 'text/markdown' : 'text/plain'
}

async function downloadBundle(url: string, token: string, filename = 'bundle.zip') {
  const res = await fetch(`${API}${url}`, { headers: { Authorization: `Bearer ${token}` } })
  if (!res.ok) { alert('Error al descargar el bundle'); return }
  const blob = await res.blob()
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = filename
  a.click()
  URL.revokeObjectURL(a.href)
}

type User = { id: string; email: string; nombre?: string; apellido?: string; created_at?: string }
type JobStatus = 'queued'|'processing'|'completed'|'failed'

// Clasificación en tres niveles que produce el backend. `valid_with_warnings`
// sigue siendo un éxito descargable: las advertencias se muestran, no
// bloquean.
type ValidationStatus = 'valid'|'valid_with_warnings'|'invalid'
type Validation = { status: ValidationStatus; warnings: string[]; errors: string[] }

// download_url solo llega cuando el bundle es realmente descargable, así
// que su ausencia es la señal de que no hay nada que ofrecer.
type Bundle = { id: string; concept_count: number; is_valid: boolean; validation?: Validation; download_url?: string }
type DocumentSummary = { id: string; filename: string; format: string }
// Entrada del listado, con la forma exacta que devuelve GET /jobs. El
// nombre del documento viaja anidado en `document`, no en la raíz.
type Job = {
  id: string
  status: JobStatus
  terminal: boolean
  error_message?: string
  document: DocumentSummary
  bundle?: Bundle | null
  created_at: string
  updated_at: string
}
type JobDetail = {
  id: string
  status: JobStatus
  terminal: boolean
  error_message?: string
  created_at: string
  updated_at: string
  document: DocumentSummary
  bundle?: Bundle | null
}
// Lo que devuelve GET /stats: el recuento lo hace el servidor sobre todos
// los trabajos del usuario, no sobre la página que el cliente tenga cargada.
type JobStats = { queued: number; processing: number; completed: number; failed: number; total: number }
type UploadPhase = 'idle'|'selected'|'uploading'|'processing'|'done'|'failed'
type DashSection = 'upload'|'documentos'|'perfil'

const STATUS_INFO: Record<JobStatus,{label:string;bg:string;color:string;border:string}> = {
  queued:     {label:'⏳ En cola',    bg:'#fffbe8',color:'#92700a',border:'#f5de80'},
  processing: {label:'⚙️ Procesando', bg:'#fff4e8',color:'#a0500a',border:'#f5c080'},
  completed:  {label:'✅ Completado', bg:'#f0faf0',color:'#2e7d32',border:'#a8d88a'},
  failed:     {label:'❌ Fallido',    bg:'#fff0f0',color:'#c0392b',border:'#f5b8b8'},
}

function Background() {
  return (
    <div style={{position:'fixed',inset:0,zIndex:0,overflow:'hidden',pointerEvents:'none'}}>
      <style>{`
        @keyframes cDrift  {0%{transform:translateX(0)}100%{transform:translateX(40px)}}
        @keyframes cDrift2 {0%{transform:translateX(0)}100%{transform:translateX(-30px)}}
        @keyframes sway    {0%,100%{transform:rotate(-2deg)}50%{transform:rotate(2deg)}}
        @keyframes sway2   {0%,100%{transform:rotate(1.5deg)}50%{transform:rotate(-1.5deg)}}
        @keyframes bobble  {0%,100%{transform:translateY(0)}50%{transform:translateY(-6px)}}
      `}</style>
      <svg viewBox="0 0 1200 700" xmlns="http://www.w3.org/2000/svg"
        style={{width:'100%',height:'100%',position:'absolute',top:0,left:0}} preserveAspectRatio="xMidYMid slice">
        <defs>
          <linearGradient id="sky" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#fff8f0"/><stop offset="55%" stopColor="#fde8c8"/><stop offset="100%" stopColor="#f5c9a0"/>
          </linearGradient>
          <linearGradient id="h2" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#c5e8b0"/><stop offset="100%" stopColor="#8cca7a"/>
          </linearGradient>
          <linearGradient id="h1" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#a8d5a2"/><stop offset="100%" stopColor="#6db56d"/>
          </linearGradient>
          <radialGradient id="sun" cx="50%" cy="50%" r="50%">
            <stop offset="0%" stopColor="#ffe28a"/><stop offset="60%" stopColor="#ffcc55" stopOpacity=".6"/><stop offset="100%" stopColor="#ffcc55" stopOpacity="0"/>
          </radialGradient>
          <filter id="sb"><feGaussianBlur stdDeviation="6"/></filter>
        </defs>
        <rect width="1200" height="700" fill="url(#sky)"/>
        <circle cx="920" cy="110" r="80" fill="url(#sun)" filter="url(#sb)"/>
        <circle cx="920" cy="110" r="36" fill="#fff5c0" opacity=".9"/>
        <circle cx="920" cy="110" r="24" fill="#fffde0"/>
        <g style={{animation:'cDrift 18s ease-in-out infinite alternate'}}>
          <ellipse cx="180" cy="130" rx="90" ry="38" fill="white" opacity=".92"/>
          <ellipse cx="145" cy="148" rx="55" ry="30" fill="white" opacity=".85"/>
          <ellipse cx="230" cy="142" rx="65" ry="26" fill="white" opacity=".82"/>
          <ellipse cx="180" cy="118" rx="52" ry="28" fill="white" opacity=".95"/>
        </g>
        <g style={{animation:'cDrift2 22s ease-in-out infinite alternate'}}>
          <ellipse cx="560" cy="88" rx="110" ry="44" fill="white" opacity=".88"/>
          <ellipse cx="510" cy="108" rx="68" ry="34" fill="white" opacity=".82"/>
          <ellipse cx="625" cy="100" rx="75" ry="30" fill="white" opacity=".8"/>
          <ellipse cx="565" cy="75" rx="60" ry="30" fill="white" opacity=".92"/>
        </g>
        <g style={{animation:'cDrift 14s 3s ease-in-out infinite alternate'}}>
          <ellipse cx="1050" cy="150" rx="95" ry="40" fill="white" opacity=".85"/>
          <ellipse cx="1005" cy="165" rx="60" ry="30" fill="white" opacity=".8"/>
          <ellipse cx="1110" cy="158" rx="70" ry="28" fill="white" opacity=".78"/>
        </g>
        <path d="M0,420 Q200,320 400,370 Q600,310 800,360 Q1000,310 1200,350 L1200,700 L0,700 Z" fill="#b8dba6" opacity=".6"/>
        <path d="M0,480 Q150,400 350,440 Q550,380 750,430 Q950,380 1200,420 L1200,700 L0,700 Z" fill="url(#h2)"/>
        <path d="M0,560 Q180,490 380,530 Q580,470 780,510 Q980,470 1200,510 L1200,700 L0,700 Z" fill="url(#h1)"/>
        <rect x="0" y="620" width="1200" height="80" fill="#7cc56f"/>
        <g style={{animation:'sway 6s ease-in-out infinite',transformOrigin:'120px 560px'}}>
          <rect x="115" y="520" width="10" height="80" fill="#7a5c3a" rx="3"/>
          <ellipse cx="120" cy="490" rx="52" ry="48" fill="#5aab52"/>
          <ellipse cx="120" cy="478" rx="44" ry="38" fill="#6ec465"/>
          <ellipse cx="108" cy="495" rx="30" ry="26" fill="#5aab52"/>
          <ellipse cx="136" cy="488" rx="28" ry="24" fill="#76cc6e"/>
        </g>
        <g style={{animation:'sway2 7s ease-in-out infinite',transformOrigin:'70px 580px'}}>
          <rect x="65" y="545" width="9" height="65" fill="#7a5c3a" rx="3"/>
          <ellipse cx="70" cy="515" rx="42" ry="40" fill="#4e9e46"/>
          <ellipse cx="70" cy="503" rx="36" ry="30" fill="#5fba57"/>
        </g>
        <g style={{animation:'sway 8s 1s ease-in-out infinite',transformOrigin:'1080px 550px'}}>
          <rect x="1075" y="510" width="10" height="90" fill="#7a5c3a" rx="3"/>
          <ellipse cx="1080" cy="478" rx="58" ry="52" fill="#5aab52"/>
          <ellipse cx="1080" cy="464" rx="48" ry="40" fill="#6ec465"/>
          <ellipse cx="1065" cy="482" rx="34" ry="28" fill="#5aab52"/>
          <ellipse cx="1098" cy="472" rx="32" ry="26" fill="#76cc6e"/>
        </g>
        <g style={{animation:'sway2 5s 2s ease-in-out infinite',transformOrigin:'1140px 570px'}}>
          <rect x="1135" y="535" width="9" height="70" fill="#7a5c3a" rx="3"/>
          <ellipse cx="1140" cy="505" rx="46" ry="42" fill="#4e9e46"/>
          <ellipse cx="1140" cy="492" rx="38" ry="32" fill="#5fba57"/>
        </g>
        {[200,340,460,700,850].map((x,i)=>(
          <g key={i}><ellipse cx={x} cy={625+i%2*5} rx={22+i*3} ry={14+i*2} fill="#5aab52" opacity=".7"/>
          <ellipse cx={x-8} cy={618+i%2*5} rx={14} ry={10} fill="#6ec465" opacity=".8"/></g>
        ))}
        {[[160,640,'#ff9eb5'],[240,635,'#ffde8a'],[330,645,'#ff9eb5'],[420,638,'#c8f0a0'],[510,642,'#ffde8a'],[600,636,'#ff9eb5'],[690,644,'#ffde8a'],[780,638,'#c8f0a0'],[870,642,'#ff9eb5'],[960,637,'#ffde8a']].map(([x,y,c],i)=>(
          <g key={i}><line x1={x as number} y1={y as number} x2={x as number} y2={(y as number)+16} stroke="#6aac3e" strokeWidth="1.5"/>
          <circle cx={x as number} cy={(y as number)-2} r="5" fill={c as string} opacity=".9"/>
          <circle cx={(x as number)-4} cy={(y as number)+1} r="3" fill={c as string} opacity=".6"/>
          <circle cx={(x as number)+4} cy={(y as number)+1} r="3" fill={c as string} opacity=".6"/></g>
        ))}
        <g style={{animation:'bobble 8s ease-in-out infinite'}}>
          <rect x="530" y="540" width="65" height="50" fill="#f5e6d0" rx="2"/>
          <polygon points="520,542 605,542 562,500" fill="#c46a4a"/>
          <rect x="555" y="558" width="16" height="32" fill="#9b6a3e" rx="2"/>
          <rect x="534" y="548" width="16" height="14" fill="#b8d8f0" rx="1"/>
          <rect x="573" y="548" width="16" height="14" fill="#b8d8f0" rx="1"/>
          <path d="M575,500 Q580,486 572,474 Q568,462 576,450" fill="none" stroke="white" strokeWidth="3" strokeLinecap="round" opacity=".6"/>
        </g>
      </svg>
    </div>
  )
}

function Landing({onGoAuth}:{onGoAuth:(m:'login'|'register')=>void}) {
  return (
    <div style={{position:'relative',zIndex:1,minHeight:'100vh',display:'flex',flexDirection:'column',alignItems:'center',justifyContent:'center',padding:'40px 20px',textAlign:'center'}}>
      <style>{`
        @keyframes fadeUp{from{opacity:0;transform:translateY(18px)}to{opacity:1;transform:translateY(0)}}
        .f1{animation:fadeUp .6s ease both}.f2{animation:fadeUp .6s .1s ease both}.f3{animation:fadeUp .6s .2s ease both}.f4{animation:fadeUp .6s .3s ease both}
        .btn-g:hover{background:#5a8a30!important;transform:translateY(-2px)!important}
        .btn-o:hover{background:rgba(255,255,255,.95)!important;transform:translateY(-2px)!important}
        .pill:hover{transform:translateY(-3px)!important}
      `}</style>
      <div style={{background:'rgba(255,252,245,0.78)',backdropFilter:'blur(18px)',borderRadius:'24px',padding:'40px 36px',maxWidth:'520px',width:'100%',boxShadow:'0 8px 40px rgba(80,60,30,.14)',border:'1.5px solid rgba(255,255,255,.85)'}}>
        <div className="f1" style={{display:'inline-flex',alignItems:'center',gap:'8px',background:'#f0f9e4',border:'1.5px solid #c6e89a',borderRadius:'20px',padding:'4px 14px',fontSize:'0.74rem',color:'#5a8a30',fontWeight:600,marginBottom:'18px'}}>
          <span style={{width:6,height:6,borderRadius:'50%',background:'#7bc444',display:'inline-block'}}/>
          ISIS4426 · Cloud Computing
        </div>
        <h1 className="f2" style={{fontSize:'clamp(1.7rem,4.5vw,2.6rem)',fontWeight:800,lineHeight:1.2,color:'#2d3a1e',marginBottom:'12px',letterSpacing:'-0.02em'}}>
          Documentos que se convierten en{' '}
          <span style={{color:'#6aac3e'}}>conocimiento</span>
        </h1>
        <p className="f3" style={{fontSize:'0.9rem',color:'#5a6645',lineHeight:1.72,marginBottom:'26px'}}>
          Sube tu documento <strong style={{color:'#3d5220'}}>Markdown (.md) o texto plano (.txt)</strong> y obtén un bundle OKF — conceptos extraídos, validados y listos para descargar. Procesamiento asíncrono en la nube.
        </p>
        <div className="f4" style={{display:'flex',gap:'10px',flexWrap:'wrap',justifyContent:'center',marginBottom:'24px'}}>
          <button className="btn-g" onClick={()=>onGoAuth('register')} style={{padding:'12px 28px',background:'#6aac3e',color:'white',border:'none',borderRadius:'12px',fontSize:'0.95rem',fontWeight:700,cursor:'pointer',transition:'all .2s',boxShadow:'0 4px 14px rgba(106,172,62,.35)'}}>
            Crear cuenta →
          </button>
          <button className="btn-o" onClick={()=>onGoAuth('login')} style={{padding:'12px 24px',background:'rgba(255,255,255,.7)',color:'#3d5220',border:'1.5px solid #b8d98a',borderRadius:'12px',fontSize:'0.95rem',fontWeight:600,cursor:'pointer',transition:'all .2s'}}>
            Ya tengo cuenta
          </button>
        </div>
        <div style={{display:'flex',gap:'8px',flexWrap:'wrap',justifyContent:'center'}}>
          {[['📄','Markdown y texto'],['⚡','Asíncrono'],['📦','Bundle OKF'],['🔒','Privado']].map(([ic,lb])=>(
            <div key={lb} className="pill" style={{display:'flex',alignItems:'center',gap:'6px',background:'white',border:'1.5px solid #e8f2d8',borderRadius:'10px',padding:'5px 11px',fontSize:'0.74rem',color:'#4a6030',fontWeight:500,transition:'all .2s',boxShadow:'0 2px 6px rgba(0,0,0,.04)'}}>
              {ic} {lb}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function AuthView({initialMode,onLogin,onBack}:{initialMode:'login'|'register';onLogin:(u:User,t:string)=>void;onBack:()=>void}) {
  const [mode,setMode]=useState<'login'|'register'>(initialMode)
  const [nombre,setNombre]=useState('')
  const [apellido,setApellido]=useState('')
  const [email,setEmail]=useState('')
  const [password,setPassword]=useState('')
  const [error,setError]=useState('')
  const [success,setSuccess]=useState('')
  const [loading,setLoading]=useState(false)

  const switchMode=(m:'login'|'register')=>{setMode(m);setError('');setSuccess('');setNombre('');setApellido('');setEmail('');setPassword('')}

  const submit=async()=>{
    setError('');setSuccess('');setLoading(true)
    try {
      const body:any={email,password}
      if(mode==='register'){body.nombre=nombre;body.apellido=apellido}
      const res=await fetch(`${API}/auth/${mode}`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})
      const text=await res.text()
      let data:any
      try{data=JSON.parse(text)}catch{data=text}
      if(!res.ok) throw new Error(typeof data==='string'?data:'Error al autenticar')
      if(mode==='register'){setSuccess('¡Cuenta creada! Inicia sesión para continuar.');switchMode('login');return}
      onLogin({...data.user},data.token)
    } catch(e:any){setError(e.message)}
    finally{setLoading(false)}
  }

  const inp:React.CSSProperties={width:'100%',padding:'10px 13px',background:'white',border:'1.5px solid #d4e8b8',borderRadius:'10px',color:'#2d3a1e',fontSize:'0.88rem',boxSizing:'border-box',outline:'none',transition:'border-color .2s',fontFamily:'inherit'}

  return (
    <div style={{position:'relative',zIndex:1,minHeight:'100vh',display:'flex',alignItems:'center',justifyContent:'center',padding:'20px'}}>
      <style>{`.ainp:focus{border-color:#6aac3e!important} @keyframes pop{from{opacity:0;transform:scale(.96)}to{opacity:1;transform:scale(1)}} .tab-active{background:#6aac3e!important;color:white!important}`}</style>
      <div style={{background:'rgba(255,252,245,0.9)',backdropFilter:'blur(20px)',border:'1.5px solid rgba(255,255,255,.9)',borderRadius:'24px',padding:'38px 34px',width:'100%',maxWidth:'400px',animation:'pop .3s ease',boxShadow:'0 12px 48px rgba(80,60,30,.15)'}}>
        <button onClick={onBack} style={{background:'none',border:'none',color:'#8aaa60',cursor:'pointer',fontSize:'0.8rem',marginBottom:'18px',display:'flex',alignItems:'center',gap:'5px',padding:0,fontWeight:600}}>← Volver</button>
        <div style={{display:'flex',background:'#f0f5e8',borderRadius:'12px',padding:'4px',marginBottom:'24px',gap:'4px'}}>
          {(['login','register'] as const).map(m=>(
            <button key={m} className={mode===m?'tab-active':''} onClick={()=>switchMode(m)}
              style={{flex:1,padding:'9px',border:'none',borderRadius:'9px',cursor:'pointer',fontSize:'0.83rem',fontWeight:700,background:'transparent',color:'#7a9a50',transition:'all .2s'}}>
              {m==='login'?'Iniciar sesión':'Crear cuenta'}
            </button>
          ))}
        </div>
        <div style={{textAlign:'center',marginBottom:'22px'}}>
          <div style={{fontSize:'1.9rem',marginBottom:'5px'}}>{mode==='login'?'🌿':'🌱'}</div>
          <div style={{fontSize:'1.1rem',fontWeight:800,color:'#2d3a1e'}}>{mode==='login'?'Bienvenido de vuelta':'Únete a la plataforma'}</div>
          <div style={{fontSize:'0.78rem',color:'#8aaa60',marginTop:'3px'}}>{mode==='login'?'Accede a tus documentos y bundles':'Crea tu cuenta y empieza a convertir'}</div>
        </div>
        {error   && <div style={{background:'#fff0f0',border:'1px solid #f5b8b8',color:'#c0392b',borderRadius:'8px',padding:'9px 12px',fontSize:'0.78rem',marginBottom:'12px'}}>{error}</div>}
        {success && <div style={{background:'#f0faf0',border:'1px solid #a8d88a',color:'#2e7d32',borderRadius:'8px',padding:'9px 12px',fontSize:'0.78rem',marginBottom:'12px'}}>✓ {success}</div>}
        {mode==='register'&&(
          <div style={{display:'grid',gridTemplateColumns:'1fr 1fr',gap:'10px',marginBottom:'10px'}}>
            <div>
              <label style={{display:'block',fontSize:'0.72rem',fontWeight:700,color:'#5a6645',marginBottom:'5px'}}>Nombre</label>
              <input className="ainp" style={inp} placeholder="María" value={nombre} onChange={e=>setNombre(e.target.value)}/>
            </div>
            <div>
              <label style={{display:'block',fontSize:'0.72rem',fontWeight:700,color:'#5a6645',marginBottom:'5px'}}>Apellido</label>
              <input className="ainp" style={inp} placeholder="García" value={apellido} onChange={e=>setApellido(e.target.value)}/>
            </div>
          </div>
        )}
        <div style={{marginBottom:'10px'}}>
          <label style={{display:'block',fontSize:'0.72rem',fontWeight:700,color:'#5a6645',marginBottom:'5px'}}>Correo electrónico</label>
          <input className="ainp" style={inp} type="email" placeholder="tu@email.com" value={email} onChange={e=>setEmail(e.target.value)} onKeyDown={e=>e.key==='Enter'&&submit()}/>
        </div>
        <div style={{marginBottom:'20px'}}>
          <label style={{display:'block',fontSize:'0.72rem',fontWeight:700,color:'#5a6645',marginBottom:'5px'}}>Contraseña</label>
          <input className="ainp" style={inp} type="password" placeholder="••••••••" value={password} onChange={e=>setPassword(e.target.value)} onKeyDown={e=>e.key==='Enter'&&submit()}/>
        </div>
        <button onClick={submit} disabled={loading} style={{width:'100%',padding:'12px',background:'#6aac3e',color:'white',border:'none',borderRadius:'12px',fontSize:'0.92rem',fontWeight:700,cursor:'pointer',boxShadow:'0 4px 14px rgba(106,172,62,.35)',opacity:loading?.7:1}}>
          {loading?'Cargando...':(mode==='login'?'Iniciar sesión':'Crear mi cuenta')}
        </button>
      </div>
    </div>
  )
}

function Sidebar({section,setSection,user,onLogout}:{section:DashSection;setSection:(s:DashSection)=>void;user:User;onLogout:()=>void}) {
  const items:[DashSection,string,string][]=[['upload','📤','Subir documento'],['documentos','📂','Mis documentos'],['perfil','👤','Mi perfil']]
  const initials=`${user.nombre?.[0]??''}${user.apellido?.[0]??''}`.toUpperCase()||user.email[0].toUpperCase()
  return (
    <aside style={{width:218,minHeight:'100vh',background:'rgba(255,252,245,0.92)',backdropFilter:'blur(16px)',borderRight:'1.5px solid rgba(200,230,160,.5)',display:'flex',flexDirection:'column',padding:'24px 0',flexShrink:0}}>
      <div style={{padding:'0 20px',marginBottom:'30px'}}>
        <div style={{fontSize:'1rem',fontWeight:800,color:'#3d5220'}}>🌿 OKF Platform</div>
        <div style={{fontSize:'0.64rem',color:'#8aaa60',marginTop:'2px'}}>Conversión documental</div>
      </div>
      <nav style={{flex:1,padding:'0 12px'}}>
        {items.map(([s,ic,lb])=>(
          <button key={s} onClick={()=>setSection(s)} style={{width:'100%',display:'flex',alignItems:'center',gap:'10px',padding:'10px 12px',border:'none',borderRadius:'10px',cursor:'pointer',fontSize:'0.84rem',fontWeight:section===s?700:500,background:section===s?'#e8f7d8':'transparent',color:section===s?'#3d5220':'#6a7a50',marginBottom:'3px',transition:'all .18s',textAlign:'left'}}>
            <span style={{fontSize:'1.05rem'}}>{ic}</span>{lb}
          </button>
        ))}
      </nav>
      <div style={{padding:'0 12px',borderTop:'1px solid #e0eccc',paddingTop:'14px',marginTop:'8px'}}>
        <div style={{display:'flex',alignItems:'center',gap:'9px',padding:'9px 11px',borderRadius:'10px',background:'#f5faea',marginBottom:'8px'}}>
          <div style={{width:32,height:32,borderRadius:'50%',background:'linear-gradient(135deg,#8bc34a,#6aac3e)',display:'flex',alignItems:'center',justifyContent:'center',color:'white',fontWeight:800,fontSize:'0.82rem',flexShrink:0}}>{initials}</div>
          <div style={{minWidth:0}}>
            <div style={{fontSize:'0.76rem',fontWeight:700,color:'#3d5220',overflow:'hidden',textOverflow:'ellipsis',whiteSpace:'nowrap'}}>{user.nombre||user.email.split('@')[0]}</div>
            <div style={{fontSize:'0.63rem',color:'#8aaa60',overflow:'hidden',textOverflow:'ellipsis',whiteSpace:'nowrap'}}>{user.email}</div>
          </div>
        </div>
        <button onClick={onLogout} style={{width:'100%',padding:'7px',background:'transparent',color:'#8aaa60',border:'1px solid #d4e8b0',borderRadius:'8px',fontSize:'0.76rem',cursor:'pointer'}}>
          Cerrar sesión
        </button>
      </div>
    </aside>
  )
}

function UploadSection({token,onJobCreated}:{token:string;onJobCreated:(j:Job)=>void}) {
  const navigate=useNavigate()
  const [phase,setPhase]=useState<UploadPhase>('idle')
  const [file,setFile]=useState<File|null>(null)
  const [drag,setDrag]=useState(false)
  const [activeJob,setActiveJob]=useState<JobDetail|null>(null)
  const [error,setError]=useState('')

  const pollJob=useCallback(async(id:string)=>{
    const res=await fetch(`${API}/jobs/${id}`,{headers:{Authorization:`Bearer ${token}`}})
    if(!res.ok) return null
    return res.json() as Promise<JobDetail>
  },[token])

  // El seguimiento para cuando el backend marca el trabajo como terminal,
  // no cuando el cliente reconoce un estado de una lista suya.
  useEffect(()=>{
    if(!activeJob||activeJob.terminal) return
    const id=setInterval(async()=>{
      const updated=await pollJob(activeJob.id)
      if(!updated) return
      setActiveJob(updated)
      if(updated.terminal){
        clearInterval(id)
        setPhase(updated.status==='completed'?'done':'failed')
        onJobCreated(updated)
      }
    },2500)
    return ()=>clearInterval(id)
  },[activeJob,pollJob,onJobCreated])

  const handleFile=(f:File)=>{
    if(!f.name.match(/\.(txt|md|markdown)$/i)){setError('Solo .txt o .md');return}
    setFile(f);setError('');setPhase('selected')
  }

  const submit=async()=>{
    if(!file) return
    setPhase('uploading');setError('')
    // El navegador deduce el tipo del registro del sistema y para .md
    // suele dejarlo vacío, lo que viaja como application/octet-stream.
    // Se declara a partir de la extensión para no depender de eso.
    const fd=new FormData()
    fd.append('file',new Blob([file],{type:mimeForFile(file.name)}),file.name)
    try {
      const res=await fetch(`${API}/documents`,{method:'POST',headers:{Authorization:`Bearer ${token}`},body:fd})
      const text2=await res.text()
      let data:any
      try{data=JSON.parse(text2)}catch{data=text2}
      if(!res.ok) throw new Error(typeof data==='string'?data:'Error al subir')
      // La respuesta de carga ya trae el documento creado, así que la
      // fila aparece con su nombre real desde el primer instante.
      const job:JobDetail={
        id:data.jobId,
        status:data.status,
        terminal:false,
        document:{id:data.document?.id??'',filename:data.document?.filename??file.name,format:data.document?.format??''},
        created_at:new Date().toISOString(),
        updated_at:new Date().toISOString(),
      }
      onJobCreated(job)
      setActiveJob(job);setPhase('processing')
    } catch(e:any){setError(e.message);setPhase('selected')}
  }

  const reset=()=>{setPhase('idle');setFile(null);setActiveJob(null);setError('')}
  const card:React.CSSProperties={background:'rgba(255,252,245,0.88)',backdropFilter:'blur(12px)',border:'1.5px solid rgba(200,230,160,.55)',borderRadius:'20px',padding:'28px',boxShadow:'0 4px 24px rgba(80,60,30,.08)'}

  if(phase==='idle'||phase==='selected') return (
    <div>
      <style>{`.dl:hover{border-color:#6aac3e!important;background:#f5fdf0!important} @keyframes spin2{to{transform:rotate(360deg)}}`}</style>
      <div style={{fontSize:'0.7rem',fontWeight:700,color:'#6aac3e',letterSpacing:'0.1em',textTransform:'uppercase',marginBottom:'14px'}}>Subir documento</div>
      <div style={card}>
        <div onDragOver={e=>{e.preventDefault();setDrag(true)}} onDragLeave={()=>setDrag(false)}
          onDrop={e=>{e.preventDefault();setDrag(false);const f=e.dataTransfer.files[0];if(f)handleFile(f)}}
          onClick={()=>!file&&document.getElementById('fi')?.click()} className="dl"
          style={{border:`2px dashed ${drag?'#6aac3e':'#c8e898'}`,borderRadius:'14px',padding:'40px 24px',textAlign:'center',cursor:file?'default':'pointer',background:drag?'#f0faf0':'#fafff5',transition:'all .2s',marginBottom:'16px'}}>
          <input id="fi" type="file" accept=".txt,.md,text/plain,text/markdown" style={{display:'none'}} onChange={e=>{const f=e.target.files?.[0];if(f)handleFile(f)}}/>
          {file?(
            <div>
              <div style={{fontSize:'2.8rem',marginBottom:'8px'}}>{/\.(md|markdown)$/i.test(file.name)?'📝':'📘'}</div>
              <div style={{fontWeight:700,color:'#2d3a1e',marginBottom:'4px'}}>{file.name}</div>
              <div style={{fontSize:'0.76rem',color:'#8aaa60',marginBottom:'10px'}}>{(file.size/1024/1024).toFixed(2)} MB</div>
              <span onClick={e=>{e.stopPropagation();reset()}} style={{fontSize:'0.76rem',color:'#a0b880',cursor:'pointer',textDecoration:'underline'}}>Cambiar archivo</span>
            </div>
          ):(
            <div>
              <div style={{fontSize:'2.8rem',marginBottom:'10px'}}>📂</div>
              <div style={{fontWeight:600,color:'#3d5220',marginBottom:'6px'}}>Arrastra tu documento o haz clic para seleccionar</div>
              <div style={{fontSize:'0.76rem',color:'#8aaa60'}}>Formatos: <strong>.txt</strong> · <strong>.md</strong> · Máx 10 MB</div>
            </div>
          )}
        </div>
        {error && <div style={{background:'#fff0f0',border:'1px solid #f5b8b8',color:'#c0392b',borderRadius:'8px',padding:'9px 12px',fontSize:'0.78rem',marginBottom:'12px'}}>{error}</div>}
        {file && <button onClick={submit} style={{width:'100%',padding:'12px',background:'#6aac3e',color:'white',border:'none',borderRadius:'12px',fontSize:'0.92rem',fontWeight:700,cursor:'pointer',boxShadow:'0 4px 14px rgba(106,172,62,.3)'}}>Convertir a bundle OKF →</button>}
      </div>
      <div style={{...card,marginTop:'14px',padding:'18px 22px'}}>
        <div style={{fontSize:'0.78rem',fontWeight:700,color:'#3d5220',marginBottom:'12px'}}>¿Cómo funciona?</div>
        {[['📤','Subes tu archivo','Se almacena seguro en la nube'],['⚙️','Lo procesamos','Extraemos conceptos y generamos el bundle'],['📦','Descargas el resultado','Un paquete OKF estructurado y listo']].map(([ic,tt,dd],i)=>(
          <div key={i} style={{display:'flex',gap:'12px',alignItems:'flex-start',paddingBottom:i<2?'12px':'0',borderBottom:i<2?'1px solid #e8f2d8':'none',marginBottom:i<2?'12px':'0'}}>
            <span style={{fontSize:'1.3rem',flexShrink:0}}>{ic}</span>
            <div><div style={{fontSize:'0.8rem',fontWeight:700,color:'#2d3a1e',marginBottom:'2px'}}>{tt}</div><div style={{fontSize:'0.74rem',color:'#7a9a60',lineHeight:1.5}}>{dd}</div></div>
          </div>
        ))}
      </div>
    </div>
  )

  if(phase==='uploading') return (
    <div style={{...card,textAlign:'center',padding:'56px 32px'}}>
      <div style={{width:56,height:56,border:'4px solid #c8e898',borderTopColor:'#6aac3e',borderRadius:'50%',animation:'spin2 1s linear infinite',margin:'0 auto 18px'}}/>
      <div style={{fontWeight:700,color:'#2d3a1e',marginBottom:'6px'}}>Subiendo documento...</div>
      <div style={{fontSize:'0.8rem',color:'#8aaa60'}}>{file?.name}</div>
    </div>
  )

  if(phase==='processing'&&activeJob) return (
    <div style={{...card,padding:'36px 28px'}}>
      <style>{`@keyframes stepPop{from{opacity:0;transform:translateX(-8px)}to{opacity:1;transform:translateX(0)}} @keyframes dotP{0%,100%{opacity:.3}50%{opacity:1}}`}</style>
      <div style={{textAlign:'center',marginBottom:'20px'}}>
        <div style={{fontSize:'2.5rem',marginBottom:'10px'}}>⚙️</div>
        <div style={{fontWeight:800,color:'#2d3a1e',fontSize:'1.1rem',marginBottom:'4px'}}>Documento recibido</div>
        <div style={{fontSize:'0.8rem',color:'#8aaa60'}}>{file?.name}</div>
      </div>
      <div style={{background:'#f5faea',border:'1.5px solid #d8f0b8',borderRadius:'12px',padding:'14px 16px',marginBottom:'18px'}}>
        <div style={{display:'flex',alignItems:'center',gap:'8px',marginBottom:'8px',flexWrap:'wrap'}}>
          <span style={{fontSize:'0.68rem',fontWeight:700,color:'#6aac3e',letterSpacing:'0.08em',textTransform:'uppercase'}}>Identificador del trabajo</span>
          <span style={{padding:'2px 9px',borderRadius:'20px',fontSize:'0.68rem',fontWeight:700,background:STATUS_INFO[activeJob.status].bg,color:STATUS_INFO[activeJob.status].color,border:`1px solid ${STATUS_INFO[activeJob.status].border}`}}>{STATUS_INFO[activeJob.status].label}</span>
        </div>
        <div style={{fontFamily:'monospace',fontSize:'0.76rem',color:'#3d5220',wordBreak:'break-all',marginBottom:'8px'}}>{activeJob.id}</div>
        <div style={{fontSize:'0.73rem',color:'#7a9a60',lineHeight:1.55}}>
          Tu documento se esta procesando. Puedes cerrar esta pantalla o subir otro documento.
        </div>
      </div>
      <div style={{display:'flex',flexDirection:'column',gap:'10px'}}>
        {[{ic:'✅',lb:'Documento recibido',done:true},{ic:activeJob.status==='processing'?'⚙️':'⏳',lb:'Extrayendo conceptos y generando bundle',done:activeJob.status==='processing'},{ic:'📦',lb:'Bundle OKF listo para descargar',done:false}].map((step,i)=>(
          <div key={i} style={{display:'flex',alignItems:'center',gap:'10px',padding:'11px 14px',borderRadius:'10px',background:step.done?'#f0faf0':'#fafff5',border:`1px solid ${step.done?'#a8d88a':'#dff0c8'}`,animation:`stepPop .3s ${i*0.1}s ease both`}}>
            <span style={{fontSize:'1.1rem'}}>{step.ic}</span>
            <span style={{fontSize:'0.83rem',fontWeight:step.done?600:400,color:step.done?'#2e7d32':'#8aaa60'}}>{step.lb}</span>
            {!step.done&&i===1&&<div style={{marginLeft:'auto',display:'flex',gap:'3px'}}>{[0,1,2].map(d=><div key={d} style={{width:5,height:5,borderRadius:'50%',background:'#8aaa60',animation:`dotP 1.2s ${d*0.3}s infinite`}}/>)}</div>}
          </div>
        ))}
      </div>
      <div style={{textAlign:'center',marginTop:'16px',fontSize:'0.74rem',color:'#a0b880'}}>Actualizando cada pocos segundos...</div>
      {/* El trabajo ya tiene identificador: se puede seguir desde su propia
          URL aunque se cierre esta pantalla. */}
      <button onClick={()=>navigate(`/jobs/${activeJob.id}`)} style={{width:'100%',marginTop:'14px',padding:'10px',background:'transparent',color:'#6aac3e',border:'1.5px solid #b8d98a',borderRadius:'12px',fontSize:'0.84rem',fontWeight:600,cursor:'pointer'}}>
        Seguirlo en su propia página →
      </button>
      <button onClick={reset} style={{width:'100%',marginTop:'8px',padding:'10px',background:'#6aac3e',color:'white',border:'none',borderRadius:'12px',fontSize:'0.86rem',fontWeight:700,cursor:'pointer',boxShadow:'0 4px 14px rgba(106,172,62,.3)'}}>
        Subir otro documento
      </button>
    </div>
  )

  if(phase==='done'&&activeJob) return (
    <div style={{...card,padding:'36px 28px',textAlign:'center'}}>
      <style>{`@keyframes bounce{0%,100%{transform:scale(1)}30%{transform:scale(1.18)}}`}</style>
      <div style={{fontSize:'3.2rem',animation:'bounce .5s ease',marginBottom:'10px'}}>🎉</div>
      <div style={{fontWeight:800,color:'#2d3a1e',fontSize:'1.15rem',marginBottom:'4px'}}>¡Bundle generado exitosamente!</div>
      <div style={{fontSize:'0.8rem',color:'#8aaa60',marginBottom:'24px'}}>{file?.name}</div>
      <div style={{background:'#f0faf0',border:'1.5px solid #a8d88a',borderRadius:'14px',padding:'18px',marginBottom:'20px'}}>
        <div style={{display:'grid',gridTemplateColumns:'1fr 1fr',gap:'10px'}}>
          <div style={{textAlign:'center',padding:'12px',background:'white',borderRadius:'10px',border:'1px solid #d4efc0'}}>
            <div style={{fontSize:'1.7rem',fontWeight:800,color:'#3d5220'}}>{activeJob.bundle?.concept_count??0}</div>
            <div style={{fontSize:'0.72rem',color:'#8aaa60',marginTop:'2px'}}>Conceptos extraídos</div>
          </div>
          <div style={{textAlign:'center',padding:'12px',background:'white',borderRadius:'10px',border:'1px solid #d4efc0'}}>
            <div style={{fontSize:'1.7rem',fontWeight:800,color:activeJob.bundle?.is_valid?'#3d5220':'#c0392b'}}>{activeJob.bundle?.is_valid?'✓':'✗'}</div>
            <div style={{fontSize:'0.72rem',color:'#8aaa60',marginTop:'2px'}}>Bundle válido</div>
          </div>
        </div>
      </div>
      {/* download_url es la autoridad: solo llega cuando el bundle se publicó. */}
      {activeJob.bundle?.download_url?(
        <button onClick={()=>downloadBundle(activeJob.bundle!.download_url!,token,file?.name?.replace(/\.[^.]+$/,'')+'.zip')} style={{width:'100%',padding:'12px',background:'#6aac3e',color:'white',border:'none',borderRadius:'12px',fontSize:'0.92rem',fontWeight:700,cursor:'pointer',boxShadow:'0 4px 14px rgba(106,172,62,.3)',marginBottom:'10px'}}>
          ⬇ Descargar bundle OKF
        </button>
      ):(
        <div style={{background:'#fff8e8',border:'1px solid #f5de80',borderRadius:'10px',padding:'10px',marginBottom:'10px',fontSize:'0.8rem',color:'#92700a'}}>⚠️ El bundle se generó pero no pasó la validación.</div>
      )}
      <button onClick={()=>navigate(`/jobs/${activeJob.id}`)} style={{width:'100%',padding:'10px',background:'transparent',color:'#6aac3e',border:'1.5px solid #b8d98a',borderRadius:'12px',fontSize:'0.86rem',fontWeight:600,cursor:'pointer',marginBottom:'8px'}}>
        Ver detalle del trabajo
      </button>
      <button onClick={reset} style={{width:'100%',padding:'10px',background:'transparent',color:'#8aaa60',border:'1.5px solid #d4e8b8',borderRadius:'12px',fontSize:'0.86rem',fontWeight:600,cursor:'pointer'}}>
        Subir otro documento
      </button>
    </div>
  )

  return (
    <div style={{...card,textAlign:'center',padding:'48px 28px'}}>
      <div style={{fontSize:'2.2rem',marginBottom:'10px'}}>😔</div>
      <div style={{fontWeight:700,color:'#c0392b',marginBottom:'6px'}}>No se pudo procesar el documento</div>
      <div style={{fontSize:'0.8rem',color:'#8aaa60',marginBottom:'20px'}}>El formato puede no ser compatible o el archivo está dañado.</div>
      <button onClick={reset} style={{padding:'10px 24px',background:'#6aac3e',color:'white',border:'none',borderRadius:'12px',fontSize:'0.86rem',fontWeight:600,cursor:'pointer'}}>Intentar de nuevo</button>
    </div>
  )
}

function DocumentosSection({jobs,token}:{jobs:Job[];token:string}) {
  const navigate=useNavigate()
  const si=STATUS_INFO
  return (
    <div>
      <style>{`@keyframes spin3{to{transform:rotate(360deg)}}.jrow:hover{border-color:#b8d98a!important}`}</style>
      <div style={{fontSize:'0.7rem',fontWeight:700,color:'#6aac3e',letterSpacing:'0.1em',textTransform:'uppercase',marginBottom:'14px'}}>
        Mis documentos {jobs.length>0&&<span style={{color:'#a8c880',fontWeight:400}}>· {jobs.length}</span>}
      </div>
      {jobs.length===0?(
        <div style={{background:'rgba(255,252,245,0.85)',border:'1.5px solid rgba(200,230,160,.5)',borderRadius:'16px',padding:'48px',textAlign:'center',backdropFilter:'blur(8px)'}}>
          <div style={{fontSize:'2.2rem',marginBottom:'10px'}}>🌾</div>
          <div style={{fontWeight:600,color:'#5a6645',marginBottom:'4px'}}>Aún no has subido documentos</div>
          <div style={{fontSize:'0.78rem',color:'#a0b880'}}>Ve a "Subir documento" para empezar</div>
        </div>
      ):jobs.map(job=>{
        const s=si[job.status]??si.queued
        return (
          <div key={job.id} className="jrow" style={{background:'rgba(255,252,245,0.85)',border:`1.5px solid ${s.border}`,borderRadius:'12px',padding:'13px 16px',marginBottom:'9px',display:'flex',alignItems:'center',gap:'10px',backdropFilter:'blur(8px)',transition:'all .2s'}}>
            <div style={{fontSize:'1.4rem'}}>{job.document?.filename?.endsWith('.md')?'📝':'📄'}</div>
            {/* La fila entera lleva al detalle, que es una ruta real. */}
            <div onClick={()=>navigate(`/jobs/${job.id}`)} style={{flex:1,minWidth:0,cursor:'pointer'}}>
              <div style={{fontWeight:600,color:'#2d3a1e',fontSize:'0.86rem',overflow:'hidden',textOverflow:'ellipsis',whiteSpace:'nowrap',marginBottom:'4px'}}>{job.document?.filename||'Documento'}</div>
              <div style={{display:'flex',alignItems:'center',gap:'7px',flexWrap:'wrap' as const}}>
                <span style={{padding:'2px 8px',borderRadius:'20px',fontSize:'0.68rem',fontWeight:700,background:s.bg,color:s.color,border:`1px solid ${s.border}`}}>{s.label}</span>
                {job.bundle&&<span style={{fontSize:'0.7rem',color:'#6a8a50'}}>📦 {job.bundle.concept_count} conceptos</span>}
                <span style={{fontSize:'0.66rem',color:'#b0c890'}}>{new Date(job.created_at).toLocaleDateString()}</span>
              </div>
            </div>
            {(job.status==='queued'||job.status==='processing')&&<div style={{width:15,height:15,border:'2px solid #6aac3e',borderTopColor:'transparent',borderRadius:'50%',animation:'spin3 1s linear infinite',flexShrink:0}}/>}
            <button onClick={()=>navigate(`/jobs/${job.id}`)} style={{padding:'6px 10px',background:'transparent',color:'#6aac3e',border:'1px solid #c8e898',borderRadius:'7px',fontSize:'0.73rem',fontWeight:600,cursor:'pointer',flexShrink:0,whiteSpace:'nowrap'}}>Detalle</button>
            {job.bundle?.download_url&&(
              <button onClick={()=>downloadBundle(job.bundle!.download_url!,token,(job.document?.filename||'bundle').replace(/\.[^.]+$/,'')+'.zip')} style={{padding:'6px 12px',background:'#6aac3e',color:'white',border:'none',borderRadius:'7px',fontSize:'0.73rem',fontWeight:600,cursor:'pointer',flexShrink:0,whiteSpace:'nowrap'}}>⬇ Descargar</button>
            )}
          </div>
        )
      })}
    </div>
  )
}

function PerfilSection({user,jobs}:{user:User;jobs:Job[]}) {
  const initials=`${user.nombre?.[0]??''}${user.apellido?.[0]??''}`.toUpperCase()||user.email[0].toUpperCase()
  const completed=jobs.filter(j=>j.status==='completed').length
  const totalConcepts=jobs.reduce((sum,j)=>sum+(j.bundle?.concept_count??0),0)
  const card:React.CSSProperties={background:'rgba(255,252,245,0.88)',backdropFilter:'blur(12px)',border:'1.5px solid rgba(200,230,160,.55)',borderRadius:'20px',padding:'24px',boxShadow:'0 4px 20px rgba(80,60,30,.07)',marginBottom:'14px'}
  return (
    <div>
      <div style={{fontSize:'0.7rem',fontWeight:700,color:'#6aac3e',letterSpacing:'0.1em',textTransform:'uppercase',marginBottom:'14px'}}>Mi perfil</div>
      <div style={card}>
        <div style={{display:'flex',alignItems:'center',gap:'18px',marginBottom:'20px'}}>
          <div style={{width:68,height:68,borderRadius:'50%',background:'linear-gradient(135deg,#8bc34a,#6aac3e)',display:'flex',alignItems:'center',justifyContent:'center',color:'white',fontWeight:800,fontSize:'1.5rem',flexShrink:0,boxShadow:'0 4px 14px rgba(106,172,62,.35)'}}>
            {initials}
          </div>
          <div>
            <div style={{fontSize:'1.2rem',fontWeight:800,color:'#2d3a1e'}}>{user.nombre&&user.apellido?`${user.nombre} ${user.apellido}`:user.email.split('@')[0]}</div>
            {user.nombre&&<div style={{fontSize:'0.8rem',color:'#8aaa60',marginTop:'2px'}}>{user.email}</div>}
          </div>
        </div>
        <div style={{display:'grid',gridTemplateColumns:'repeat(3,1fr)',gap:'10px'}}>
          {[[jobs.length.toString(),'Documentos','📄'],[completed.toString(),'Bundles','📦'],[totalConcepts.toString(),'Conceptos','🧠']].map(([val,lb,ic])=>(
            <div key={lb} style={{textAlign:'center',padding:'14px 6px',background:'#f5faea',borderRadius:'12px',border:'1px solid #d8f0b8'}}>
              <div style={{fontSize:'1.3rem',marginBottom:'3px'}}>{ic}</div>
              <div style={{fontSize:'1.4rem',fontWeight:800,color:'#3d5220'}}>{val}</div>
              <div style={{fontSize:'0.68rem',color:'#8aaa60',marginTop:'2px'}}>{lb}</div>
            </div>
          ))}
        </div>
      </div>
      <div style={card}>
        <div style={{fontSize:'0.78rem',fontWeight:700,color:'#3d5220',marginBottom:'14px'}}>Información de la cuenta</div>
        {[['👤','Nombre completo',user.nombre&&user.apellido?`${user.nombre} ${user.apellido}`:'—'],['📧','Correo',user.email],['🔑','ID de usuario',user.id]].map(([ic,lb,val])=>(
          <div key={lb} style={{display:'flex',gap:'10px',alignItems:'flex-start',paddingBottom:'10px',borderBottom:'1px solid #e8f2d8',marginBottom:'10px'}}>
            <span style={{fontSize:'0.95rem',flexShrink:0,marginTop:'1px'}}>{ic}</span>
            <div>
              <div style={{fontSize:'0.7rem',color:'#8aaa60',fontWeight:600,marginBottom:'2px'}}>{lb}</div>
              <div style={{fontSize:'0.82rem',color:'#2d3a1e',fontFamily:lb==='ID de usuario'?'monospace':'inherit',wordBreak:'break-all'}}>{val as string}</div>
            </div>
          </div>
        ))}
        <div style={{fontSize:'0.72rem',color:'#a8c890'}}>🔒 Tus datos solo se usan dentro de esta plataforma académica.</div>
      </div>
    </div>
  )
}

// Lista de hallazgos de la validación. Las advertencias no impiden la
// descarga; los errores sí, y por eso se presentan distinto.
function FindingList({title,items,tone}:{title:string;items:string[];tone:'warning'|'error'}) {
  if (items.length===0) return null
  const c = tone==='error'
    ? {bg:'#fff0f0',border:'#f5b8b8',color:'#c0392b'}
    : {bg:'#fff8e8',border:'#f5de80',color:'#92700a'}
  return (
    <div style={{background:c.bg,border:`1px solid ${c.border}`,borderRadius:'10px',padding:'12px 14px',marginBottom:'10px',textAlign:'left'}}>
      <div style={{fontSize:'0.76rem',fontWeight:700,color:c.color,marginBottom:'6px'}}>{title}</div>
      <ul style={{margin:0,paddingLeft:'18px',color:c.color,fontSize:'0.78rem',lineHeight:1.6}}>
        {items.map((item,i)=><li key={i}>{item}</li>)}
      </ul>
    </div>
  )
}

// Vista de detalle de un trabajo, direccionable como /jobs/:id.
//
// La URL con el identificador es lo que permite demostrar el aislamiento:
// basta pegar el id de otro usuario para ver que el servidor lo niega.
function JobDetailView({token,onExpired}:{token:string;onExpired:()=>void}) {
  const {id}=useParams<{id:string}>()
  const navigate=useNavigate()
  const [job,setJob]=useState<JobDetail|null>(null)
  const [state,setState]=useState<'loading'|'ok'|'notfound'|'expired'|'error'>('loading')

  const load=useCallback(async()=>{
    if(!id) return
    try {
      const res=await fetch(`${API}/jobs/${id}`,{headers:{Authorization:`Bearer ${token}`}})
      // 404 responde tanto a un identificador inexistente como a uno
      // ajeno: el servidor no distingue, para no revelar nada.
      if(res.status===404){setState('notfound');return}
      if(res.status===401){setState('expired');return}
      if(!res.ok){setState('error');return}
      setJob(await res.json() as JobDetail)
      setState('ok')
    } catch { setState('error') }
  },[id,token])

  useEffect(()=>{void load()},[load])

  // Se refresca mientras el trabajo no sea terminal. Quién es terminal lo
  // decide el backend con su bandera, no una lista de estados aquí.
  const terminal=job?.terminal??true
  useEffect(()=>{
    if(state!=='ok'||terminal) return
    const timer=setInterval(()=>{void load()},2500)
    return ()=>clearInterval(timer)
  },[state,terminal,load])

  const card:React.CSSProperties={background:'rgba(255,252,245,0.9)',backdropFilter:'blur(12px)',border:'1.5px solid rgba(200,230,160,.55)',borderRadius:'20px',padding:'28px',boxShadow:'0 4px 24px rgba(80,60,30,.08)'}
  const page=(children:React.ReactNode)=>(
    <div style={{position:'relative',zIndex:1,minHeight:'100vh',display:'flex',alignItems:'center',justifyContent:'center',padding:'32px 20px'}}>
      <style>{`@keyframes spinD{to{transform:rotate(360deg)}}`}</style>
      <div style={{width:'100%',maxWidth:'620px'}}>
        <button onClick={()=>navigate('/app/documentos')} style={{background:'none',border:'none',color:'#7a9a50',cursor:'pointer',fontSize:'0.82rem',fontWeight:600,marginBottom:'14px',padding:0}}>← Volver a mis documentos</button>
        {children}
      </div>
    </div>
  )

  if(state==='loading') return page(
    <div style={{...card,textAlign:'center',padding:'56px 28px'}}>
      <div style={{width:44,height:44,border:'4px solid #c8e898',borderTopColor:'#6aac3e',borderRadius:'50%',animation:'spinD 1s linear infinite',margin:'0 auto 16px'}}/>
      <div style={{color:'#8aaa60',fontSize:'0.85rem'}}>Cargando el trabajo...</div>
    </div>
  )

  if(state==='notfound') return page(
    <div style={{...card,textAlign:'center',padding:'48px 28px'}}>
      <div style={{fontSize:'2.6rem',marginBottom:'10px'}}>🔒</div>
      <div style={{fontWeight:800,color:'#2d3a1e',fontSize:'1.1rem',marginBottom:'8px'}}>Trabajo no encontrado</div>
      <p style={{fontSize:'0.83rem',color:'#7a9a60',lineHeight:1.65,marginBottom:'20px'}}>
        No hay ningún trabajo tuyo con ese identificador. Si pertenece a otra
        cuenta la respuesta es la misma: el servidor no revela si existe.
      </p>
      <code style={{fontSize:'0.72rem',color:'#a0b880',wordBreak:'break-all'}}>{id}</code>
    </div>
  )

  if(state==='expired') return page(
    <div style={{...card,textAlign:'center',padding:'48px 28px'}}>
      <div style={{fontSize:'2.4rem',marginBottom:'10px'}}>⏰</div>
      <div style={{fontWeight:700,color:'#2d3a1e',marginBottom:'8px'}}>Tu sesión expiró</div>
      <div style={{fontSize:'0.82rem',color:'#8aaa60',marginBottom:'18px'}}>Vuelve a iniciar sesión para consultar este trabajo.</div>
      <button onClick={onExpired} style={{padding:'9px 22px',background:'#6aac3e',color:'white',border:'none',borderRadius:'10px',fontSize:'0.84rem',fontWeight:600,cursor:'pointer'}}>Iniciar sesión</button>
    </div>
  )

  if(state!=='ok'||!job) return page(
    <div style={{...card,textAlign:'center',padding:'48px 28px'}}>
      <div style={{fontSize:'2.4rem',marginBottom:'10px'}}>😔</div>
      <div style={{fontWeight:700,color:'#c0392b',marginBottom:'8px'}}>No se pudo consultar el trabajo</div>
      <button onClick={()=>{setState('loading');void load()}} style={{padding:'9px 20px',background:'#6aac3e',color:'white',border:'none',borderRadius:'10px',fontSize:'0.84rem',fontWeight:600,cursor:'pointer'}}>Reintentar</button>
    </div>
  )

  const s=STATUS_INFO[job.status]??STATUS_INFO.queued
  const bundle=job.bundle
  const validation=bundle?.validation

  return page(
    <div style={card}>
      <div style={{marginBottom:'18px'}}>
        <div style={{fontSize:'0.7rem',fontWeight:700,color:'#6aac3e',letterSpacing:'0.1em',textTransform:'uppercase',marginBottom:'6px'}}>Detalle del trabajo</div>
        <div style={{fontSize:'1.15rem',fontWeight:800,color:'#2d3a1e',wordBreak:'break-word'}}>{job.document?.filename||'Documento'}</div>
        <div style={{fontSize:'0.7rem',color:'#a0b880',fontFamily:'monospace',marginTop:'4px',wordBreak:'break-all'}}>{job.id}</div>
      </div>

      <div style={{display:'flex',alignItems:'center',gap:'10px',flexWrap:'wrap',marginBottom:'18px'}}>
        <span style={{padding:'4px 12px',borderRadius:'20px',fontSize:'0.76rem',fontWeight:700,background:s.bg,color:s.color,border:`1px solid ${s.border}`}}>{s.label}</span>
        {!job.terminal&&<div style={{display:'flex',alignItems:'center',gap:'7px',fontSize:'0.74rem',color:'#a0b880'}}>
          <div style={{width:13,height:13,border:'2px solid #6aac3e',borderTopColor:'transparent',borderRadius:'50%',animation:'spinD 1s linear infinite'}}/>
          Actualizando cada pocos segundos...
        </div>}
        <span style={{fontSize:'0.7rem',color:'#b0c890',marginLeft:'auto'}}>{new Date(job.created_at).toLocaleString()}</span>
      </div>

      {job.status==='failed'&&(
        <>
          <FindingList tone="error" title="El trabajo falló" items={job.error_message?[job.error_message]:['El worker no pudo completar la conversión.']}/>
          {validation&&<FindingList tone="error" title="La validación rechazó el bundle" items={validation.errors}/>}
          <div style={{background:'#faf8f4',border:'1px dashed #d8ccb8',borderRadius:'10px',padding:'11px 14px',fontSize:'0.78rem',color:'#8a7a60'}}>
            No hay descarga disponible: un bundle que no supera la validación no se publica.
          </div>
        </>
      )}

      {job.status==='completed'&&bundle&&(
        <>
          <div style={{display:'grid',gridTemplateColumns:'1fr 1fr',gap:'10px',marginBottom:'14px'}}>
            <div style={{textAlign:'center',padding:'14px',background:'white',borderRadius:'10px',border:'1px solid #d4efc0'}}>
              <div style={{fontSize:'1.6rem',fontWeight:800,color:'#3d5220'}}>{bundle.concept_count}</div>
              <div style={{fontSize:'0.7rem',color:'#8aaa60',marginTop:'2px'}}>Conceptos</div>
            </div>
            <div style={{textAlign:'center',padding:'14px',background:'white',borderRadius:'10px',border:'1px solid #d4efc0'}}>
              <div style={{fontSize:'0.86rem',fontWeight:800,color:validation?.status==='invalid'?'#c0392b':validation?.status==='valid_with_warnings'?'#92700a':'#3d5220',marginTop:'6px'}}>
                {validation?.status==='invalid'?'Inválido':validation?.status==='valid_with_warnings'?'Válido con advertencias':'Válido'}
              </div>
              <div style={{fontSize:'0.7rem',color:'#8aaa60',marginTop:'4px'}}>Validación</div>
            </div>
          </div>

          {validation&&<FindingList tone="warning" title="Advertencias de la validación" items={validation.warnings}/>}
          {validation&&<FindingList tone="error" title="Errores de la validación" items={validation.errors}/>}

          {bundle.download_url?(
            <button onClick={()=>downloadBundle(bundle.download_url!,token,(job.document?.filename||'bundle').replace(/\.[^.]+$/,'')+'.zip')}
              style={{width:'100%',padding:'12px',background:'#6aac3e',color:'white',border:'none',borderRadius:'12px',fontSize:'0.92rem',fontWeight:700,cursor:'pointer',boxShadow:'0 4px 14px rgba(106,172,62,.3)'}}>
              ⬇ Descargar bundle OKF
            </button>
          ):(
            <div style={{background:'#faf8f4',border:'1px dashed #d8ccb8',borderRadius:'10px',padding:'11px 14px',fontSize:'0.78rem',color:'#8a7a60'}}>
              El bundle no se publicó, así que no hay descarga disponible.
            </div>
          )}
        </>
      )}

      {!job.terminal&&(
        <div style={{background:'#fafff5',border:'1px solid #dff0c8',borderRadius:'10px',padding:'14px',fontSize:'0.8rem',color:'#7a9a60',lineHeight:1.6}}>
          El documento se está procesando. Puedes cerrar esta página y consultar el estado en documentos.
        </div>
      )}
    </div>
  )
}

const DASH_SECTIONS:DashSection[]=['upload','documentos','perfil']

// Observabilidad básica del flujo: cuántos trabajos hay en cada estado.
// Se dibuja siempre, incluso en cero, para que un contador vacío se
// distinga de un contador ausente.
function MetricsBar({stats}:{stats:JobStats|null}) {
  const cells:[string,number,string,string][] = stats
    ? [
        ['En cola',     stats.queued,     STATUS_INFO.queued.color,     STATUS_INFO.queued.bg],
        ['Procesando',  stats.processing, STATUS_INFO.processing.color, STATUS_INFO.processing.bg],
        ['Completados', stats.completed,  STATUS_INFO.completed.color,  STATUS_INFO.completed.bg],
        ['Fallidos',    stats.failed,     STATUS_INFO.failed.color,     STATUS_INFO.failed.bg],
        ['Total',       stats.total,      '#3d5220',                    '#f5faea'],
      ]
    : []

  return (
    <div style={{marginBottom:'20px'}}>
      <div style={{fontSize:'0.7rem',fontWeight:700,color:'#6aac3e',letterSpacing:'0.1em',textTransform:'uppercase',marginBottom:'10px'}}>
        Métricas del flujo
      </div>
      <div style={{display:'grid',gridTemplateColumns:'repeat(5,1fr)',gap:'8px'}}>
        {cells.length===0
          ? <div style={{gridColumn:'1 / -1',fontSize:'0.76rem',color:'#a0b880'}}>Cargando métricas...</div>
          : cells.map(([label,value,color,bg])=>(
            <div key={label} style={{textAlign:'center',padding:'10px 4px',background:bg,borderRadius:'10px',border:'1px solid rgba(200,230,160,.7)'}}>
              <div style={{fontSize:'1.3rem',fontWeight:800,color}}>{value}</div>
              <div style={{fontSize:'0.64rem',color:'#7a9a60',marginTop:'2px'}}>{label}</div>
            </div>
          ))}
      </div>
    </div>
  )
}

function Dashboard({user,token,onLogout}:{user:User;token:string;onLogout:()=>void}) {
  const navigate=useNavigate()
  const {section:raw}=useParams<{section:string}>()
  const section=DASH_SECTIONS.find(s=>s===raw)
  const setSection=(s:DashSection)=>navigate(`/app/${s}`)
  const [jobs,setJobs]=useState<Job[]>([])
  const [stats,setStats]=useState<JobStats|null>(null)

  // Lista y métricas se piden juntas para que lo que se ve en pantalla
  // corresponda al mismo instante.
  const refresh=useCallback(async()=>{
    try {
      const [jobsRes,statsRes]=await Promise.all([
        fetch(`${API}/jobs`,{headers:{Authorization:`Bearer ${token}`}}),
        fetch(`${API}/stats`,{headers:{Authorization:`Bearer ${token}`}}),
      ])
      // Una sesión guardada puede haber caducado. Sin este corte la
      // aplicación seguiría en pie fallando en silencio en cada petición.
      if(jobsRes.status===401||statsRes.status===401){onLogout();return}
      if(jobsRes.ok){
        const data=await jobsRes.json()
        if(Array.isArray(data)) setJobs(data)
      }
      if(statsRes.ok){
        const data=await statsRes.json()
        if(data?.jobs) setStats(data.jobs as JobStats)
      }
    } catch { /* se reintenta en el siguiente refresco */ }
  },[token,onLogout])

  useEffect(()=>{void refresh()},[refresh])

  // La lista se refresca sola mientras quede algún trabajo vivo, y para
  // cuando todos son terminales: entonces ya no hay nada que esperar.
  const pending=jobs.some(job=>!job.terminal)
  useEffect(()=>{
    if(!pending) return
    const timer=setInterval(()=>{void refresh()},3000)
    return ()=>clearInterval(timer)
  },[pending,refresh])

  // Memorizado a propósito: la pantalla de subida lo tiene entre las
  // dependencias de su intervalo, y este panel se re-renderiza cada pocos
  // segundos al refrescar la lista. Sin memorizar, ese intervalo se
  // destruiría y recrearía antes de llegar a dispararse.
  const handleJobCreated=useCallback((job:Job)=>setJobs(prev=>{
    const exists=prev.find(j=>j.id===job.id)
    return exists?prev.map(j=>j.id===job.id?job:j):[job,...prev]
  }),[])
  if(!section) return <Navigate to="/app/upload" replace/>

  return (
    <div style={{position:'relative',zIndex:1,minHeight:'100vh',display:'flex'}}>
      <Sidebar section={section} setSection={setSection} user={user} onLogout={onLogout}/>
      <main style={{flex:1,padding:'34px 36px',overflowY:'auto',maxWidth:'680px'}}>
        <MetricsBar stats={stats}/>
        {section==='upload'     && <UploadSection token={token} onJobCreated={handleJobCreated}/>}
        {section==='documentos' && <DocumentosSection jobs={jobs} token={token}/>}
        {section==='perfil'     && <PerfilSection user={user} jobs={jobs}/>}
      </main>
    </div>
  )
}

function AppRoutes() {
  const navigate=useNavigate()
  const [params]=useSearchParams()
  const [session,setSession]=useState<Session|null>(loadSession)

  const handleLogin=(user:User,token:string)=>{
    const next={user,token}
    setSession(next);saveSession(next);navigate('/app')
  }
  const handleLogout=()=>{setSession(null);saveSession(null);navigate('/')}

  // Cada ruta privada construye su elemento solo si hay sesión. El
  // condicional no puede estar dentro del componente ni en un ayudante que
  // reciba el JSX ya creado: las props se evalúan al construirlo, así que
  // `session.user` se leería incluso en las rutas públicas.
  return (
    <Routes>
      <Route path="/" element={session
        ? <Navigate to="/app" replace/>
        : <Landing onGoAuth={m=>navigate(`/auth?mode=${m}`)}/>}/>
      <Route path="/auth" element={session
        ? <Navigate to="/app" replace/>
        : <AuthView initialMode={params.get('mode')==='register'?'register':'login'} onLogin={handleLogin} onBack={()=>navigate('/')}/>}/>
      <Route path="/app" element={<Navigate to="/app/upload" replace/>}/>
      <Route path="/app/:section" element={session
        ? <Dashboard user={session.user} token={session.token} onLogout={handleLogout}/>
        : <Navigate to="/auth" replace/>}/>
      <Route path="/jobs/:id" element={session
        ? <JobDetailView token={session.token} onExpired={handleLogout}/>
        : <Navigate to="/auth" replace/>}/>
      <Route path="*" element={<Navigate to="/" replace/>}/>
    </Routes>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <div style={{background:'#fdf6ec',minHeight:'100vh',fontFamily:"'Segoe UI',system-ui,sans-serif"}}>
        <Background/>
        <AppRoutes/>
      </div>
    </BrowserRouter>
  )
}
