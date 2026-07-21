#include "ProjectMarkerService.h"

#include <algorithm>
#include <cmath>
#include <vector>

namespace vit
{

namespace
{

const juce::Identifier kMarkersTree ("VIT_PROJECT_MARKERS");
const juce::Identifier kMarkerTree ("VIT_PROJECT_MARKER");
const juce::Identifier kMarkerID ("marker_id");
const juce::Identifier kName ("name");
const juce::Identifier kStartSeconds ("start_seconds");
const juce::Identifier kEndSeconds ("end_seconds");
const juce::Identifier kKind ("kind");
const juce::Identifier kColor ("color");
const juce::Identifier kSource ("source");
const juce::Identifier kCreatedBy ("created_by");
const juce::Identifier kConfidence ("confidence");
const juce::Identifier kSectionID ("section_id");
const juce::Identifier kSchemaVersion ("schema_version");

constexpr double kTimeEpsilonSeconds = 0.0005;

juce::ValueTree markersTree (te::Edit& edit)
{
    return edit.state.getChildWithName (kMarkersTree);
}

juce::ValueTree ensureMarkersTree (te::Edit& edit, juce::UndoManager* undo)
{
    auto tree = markersTree (edit);
    if (tree.isValid())
        return tree;

    tree = juce::ValueTree (kMarkersTree);
    tree.setProperty (kSchemaVersion, "vit_project_markers.v1", nullptr);
    edit.state.addChild (tree, -1, undo);
    return tree;
}

bool numericVarToDouble (const juce::var& value, double& out)
{
    if (value.isDouble() || value.isInt() || value.isInt64())
    {
        out = static_cast<double> (value);
        return std::isfinite (out);
    }

    const auto text = value.toString().trim();
    if (text.isEmpty())
        return false;

    const auto parsed = text.getDoubleValue();
    if (! std::isfinite (parsed))
        return false;

    out = parsed;
    return true;
}

juce::String firstStringProperty (const juce::DynamicObject& object,
                                  std::initializer_list<const char*> keys)
{
    for (auto* key : keys)
    {
        const auto value = object.getProperty (key).toString().trim();
        if (value.isNotEmpty())
            return value;
    }

    return {};
}

bool firstNumericProperty (const juce::DynamicObject& object,
                           std::initializer_list<const char*> keys,
                           double& out)
{
    for (auto* key : keys)
    {
        const auto value = object.getProperty (key);
        if (! value.isVoid() && numericVarToDouble (value, out))
            return true;
    }

    return false;
}

juce::String normalizedColor (const juce::String& value, int index)
{
    static const juce::StringArray palette {
        "#4E8CFF", "#27BFA5", "#F2A33A", "#D85C7A",
        "#8D7CFF", "#60B15A", "#CF6B45", "#38A1C5"
    };

    const auto trimmed = value.trim();
    if (trimmed.startsWithChar ('#') && (trimmed.length() == 7 || trimmed.length() == 9))
        return trimmed;

    return palette[index % palette.size()];
}

juce::String makeMarkerID (int index, double startSeconds, double endSeconds, const juce::String& name)
{
    const auto key = juce::String (index) + "|"
                   + juce::String (startSeconds, 3) + "|"
                   + juce::String (endSeconds, 3) + "|"
                   + name.trim();
    return "marker_" + juce::String::toHexString (static_cast<juce::int64> (key.hashCode64()));
}

double doubleProperty (const juce::ValueTree& tree, const juce::Identifier& id, double fallback = 0.0)
{
    double parsed = 0.0;
    return numericVarToDouble (tree.getProperty (id), parsed) ? parsed : fallback;
}

bool boolPropertyOrDefault (const juce::DynamicObject& object, const char* key, bool fallback)
{
    const auto value = object.getProperty (key);
    if (value.isVoid())
        return fallback;

    if (value.isBool())
        return static_cast<bool> (value);

    const auto text = value.toString().trim().toLowerCase();
    if (text == "true" || text == "1" || text == "yes")
        return true;
    if (text == "false" || text == "0" || text == "no")
        return false;

    return fallback;
}

bool hasUsableProperty (const juce::DynamicObject& object, const char* key)
{
    const auto value = object.getProperty (key);
    return ! value.isVoid() && value.toString().trim().isNotEmpty();
}

juce::var markerValueTreeToVar (const juce::ValueTree& marker)
{
    auto out = std::make_unique<juce::DynamicObject>();
    const auto markerID = marker.getProperty (kMarkerID).toString().trim();
    const auto name = marker.getProperty (kName).toString().trim();
    const double startSeconds = doubleProperty (marker, kStartSeconds);
    const bool hasEnd = marker.hasProperty (kEndSeconds)
                        && ! marker.getProperty (kEndSeconds).isVoid()
                        && marker.getProperty (kEndSeconds).toString().trim().isNotEmpty();
    const double endSeconds = hasEnd ? doubleProperty (marker, kEndSeconds) : 0.0;
    const bool isRange = hasEnd && endSeconds > startSeconds + kTimeEpsilonSeconds;

    out->setProperty ("marker_id", markerID);
    out->setProperty ("id", markerID);
    out->setProperty ("name", name.isNotEmpty() ? name : markerID);
    out->setProperty ("start_seconds", startSeconds);
    out->setProperty ("kind", isRange ? "range" : "position");
    if (isRange)
    {
        out->setProperty ("end_seconds", endSeconds);
        out->setProperty ("duration_seconds", std::max (0.0, endSeconds - startSeconds));
    }
    if (marker.hasProperty (kColor))
        out->setProperty ("color", marker.getProperty (kColor));
    if (marker.hasProperty (kSource))
        out->setProperty ("source", marker.getProperty (kSource));
    if (marker.hasProperty (kCreatedBy))
        out->setProperty ("created_by", marker.getProperty (kCreatedBy));
    if (marker.hasProperty (kConfidence))
        out->setProperty ("confidence", marker.getProperty (kConfidence));
    if (marker.hasProperty (kSectionID))
        out->setProperty ("section_id", marker.getProperty (kSectionID));

    return juce::var (out.release());
}

juce::ValueTree findMarkerByID (juce::ValueTree markers, const juce::String& markerID)
{
    for (int i = 0; i < markers.getNumChildren(); ++i)
    {
        auto child = markers.getChild (i);
        if (child.hasType (kMarkerTree) && child.getProperty (kMarkerID).toString() == markerID)
            return child;
    }

    return {};
}

juce::Array<juce::var> snapshotArrayFromTree (const juce::ValueTree& tree)
{
    std::vector<juce::ValueTree> source;
    for (int i = 0; i < tree.getNumChildren(); ++i)
    {
        const auto child = tree.getChild (i);
        if (child.hasType (kMarkerTree))
            source.push_back (child);
    }

    std::sort (source.begin(), source.end(), [] (const juce::ValueTree& a, const juce::ValueTree& b)
    {
        const double as = doubleProperty (a, kStartSeconds);
        const double bs = doubleProperty (b, kStartSeconds);
        return as < bs;
    });

    juce::Array<juce::var> markers;
    for (const auto& marker : source)
        markers.add (markerValueTreeToVar (marker));

    return markers;
}

juce::String sourceFilterForApply (const juce::DynamicObject& object)
{
    const auto source = firstStringProperty (object, { "source", "replace_source" });
    return source.isNotEmpty() ? source : "epm_a5";
}

} // namespace

ProjectMarkerService::ProjectMarkerService (EditGetter editGetter,
                                            SaveProjectAction saveProjectAction)
    : getEdit (std::move (editGetter)),
      saveProject (std::move (saveProjectAction))
{
}

juce::String ProjectMarkerService::makeStatusReply (const juce::String& status, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", status);
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ProjectMarkerService::makeErrorReply (const juce::String& message)
{
    return makeStatusReply ("error", message);
}

juce::var ProjectMarkerService::createMarkersSnapshot (te::Edit& edit)
{
    const auto tree = markersTree (edit);
    if (! tree.isValid())
        return juce::var (juce::Array<juce::var>());

    return juce::var (snapshotArrayFromTree (tree));
}

juce::String ProjectMarkerService::handleListMarkers (const juce::DynamicObject&, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto response = std::make_unique<juce::DynamicObject>();
    const auto markers = createMarkersSnapshot (*edit);
    const auto* markerArray = markers.getArray();
    response->setProperty ("status", "ok");
    response->setProperty ("markers", markers);
    response->setProperty ("marker_count", markerArray != nullptr ? markerArray->size() : 0);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ProjectMarkerService::handleUpsertMarker (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    double startSeconds = 0.0;
    if (! firstNumericProperty (object, { "start_seconds", "start", "time_seconds" }, startSeconds)
        || ! std::isfinite (startSeconds)
        || startSeconds < 0.0)
        return makeErrorReply ("project.markers.upsert requires start_seconds >= 0");

    const bool hasEnd = hasUsableProperty (object, "end_seconds") || hasUsableProperty (object, "end");
    double endSeconds = 0.0;
    if (hasEnd)
    {
        if (! firstNumericProperty (object, { "end_seconds", "end" }, endSeconds)
            || ! std::isfinite (endSeconds)
            || endSeconds <= startSeconds + kTimeEpsilonSeconds)
            return makeErrorReply ("Range marker end_seconds must be greater than start_seconds");
    }

    auto markerID = firstStringProperty (object, { "marker_id", "id" });
    const auto name = firstStringProperty (object, { "name", "label", "title" });
    if (markerID.isEmpty())
        markerID = makeMarkerID (0, startSeconds, hasEnd ? endSeconds : 0.0, name);

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Upsert project marker");
    auto markers = ensureMarkersTree (*edit, &undo);
    auto marker = findMarkerByID (markers, markerID);
    const bool created = ! marker.isValid();

    if (created)
        marker = juce::ValueTree (kMarkerTree);

    marker.setProperty (kMarkerID, markerID, nullptr);
    marker.setProperty (kName, name.isNotEmpty() ? name : (hasEnd ? "Section" : "Marker"), &undo);
    marker.setProperty (kStartSeconds, startSeconds, &undo);
    marker.setProperty (kKind, hasEnd ? "range" : "position", &undo);
    if (hasEnd)
        marker.setProperty (kEndSeconds, endSeconds, &undo);
    else if (marker.hasProperty (kEndSeconds))
        marker.removeProperty (kEndSeconds, &undo);

    marker.setProperty (kColor, normalizedColor (firstStringProperty (object, { "color", "colour" }), 0), &undo);

    const auto source = firstStringProperty (object, { "source" });
    if (source.isNotEmpty())
        marker.setProperty (kSource, source, &undo);
    else if (created)
        marker.setProperty (kSource, "gui", &undo);

    const auto createdBy = firstStringProperty (object, { "created_by" });
    if (createdBy.isNotEmpty())
        marker.setProperty (kCreatedBy, createdBy, &undo);
    else if (created)
        marker.setProperty (kCreatedBy, "user", &undo);

    const auto confidence = firstStringProperty (object, { "confidence" });
    if (confidence.isNotEmpty())
        marker.setProperty (kConfidence, confidence, &undo);

    const auto sectionID = firstStringProperty (object, { "section_id" });
    if (sectionID.isNotEmpty())
        marker.setProperty (kSectionID, sectionID, &undo);

    if (created)
        markers.addChild (marker, -1, &undo);

    edit->dispatchPendingUpdatesSynchronously();

    if (saveProject && ! saveProject())
        return makeErrorReply ("Marker updated in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", created ? "Project marker created" : "Project marker updated");
    response->setProperty ("created", created);
    response->setProperty ("marker", markerValueTreeToVar (marker));
    response->setProperty ("markers", createMarkersSnapshot (*edit));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ProjectMarkerService::handleApplySectionMarkers (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    auto* sections = object.getProperty ("sections").getArray();
    if (sections == nullptr)
        sections = object.getProperty ("markers").getArray();

    if (sections == nullptr || sections->isEmpty())
        return makeErrorReply ("project.markers.apply_section_markers requires a non-empty sections array");

    const bool replaceExisting = boolPropertyOrDefault (object, "replace_existing", true);
    const auto source = sourceFilterForApply (object);
    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Apply project section markers");
    auto markers = ensureMarkersTree (*edit, &undo);

    if (replaceExisting)
    {
        for (int i = markers.getNumChildren() - 1; i >= 0; --i)
        {
            const auto child = markers.getChild (i);
            if (child.hasType (kMarkerTree)
                && child.getProperty (kSource).toString().trim().equalsIgnoreCase (source))
                markers.removeChild (i, &undo);
        }
    }

    juce::Array<juce::var> writtenMarkers;
    int skipped = 0;

    for (int i = 0; i < sections->size(); ++i)
    {
        auto* section = sections->getReference (i).getDynamicObject();
        if (section == nullptr)
        {
            ++skipped;
            continue;
        }

        double startSeconds = 0.0;
        double endSeconds = 0.0;
        if (! firstNumericProperty (*section, { "start_seconds", "start", "time_seconds" }, startSeconds)
            || ! firstNumericProperty (*section, { "end_seconds", "end" }, endSeconds)
            || ! std::isfinite (startSeconds)
            || ! std::isfinite (endSeconds)
            || startSeconds < 0.0
            || endSeconds <= startSeconds + kTimeEpsilonSeconds)
        {
            ++skipped;
            continue;
        }

        const auto name = firstStringProperty (*section, { "name", "label", "title", "section_id" });
        const auto sectionID = firstStringProperty (*section, { "section_id", "id" });
        auto markerID = firstStringProperty (*section, { "marker_id", "id" });
        if (markerID.isEmpty() || markerID == sectionID)
            markerID = makeMarkerID (i, startSeconds, endSeconds, name);

        auto marker = juce::ValueTree (kMarkerTree);
        marker.setProperty (kMarkerID, markerID, nullptr);
        marker.setProperty (kName, name.isNotEmpty() ? name : ("Section " + juce::String (i + 1)), nullptr);
        marker.setProperty (kStartSeconds, startSeconds, nullptr);
        marker.setProperty (kEndSeconds, endSeconds, nullptr);
        marker.setProperty (kKind, "range", nullptr);
        marker.setProperty (kColor, normalizedColor (firstStringProperty (*section, { "color", "colour" }), i), nullptr);
        marker.setProperty (kSource, source, nullptr);
        const auto createdBy = firstStringProperty (*section, { "created_by" });
        marker.setProperty (kCreatedBy, createdBy.isNotEmpty() ? createdBy : "agent", nullptr);
        const auto confidence = firstStringProperty (*section, { "confidence" });
        if (confidence.isNotEmpty())
            marker.setProperty (kConfidence, confidence, nullptr);
        if (sectionID.isNotEmpty())
            marker.setProperty (kSectionID, sectionID, nullptr);

        markers.addChild (marker, -1, &undo);
        writtenMarkers.add (markerValueTreeToVar (marker));
    }

    edit->dispatchPendingUpdatesSynchronously();

    if (writtenMarkers.isEmpty())
        return makeErrorReply ("No valid section markers were written");

    if (saveProject && ! saveProject())
        return makeErrorReply ("Markers updated in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Project section markers applied");
    response->setProperty ("written_count", writtenMarkers.size());
    response->setProperty ("skipped_count", skipped);
    response->setProperty ("markers", juce::var (writtenMarkers));
    response->setProperty ("all_markers", createMarkersSnapshot (*edit));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ProjectMarkerService::handleRenameMarker (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto markerID = firstStringProperty (object, { "marker_id", "id" });
    const auto name = firstStringProperty (object, { "name", "new_name", "label" });

    if (markerID.isEmpty())
        return makeErrorReply ("project.markers.rename requires marker_id");

    if (name.isEmpty())
        return makeErrorReply ("project.markers.rename requires a non-empty name");

    auto markers = markersTree (*edit);
    auto marker = markers.isValid() ? findMarkerByID (markers, markerID) : juce::ValueTree();
    if (! marker.isValid())
        return makeErrorReply ("Marker not found: " + markerID);

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Rename project marker");
    marker.setProperty (kName, name, &undo);
    edit->dispatchPendingUpdatesSynchronously();

    if (saveProject && ! saveProject())
        return makeErrorReply ("Marker renamed in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Project marker renamed");
    response->setProperty ("marker", markerValueTreeToVar (marker));
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String ProjectMarkerService::handleDeleteMarker (const juce::DynamicObject& object, const juce::String&) const
{
    auto* edit = getEdit != nullptr ? getEdit() : nullptr;

    if (edit == nullptr)
        return makeErrorReply ("No active edit loaded");

    const auto markerID = firstStringProperty (object, { "marker_id", "id" });
    if (markerID.isEmpty())
        return makeErrorReply ("project.markers.delete requires marker_id");

    auto markers = markersTree (*edit);
    if (! markers.isValid())
        return makeErrorReply ("Marker not found: " + markerID);

    auto& undo = edit->getUndoManager();
    undo.beginNewTransaction ("Delete project marker");
    bool removed = false;

    for (int i = markers.getNumChildren() - 1; i >= 0; --i)
    {
        const auto child = markers.getChild (i);
        if (child.hasType (kMarkerTree) && child.getProperty (kMarkerID).toString() == markerID)
        {
            markers.removeChild (i, &undo);
            removed = true;
            break;
        }
    }

    if (! removed)
        return makeErrorReply ("Marker not found: " + markerID);

    edit->dispatchPendingUpdatesSynchronously();

    if (saveProject && ! saveProject())
        return makeErrorReply ("Marker deleted in memory but failed to save project");

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    response->setProperty ("message", "Project marker deleted");
    response->setProperty ("deleted_count", 1);
    response->setProperty ("marker_id", markerID);
    response->setProperty ("markers", createMarkersSnapshot (*edit));
    return juce::JSON::toString (juce::var (response.release()));
}

} // namespace vit
