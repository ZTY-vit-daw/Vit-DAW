param([string]$Source,[string]$Resource,[string]$Output)
$ErrorActionPreference='Stop'
$errors=$null;$tokens=$null
$ast=[System.Management.Automation.Language.Parser]::ParseFile($Source,[ref]$tokens,[ref]$errors)
$helper=$ast.Find({param($n)$n -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $n.Name -eq 'Get-ForensicsResourceReservation'},$true)
$version=$PSVersionTable.PSVersion.ToString()
$acp=[System.Text.Encoding]::Default.CodePage
$bytes=[IO.File]::ReadAllBytes($Source)
$bom=$bytes.Length -ge 3 -and $bytes[0] -eq 239 -and $bytes[1] -eq 187 -and $bytes[2] -eq 191
$accepted=$false;$reason='';$matched=''
try {
 if(@($errors).Count){throw ($errors|Out-String)}
 . ([scriptblock]::Create($helper.Extent.Text))
 $lines=[IO.File]::ReadAllLines($Resource,[Text.Encoding]::UTF8)
 $matched=Get-ForensicsResourceReservation -Lines $lines -ExpectedCard '404'
 $accepted=$true
}catch{$reason=$_.Exception.Message}
[ordered]@{powershell_version=$version;edition=$PSVersionTable.PSEdition;acp=$acp;source_has_utf8_bom=$bom;parser_errors=@($errors).Count;actual_resource_accepted=$accepted;reason=$reason;matched_line=$matched;helper_as_parsed=$helper.Extent.Text}|ConvertTo-Json -Depth 4|Out-File -LiteralPath $Output -Encoding utf8
if(-not $accepted){exit 1}