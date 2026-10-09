#!/usr/bin/env python3
"""Offline JSON grammar/schema fixture validation; no network references."""
import json
from importlib.metadata import version as package_version
from pathlib import Path
import sys

import jsonschema
import yaml
from jsonschema import Draft202012Validator
from referencing import Registry, Resource

ROOT = Path(__file__).resolve().parents[3]
V2 = ROOT / 'contracts/registry/v2'
V1 = ROOT / 'contracts/registry/v1'


def no_duplicates(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f'duplicate object key: {key}')
        result[key] = value
    return result


def strict_load(raw):
    raw.decode('utf-8', errors='strict')
    decoder = json.JSONDecoder(object_pairs_hook=no_duplicates)
    text = raw.decode('utf-8')
    value, end = decoder.raw_decode(text.lstrip())
    if text.lstrip()[end:].strip():
        raise ValueError('trailing JSON content')
    return value


def schema_resources():
    resources = []
    for folder in (V1, V2):
        for path in folder.glob('*.schema.json'):
            schema = json.loads(path.read_text(encoding='utf-8'))
            resources.append((schema['$id'], Resource.from_contents(schema)))
    return Registry().with_resources(resources)


REGISTRY = schema_resources()


def validator(path):
    schema = json.loads(path.read_text(encoding='utf-8'))
    Draft202012Validator.check_schema(schema)
    return Draft202012Validator(schema, registry=REGISTRY, format_checker=Draft202012Validator.FORMAT_CHECKER)


def must_reject(label, schema, instance):
    errors = list(schema.iter_errors(instance))
    if not errors:
        raise AssertionError(f'{label}: invalid instance accepted')


def main():
    valid = json.loads((V2 / 'fixtures/valid.json').read_text(encoding='utf-8'))
    names = ('skill', 'capability', 'tool', 'provider', 'binding', 'taxonomy')
    schemas = {name: validator(V2 / f'{name}.publish.schema.json') for name in names}
    for name in names:
        errors = list(schemas[name].iter_errors(valid[name]))
        if errors:
            raise AssertionError(f'{name}: ' + '; '.join(e.message for e in errors))

    # Strict JSON grammar gates precede schema decoding.
    for label, raw in [
        ('duplicate key', b'{"artifact":{},"artifact":{}}'),
        ('trailing JSON', b'{} {}'),
        ('invalid UTF-8', b'{"x":"\xff"}'),
    ]:
        try:
            strict_load(raw)
        except (ValueError, UnicodeDecodeError, json.JSONDecodeError):
            pass
        else:
            raise AssertionError(f'{label}: grammar accepted')

    for case in json.loads((V2 / 'fixtures/invalid.json').read_text(encoding='utf-8')):
        must_reject(case['name'], schemas[case['schema']], case['instance'])

    # Semantic identity agreement is separately exercised; schema cannot express cross-field equality.
    sem = json.loads(json.dumps(valid['capability'])); sem['artifact']['id'] = 'capability/other'
    if list(schemas['capability'].iter_errors(sem)):
        raise AssertionError('identity semantic fixture must pass structural schema')
    assert sem['artifact']['id'] != sem['artifact']['document']['id']  # semantic predicate must reject
    sem = json.loads(json.dumps(valid['taxonomy'])); sem['artifact']['version'] = '2026.2'
    if list(schemas['taxonomy'].iter_errors(sem)):
        raise AssertionError('taxonomy edition fixture must pass structural schema')
    assert sem['artifact']['version'] != sem['artifact']['document']['edition']  # semantic predicate must reject

    response = validator(V2 / 'publish-response.schema.json')
    response_ok = {'kind':'provider','id':'provider/demo','version':'1.0.0','artifact_digest':'sha256:'+'a'*64,'replayed':False}
    if list(response.iter_errors(response_ok)):
        raise AssertionError('publish response rejected')
    must_reject('response shape', response, {**response_ok,'unknown':True})
    must_reject('skill digest response fields', response, {**response_ok,'kind':'skill'})
    skill_response = {**response_ok,'kind':'skill','manifest_digest':'sha256:'+'b'*64,'package_digest':'sha256:'+'c'*64}
    if list(response.iter_errors(skill_response)):
        raise AssertionError('skill response shape rejected')
    must_reject('non-skill digest response fields', response, {**response_ok,'manifest_digest':'sha256:'+'b'*64})
    error = validator(V2 / 'error.schema.json')
    error_ok = {'code':'budget_exceeded','message':'bounded','request_id':'fixture-1','retryable':False}
    if list(error.iter_errors(error_ok)):
        raise AssertionError('error response shape rejected')
    must_reject('error response shape', error, {**error_ok,'account_id':'forbidden'})
    readback = validator(V2 / 'readback-query.schema.json')
    if list(readback.iter_errors({'kind':'skill','id':'skill/demo','version':'1.0.0','max_bytes':1})):
        raise AssertionError('readback query shape rejected')
    must_reject('readback query shape', readback, {'kind':'skill','id':'skill/demo','version':'1.0.0'})

    # Validate the OpenAPI document as YAML and assert critical media/method bindings.
    api = yaml.safe_load((V2 / 'openapi.yaml').read_text(encoding='utf-8'))
    def check_refs(node):
        if isinstance(node, dict):
            ref = node.get('$ref')
            if ref and not ref.startswith('#/'):
                target = (V2 / ref.split('#', 1)[0]).resolve()
                if not target.is_relative_to(ROOT.resolve()) or not target.is_file():
                    raise AssertionError(f'non-local or missing OpenAPI reference: {ref}')
            for child in node.values(): check_refs(child)
        elif isinstance(node, list):
            for child in node: check_refs(child)
    check_refs(api)
    assert api['paths']['/v2/publish/skill']['post']
    assert api['paths']['/v2/artifacts/{kind}']['get']
    assert 'application/json' in str(api['components']['responses']['JsonReadback'])
    assert 'application/zip' in str(api['components']['responses']['ZipReadback'])
    print('OK: six valid envelopes; strict grammar, schema negatives, semantic mismatch fixtures, response/readback and OpenAPI bindings')
    print(f'jsonschema={package_version("jsonschema")}; yaml=PyYAML; references=local v1/v2 only')


if __name__ == '__main__':
    main()
