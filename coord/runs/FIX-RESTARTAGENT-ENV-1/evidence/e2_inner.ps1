param([string]$OutDir2)
$p = Join-Path $OutDir2 "e2_grandchild_env.txt"
Remove-Item -LiteralPath $p -ErrorAction SilentlyContinue
# inner script deliberately does NOT touch any env var
Start-Process -FilePath $env:ComSpec -ArgumentList "/c", ("set > """ + $p + """") -WindowStyle Hidden -Wait
