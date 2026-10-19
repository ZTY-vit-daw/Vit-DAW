#!/bin/bash
# $1 = request json file, $2 = tag
RUN=$(cat /tmp/yue2_run_dir.txt)
OUT="E:/AI/YuE2/outputs"
T0=$(date +%s)
RESP=$(curl -s -X POST http://127.0.0.1:8087/synth -H "Content-Type: application/json; charset=utf-8" --data-binary @"$1")
ID=$(echo "$RESP" | sed -E 's/.*"id"[: ]*"([^"]+)".*/\1/')
echo "$2 POST -> id=$ID"
while true; do
  ST=$(curl -s "http://127.0.0.1:8087/job?id=$ID")
  EL=$(( $(date +%s) - T0 ))
  if [ "$ST" != '{"status":"running"}' ]; then echo "$2 [$EL s] $ST"; break; fi
  if [ $EL -gt 540 ]; then echo "$2 TIMEOUT"; exit 1; fi
  sleep 3
done
curl -s "http://127.0.0.1:8087/job?id=$ID&result=1" -o "$OUT/${2}_raw.bin" -D "$RUN/${2}_headers.txt"
echo "$2 total=$(( $(date +%s) - T0 ))s"
python "$RUN/parse_result.py" "$RUN" "$OUT/${2}_raw.bin" "$OUT/${2}.wav" "$2"
ffprobe -v error -show_entries stream=codec_name,sample_rate,channels -show_entries format=duration -of csv=p=0 "$OUT/${2}.wav" | tr '\n' ' ' && echo
sha256sum "$OUT/${2}.wav"
