# Plugin Probe native host

`pluginprobe_vst3_worker` is a one-plugin-per-process VST3 observation worker.
It accepts newline-delimited JSON on stdin and writes one JSON response per
line. The supported operations are `load`, `snapshot`, `render`, `unload`, and
`shutdown`. Parameter writes, saved-state mutation, mappings, and project
execution are deliberately unavailable.

`pluginprobe_vst3_witness` is an isolated GUI host for human inspection and
listening with local probe audio. Closing it discards the transient instance.
