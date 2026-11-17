import pathlib,subprocess,json,sys
fe=pathlib.Path(sys.argv[1]);run=pathlib.Path(sys.argv[2])
old=subprocess.check_output(['git','-C',str(fe),'show','5a92006:VitShell/tests/win32_noactivate_restore_probe.cpp'],text=True,encoding='utf8')
new=(fe/'VitShell/tests/win32_noactivate_restore_probe.cpp').read_text()
def part(text,start,end):return text.split(start,1)[1].split(end,1)[0]
checks={
'inset_and_same_DPI_predicate_unchanged':part(old,'static bool insetOrdinary(','static bool conversionDiagnostic(')==part(new,'static bool insetOrdinary(','static bool conversionDiagnostic('),
'freezeGeometry_unchanged':part(old,'static bool freezeGeometry(','static transition_observer::ZTrace parentZTrace(')==part(new,'static bool freezeGeometry(','static transition_observer::ZTrace parentZTrace('),
'geometryTrace_ordinary_native_gates_unchanged':part(old,'static const char* geometryTrace(','static const char* readGeometryTrace(')==part(new,'static const char* geometryTrace(','static const char* readGeometryTrace('),
'five_operations_engine_FG_teardown_unchanged':part(old,'static int godotProbe(','int WINAPI wWinMain(')==part(new,'static int godotProbe(','int WINAPI wWinMain(')
}
(run/'unchanged_gates.json').write_text(json.dumps(checks,indent=2)+'\n')
print(json.dumps(checks));sys.exit(0 if all(checks.values()) else 1)