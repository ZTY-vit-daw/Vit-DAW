package capabilitycontext

func applyContextBuilderMetadata(pack *Pack, manifest ContextManifest) {
	if pack == nil {
		return
	}
	manifest = normalizeContextManifest(manifest)
	pack.ContextManifestID = manifest.ManifestID
	pack.ContextBuilder = CapabilityContextBuilderVersion
}
