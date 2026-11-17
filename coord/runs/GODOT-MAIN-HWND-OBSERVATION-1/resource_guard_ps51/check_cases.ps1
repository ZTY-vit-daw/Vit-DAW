param([string]$Source,[string]$Output)
$ErrorActionPreference='Stop'
$errors=$null;$tokens=$null
$ast=[System.Management.Automation.Language.Parser]::ParseFile($Source,[ref]$tokens,[ref]$errors)
if(@($errors).Count){throw ($errors|Out-String)}
$helper=$ast.Find({param($n)$n -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $n.Name -eq 'Get-ForensicsResourceReservation'},$true)
. ([scriptblock]::Create($helper.Extent.Text))
function From-Escape([string]$s){[regex]::Unescape($s)}
$header=From-Escape '# PC \u771f\u5b9e\u8fd0\u884c\u6808\u5360\u7528\u8bb0\u5f55'
$reserved=From-Escape '\u9884\u7559';$occupied=From-Escape '\u5360\u7528';$continued=From-Escape '\u7eed\u9884\u7559';$released=From-Escape '\u91ca\u653e';$idle=From-Escape '\u7a7a\u95f2'
$good403='- 2026-11-17 15:23 +0800 '+$reserved+' / **403test**:owner=Codex `/root/restore403`'
$good404=$good403.Replace('403test','404test')
$cases=@(@('403_reserved',@($header,$good403),'403',$true),@('404_reserved',@($header,$good404),'404',$true),@('404_occupied',@($header,$good404.Replace($reserved,$occupied)),'404',$true),@('404_continued',@($header,$good404.Replace($reserved,$continued)),'404',$true),@('wrong_owner',@($header,$good404.Replace('/root/restore403','/root/other')),'404',$false),@('owner_prefix',@($header,$good404.Replace('/root/restore403','/root/restore403-other')),'404',$false),@('wrong_card',@($header,$good403),'404',$false),@('released',@($header,$good404.Replace($reserved,$released)),'404',$false),@('idle',@($header,('- 2026-11-17 '+$idle+' / no owner')),'404',$false),@('history_match',@($header,$good404.Replace($reserved,$released),$good404),'404',$false),@('missing_header',@($good404),'404',$false),@('empty_records',@($header),'404',$false),@('empty_input',@(),'404',$false),@('unsupported_card',@($header,$good404),'405',$false))
$results=@()
foreach($case in $cases){$accepted=$false;$reason='';try{$null=Get-ForensicsResourceReservation -Lines $case[1] -ExpectedCard $case[2];$accepted=$true}catch{$reason=$_.Exception.Message};$results += [ordered]@{case=$case[0];expected_accepted=$case[3];accepted=$accepted;pass=($accepted -eq $case[3]);reason=$reason}}
[ordered]@{powershell_version=$PSVersionTable.PSVersion.ToString();edition=$PSVersionTable.PSEdition;acp=[Text.Encoding]::Default.CodePage;parser_errors=0;cases=$results;failures=@($results|Where-Object {-not $_.pass}).Count;script_top_level_invoked=$false}|ConvertTo-Json -Depth 5|Out-File -LiteralPath $Output -Encoding utf8
if(@($results|Where-Object {-not $_.pass}).Count){exit 1}