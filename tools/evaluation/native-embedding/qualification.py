#!/usr/bin/env python3
"""Pinned CPU qualification; run under24GiB/no-swap/4CPU/2700s cgroup.
No downloads/providers. Hard preregistered parity thresholds; failure stops.
"""
import argparse,gzip,hashlib,json,math,os,secrets,signal,socket,subprocess,tempfile,time,urllib.request,urllib.error
from pathlib import Path
HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[2]
DATA=ROOT/'tools/evaluation/inbox-development'
OLD=ROOT/'tools/evaluation/inbox-development-results'
BINARY_SHA='436cc65b98927488311f1682658ec8b14f4e833107a37f5ce546225e9a3f9fcc'
MODEL_SHA='68bae29d62fb8c7d23e98d21fd4662753ddd636e6b62b8a70dfe059a9844f216'
PREFIX={'query':'task: search result | query: ','document':'title: none | text: '}
def sha(b):return hashlib.sha256(b).hexdigest()
def emit(p,j):p.write_text(json.dumps(j,sort_keys=True,indent=2,allow_nan=False)+'\n')
def pack(p,b):p.write_bytes(gzip.compress(b,mtime=0));return {'sha256':sha(p.read_bytes()),'uncompressed_sha256':sha(b)}
def main():
 ap=argparse.ArgumentParser(description=__doc__);ap.add_argument('--source',required=True);ap.add_argument('--binary',required=True);ap.add_argument('--model',required=True);ap.add_argument('--python',required=True);ap.add_argument('--reference-model',required=True);a=ap.parse_args()
 out=HERE/'results';out.mkdir(exist_ok=True);commands=[];started=time.monotonic()
 assert sha(Path(a.binary).read_bytes())==BINARY_SHA,'binary pin mismatch'
 assert sha(Path(a.model).read_bytes())==MODEL_SHA,'model pin mismatch'
 env=dict(os.environ,GOMAXPROCS='4',OMP_NUM_THREADS='4',OPENBLAS_NUM_THREADS='4',HF_HUB_OFFLINE='1',TRANSFORMERS_OFFLINE='1');env['PATH']='/usr/local/go/bin:'+env.get('PATH','')
 def run(name,cmd,limit=600):
  begin=time.monotonic();p=subprocess.run(cmd,cwd=ROOT,env=env,timeout=limit,capture_output=True,check=False)
  public=p.stderr.decode(errors='replace')
  for value,replacement in ((str(ROOT),'$SOURCE'),(str(Path(a.python).parents[2]),'$RUNTIME'),(str(Path(a.binary).parents[2]),'$NATIVE')):public=public.replace(value,replacement)
  (out/(name+'.log')).write_text(public);commands.append({'stage':name,'returncode':p.returncode,'seconds':time.monotonic()-begin,'raw_stderr_sha256':sha(p.stderr),'portable_stderr_sha256':sha(public.encode())});emit(out/'commands.json',commands)
  if p.returncode:pack(out/(name+'.failed-stdout.gz'),p.stdout);raise RuntimeError('qualification stage failed: '+name)
  return p.stdout
 with tempfile.TemporaryDirectory(prefix='gist-native-private-') as temp:
  work=Path(temp);key=secrets.token_hex(32);keyfile=work/'key';keyfile.write_text(key);keyfile.chmod(0o600)
  server_socket=socket.socket();server_socket.bind(('127.0.0.1',0));port=server_socket.getsockname()[1];server_socket.close()
  origin=f'http://127.0.0.1:{port}';alias='gist-native-embeddinggemma2-qualified'
  nativecmd=[a.binary,'--model',a.model,'--embedding','--pooling','mean','--embd-normalize','2','--ctx-size','8192','--batch-size','8192','--ubatch-size','8192','--parallel','1','--threads','4','--threads-batch','4','--n-gpu-layers','0','--no-warmup','--host','127.0.0.1','--port',str(port),'--alias',alias,'--api-key-file',str(keyfile)]
  rawserver=work/'server.raw.log';log=rawserver.open('wb');server=subprocess.Popen(nativecmd,stdout=log,stderr=log,env=env,start_new_session=True)
  class NoRedirect(urllib.request.HTTPRedirectHandler):
   def redirect_request(self,*args,**kwargs):return None
  opener=urllib.request.build_opener(urllib.request.ProxyHandler({}),NoRedirect())
  def http(path,body=None,auth=True):
   headers={'Content-Type':'application/json'}
   if auth:headers['Authorization']='Bearer '+key
   req=urllib.request.Request(origin+path,data=json.dumps(body).encode() if body is not None else None,headers=headers)
   with opener.open(req,timeout=30) as response:return json.load(response)
  try:
   for _ in range(100):
    if server.poll() is not None:raise RuntimeError('native server exited before readiness')
    try:models=http('/v1/models');break
    except (OSError,urllib.error.URLError):time.sleep(.1)
   else:raise RuntimeError('native readiness deadline')
   assert models['data'][0]['id']==alias,'configured model alias mismatch'
   try:http('/v1/models',auth=False);raise AssertionError('missing API key accepted')
   except urllib.error.HTTPError as e:assert e.code==401
   # All fixed35 inputs plus frozen synthetic Unicode/special-token/empty probes.
   fixed=gzip.decompress((OLD/'embedding-inputs.jsonl.gz').read_bytes());probes=(HERE/'probes.jsonl').read_bytes();requests=work/'requests.jsonl';requests.write_bytes(fixed+probes)
   rows=[json.loads(line) for line in (fixed+probes).decode().splitlines()];entries=[{'purpose':r['purpose'],'text':t} for r in rows for t in r['texts']];assert len(entries)==39
   # Exact trusted Python tokenizer IDs, no inference yet.
   token_program="import json,sys;from transformers import AutoTokenizer;t=AutoTokenizer.from_pretrained(sys.argv[1],local_files_only=True);print(json.dumps([t(x,add_special_tokens=True,truncation=False)['input_ids'] for x in json.load(sys.stdin)]))"
   texts=[PREFIX[e['purpose']]+e['text'] for e in entries]
   tp=subprocess.run([a.python,'-c',token_program,a.reference_model],input=json.dumps(texts).encode(),capture_output=True,env=env,timeout=120,check=True)
   reference_tokens=json.loads(tp.stdout);actual_tokens=[http('/tokenize',{'content':text,'add_special':True,'parse_special':True,'with_pieces':False})['tokens'] for text in texts]
   tokenchecks=[{'index':i,'purpose':e['purpose'],'text_sha256':sha(e['text'].encode()),'reference':r,'native':n,'equal':r==n} for i,(e,r,n) in enumerate(zip(entries,reference_tokens,actual_tokens,strict=True))]
   pack(out/'tokens.json.gz',json.dumps(tokenchecks,sort_keys=True).encode())
   if not all(c['equal'] for c in tokenchecks):emit(out/'parity.json',{'status':'FAILED_TOKEN_PARITY','checks':len(tokenchecks),'mismatched_indices':[c['index'] for c in tokenchecks if not c['equal']]});raise RuntimeError('exact token parity failed; no threshold relaxation')
   config=work/'config.json';emit(config,{'Origin':origin,'APIKey':key,'BinarySHA256':BINARY_SHA,'ModelSHA256':MODEL_SHA,'Dimension':768,'Alias':alias});config.chmod(0o600)
   gobin=work/'nativevectors';prepeval=work/'prepeval'
   run('build-nativevectors',['go','build','-p','4','-o',str(gobin),'./tools/evaluation/native-embedding/cmd/nativevectors'])
   run('build-prepeval',['go','build','-p','4','-o',str(prepeval),'./tools/evaluation/cmd/prepeval'])
   allchecks=[];native={}
   for dim in (256,768):
    # Four extra probes use the unchanged official Python adapter; fixed35
    # references are retained immutable outputs from the previous qualified run.
    pv=work/f'python-probes-{dim}.json';pq=out/f'python-probes-{dim}-qualification.json'
    run(f'python-probes-{dim}',[a.python,str(ROOT/'tools/evaluation/embedding_adapter.py'),'--model-dir',a.reference_model,'--dtype','bfloat16','--dimension',str(dim),'--input',str(HERE/'probes.jsonl'),'--output',str(pv),'--qualification',str(pq)],1200)
    pack(out/f'python-probes-{dim}.json.gz',pv.read_bytes())
    reference=json.loads(gzip.decompress((OLD/f'vectors-{dim}.json.gz').read_bytes()))['entries']+json.loads(pv.read_bytes())['entries']
    nb=run(f'native-{dim}',[str(gobin),'--config',str(config),'--input',str(requests),'--dimension',str(dim)],1200);pack(out/f'native-{dim}.json.gz',nb);nv=json.loads(nb);native[dim]=nv
    assert len(nv['entries'])==39
    for i,(r,n) in enumerate(zip(reference,nv['entries'],strict=True)):
     assert (r['purpose'],r['text'])==(n['purpose'],n['text'])
     x=r['vector'];y=n['vector'];nx=math.sqrt(sum(v*v for v in x));ny=math.sqrt(sum(v*v for v in y));cos=sum(v*w for v,w in zip(x,y,strict=True))/(nx*ny)
     good=len(y)==dim and all(math.isfinite(v) for v in y) and abs(ny-1)<=1e-5 and cos>=.9999
     allchecks.append({'dimension':dim,'index':i,'cosine':cos,'native_norm':ny,'passed':good})
   parity={'status':'PASS' if all(c['passed'] for c in allchecks) else 'FAILED_VECTOR_PARITY','token_inputs':39,'fixed_inputs':35,'probe_inputs':4,'minimum_cosine':min(c['cosine'] for c in allchecks),'cosine_threshold':.9999,'norm_tolerance':1e-5,'checks':allchecks}
   emit(out/'parity.json',parity)
   if parity['status']!='PASS':raise RuntimeError('native vector parity failed; no threshold relaxation')
   # Actual overflow must fail, never silently truncate; no numeric result accepted.
   overflow=PREFIX['query']+'x '*8193;ot=http('/tokenize',{'content':overflow,'add_special':True,'parse_special':True})['tokens'];assert len(ot)>8192
   try:http('/v1/embeddings',{'input':overflow,'encoding_format':'float'});raise AssertionError('native overflow accepted')
   except urllib.error.HTTPError as e:emit(out/'overflow.json',{'token_count':len(ot),'http_status':e.code,'numeric_output_accepted':False})
   closure=[]
   for dim in (256,768):
    vf=work/f'native-{dim}.json';emit(vf,native[dim])
    for kind in ('source_cold','source_warm'):
     raw=run(f'cohort-{dim}-{kind}',[str(prepeval),'--source',a.source,'--inputs',str(DATA/'development.inputs.jsonl'),'--catalog',str(DATA/'catalog.json'),'--catalog-sha256','dd769302f83ac062af19bb465baf48b2f6ac9a3146da7d302e40f5c51e7db67a','--task-policy',str(DATA/'task-policy.json'),'--task-policy-sha256','79d93387f61b9532e92296efe4792b08b9ce5fbc2a6fc9fc68dcd3c3b0c85217','--top-k','4','--run-kind',kind,'--vectors',str(vf)]);pack(out/f'cohort-{dim}-{kind}.jsonl.gz',raw)
     old={ (r['id'],r['arm']):r for r in map(json.loads,gzip.decompress((OLD/f'{dim}-k4-{kind}.jsonl.gz').read_bytes()).decode().splitlines())}
     for n in map(json.loads,raw.decode().splitlines()):
      r=old[(n['id'],n['arm'])];fields=('status','comparison_status','selected','complete_context','tools','capabilities')
      closure.append({'dimension':dim,'kind':kind,'id':n['id'],'arm':n['arm'],'equal':all(n.get(f)==r.get(f) for f in fields)})
   emit(out/'closure-parity.json',{'records':len(closure),'all_equal':all(r['equal'] for r in closure),'checks':closure})
   assert all(r['equal'] for r in closure),'downstream readiness/closure mismatch'
   emit(out/'manifest.json',{'source':a.source,'binary_sha256':BINARY_SHA,'model_sha256':MODEL_SHA,'engine_commit':'620d2225cc436ab0b001e3200e590e3b286ef9ab','status':'PASS','token_inputs':39,'vector_comparisons':78,'cohort_records':len(closure),'elapsed_seconds':time.monotonic()-started,'heldout_accessed':False,'provider_calls':0,'model_downloads_during_qualification':0,'native_identity':{str(d):native[d]['model_version'] for d in native}})
  finally:
   if server.poll() is None:
    os.killpg(server.pid,signal.SIGTERM)
    try:server.wait(timeout=5)
    except subprocess.TimeoutExpired:os.killpg(server.pid,signal.SIGKILL);server.wait(timeout=5)
   log.close();raw=rawserver.read_bytes();public=raw.decode(errors='replace')
   for secret,replacement in ((key,'$KEY'),(str(work),'$PRIVATE'),(str(Path(a.binary).parents[2]),'$NATIVE')):public=public.replace(secret,replacement)
   (out/'server.portable.log').write_text(public);emit(out/'server-log-provenance.json',{'raw_sha256':sha(raw),'portable_sha256':sha(public.encode()),'child_reaped':server.poll() is not None,'exit_status':server.returncode})
if __name__=='__main__':main()
