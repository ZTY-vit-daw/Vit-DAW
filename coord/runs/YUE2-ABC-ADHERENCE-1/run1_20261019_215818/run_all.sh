#!/bin/bash
# YUE2-ABC-ADHERENCE-1 pipeline: render each stimulus, transcribe back.
ADH=$(cat /tmp/adh_dir.txt)
cd "$ADH" || exit 1
for req in stimuli/*.json; do
  tag=$(basename "$req" .json)
  if [ -f "transcriptions/$tag.abc" ]; then echo "$tag SKIP(done)"; continue; fi
  t0=$(date +%s)
  resp=$(curl -s -X POST http://127.0.0.1:8087/synth \
    -H "Content-Type: application/json; charset=utf-8" --data-binary @"$req")
  id=$(echo "$resp" | sed -E 's/.*"id"[: ]*"([^"]+)".*/\1/')
  case "$id" in *id*) ;; *) echo "$tag RENDER_FAIL $resp"; continue;; esac
  while :; do
    st=$(curl -s "http://127.0.0.1:8087/job?id=$id")
    [ "$st" != '{"status":"running"}' ] && break
    sleep 3
  done
  t1=$(date +%s)
  case "$st" in *done*) ;; *) echo "$tag RENDER_BADSTATUS $st"; continue;; esac
  curl -s "http://127.0.0.1:8087/job?id=$id&result=1" \
    -o "renders/${tag}_raw.bin" -D "renders/${tag}_headers.txt"
  python parse_render.py "renders/${tag}_raw.bin" "renders/$tag.wav" "renders/${tag}_replay.json" \
    || { echo "$tag PARSE_FAIL"; continue; }
  [ -f "renders/$tag.wav" ] || { echo "$tag NO_WAV"; continue; }
  t2=$(date +%s)
  tr=$(curl -s -X POST http://127.0.0.1:8087/transcribe -F "audio=@renders/$tag.wav")
  tid=$(echo "$tr" | sed -E 's/.*"id"[: ]*"([^"]+)".*/\1/')
  case "$tid" in *id*) ;; *) echo "$tag TRANSCRIBE_FAIL $tr"; continue;; esac
  while :; do
    ts=$(curl -s "http://127.0.0.1:8087/job?id=$tid")
    [ "$ts" != '{"status":"running"}' ] && break
    sleep 3
  done
  t3=$(date +%s)
  case "$ts" in *done*) ;; *) echo "$tag TRANSCRIBE_BADSTATUS $ts"; continue;; esac
  curl -s "http://127.0.0.1:8087/job?id=$tid&result=1" -o "transcriptions/${tag}_tr.json"
  python -c "import json,sys; j=json.load(open('transcriptions/${tag}_tr.json',encoding='utf-8')); open('transcriptions/${tag}.abc','w',encoding='utf-8').write(j.get('abc',''))"
  [ -s "transcriptions/$tag.abc" ] || { echo "$tag EMPTY_ABC"; continue; }
  echo "$tag OK render=$((t1-t0))s fetch=$((t2-t1))s transcribe=$((t3-t2))s"
done
echo "ALL_DONE"
