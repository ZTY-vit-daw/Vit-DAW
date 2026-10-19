#!/bin/bash
# Salvage: all 29 jobs were successfully submitted (ids in pipeline.log) but
# mis-flagged RENDER_FAIL by a bad id check. Poll each id, fetch, transcribe.
ADH=$(cat /tmp/adh_dir.txt)
cd "$ADH" || exit 1
grep 'RENDER_FAIL' pipeline.log | while read -r tag _ resp; do
  if [ -f "transcriptions/$tag.abc" ]; then echo "$tag SKIP(done)"; continue; fi
  id=$(echo "$resp" | sed -E 's/.*"id"[: ]*"([^"]+)".*/\1/')
  if ! [[ "$id" =~ ^[0-9a-f]{8,}$ ]]; then echo "$tag BAD_ID $resp"; continue; fi
  t0=$(date +%s)
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
  if ! [[ "$tid" =~ ^[0-9a-f]{8,}$ ]]; then echo "$tag TRANSCRIBE_FAIL $tr"; continue; fi
  while :; do
    ts=$(curl -s "http://127.0.0.1:8087/job?id=$tid")
    [ "$ts" != '{"status":"running"}' ] && break
    sleep 3
  done
  t3=$(date +%s)
  case "$ts" in *done*) ;; *) echo "$tag TRANSCRIBE_BADSTATUS $ts"; continue;; esac
  curl -s "http://127.0.0.1:8087/job?id=$tid&result=1" -o "transcriptions/${tag}_tr.json"
  python -c "import json; j=json.load(open('transcriptions/${tag}_tr.json',encoding='utf-8')); open('transcriptions/${tag}.abc','w',encoding='utf-8').write(j.get('abc',''))"
  [ -s "transcriptions/$tag.abc" ] || { echo "$tag EMPTY_ABC"; continue; }
  echo "$tag OK render=$((t1-t0))s transcribe=$((t3-t2))s"
done
echo "SALVAGE_DONE"
