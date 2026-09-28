#include "RefSchema.h"

namespace vit::refschema
{

juce::String escapeRefSegment (const juce::String& segment)
{
    static constexpr char hex[] = "0123456789ABCDEF";
    const std::string raw = segment.toStdString(); // UTF-8 bytes
    std::string out;
    out.reserve (raw.size());
    for (const unsigned char c : raw)
    {
        switch (c)
        {
            case '%':
            case '/':
            case '@':
            case '#':
            case ':':
                out.push_back ('%');
                out.push_back (hex[(c >> 4) & 0xF]);
                out.push_back (hex[c & 0xF]);
                break;
            default:
                out.push_back (static_cast<char> (c));
                break;
        }
    }
    return juce::String (out);
}

juce::String formatEvidenceRef (const juce::String& kind,
                                const juce::String& scopeKind,
                                const juce::String& scopeValue,
                                bool allTime,
                                juce::int64 sampleStart,
                                juce::int64 sampleEnd,
                                const juce::String& snapshot)
{
    // Empty segments are grammar violations on the agent parse side; the
    // builders below guarantee non-empty inputs, so this is a programmer
    // assertion, not a runtime recovery path.
    jassert (scopeKind.isNotEmpty());
    jassert (scopeValue.isNotEmpty());
    jassert (snapshot.isNotEmpty());

    juce::String window = "t=all";
    if (! allTime)
        window = "t=" + juce::String (sampleStart) + ".." + juce::String (sampleEnd);

    return juce::String (kRefSchemePrefix) + kind
         + "/" + escapeRefSegment (scopeKind)
         + ":" + escapeRefSegment (scopeValue)
         + "/" + window
         + "@" + escapeRefSegment (snapshot)
         + "#" + kRefHashUnCASed;
}

namespace
{

juce::String l3SourceSnapshot (const juce::String& sourceRevision,
                               const juce::String& filePath)
{
    return sourceRevision.isNotEmpty() ? sourceRevision : filePath;
}

} // namespace

juce::String makeL2RenderProbeRef (const juce::String& trackId,
                                   const juce::String& renderRevision)
{
    if (trackId.isNotEmpty())
        return formatEvidenceRef (kKindL2RenderProbe, kScopeKindTrack, trackId,
                                  true, 0, 0, renderRevision);
    return formatEvidenceRef (kKindL2RenderProbe, kScopeKindProject, "current",
                              true, 0, 0, renderRevision);
}

juce::String makeL3FeatureRef (const juce::String& featureName,
                               const juce::String& sourceRevision,
                               const juce::String& filePath)
{
    return formatEvidenceRef (kKindL3, kScopeKindFeature, featureName,
                              true, 0, 0, l3SourceSnapshot (sourceRevision, filePath));
}

juce::String makeL3BandRef (const juce::String& bandName,
                            const juce::String& sourceRevision,
                            const juce::String& filePath)
{
    return formatEvidenceRef (kKindL3, kScopeKindBand, bandName,
                              true, 0, 0, l3SourceSnapshot (sourceRevision, filePath));
}

} // namespace vit::refschema
