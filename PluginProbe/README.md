# Plugin Probe

Plugin Probe is an isolated, observation-only VST3 inspection subsystem. It
captures plugin identity and parameter surfaces and can render fixed probe
audio without connecting to or mutating a Vit project.

It does not create profiles, infer parameter mappings, write plugin parameters,
or grant execution authority. The Go loopback adapter exposes only load,
snapshot, render, unload, and health routes.
