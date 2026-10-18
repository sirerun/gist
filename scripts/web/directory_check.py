#!/usr/bin/env python3
"""Exercise embedded directory UI with synthetic public/private API responses."""
import argparse,json
from pathlib import Path
from urllib.parse import urlparse,parse_qs
from playwright.sync_api import sync_playwright,expect

def main():
    ap=argparse.ArgumentParser();ap.add_argument('--base-url',required=True);ap.add_argument('--output',type=Path,required=True);ap.add_argument('--chromium');a=ap.parse_args()
    assert urlparse(a.base_url).hostname in ('127.0.0.1','localhost')
    a.output.mkdir(parents=True,exist_ok=True);base=a.base_url.rstrip('/')
    record={'id':'fixture/mail','service_id':'mail','title':'Fixture mail','description':'Synthetic setup metadata','version':'1.0.0','kind':'mcp','source':{'id':'fixture','entry_id':'mail','url':'https://example.com/source'},'remotes':[{'url':'https://example.com/mcp','transport':'streamable-http'}],'auth':{'type':'bearer','credential_names':['MAIL_TOKEN']},'recommended':True,'featured':False}
    state={'private':False,'mode':'normal','saved':[]};calls=[]
    def api(route):
        u=urlparse(route.request.url);q=parse_qs(u.query);method=route.request.method;calls.append((u.path,method,q))
        def reply(status,data):route.fulfill(status=status,content_type='application/json',body=json.dumps(data))
        if u.path.startswith('/app/api/'):
            if not state['private']:reply(401,{'code':'sign_in_required'});return
            if u.path.endswith('/session'):reply(200,{'authenticated':True,'workspace_name':'Fixture workspace','csrf_token':'synthetic-csrf'});return
            if u.path.endswith('/directory'):
                if method=='POST':
                    assert route.request.headers['x-gist-csrf']=='synthetic-csrf'
                    r=json.loads(route.request.post_data)['record'];assert 'token' not in r['auth'];state['saved'].append({'id':'fixture-entry','record':r});reply(201,state['saved'][-1]);return
                if method=='DELETE':
                    assert route.request.headers['x-gist-csrf']=='synthetic-csrf';state['saved']=[];route.fulfill(status=204);return
                reply(200,{'items':state['saved']});return
            reply(200,{'items':[]});return
        if state['mode']=='error':reply(503,{'code':'service_unavailable'});return
        r=dict(record)
        if state['mode']=='hostile':r.update(title='<img src=x onerror="window.injected=true">',description='<script>window.injected=true</script>',homepage='javascript:alert(1)')
        if u.path.endswith('/item'):reply(200,r);return
        if u.path.endswith('/export'):reply(200,{'format':'gist.directory.entry/v1','record':r});return
        values=[] if q.get('q')==['no-match'] else [r]
        cursor='next-page' if state['mode']=='paged' and 'cursor' not in q else ''
        reply(200,{'items':values,'total':6504,'snapshot_revision':'fixture','next_cursor':cursor})
    with sync_playwright() as p:
        opts={'headless':True}
        if a.chromium:opts['executable_path']=a.chromium
        browser=p.chromium.launch(**opts);context=browser.new_context(viewport={'width':1440,'height':1100},accept_downloads=True)
        context.route(base+'/api/directory**',api);context.route(base+'/app/api/**',api)
        page=context.new_page();errors=[];page.on('pageerror',lambda e:errors.append(str(e)))
        def visit(path):assert page.goto(base+path).status==200
        def overflow():assert page.evaluate('document.documentElement.scrollWidth <= innerWidth + 1'),page.url
        visit('/library');expect(page.locator('#directory-results')).to_contain_text('Fixture mail');expect(page.locator('#directory-results')).not_to_contain_text('Featured');expect(page.locator('#directory-status')).to_contain_text('6,504');overflow()
        page.screenshot(path=str(a.output/'directory-desktop.png'),full_page=True)
        page.get_by_role('button',name='OpenAPI',exact=True).click();assert any(c[2].get('kind')==['openapi'] for c in calls)
        page.get_by_label('Search integrations',exact=True).fill('no-match');page.locator('#directory-search').get_by_role('button').click();expect(page.locator('#directory-notice')).to_contain_text('No matching')
        state['mode']='paged';visit('/library');expect(page.locator('#directory-more')).to_be_visible();page.locator('#directory-more').click();expect(page.locator('#directory-more')).to_be_hidden()
        state['mode']='error';visit('/library');expect(page.get_by_role('button',name='Try again',exact=True)).to_be_visible();state['mode']='normal';page.get_by_role('button',name='Try again',exact=True).click();expect(page.locator('#directory-results')).to_contain_text('Fixture mail')
        visit('/library/server?id=fixture%2Fmail');expect(page.locator('#directory-detail')).to_contain_text('workflow');expect(page.locator('#directory-detail')).to_contain_text('https://example.com/mcp')
        with page.expect_download() as info:page.get_by_role('button',name='Export setup descriptor').click()
        assert json.loads(Path(info.value.path()).read_text())['format']=='gist.directory.entry/v1'
        state['mode']='hostile';visit('/library/server?id=fixture%2Fmail');expect(page.locator('#directory-detail')).to_contain_text('<img');assert not page.evaluate('Boolean(window.injected)');assert page.locator('#directory-detail img').count()==0;assert page.locator('#directory-detail a[href^="javascript:"]').count()==0
        state['mode']='normal';visit('/workspace/servers');expect(page.locator('#private-notice')).to_contain_text('Sign in');state['private']=True;visit('/workspace/servers');expect(page.locator('#private-create')).to_be_visible()
        page.get_by_label('Name',exact=True).fill('My local server');page.get_by_label('Endpoint, spec URL or local executable path',exact=True).fill('http://localhost:8787/mcp');page.get_by_role('button',name='Save private integration',exact=True).click();expect(page.locator('#private-results')).to_contain_text('My local server');expect(page.locator('#private-save-status')).to_contain_text('Saved privately');assert state['saved'][0]['record']['remotes'][0]['url']=='http://localhost:8787/mcp'
        page.screenshot(path=str(a.output/'private-desktop.png'),full_page=True);page.get_by_role('button',name='Remove',exact=True).click();expect(page.locator('#private-results')).to_contain_text('No private integrations')
        for width in (390,320,768):
            page.set_viewport_size({'width':width,'height':844})
            for path in ('/','/library','/library/server?id=fixture%2Fmail','/workspace','/workspace/servers'):
                visit(path)
                if path=='/library':expect(page.locator('#directory-results')).to_contain_text('Fixture mail')
                overflow()
                if width==390 and path=='/library':page.screenshot(path=str(a.output/'directory-mobile.png'),full_page=True)
        assert not errors,errors;assert page.evaluate('localStorage.length')==0;browser.close()
    (a.output/'receipt.json').write_text(json.dumps({'passed':True,'api':'synthetic intercepted','provider_calls':0,'viewports':[1440,768,390,320],'requests':len(calls),'flows':['anonymous browsing','search','filters','paging','retry','metadata injection','descriptor export','private session','CSRF write','remove','responsive']},indent=2)+'\n')
    print('PASS directory UI synthetic journeys')
if __name__=='__main__':main()
