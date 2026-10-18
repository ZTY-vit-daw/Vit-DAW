// FIX-MASTERVOL-CALIBRATION-1 acceptance tests: project templates must carry
// the -3.0 dB master volume *fader position*, not the legacy 1.0 (+6.0 dB) value.
//
// Engine semantics pinned by KERNEL-RENDERGAIN-1 (132) evidence,
// tracktion_engine/modules/tracktion_engine/utilities/tracktion_AudioUtilities.cpp:136-156:
//
//     volumeFaderPositionToDB (pos)  = 20 * ln (pos) + 6
//     pos = 1.0        -> +6.0 dB       (fader top, NOT unity)
//     pos = 0.7408182  ->   0.0 dB      (unity)
//     pos = 0.6376282  ->  -3.0 dB      (project default)
//
// Checks:
// T1  Embedded default template + blank (new_project) template expose the
//     -3.0 dB fader position.
// T2  VitApp/Workspace/default_project.xml (headless startup project) exposes
//     the same -3.0 dB fader position.
// T3  Legacy value 1.0 still maps to +6.0 dB: existing projects are NOT
//     silently migrated (ruling (2) of FIX-MASTERVOL-CALIBRATION-1).
// T4  PCM clip boundary: a 0.9 source peak is clamped with the legacy +6.0 dB
//     template but stays below unity with the -3.0 dB templates (16/24-bit
//     export clipping no longer arrives 6 dB early). Arithmetic only; the real
//     render evidence lives in the run artifacts of this card.
// T5  scripts/gen_vit_project.py still generates a -3.0 dB fader position.
// T6  The templates no longer contain a MASTERVOLUME volume="1.0" attribute,
//     and the committed existing user project is left untouched.

#include "../Source/Core/VitPaths.h"

#include <cmath>
#include <cstdio>
#include <cstdlib>
#include <string>

#ifndef VIT_TEST_VITAPP_SOURCE_DIR
 #define VIT_TEST_VITAPP_SOURCE_DIR "../.."
#endif

#ifndef VIT_TEST_REPO_ROOT
 #define VIT_TEST_REPO_ROOT "../../.."
#endif

