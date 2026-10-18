#!/usr/bin/env python3
"""Check the shipped UI against synthetic API responses, never provider accounts.

Serve the real webui Handler on an ephemeral local port, then run this with
--base-url. Playwright and a compatible Chromium must be supplied by the runner.
Go integration tests separately qualify the real session/catalog adapter.
"""
import argparse
import json
import re
from pathlib import Path
from urllib.parse import parse_qs, urlparse

from playwright.sync_api import sync_playwright, expect


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--base-url', required=True)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--chromium')
    args = parser.parse_args()
    base = args.base_url.rstrip('/')
    if urlparse(base).hostname not in ('127.0.0.1', 'localhost'):
        parser.error('Use an owned local preview; this check intercepts synthetic APIs.')
    args.output.mkdir(parents=True, exist_ok=True)
    state = {'mode': 'signed-out'}
    calls = []
    item = {'kind': 'skill', 'id': 'example/inbox', 'version': '1.0.0',
            'state': 'published', 'title': 'Fixture inbox briefing',
            'description': 'Synthetic read-only mail workflow.',
            'digest': {'algorithm': 'sha256', 'value': 'a' * 64},
            'metadata': {'name': 'Fixture inbox briefing', 'description': 'Synthetic workflow'}}

    def api(route):
        u = urlparse(route.request.url)
        q = parse_qs(u.query)
        calls.append({'path': u.path, 'method': route.request.method, 'query': q})
        def reply(status, data):
            route.fulfill(status=status, content_type='application/json',
                          headers={'Cache-Control': 'no-store'}, body=json.dumps(data))
        if u.path == '/app/api/logout':
            assert route.request.method == 'POST'
            state['mode'] = 'signed-out'
            route.fulfill(status=204, body='')
        elif state['mode'] == 'signed-out':
            reply(401, {'code': 'sign_in_required', 'message': 'Sign in to open the library.'})
        elif state['mode'] == 'unavailable':
            reply(503, {'code': 'service_unavailable', 'message': 'The library is temporarily unavailable.'})
        elif u.path == '/app/api/session':
            reply(200, {'authenticated': True, 'workspace_id': 'fixture',
                        'workspace_name': 'Fixture workspace', 'subject': '123'})
        elif u.path == '/app/api/artifact':
            if state['mode'] == 'missing':
                reply(404, {'code': 'not_found', 'message': 'This version is unavailable.'})
            else:
                value = dict(item)
                if state['mode'] == 'hostile':
                    value['title'] = '<img src=x onerror="window.injected=true">'
                    value['description'] = '<script>window.injected=true</script>'
                reply(200, value)
        elif u.path == '/app/api/catalog':
            values = [] if state['mode'] == 'empty' or q.get('q') == ['no-match'] else [item]
            if q.get('kind') == ['tool']:
                values = [{**item, 'kind': 'tool', 'title': 'Fixture mail tool'}]
            next_cursor = 'fixture-next' if state['mode'] == 'paged' and not q.get('cursor') else ''
            reply(200, {'items': values, 'next_cursor': next_cursor})
        else:
            reply(404, {'code': 'not_found'})

    with sync_playwright() as p:
        launch = {'headless': True}
        if args.chromium:
            launch['executable_path'] = args.chromium
        browser = p.chromium.launch(**launch)
        context = browser.new_context(viewport={'width': 1440, 'height': 1100})
        context.route(base + '/app/api/**', api)
        page = context.new_page()
        errors = []
        page.on('pageerror', lambda e: errors.append(str(e)))

        def visit(path):
            response = page.goto(base + path)
            assert response.status == 200, (path, response.status)
            page.wait_for_function('Array.from(document.images).every(i=>i.complete && i.naturalWidth>0)')
        def no_overflow():
            assert page.evaluate('document.documentElement.scrollWidth <= innerWidth + 1'), page.url

        visit('/')
        expect(page.get_by_role('heading', name='Give your AI the right skills.')).to_be_visible()
        expect(page.get_by_role('link', name='Explore the library')).to_be_visible()
        no_overflow()
        page.screenshot(path=str(args.output / 'home-desktop.png'), full_page=True)
        visit('/signin')
        start = page.locator('a[href^="/auth/github/start"]')
        expect(start).to_be_visible()
        assert parse_qs(urlparse(start.get_attribute('href')).query)['return_to'] == ['/workspace']
        for path in ('/developers', '/docs'):
            visit(path)
            no_overflow()
        visit('/workspace')
        expect(page.locator('#catalog-notice')).to_contain_text(re.compile('sign.in', re.I))

        state['mode'] = 'signed-in'
        visit('/workspace')
        expect(page.locator('#catalog-results')).to_contain_text(item['title'])
        page.screenshot(path=str(args.output / 'library-desktop.png'), full_page=True)
        page.get_by_role('button', name='Tools', exact=True).click()
        expect(page.locator('#catalog-results')).to_contain_text('Fixture mail tool')
        page.get_by_role('button', name='All', exact=True).click()
        page.get_by_label('Search the catalog', exact=True).fill('no-match')
        page.locator('#catalog-search').get_by_role('button', name='Search').click()
        expect(page.locator('#catalog-results')).not_to_contain_text(item['title'])
        expect(page.locator('#catalog-notice')).to_contain_text('match')
        before = len([c for c in calls if c['path'].endswith('/catalog')])
        page.get_by_label('Search the catalog', exact=True).fill('é' * 101)
        page.locator('#catalog-search').get_by_role('button', name='Search').click()
        expect(page.locator('#catalog-notice')).to_contain_text('shorter search')
        expect(page.get_by_role('button', name='Try again')).to_have_count(0)
        assert len([c for c in calls if c['path'].endswith('/catalog')]) == before
        page.get_by_label('Search the catalog', exact=True).fill('é' * 100)
        page.locator('#catalog-search').get_by_role('button', name='Search').click()
        expect(page.locator('#catalog-results')).to_contain_text(item['title'])
        state['mode'] = 'empty'
        visit('/workspace')
        expect(page.locator('#catalog-notice')).to_contain_text('no published catalog records')
        state['mode'] = 'paged'
        visit('/workspace')
        expect(page.locator('#load-more')).to_be_visible()
        page.locator('#load-more').click()
        expect(page.locator('#load-more')).to_be_hidden()
        assert any(c['query'].get('cursor') == ['fixture-next'] for c in calls)

        state['mode'] = 'signed-in'
        visit('/library/item?kind=skill&id=example%2Finbox&version=1.0.0')
        expect(page.get_by_role('heading', name=item['title'])).to_be_visible()
        expect(page.locator('#artifact')).to_contain_text('1.0.0')
        expect(page.locator('#artifact')).to_contain_text('a' * 64)
        page.screenshot(path=str(args.output / 'detail-desktop.png'), full_page=True)
        state['mode'] = 'hostile'
        visit('/library/item?kind=skill&id=example%2Finbox&version=1.0.0')
        expect(page.locator('#artifact')).to_contain_text('<img')
        assert not page.evaluate('Boolean(window.injected)')
        assert page.locator('#artifact img').count() == 0
        state['mode'] = 'missing'
        visit('/library/item?kind=skill&id=example%2Finbox&version=1.0.0')
        expect(page.locator('#artifact')).to_contain_text('not found')
        state['mode'] = 'unavailable'
        visit('/workspace')
        expect(page.locator('#catalog-notice')).to_contain_text('unavailable')
        expect(page.get_by_role('button', name='Try again')).to_be_visible()
        state['mode'] = 'signed-in'
        page.get_by_role('button', name='Try again').click()
        expect(page.locator('#catalog-results')).to_contain_text(item['title'])
        visit('/workspace')
        page.get_by_role('button', name='Sign out').click()
        expect(page.get_by_role('link', name='Sign in with GitHub')).to_be_visible()
        assert any(c['path'].endswith('/logout') for c in calls)

        for width in (390, 320, 768):
            page.set_viewport_size({'width': width, 'height': 844})
            for path in ('/', '/signin', '/developers', '/docs', '/workspace'):
                visit(path)
                no_overflow()
                if path == '/' and width == 390:
                    page.screenshot(path=str(args.output / 'home-mobile.png'), full_page=True)
        assert page.evaluate('localStorage.length') == 0
        assert not errors, errors
        context.close()
        browser.close()
    (args.output / 'browser-receipt.json').write_text(json.dumps({
        'passed': True, 'api': 'synthetic intercepted responses',
        'provider_calls': 0, 'viewports': [1440, 768, 390, 320],
        'flows': ['public pages', 'sign-in link', 'signed-out', 'search', 'kind filter',
                  'empty', 'paging', 'exact detail', 'metadata injection', 'missing',
                  'service error and retry', 'logout', 'responsive layout'],
        'api_requests': len(calls), 'page_errors': errors}, indent=2) + '\n')
    print('PASS: browser journeys; synthetic API responses; no provider calls')


if __name__ == '__main__':
    main()
