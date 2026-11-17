import pathlib,re,subprocess,json,sys
root=pathlib.Path(sys.argv[1]);run=pathlib.Path(sys.argv[2])
old=subprocess.check_output(['git','-C',str(root),'show','24c6664:VitShell/tests/win32_noactivate_restore_probe.cpp'],text=True,encoding='utf8')
new=(root/'VitShell/tests/win32_noactivate_restore_probe.cpp').read_text(encoding='utf8')
def section(s,start,end):return s.split(start,1)[1].split(end,1)[0].replace('\r\n','\n')
checks={
 'geometryTrace_unchanged':section(old,'static const char* geometryTrace(','static const char* readGeometryTrace(')==section(new,'static const char* geometryTrace(','static const char* readGeometryTrace('),
 'JSONL_decoder_unchanged':section(old,'static const char* readGeometryTrace(','static int geometryTraceExit(')==section(new,'static const char* readGeometryTrace(','static int geometryTraceExit('),
 'godot_candidate_path_unchanged':section(old,'static int godotProbe(','int WINAPI wWinMain(')==section(new,'static int godotProbe(','int WINAPI wWinMain('),
 'mutation_helpers_unchanged':section(old,'static GeometryMutation mutationBegin(','static bool convertRect(')==section(new,'static GeometryMutation mutationBegin(','struct SameDpiDomain'),
 'native_geometry_tolerance_unchanged':section(old,'static bool geometryNativeNormal(','static GeoRect jsonRect(')==section(new,'static bool geometryNativeNormal(','static GeoRect jsonRect(')
}
(run/'unchanged_criteria_checks.json').write_text(json.dumps(checks,indent=2)+'\n')
print(json.dumps(checks));sys.exit(0 if all(checks.values()) else 1)