#define VIT_CHECK(expression) \
    ((expression) ? void() : vitFailCheck (__FILE__, __LINE__, #expression))

namespace
{

// decibelsToVolumeFaderPosition (-3.0) = exp ((-3 - 6) / 20)
constexpr double kMinus3DbFaderPosition = 0.6376281516217733;
// decibelsToVolumeFaderPosition (0.0) = exp ((0 - 6) / 20)
constexpr double kUnityFaderPosition = 0.7408182206817179;
// Legacy template value: on the fader scale this is the +6.0 dB top position.
constexpr double kLegacyTemplateFaderPosition = 1.0;

[[noreturn]] void vitFailCheck (const char* file, int line, const char* expression)
{
    std::fprintf (stderr, "%s:%d: check failed: %s\n", file, line, expression);
    std::_Exit (134);
}

void checkNear (double actual, double expected, double tolerance,
                const char* what, const char* file, int line)
{
    if (std::fabs (actual - expected) > tolerance)
    {
        std::fprintf (stderr, "%s:%d: %s: expected %.12f, got %.12f (tol %g)\n",
                      file, line, what, expected, actual, tolerance);
        std::_Exit (134);
    }
}

#define VIT_CHECK_NEAR(actual, expected, tolerance) \
    checkNear ((actual), (expected), (tolerance), #actual, __FILE__, __LINE__)

// Mirrors tracktion_AudioUtilities.cpp:136-156 (see file header). Kept local so
// this regression test needs only juce_core; the engine mapping itself is pinned
// by the 132 stack legs and by the on-disk template values checked here.
double faderPositionToDb (double position)
{
    return 20.0 * std::log (position) + 6.0;
}

double faderPositionToGain (double position)
{
    return std::pow (10.0, faderPositionToDb (position) / 20.0);
}

juce::String extractMasterVolumeFaderPosition (const juce::String& xmlText,
                                               const char* label)
{
    const auto xml = juce::XmlDocument::parse (xmlText);
    VIT_CHECK (xml != nullptr);

    const auto* masterVolume = xml->getChildByName ("MASTERVOLUME");
    VIT_CHECK (masterVolume != nullptr);

    const auto* plugin = masterVolume->getChildByName ("PLUGIN");
    VIT_CHECK (plugin != nullptr);
    VIT_CHECK (plugin->getStringAttribute ("type") == "volume");

    const auto raw = plugin->getStringAttribute ("volume");
    VIT_CHECK (raw.isNotEmpty());

    std::printf ("  %s: MASTERVOLUME volume=\"%s\"\n", label, raw.toRawUTF8());
    return raw;
}

void checkMinus3DbFaderPosition (const juce::String& xmlText,
                                 const char* label,
                                 bool expectLegacyValue = false)
{
    const auto raw = extractMasterVolumeFaderPosition (xmlText, label);
    const auto position = raw.getDoubleValue();

    if (expectLegacyValue)
    {
        VIT_CHECK_NEAR (position, kLegacyTemplateFaderPosition, 1.0e-9);
    }
    else
    {
        VIT_CHECK_NEAR (position, kMinus3DbFaderPosition, 1.0e-9);
        // The engine stores the volume property as float32; accept the float32
        // representation of the same fader position as well.
        VIT_CHECK (std::fabs (position - 0.6376281380653381) < 1.0e-9
                   || std::fabs (position - kMinus3DbFaderPosition) < 1.0e-9);
    }

    VIT_CHECK_NEAR (faderPositionToDb (position), expectLegacyValue ? 6.0 : -3.0, 1.0e-3);
}

juce::String readProjectFile (const juce::File& vitAppDir, const char* relativePath)
{
    const auto file = vitAppDir.getChildFile (relativePath);
    VIT_CHECK (file.existsAsFile());
    return file.loadFileAsString();
}

void runEmbeddedTemplateChecks()
{
    const juce::String cases[][2]
    {
        { "default_project_xml_template", vit::paths::detail::getDefaultProjectXmlTemplate() },
        { "blank_project_xml_template",   vit::paths::detail::getBlankProjectXmlTemplate() },
    };

    for (const auto& testCase : cases)
    {
        checkMinus3DbFaderPosition (testCase[1], testCase[0].toRawUTF8());
        VIT_CHECK (! testCase[1].contains ("volume=\"1.0\""));
        VIT_CHECK (testCase[1].contains ("volume=\"0.6376281516\""));
    }
}

void runOnDiskDefaultProjectCheck()
{
    const auto vitAppDir = juce::File (VIT_TEST_VITAPP_SOURCE_DIR);
    const auto xmlText = readProjectFile (vitAppDir, "Workspace/default_project.xml");
    checkMinus3DbFaderPosition (xmlText, "Workspace/default_project.xml");
    VIT_CHECK (! xmlText.contains ("volume=\"1.0\""));
}

void runLegacyValueStaysAtPlus6Db()
{
    // Ruling (2): existing projects keep their stored fader position. 1.0 is the
    // +6.0 dB fader top, so old projects stay +6.0 dB rather than being migrated.
    VIT_CHECK_NEAR (faderPositionToDb (kLegacyTemplateFaderPosition), 6.0, 1.0e-9);
    VIT_CHECK_NEAR (faderPositionToGain (kLegacyTemplateFaderPosition) / 1.99526231, 1.0, 1.0e-7);

    // Unity position sanity check (0 dB) for the mapping itself.
    VIT_CHECK_NEAR (faderPositionToDb (kUnityFaderPosition), 0.0, 1.0e-9);
    VIT_CHECK_NEAR (faderPositionToDb (kMinus3DbFaderPosition), -3.0, 1.0e-12);
}

void runPcmClipBoundaryCheck()
{
    // 132 E5 used a 0.9 peak source to expose the PCM clamp: +6.0 dB produced
    // peak 1.79566 (16/24-bit clamp) while the PCM-safe expectation after this
    // fix is 0.9 * gain(-3 dB) = 0.6372.
    constexpr double sourcePeak = 0.9;

    const auto legacyPeak = sourcePeak * faderPositionToGain (kLegacyTemplateFaderPosition);
    const auto fixedPeak = sourcePeak * faderPositionToGain (kMinus3DbFaderPosition);

    std::printf ("  PCM clip boundary: legacy peak=%.6f fixed peak=%.6f\n", legacyPeak, fixedPeak);

    VIT_CHECK_NEAR (legacyPeak, 1.7957, 1.0e-3);
    VIT_CHECK (legacyPeak > 1.0);   // +6 dB: 16/24-bit render output would clamp
    VIT_CHECK_NEAR (fixedPeak, 0.6372, 1.0e-3);
    VIT_CHECK (fixedPeak < 1.0);    // -3 dB: 16/24-bit render output stays unclamped
    VIT_CHECK (legacyPeak - fixedPeak > 1.15); // the 6 dB = 1.159 gap this fix removes
}

void runGenVitProjectScriptCheck()
{
    const auto repoRoot = juce::File (VIT_TEST_REPO_ROOT);
    const auto scriptText = readProjectFile (repoRoot, "scripts/gen_vit_project.py");

    const auto masterVolumeIndex = scriptText.indexOf ("master_volume");
    VIT_CHECK (masterVolumeIndex >= 0);

    const auto tail = scriptText.substring (masterVolumeIndex).substring (0, 2000);
    const auto volumeAttribute = juce::String ("volume=\"");
    const auto attributeIndex = tail.indexOf (volumeAttribute);
    VIT_CHECK (attributeIndex >= 0);

    const auto valueStart = attributeIndex + volumeAttribute.length();
    const auto valueEnd = tail.indexOfChar (valueStart, '"');
    VIT_CHECK (valueEnd > valueStart);

    const auto raw = tail.substring (valueStart, valueEnd);
    const auto position = raw.getDoubleValue();
    std::printf ("  scripts/gen_vit_project.py: MASTERVOLUME volume=\"%s\"\n", raw.toRawUTF8());

    // Same -3.0 dB fader position as the templates (float32-exact literal is OK).
    VIT_CHECK (std::fabs (position - kMinus3DbFaderPosition) < 1.0e-6);
    VIT_CHECK_NEAR (faderPositionToDb (position), -3.0, 1.0e-6);
}

void runExistingProjectNotMigratedCheck()
{
    // Ruling (2): the committed existing user project is a *stored* project, not
    // a template. It keeps volume="1.0" (+6.0 dB); no silent migration happens.
    const auto vitAppDir = juce::File (VIT_TEST_VITAPP_SOURCE_DIR);
    const auto existingProject = readProjectFile (vitAppDir, "Workspace/VitDAW_Project.tracktionedit");
    VIT_CHECK (existingProject.contains ("volume=\"1.0\""));
    checkMinus3DbFaderPosition (existingProject, "Workspace/VitDAW_Project.tracktionedit", true);
}

} // namespace

int main()
{
    runEmbeddedTemplateChecks();
    runOnDiskDefaultProjectCheck();
    runLegacyValueStaysAtPlus6Db();
    runPcmClipBoundaryCheck();
    runGenVitProjectScriptCheck();
    runExistingProjectNotMigratedCheck();

    std::printf ("VitMasterVolumeCalibrationTests: all checks passed\n");
    return 0;
}
