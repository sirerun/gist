#!/usr/bin/env python3
"""Stdlib-only artifact verification and selection reproduction; no inference."""
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import statistics
import subprocess
import sys

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
def digest(raw): return hashlib.sha256(raw).hexdigest()
def read(path): return json.loads(path.read_text())
def main():
 manifest=read(HERE/'run-manifest.json')
 data=ROOT/'tools/evaluation/inbox-development'
 for name,expected in manifest['frozen_artifacts'].items():
  assert digest((data/name).read_bytes())==expected, ('cohort hash',name)
 for name,expected in manifest['artifacts'].items():
  raw=(HERE/name).read_bytes()
  assert digest(raw)==expected['sha256'], ('compressed hash',name)
  plain=gzip.decompress(raw)
  assert digest(plain)==expected['uncompressed_sha256'] and len(plain)==expected['uncompressed_bytes'], ('raw hash',name)
 rows=[]; timings={}
 for dim in (256,768):
  qualified=read(HERE/f'embedding-{dim}.json')
  assert qualified['dimension']==dim and qualified['dtype']=='bfloat16'
  assert qualified['trusted_file_set_sha256']=='e3faeaceb35dcd6de5edb05a700a55e827ecb35c503faf9d75e272cfe195703d'
  for k in (4,8,16):
   for kind in ('source_cold','source_warm'):
    raw=gzip.decompress((HERE/f'{dim}-k{k}-{kind}.jsonl.gz').read_bytes())
    batch=list(map(json.loads,raw.decode().splitlines())); assert len(batch)==210
    identities=set()
    for r in batch:
     assert r['source']==manifest['source'] and r['inputs_sha256']==manifest['frozen_artifacts']['development.inputs.jsonl']
     assert r['catalog_sha256']==manifest['frozen_artifacts']['catalog.json'] and r['catalog_version']=='sha256:'+r['catalog_sha256']
     assert r['policy_version']==manifest['policy_version'] and r['recipe_version']==manifest['recipe_version']
     assert r['top_k']==k and r['semantic_floor']==.30 and r['rrf_constant']==60 and r['run_kind']==kind and r['attempt_index']==1
     assert r['family']=='maintained-inbox-briefing-closed-menu-v1' and r['phase']=='development'
     if r['arm'] in ('E0','H0','H1'): assert r['model_version']==qualified['model_version']
     if r['arm'] in ('C0','C1'): assert r['status']==r['comparison_status']=='NOT_RUN'
     assert r['usage']['provider_calls']==r['usage']['paid_model_calls']==0
     assert all(r['usage'][name] is None for name in ('main_model_tokens','cached_tokens','embedding_tokens','dollars'))
     identity=(r['id'],r['arm']); assert identity not in identities;identities.add(identity)
     if dim==256 and k==4 and r['arm'] not in ('C0','C1'):
      timings.setdefault(r['arm']+':'+kind,[]).append(r['elapsed_seconds'])
    rows+=batch
 assert len(rows)==2520
 before=(HERE/'selection.json').read_bytes()
 result=subprocess.run([sys.executable,str(HERE/'run.py'),'--summarize-only'],check=True,capture_output=True)
 assert before==(HERE/'selection.json').read_bytes(), 'selection not reproducible'
 # Reproduce all post-generation reports from retained records and frozen labels.
 spec=importlib.util.spec_from_file_location('report',ROOT/'tools/evaluation/report.py');report=importlib.util.module_from_spec(spec);spec.loader.exec_module(report)
 labels=list(map(json.loads,(data/'development.labels.jsonl').read_text().splitlines()))
 for dim in (256,768):
  for k in (4,8,16):
   for kind in ('source_cold','source_warm'):
    stem=f'{dim}-k{k}-{kind}'; batch=list(map(json.loads,gzip.decompress((HERE/(stem+'.jsonl.gz')).read_bytes()).decode().splitlines()))
    actual=report.aggregate(batch);actual['selected_semantic_model_version']=report.semantic_model_identity(batch);actual['source_readiness_grading']=report.grade(batch,labels)
    assert actual==read(HERE/(stem+'-report.json')), ('report',stem)
 timing_summary={key:{'attempts':len(v),'median_seconds':statistics.median(v),'maximum_seconds':max(v),'p95_nearest_rank_seconds':sorted(v)[int(.95*len(v)+.999999)-1]} for key,v in timings.items()}
 output={'schema':'gist.inbox-development.verification/v1','verified_attempts':len(rows),'reports_reproduced':12,'selection_reproduced':True,'selected_config_source_timings_exclude_inference':timing_summary}
 (HERE/'verification.json').write_text(json.dumps(output,indent=2,sort_keys=True)+'\n')
 print(json.dumps({'verified_attempts':len(rows),'reports_reproduced':12,'selection_reproduced':True}))
if __name__=='__main__': main()
