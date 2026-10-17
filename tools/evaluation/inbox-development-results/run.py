#!/usr/bin/env python3
"""Admitted grid: run under cgroup24GiB/no-swap/4CPU/2700s; no network/provider.
Replay with --source SHA --python QUALIFIED_PYTHON --model-dir OFFLINE_MODEL.
Recompute selection without inference using --summarize-only.
"""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
DATA = ROOT / 'tools/evaluation/inbox-development'
EXPECTED = {'development.inputs.jsonl':'eb05b86e2498835b1607704abd75ea0f021a8bb0c520ce1fea63067bd116011b',
 'development.labels.jsonl':'b8dce1bf1c8675cbd8c1714b6fde17bc037bf930f2759adcb0b07b56f9f56619',
 'task-policy.json':'79d93387f61b9532e92296efe4792b08b9ce5fbc2a6fc9fc68dcd3c3b0c85217',
 'catalog.json':'dd769302f83ac062af19bb465baf48b2f6ac9a3146da7d302e40f5c51e7db67a'}
RUNTIME = {'torch':'2.12.0.dev20260326+cu128','transformers':'5.20.1','sentence-transformers':'6.1.0','tokenizers':'0.23.3','numpy':'2.4.2'}
def sha(raw): return hashlib.sha256(raw).hexdigest()
def emit(path,obj): path.write_text(json.dumps(obj,indent=2,sort_keys=True,allow_nan=False)+'\n')
def compress(path,raw):
 path.write_bytes(gzip.compress(raw,mtime=0))
 return {'sha256':sha(path.read_bytes()),'uncompressed_sha256':sha(raw),'uncompressed_bytes':len(raw)}

def summarize():
 labels={r['id']:r for r in map(json.loads,(DATA/'development.labels.jsonl').read_text().splitlines())}
 configs=[]; total=0; all_counts={}
 for dim in (256,768):
  for k in (4,8,16):
   rows=[]
   for kind in ('source_cold','source_warm'):
    rows+=list(map(json.loads,gzip.decompress((HERE/f'{dim}-k{k}-{kind}.jsonl.gz').read_bytes()).decode().splitlines()))
   total+=len(rows); counts={}
   for r in rows:
    key=r['arm']+':'+r['status']+':'+r['comparison_status']
    counts[key]=counts.get(key,0)+1; all_counts[key]=all_counts.get(key,0)+1
   eligible=[r for r in rows if r['arm']=='H1' and r['comparison_status']=='ELIGIBLE' and r['status']!='NOT_RUN']
   complete=[r for r in eligible if r['status']=='ready' and r['complete_context']]
   exact=[r for r in complete if r['selected'] in labels[r['id']]['allowed_skill_versions']
          and set(r.get('capabilities',[]))==set(labels[r['id']]['required_capabilities'])
          and any(set(r.get('tools',[]))==set(option) for option in labels[r['id']]['eligible_tool_alternatives'])]
   configs.append({'dimension':dim,'top_k':k,'h1_eligible':len(eligible),'h1_ready_complete':len(complete),'h1_exact_closure':len(exact),
    'h1_ready_complete_rate':len(complete)/len(eligible) if eligible else None,
    'h1_exact_closure_rate':len(exact)/len(eligible) if eligible else None,'counts':counts})
 eligible=[c for c in configs if c['h1_eligible']]
 selected=max(eligible,key=lambda c:(c['h1_ready_complete_rate'],c['h1_exact_closure_rate'],-c['dimension'],-c['top_k'])) if eligible else None
 emit(HERE/'selection.json',{'schema':'gist.inbox-development.selection/v1','actual_attempt_records':total,'construction_families':1,
  'configurations':configs,'all_counts':all_counts,'selected_development_configuration':{k:selected[k] for k in ('dimension','top_k')} if selected else None,
  'selection_rule':'H1 complete positive preparation rate, then exact closure; ties smaller dimension then k',
  'uncertainty':'NOT_ESTIMABLE_FOR_GENERALIZATION: one closed-menu construction family; no noninferiority claim',
  'whole_task_quality_or_live_provider_evidence':False,'default_change':False})

