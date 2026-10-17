#!/usr/bin/env python3
"""Reproduce retained native parity using stdlib only; no runtime/inference."""
import gzip,hashlib,json,math
from pathlib import Path
BASE=Path(__file__).resolve().parent
OLD=BASE.parent/'inbox-development-results'
def read(p):return json.loads(p.read_text())
def packed(p):return json.loads(gzip.decompress(p.read_bytes()))
def main():
 results=BASE/'results';manifest=read(results/'manifest.json');parity=read(results/'parity.json')
 assert manifest['source']=='b29492d4659a93b07d9b501ce664a8cbf6a84989'
 tokens=packed(results/'tokens.json.gz');assert len(tokens)==39
 assert all(t['equal'] and t['native']==t['reference'] and t['native'][0]==2 and t['native'][-1]==1 for t in tokens)
 checks=[]
 for dim in (256,768):
  native=packed(results/f'native-{dim}.json.gz');assert native['model_version']==manifest['native_identity'][str(dim)]
  reference=packed(OLD/f'vectors-{dim}.json.gz')['entries']+packed(results/f'python-probes-{dim}.json.gz')['entries']
  assert len(native['entries'])==len(reference)==39
  for i,(n,r) in enumerate(zip(native['entries'],reference,strict=True)):
   assert (n['purpose'],n['text'])==(r['purpose'],r['text'])
   assert len(n['vector'])==dim and all(math.isfinite(x) for x in n['vector'])
   norm=math.sqrt(sum(x*x for x in n['vector']));rnorm=math.sqrt(sum(x*x for x in r['vector']))
   cosine=sum(x*y for x,y in zip(n['vector'],r['vector'],strict=True))/(norm*rnorm)
   assert abs(norm-1)<=1e-5 and cosine>=.9999
   checks.append({'dimension':dim,'index':i,'cosine':cosine,'native_norm':norm,'passed':True})
 assert checks==parity['checks'] and parity['status']=='PASS'
 records=0
 for dim in (256,768):
  for kind in ('source_cold','source_warm'):
   old={(r['id'],r['arm']):r for r in map(json.loads,gzip.decompress((OLD/f'{dim}-k4-{kind}.jsonl.gz').read_bytes()).decode().splitlines())}
   rows=list(map(json.loads,gzip.decompress((results/f'cohort-{dim}-{kind}.jsonl.gz').read_bytes()).decode().splitlines()));assert len(rows)==210
   for n in rows:
    r=old[(n['id'],n['arm'])];assert n['source']==manifest['source']
    assert n['policy_version']==r['policy_version'] and n['catalog_sha256']==r['catalog_sha256'] and n['inputs_sha256']==r['inputs_sha256']
    assert all(n.get(f)==r.get(f) for f in ('status','comparison_status','selected','complete_context','tools','capabilities'))
    if n['arm'] in ('E0','H0','H1'):assert n['model_version']==manifest['native_identity'][str(dim)]
   records+=len(rows)
 assert records==840 and read(results/'closure-parity.json')['all_equal']
 assert read(results/'overflow.json')=={'http_status':500,'numeric_output_accepted':False,'token_count':8203}
 assert read(results/'owned-bind.json')['exact_child_bind_line_observed_before_http']
 assert read(results/'server-log-provenance.json')['child_reaped']
 if (BASE/'SHA256SUMS').exists():
  for line in (BASE/'SHA256SUMS').read_text().splitlines():
   expected,name=line.split('  ',1);assert hashlib.sha256((BASE/name).read_bytes()).hexdigest()==expected,name
 print(json.dumps({'exact_token_inputs':39,'vector_comparisons':78,'minimum_cosine':min(c['cosine'] for c in checks),'cohort_records':records,'owned_bind_before_http':True,'status':'PASS'},sort_keys=True))
if __name__=='__main__':main()
