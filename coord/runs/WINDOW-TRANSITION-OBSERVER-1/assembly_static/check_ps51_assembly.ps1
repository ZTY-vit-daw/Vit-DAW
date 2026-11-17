param([string]$Source,[string]$Root,[string]$MainTree,[string]$Dll)
$ErrorActionPreference='Stop'
$errors=$null;$tokens=$null;$ast=[Management.Automation.Language.Parser]::ParseFile($Source,[ref]$tokens,[ref]$errors)
if(@($errors).Count){$errors|Out-String|Out-File (Join-Path $Root parse_errors.txt);exit 1}
$helper=$ast.Find({param($n)$n -is [Management.Automation.Language.FunctionDefinitionAst] -and $n.Name -eq 'Get-ForensicsResourceReservation'},$true)
. ([scriptblock]::Create($helper.Extent.Text))
$header=[regex]::Unescape('# PC \u771f\u5b9e\u8fd0\u884c\u6808\u5360\u7528\u8bb0\u5f55')
$reserved=[regex]::Unescape('\u9884\u7559');$released=[regex]::Unescape('\u91ca\u653e')
$line='- 2026-11-17 18:20 +0800 '+$reserved+' / **406test**:owner=Codex `/root/restore403`'
$cases=@(@('406_reserved',@($header,$line),'406',$true),@('403_compatible',@($header,$line.Replace('406test','403test')),'403',$true),@('404_compatible',@($header,$line.Replace('406test','404test')),'404',$true),@('wrong_card',@($header,$line.Replace('406test','404test')),'406',$false),@('wrong_owner',@($header,$line.Replace('/root/restore403','/root/other')),'406',$false),@('released',@($header,$line.Replace($reserved,$released)),'406',$false),@('history',@($header,$line.Replace($reserved,$released),$line),'406',$false),@('missing_header',@($line),'406',$false),@('empty',@(),'406',$false))
$result=@();foreach($case in $cases){$accepted=$false;$reason='';try{$null=Get-ForensicsResourceReservation -Lines $case[1] -ExpectedCard $case[2];$accepted=$true}catch{$reason=$_.Exception.Message};$result += [ordered]@{case=$case[0];accepted=$accepted;expected=$case[3];pass=($accepted -eq $case[3]);reason=$reason}}
$actual=@(& git -C $MainTree show origin/main:coord/resources/PC-RUNTIME-STACK.md);if($LASTEXITCODE -ne 0){exit $LASTEXITCODE}
$actual|Select-Object -First 5|Out-File (Join-Path $Root actual_resource_utf8.txt) -Encoding utf8
$accepted=$false;$reason='';try{$null=Get-ForensicsResourceReservation -Lines $actual -ExpectedCard '406';$accepted=$true}catch{$reason=$_.Exception.Message}
[ordered]@{accepted=$accepted;expected=$false;pass=(-not $accepted);reason=$reason;boundary='Actual top record is released; rejection is required until future runtime reservation'}|ConvertTo-Json|Out-File (Join-Path $Root actual_lease_check.json) -Encoding utf8
# Execute only generation assignments and file-writing blocks from the AST; never script top-level.
$TestTransitionObserver=$true;$observerDll=$Dll;$project=Join-Path $Root generated_fixture;$null=New-Item -ItemType Directory -Path $project
$nonce='406statictestnonce000000000000001';$engineObservation=Join-Path $project observation.jsonl;$observationPath=$engineObservation.Replace('\','/')
$assemblyBlock=$ast.Find({param($n)$n -is [Management.Automation.Language.IfStatementAst] -and $n.Extent.Text.StartsWith('if ($TestTransitionObserver) {') -and $n.Extent.Text.Contains('$extensionText=@')},$true)
if($null -eq $assemblyBlock){throw 'fixture AST block missing'}
. ([scriptblock]::Create($assemblyBlock.Extent.Text))
foreach($name in @('$projectText','$sceneText','$observerText')){$assignment=$ast.Find({param($n)$n -is [Management.Automation.Language.AssignmentStatementAst] -and $n.Left.Extent.Text -eq $name},$true);if($null -eq $assignment){throw 'fixture assignment missing'};. ([scriptblock]::Create($assignment.Extent.Text))}
[IO.File]::WriteAllText((Join-Path $project project.godot),$projectText,[Text.UTF8Encoding]::new($false))
[IO.File]::WriteAllText((Join-Path $project main.tscn),$sceneText,[Text.UTF8Encoding]::new($false))
[IO.File]::WriteAllText((Join-Path $project observer.gd),$observerText,[Text.UTF8Encoding]::new($false))
$checks=@();foreach($file in @('project.godot','main.tscn','observer.gd','test.gdextension')){$p=Join-Path $project $file;$b=[IO.File]::ReadAllBytes($p);$bom=$b.Length -ge 3 -and $b[0] -eq 239 -and $b[1] -eq 187 -and $b[2] -eq 191;$checks += [ordered]@{file=$file;utf8_bom=$bom;sha256=(Get-FileHash -LiteralPath $p).Hash;first_bytes=[BitConverter]::ToString($b[0..([Math]::Min(15,$b.Length-1))])}}
$gd=[IO.File]::ReadAllText((Join-Path $project observer.gd),[Text.Encoding]::UTF8)
$descriptor=[IO.File]::ReadAllText((Join-Path $project test.gdextension),[Text.Encoding]::UTF8)
$staticOK=$gd.Contains('OS.set_environment("VIT_TEST_TRANSITION_NONCE"') -and $gd.Contains('GDExtensionManager.load_extension("res://test.gdextension")') -and $gd.Contains('GDExtensionManager.unload_extension("res://test.gdextension")') -and $gd.Contains('request == "'+$nonce+'"') -and $gd.Contains('get_tree().quit(0 if success else 2)') -and $descriptor.Contains('entry_symbol="vit_window_transition_test_init"') -and (Get-FileHash -LiteralPath $Dll).Hash -eq (Get-FileHash -LiteralPath (Join-Path $project window_transition_observer.dll)).Hash
[ordered]@{powershell_version=$PSVersionTable.PSVersion.ToString();acp=[Text.Encoding]::Default.CodePage;parser_errors=0;helper_cases=$result;fixture_files=$checks;fixture_static_ok=$staticOK;script_top_level_invoked=$false;Godot_or_DLL_loaded=$false;boundary='Generated text/entrypoint/hash contract only; no GDScript runtime parse claim'}|ConvertTo-Json -Depth 6|Out-File (Join-Path $Root static_checks.json) -Encoding utf8
if(@($result|Where-Object {-not $_.pass}).Count -or @($checks|Where-Object {$_.utf8_bom}).Count -or -not $staticOK -or $accepted){exit 1}