def main():
 parser=argparse.ArgumentParser(description=__doc__); parser.add_argument('--source',required=True);parser.add_argument('--python',required=True);parser.add_argument('--model-dir',required=True);args=parser.parse_args()
 started=time.monotonic(); commands=[]; artifacts={}
 for name,digest in EXPECTED.items():
  if sha((DATA/name).read_bytes())!=digest: raise ValueError('frozen cohort mismatch: '+name)
 env=dict(os.environ,GOMAXPROCS='4',OMP_NUM_THREADS='4',MKL_NUM_THREADS='4',OPENBLAS_NUM_THREADS='4',HF_HUB_OFFLINE='1',TRANSFORMERS_OFFLINE='1')
 env['PATH']='/usr/local/go/bin:'+env.get('PATH','')
 def run(name,command,timeout=600):
  before=time.monotonic(); result=subprocess.run(command,cwd=ROOT,env=env,capture_output=True,timeout=timeout,check=False)
  raw=result.stderr; public=raw.decode(errors='replace')
  for private,portable in ((str(ROOT),'$SOURCE'),(args.model_dir,'$MODEL'),(args.python,'$PYTHON')): public=public.replace(private,portable)
  (HERE/(name+'.log')).write_text(public)
  commands.append({'stage':name,'returncode':result.returncode,'elapsed_seconds':time.monotonic()-before,'raw_stderr_sha256':sha(raw),'portable_stderr_sha256':sha(public.encode()),'timeout_seconds':timeout})
  emit(HERE/'commands.json',commands)
  if result.returncode:
   compress(HERE/(name+'.failed-stdout.gz'),result.stdout);raise RuntimeError('failed without retry: '+name)
  return result.stdout
 runtime=json.loads(run('runtime',[args.python,'-c',"import importlib.metadata,json,sys; print(json.dumps({'python':sys.version.split()[0],**{n:importlib.metadata.version(n) for n in "+repr(list(RUNTIME))+"}}))"]))
 if runtime!=dict(RUNTIME,python='3.12.3'):raise ValueError('qualified runtime mismatch')
 with tempfile.TemporaryDirectory(prefix='.work-',dir=HERE) as temporary:
  work=Path(temporary);binary=work/'prepeval'
  run('build',['go','build','-p','4','-o',str(binary),'./tools/evaluation/cmd/prepeval'])
  common=[str(binary),'--source',args.source,'--inputs',str(DATA/'development.inputs.jsonl'),'--catalog',str(DATA/'catalog.json'),'--catalog-sha256',EXPECTED['catalog.json'],'--task-policy',str(DATA/'task-policy.json'),'--task-policy-sha256',EXPECTED['task-policy.json']]
  requests=run('embedding-inputs',common+['--embedding-inputs']);request_file=work/'embedding-inputs.jsonl';request_file.write_bytes(requests)
  artifacts['embedding-inputs.jsonl.gz']=compress(HERE/'embedding-inputs.jsonl.gz',requests)
  for dim in (256,768):
   vectors=work/f'vectors-{dim}.json';qualification=HERE/f'embedding-{dim}.json'
   stdout=run(f'embedding-{dim}',[args.python,str(ROOT/'tools/evaluation/embedding_adapter.py'),'--model-dir',args.model_dir,'--dtype','bfloat16','--dimension',str(dim),'--input',str(request_file),'--output',str(vectors),'--qualification',str(qualification)],1200)
   (HERE/f'embedding-{dim}.stdout.json').write_bytes(stdout)
   artifacts[f'vectors-{dim}.json.gz']=compress(HERE/f'vectors-{dim}.json.gz',vectors.read_bytes())
   for k in (4,8,16):
    for kind in ('source_cold','source_warm'):
     stem=f'{dim}-k{k}-{kind}'
     attempts=run(stem,common+['--vectors',str(vectors),'--top-k',str(k),'--run-kind',kind]);attempt_file=work/(stem+'.jsonl');attempt_file.write_bytes(attempts)
     artifacts[stem+'.jsonl.gz']=compress(HERE/(stem+'.jsonl.gz'),attempts)
     report=run(stem+'-report',['python3',str(ROOT/'tools/evaluation/report.py'),'--attempts',str(attempt_file),'--labels',str(DATA/'development.labels.jsonl')])
     (HERE/(stem+'-report.json')).write_bytes(report)
 emit(HERE/'run-manifest.json',{'schema':'gist.inbox-development.run/v1','source':args.source,'frozen_artifacts':EXPECTED,'runtime':runtime,'artifacts':artifacts,
  'heldout_accessed':False,'provider_calls':0,'paid_model_calls':0,'model_downloads':0,'elapsed_seconds':time.monotonic()-started,
  'policy_version':'sha256:'+EXPECTED['task-policy.json'],'recipe_version':'inbox-v1','source_process_label_limit':'separate CLI process labels; not inference-cache state','construction_families':1,'generalization_or_adoption_claim':False})
 summarize()
if __name__=='__main__':
 if os.sys.argv[1:]==['--summarize-only']:summarize()
 else:main()
