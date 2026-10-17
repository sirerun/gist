#!/usr/bin/env python3
"""Grade frozen synthetic observations; never execute the evaluated service."""
import argparse
import json
import math
from pathlib import Path


def jsonl(path):
    return [json.loads(line) for line in Path(path).read_text().splitlines() if line.strip()]


def percentile(values, p):
    if not values:
        raise ValueError('empty observations')
    return sorted(values)[math.ceil(p * len(values)) - 1]


def identities(selected):
    return {key: {'binding_id': value['Binding']['Ref']['ID'],
                  'tool_id': value['Tool']['Ref']['ID'],
                  'connection_id': value['Connection']['ID'],
                  'account_id': value['Connection']['AccountID']}
            for key, value in selected.items()}


def grade_cohort(labels, attempts, inventory=None):
    expected = {row['id']: row for row in labels}
    if len(expected) != len(labels):
        raise ValueError('duplicate labels')
    seen = set()
    rows = []
    ready_hits = false_ready = exposures = 0
    for row in attempts:
        case = row['id']
        if case in seen or case not in expected:
            raise ValueError('duplicate or unknown cohort observation: ' + case)
        seen.add(case)
        label = expected[case]
        result = row.get('result') or {}
        bundle = result.get('Bundle')
        ready = result.get('Status') == 'ready' and not row.get('error')
        actual_selected = identities(bundle.get('Selected', {})) if bundle else {}
        exact = actual_selected == label['selected']
        if inventory is not None and label['ready']:
            case_inventory = next(c for c in inventory['cases'] if c['input']['id'] == case)
            frozen = {}
            for requirement, identity in label['selected'].items():
                matches = [candidate for candidate in case_inventory['admitted_candidates'][requirement]
                           if identities({requirement: candidate})[requirement] == identity]
                if len(matches) != 1:
                    raise ValueError('label does not name exactly one inventory candidate: ' + case)
                frozen[requirement] = matches[0]
            exact = exact and bundle.get('Selected') == frozen and bundle.get('Skill') == case_inventory['request']['Skill']
            actual_tools = sorted(json.dumps({'pin': tool['Pin'], 'connection': tool['Connection']}, sort_keys=True) for tool in bundle.get('Tools', [])) if bundle else []
            frozen_tools = sorted(json.dumps({'pin': candidate['Tool'], 'connection': candidate['Connection']}, sort_keys=True) for candidate in frozen.values())
            exact = exact and actual_tools == frozen_tools
        tool_closure = False
        if bundle:
            tools = bundle.get('Tools') or []
            tool_closure = sorted((tool['Pin']['Ref']['ID'], tool['Connection']['ID'], tool['Connection']['AccountID']) for tool in tools) == sorted((item['tool_id'], item['connection_id'], item['account_id']) for item in label['selected'].values())
        disclosed = json.dumps({'result': result, 'error': row.get('error')}, sort_keys=True)
        exposed = any(token in disclosed for token in label['forbidden_disclosure'])
        false_ready += int(ready and not label['ready'])
        exposures += int(exposed)
        correct = ready == label['ready'] and not exposed
        if label['ready']:
            correct = correct and exact and tool_closure
            ready_hits += int(correct)
        else:
            correct = correct and not bundle
        rows.append({'id': case, 'expected_ready': label['ready'], 'observed_status': result.get('Status'), 'error': row.get('error'), 'exact_selected': exact, 'private_exposure': exposed, 'pass': correct, 'elapsed_ns': row['elapsed_ns'], 'response_bytes': row['response_bytes']})
    missing = sorted(set(expected) - seen)
    ready_total = sum(row['ready'] for row in labels)
    return {'cases': rows, 'missing': missing, 'expected_cases': len(labels), 'observed_cases': len(attempts), 'outcome_coverage': len(seen) / len(labels), 'ready_hits': ready_hits, 'ready_total': ready_total, 'ready_recall': ready_hits / ready_total if ready_total else None, 'false_ready': false_ready, 'private_exposures': exposures, 'pass': not missing and all(row['pass'] for row in rows)}


