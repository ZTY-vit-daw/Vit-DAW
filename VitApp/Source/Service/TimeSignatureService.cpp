#include "TimeSignatureService.h"

namespace vit
{

TimeSignatureService::TimeSignatureService (EditGetter editGetter, AutoPersist autoPersistAction)
    : getEdit (std::move (editGetter)),
      autoPersist (std::move (autoPersistAction))
{
}

namespace
{

// Strict unsigned-integer part for the "N/D" wire form: digits only, no
// sign/space/decimal tolerance (juce::String::getIntValue silently returns 0
// on junk — that must never become a persisted numerator/denominator).
bool parseStrictPositiveInt (const juce::String& text, int& valueOut)
{
    if (text.isEmpty() || text.length() > 9)
        return false;

    for (int i = 0; i < text.length(); ++i)
    {
        const auto c = text[i];
        if (c < '0' || c > '9')
            return false;
    }

    const int value = text.getIntValue();
    if (value < TimeSignatureService::kMinTimeSigValue || value > TimeSignatureService::kMaxTimeSigValue)
        return false;

    valueOut = value;
    return true;
}

juce::String rangeHint()
{
    return juce::String (TimeSignatureService::kMinTimeSigValue) + ".."
         + juce::String (TimeSignatureService::kMaxTimeSigValue);
}

} // namespace

juce::String TimeSignatureService::handleSetTimeSignature (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    // Discrete ints only: a JSON 2.5 or "2" is a caller error, not something
    // to round or coerce (fail-closed, same discipline as set_project_key's
    // scale whitelist; unlike set_tempo's bpm which is continuous).
    const auto numeratorVar = object.getProperty ("numerator");
    const auto denominatorVar = object.getProperty ("denominator");

    if (! numeratorVar.isInt() && ! numeratorVar.isInt64())
        return makeErrorReply ("set_time_signature requires an integer numerator field");

    if (! denominatorVar.isInt() && ! denominatorVar.isInt64())
        return makeErrorReply ("set_time_signature requires an integer denominator field");

    const auto numerator = static_cast<int> (numeratorVar);
    const auto denominator = static_cast<int> (denominatorVar);

    juce::String error;
    if (! applyToEdit (*edit, numerator, denominator, error))
        return makeErrorReply (error);

    if (! autoPersist())
        return makeErrorReply ("Time signature updated in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Time signature updated");
    response->setProperty ("time_signature", juce::String (numerator) + "/" + juce::String (denominator));
    response->setProperty ("numerator", numerator);
    response->setProperty ("denominator", denominator);
    return juce::JSON::toString (juce::var (response.release()));
}

bool TimeSignatureService::parseTimeSignatureString (const juce::String& text, int& numerator, int& denominator, juce::String& error)
{
    // W01 contract wire form (docs/LEGAME_DESIGN_CONTRACT_V0.md §4.6): a
    // single "N/D" with N,D positive integers. Everything else — missing or
    // extra '/', empty parts, signs, decimals, whitespace, out-of-range — is
    // rejected with a specific error so callers can fail closed.
    const auto trimmed = text.trim();

    if (! trimmed.containsChar ('/') || trimmed.indexOf ("/") != trimmed.lastIndexOf ("/"))
    {
        error = "time_signature must be in the form N/D";
        return false;
    }

    const auto numeratorText = trimmed.upToFirstOccurrenceOf ("/", false, false);
    const auto denominatorText = trimmed.fromLastOccurrenceOf ("/", false, false);

    int parsedNumerator = 0;
    int parsedDenominator = 0;

    if (! parseStrictPositiveInt (numeratorText, parsedNumerator))
    {
        error = "time_signature numerator '" + numeratorText + "' must be an integer in " + rangeHint();
        return false;
    }

    if (! parseStrictPositiveInt (denominatorText, parsedDenominator))
    {
        error = "time_signature denominator '" + denominatorText + "' must be an integer in " + rangeHint();
        return false;
    }

    numerator = parsedNumerator;
    denominator = parsedDenominator;
    return true;
}

bool TimeSignatureService::applyToEdit (te::Edit& edit, int numerator, int denominator, juce::String& error)
{
    if (numerator < kMinTimeSigValue || numerator > kMaxTimeSigValue
        || denominator < kMinTimeSigValue || denominator > kMaxTimeSigValue)
    {
        error = "time signature values must be integers in " + rangeHint();
        return false;
    }

    auto* timeSig = edit.tempoSequence.getTimeSig (0);

    if (timeSig == nullptr)
    {
        error = "Unable to access master time signature";
        return false;
    }

    timeSig->numerator = numerator;
    timeSig->denominator = denominator;

    // Same latent save hole as set_tempo / set_project_key: the CachedValue
    // write does not synchronously mark the edit changed, so an
    // open_project'd edit would silently skip the disk write while the
    // persist funnel reports success. Register the change explicitly.
    edit.markAsChanged();

    error = {};
    return true;
}

juce::String TimeSignatureService::makeErrorReply (const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "error");
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

} // namespace vit
