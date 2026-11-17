import ast,json,pathlib,re,sys
source=pathlib.Path(sys.argv[1]).read_text(encoding='utf-8')
section=source.split('static void write_record(',1)[1].split('observer.nonce_ascii,',1)[0]
literals=re.findall(r'"(?:\\.|[^"\\])*"',section)
format_string=''.join(ast.literal_eval(x) for x in literals)
format_string=re.sub(r'%(?:ll|l)?[ud]', '0', format_string).replace('[%s]','[0]').replace('%s','schema407')
expected=json.loads(format_string)
fixture=pathlib.Path(sys.argv[2])
rows=[json.loads(x) for x in fixture.read_text().splitlines()]
actual=next(x for x in rows if x['kind']=='native_message')
def keys(value,prefix=''):
    found=set()
    if isinstance(value,dict):
        for k,v in value.items():
            path=prefix+'.'+k if prefix else k
            found.add(path)
            found.update(keys(v,path))
    return found
expected_keys,actual_keys=keys(expected),keys(actual)
result={'schema_source':'actual observer write_record snprintf format, statically decoded','fixture':str(fixture),'missing':sorted(expected_keys-actual_keys),'extra':sorted(actual_keys-expected_keys),'field_paths':sorted(expected_keys),'pass':expected_keys==actual_keys,'observer_executed':False}
pathlib.Path(sys.argv[3]).write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps({'pass':result['pass'],'field_paths':len(expected_keys),'missing':result['missing'],'extra':result['extra']}))
sys.exit(0 if result['pass'] else 1)