def grade_scale(config, runs):
    expected = {(r['logical_records'], r['private_partitions']) for r in config['scale_runs']}
    seen = set()
    rows = []
    threshold = config['thresholds']
    for run in runs:
        key = (run['logical_records'], run['private_partitions'])
        if key not in expected or key in seen:
            raise ValueError('unknown or duplicate scale run: ' + str(key))
        seen.add(key)
        metrics = {}
        checks = dict(run['checks'])
        checks['complete_safety_checks'] = all(run['checks'].get(name) is True for name in config['additional_checks'])
        checks['run_completed'] = not run.get('error')
        checks['projection_complete'] = run['snapshot_projection_candidates'] == key[0]
        for name in config['query_classes']:
            observations = run['query_classes'][name]
            latencies = [r['elapsed_ns'] / 1e6 for r in observations]
            p95 = percentile(latencies, .95)
            limit = threshold['broad_p95_ms'] if name == 'common_lexical' else threshold['exact_narrow_p95_ms']
            metrics[name] = {'samples': len(observations), 'p50_ms': percentile(latencies, .5), 'p95_ms': p95, 'max_result_bytes': max(r['result_bytes'] for r in observations), 'statuses': sorted(set(r['status'] for r in observations))}
            expected_status, expected_count = {'exact_hit': ('hit', 1), 'exact_miss': ('miss', 0), 'unique_lexical_hit': ('complete', 1), 'lexical_no_match': ('complete', 0), 'common_lexical': ('incomplete_budget', 0) if key[0] / key[1] * .8 > 4096 else ('complete', 64)}[name]
            checks[name + '_correct_outcome'] = all(not r.get('error') and r['status'] == expected_status and r['candidates'] == expected_count and 0 <= r['result_bytes'] <= 256 * 1024 and r['elapsed_ns'] >= 0 for r in observations)
            checks[name + '_cold_complete'] = len(run['cold'][name]) == key[1] and all(not r.get('error') and r['status'] == expected_status and r['candidates'] == expected_count for r in run['cold'][name])
            checks[name + '_sample_count'] = len(observations) == config['warm_samples_per_query_class']
            checks[name + '_latency'] = p95 <= limit
        checks['build_threshold'] = run['build_ns'] <= threshold['build_100k_seconds'] * 1e9
        checks['update_threshold'] = run['update_ns'] <= threshold['update_100_seconds'] * 1e9
        checks['rss_threshold'] = 0 < run['peak_rss_bytes'] <= threshold['peak_rss_bytes']
        checks['projection_measured'] = run['snapshot_projection_bytes'] > 0
        rows.append({'logical_records': key[0], 'private_partitions': key[1], 'queries': metrics, 'build_seconds': run['build_ns'] / 1e9, 'update_seconds': run['update_ns'] / 1e9, 'rebuild_seconds': run['rebuild_ns'] / 1e9, 'rollback_seconds': run['rollback_ns'] / 1e9, 'peak_rss_bytes': run['peak_rss_bytes'], 'snapshot_projection_bytes': run['snapshot_projection_bytes'], 'checks': checks, 'pass': bool(run['checks']) and all(value is True for value in checks.values())})
    missing = sorted(expected - seen)
    return {'runs': rows, 'missing': missing, 'pass': not missing and all(r['pass'] for r in rows)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', required=True)
    parser.add_argument('--labels', required=True)
    parser.add_argument('--inventory', required=True)
    parser.add_argument('--cohort', required=True)
    parser.add_argument('--scale', action='append', required=True)
    args = parser.parse_args()
    config = json.loads(Path(args.config).read_text())
    cohort = grade_cohort(jsonl(args.labels), jsonl(args.cohort), json.loads(Path(args.inventory).read_text()))
    scale = grade_scale(config, [json.loads(Path(p).read_text()) for p in args.scale])
    print(json.dumps({'cohort': cohort, 'scale': scale, 'pass': cohort['pass'] and scale['pass'], 'tokens': None, 'model_calls': 0, 'provider_calls': 0, 'claim_limits': config['claim_limits']}, indent=2, sort_keys=True))


if __name__ == '__main__':
    main()
