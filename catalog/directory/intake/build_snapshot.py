#!/usr/bin/env python3
"""Explicit bounded metadata intake. Never fetches advertised server/spec URLs.
Fetch official pages once, then reproduce from committed gzip inputs offline.
"""
import argparse, datetime, gzip, hashlib, json, pathlib, re, signal, time, urllib.parse, urllib.request
BASE='https://registry.modelcontextprotocol.io/v0.1/servers'
LOCAL_REVISION='a6c7bd243ee1b5294fce4e0daf82c05647dc09cb'
LOCAL_SHA256='00629b49418b88cd965e202d53a695e5c2b19a0ff8e437fe0399803a9d0b174e'
HERE=pathlib.Path(__file__).resolve().parent
ROOT=HERE.parent

def dump(path,obj):
    path.write_text(json.dumps(obj,ensure_ascii=False,indent=2)+'\n')
def digest(raw): return hashlib.sha256(raw).hexdigest()
def safe_url(value):
    import ipaddress
    try:
        u=urllib.parse.urlsplit(value)
        if any(c in value for c in '{}'):return False
        if u.port is not None and not 1<=u.port<=65535:return False
        if u.scheme!='https' or not u.hostname or u.username is not None or u.password is not None or u.query or u.fragment: return False
        host=u.hostname.rstrip('.').lower()
        if not all(re.fullmatch(r'[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?',label) for label in host.split('.')):return False
        if '.' not in host or host=='localhost' or host.endswith(('.local','.localhost','.internal')):return False
        try:
            if not ipaddress.ip_address(host).is_global:return False
        except ValueError:pass
        return len(value)<=2048 and not any(ord(x)<32 for x in value)
    except (ValueError,TypeError): return False

def clean(value,maxlen):
    return ' '.join(str(value or '').split())[:maxlen]

def projection(pages):
    """Retain only fields used by Gist; drop header/argument/default values."""
    out=[]
    for raw in pages:
        page=json.loads(raw);servers=[]
        for wrapper in page.get('servers',[]):
            original=wrapper.get('server',{});server={k:original[k] for k in ('name','title','description','version') if k in original}
            if safe_url(original.get('repository',{}).get('url','')):server['repository']={'url':original['repository']['url']}
            if safe_url(original.get('websiteUrl','')):server['websiteUrl']=original['websiteUrl']
            server['remotes']=[{'type':r.get('type',''),'url':r.get('url','')} for r in original.get('remotes',[]) if safe_url(r.get('url',''))]
            packages=[]
            for value in original.get('packages',[]):
                package={k:value[k] for k in ('registryType','identifier','version','runtimeHint') if k in value}
                package['environmentVariables']=[{'name':e.get('name',''),'isSecret':bool(e.get('isSecret'))} for e in value.get('environmentVariables',[])]
                packages.append(package)
            server['packages']=packages
            meta=wrapper.get('_meta',{}).get('io.modelcontextprotocol.registry/official',{})
            servers.append({'server':server,'_meta':{'io.modelcontextprotocol.registry/official':{k:meta[k] for k in ('status','isLatest') if k in meta}}})
        out.append(json.dumps({'servers':servers},ensure_ascii=False,separators=(',',':')))
    return json.dumps({'pages':out},ensure_ascii=False,separators=(',',':')).encode()

class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise RuntimeError('directory intake redirects are forbidden')

def timeout_alarm(signum, frame):
    raise TimeoutError('directory intake total deadline exceeded')

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--fetch',action='store_true');p.add_argument('--local-registry',type=pathlib.Path)
    p.add_argument('--max-pages',type=int,default=40);p.add_argument('--max-bytes',type=int,default=16*1024*1024)
    p.add_argument('--timeout',type=int,default=180)
    a=p.parse_args()
    if not 1<=a.max_pages<=100 or not 1024<=a.max_bytes<=32*1024*1024 or not 1<=a.timeout<=300:p.error('unsafe intake budget')
    raw_path=HERE/'official-projection.json.gz';local_path=HERE/'local-registry.json.gz';meta_path=HERE/'source-lock.json'
    if a.fetch:
        if a.local_registry is None:p.error('--fetch requires explicit --local-registry')
        local=a.local_registry.read_bytes()
        if len(local)>16*1024*1024:raise RuntimeError('local input budget exceeded')
        if digest(local)!=LOCAL_SHA256:raise RuntimeError('local input differs from pinned reviewed revision; update source pins through review')
        signal.signal(signal.SIGALRM,timeout_alarm);signal.setitimer(signal.ITIMER_REAL,a.timeout)
        opener=urllib.request.build_opener(NoRedirect())
        start=time.monotonic();pages=[];cursor='';seen=set();size=0;complete=False
        for _ in range(a.max_pages):
            if time.monotonic()-start>a.timeout:break
            params={'limit':'100','version':'latest'}
            if cursor:params['cursor']=cursor
            request=urllib.request.Request(BASE+'?'+urllib.parse.urlencode(params),headers={'User-Agent':'Gist-directory-metadata-intake/1'})
            with opener.open(request,timeout=min(20,a.timeout)) as r:
                if r.geturl().split('?')[0]!=BASE:raise RuntimeError('unexpected intake redirect')
                raw=r.read(min(2*1024*1024,a.max_bytes-size)+1)
            size+=len(raw)
            if len(raw)>2*1024*1024 or size>a.max_bytes:raise RuntimeError('intake byte budget exceeded')
            page=json.loads(raw);pages.append(raw.decode('utf-8'))
            cursor=page.get('metadata',{}).get('nextCursor','')
            if not cursor:complete=True;break
            if cursor in seen:raise RuntimeError('repeated upstream cursor')
            seen.add(cursor)
        signal.setitimer(signal.ITIMER_REAL,0)
        stamp=datetime.datetime.now(datetime.timezone.utc).isoformat().replace('+00:00','Z')
        official=json.dumps({'pages':pages},ensure_ascii=False,separators=(',',':')).encode()
        projected=projection(pages)
        raw_path.write_bytes(gzip.compress(projected,mtime=0));local_path.write_bytes(gzip.compress(local,mtime=0))
        dump(meta_path,{'retrieved_at':stamp,'official_complete':complete,'official_pages':len(pages),'official_bytes':size,'official_sha256':digest(official),'official_projection_sha256':digest(projected),'official_page_sha256':[digest(p.encode()) for p in pages],'local_sha256':digest(local),'local_revision':LOCAL_REVISION,'max_pages':a.max_pages,'max_bytes':a.max_bytes,'deadline_seconds':a.timeout})
    lock=json.loads(meta_path.read_text());official_raw=gzip.decompress(raw_path.read_bytes());local_raw=gzip.decompress(local_path.read_bytes())
    if digest(official_raw)!=lock['official_projection_sha256'] or digest(local_raw)!=lock['local_sha256']:raise RuntimeError('source lock mismatch')
    entries=[];rejected=[];duplicates=[];seen=set()
    def add(r):
        if r['id'] in seen:duplicates.append(r['id']);return
        seen.add(r['id']);entries.append(r)
    def base(id,kind,title,description,source,entryid,url,version=''):
        return dict(id=id,kind=kind,service_id=entryid,title=clean(title,200),description=clean(description,3000),version=clean(version,200),tags=[],source=dict(id=source,entry_id=entryid,url=url),auth={'type':'unknown'},recommended=False,official=False,featured=False)
    local=json.loads(local_raw)
    for x in local['apis']:
        name=x.get('name','');spec=x.get('spec_url','')
        if not name or not safe_url(spec):rejected.append({'source':'sirerun-openapi','id':name,'reason':'unsafe or missing HTTPS specification URL'});continue
        r=base('openapi:'+name,'openapi',name.replace('-',' ').title(),x.get('description',''),'sirerun-openapi',name,'https://github.com/sirerun/mcp-registry/blob/'+LOCAL_REVISION+'/registry.json')
        r['tags']=[clean(t,64) for t in x.get('tags',[])[:20] if clean(t,64)]
        r['openapi']={'spec_url':spec,'mint_guide_url':'https://github.com/sirerun/mint'}
        r['auth']={'type':clean(x.get('auth_type','unknown'),64)}
        env=x.get('auth_env_var','')
        if re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]{0,127}',env):r['auth']['credential_names']=[env]
        add(r)
    official_count=0;skipped=0
    for raw in json.loads(official_raw)['pages']:
        for wrapper in json.loads(raw).get('servers',[]):
            official_count+=1
            x=wrapper.get('server',{});meta=wrapper.get('_meta',{}).get('io.modelcontextprotocol.registry/official',{})
            name=x.get('name','');version=x.get('version','')
            if meta.get('status')!='active' or meta.get('isLatest') is not True:skipped+=1;continue
            if not name or not version:rejected.append({'source':'mcp-official','id':name,'reason':'missing identity/version'});continue
            r=base('mcp:'+name,'mcp',x.get('title') or name,x.get('description',''),'mcp-official',name,'https://registry.modelcontextprotocol.io/?q='+urllib.parse.quote(name),version)
            # Source URL uses a stable credential-free registry page; identity is separate.
            r['source']['url']='https://registry.modelcontextprotocol.io/'
            repository=x.get('repository',{}).get('url','')
            if safe_url(repository):r['repository']=repository
            website=x.get('websiteUrl','')
            if safe_url(website):r['homepage']=website
            remotes=[{'url':v.get('url',''),'transport':v.get('type','')} for v in x.get('remotes',[]) if safe_url(v.get('url','')) and v.get('type') in ('streamable-http','sse')][:16]
            packages=[];credentials=[]
            for v in x.get('packages',[])[:16]:
                reg=v.get('registryType','');identifier=v.get('identifier','')
                if reg not in ('npm','pypi','oci') or not identifier or any(c in identifier for c in ' \t\r\n?&#') or '://' in identifier:continue
                packages.append(dict(registry=reg,identifier=clean(identifier,512),version=clean(v.get('version',''),200),runtime=clean(v.get('runtimeHint',''),64)))
                for ev in v.get('environmentVariables',[]):
                    key=ev.get('name','')
                    if ev.get('isSecret') and re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]{0,127}',key):credentials.append(key)
            if remotes:r['remotes']=remotes
            if packages:r['packages']=packages
            if not remotes and not packages:rejected.append({'source':'mcp-official','id':name,'reason':'no supported credential-free public endpoint or package descriptor'});continue
            if credentials:r['auth']={'type':'credential-required','credential_names':sorted(set(credentials))[:24]}
            add(r)
    curated=json.loads((HERE/'curated.json').read_text())
    for c in curated:
        existing=next((r for r in entries if r['id']==c['id']),None)
        if existing:
            entries.remove(existing);seen.remove(c['id'])
        c['source']={'id':'gist-curation','entry_id':c['id'],'url':c['official_evidence_url']}
        add(c)
    entries.sort(key=lambda r:r['id'])
    sources=[dict(id='sirerun-openapi',url='https://github.com/sirerun/mcp-registry',revision=lock['local_revision'],sha256=lock['local_sha256'],retrieved_at=lock['retrieved_at'],license='Apache-2.0',attribution='Sire Run mcp-registry contributors; OpenAPI metadata, upstream verification flags not imported.'),dict(id='mcp-official',url=BASE,revision=lock['official_sha256'],sha256=lock['official_sha256'],retrieved_at=lock['retrieved_at'],license='MIT (registry repository; publisher metadata retains publisher rights)',attribution='Model Context Protocol Registry and individual server publishers. Registry membership is not vendor endorsement or qualification.'),dict(id='gist-curation',url='https://github.com/sirerun/gist',revision=lock.get('curated_revision','initial-zati-essentials-v1'),sha256=digest((HERE/'curated.json').read_bytes()),retrieved_at=lock.get('curated_retrieved_at',lock['retrieved_at']),license='Apache-2.0',attribution='Gist maintainers; vendor-linked setup recommendations, no tested-workflow claim.')]
    content=json.dumps({'sources':sources,'entries':entries},sort_keys=True,separators=(',',':')).encode()
    snapshot=dict(format='gist.directory.snapshot/v1',revision='sha256:'+digest(content),generated_at=max(s['retrieved_at'] for s in sources),sources=sources,entries=entries)
    dump(ROOT/'snapshot.json',snapshot)
    dump(HERE/'receipt.json',dict(local_input=len(local['apis']),official_input=official_count,official_active_latest_skipped=skipped,official_complete=lock['official_complete'],accepted=len(entries),by_kind={kind:sum(r['kind']==kind for r in entries) for kind in ('mcp','openapi')},curated_recommended=sum(bool(r.get('recommended')) for r in curated),featured=0,duplicate_count=len(duplicates),duplicates=duplicates,rejected_count=len(rejected),rejected=rejected,revision=snapshot['revision']))
    print(json.dumps({k:v for k,v in json.loads((HERE/'receipt.json').read_text()).items() if k not in ('rejected','duplicates')}))
if __name__=='__main__':main()
