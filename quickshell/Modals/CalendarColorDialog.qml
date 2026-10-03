import QtQuick
import qs.Common
import qs.Services
import qs.Widgets
import qs.DankCommon.Widgets

DankOverlayDialog {
    id: root

    property var calendar: null
    readonly property bool hasOverride: !!(calendar && calendar.hasColorOverride)
    readonly property var hexPattern: /^#[0-9a-fA-F]{6}$/

    function show(cal) {
        calendar = cal;
        hexField.text = (cal.color || "").toLowerCase();
        open();
    }

    function useSyncedColor() {
        DankCalService.setCalendarColor(calendar.id, "");
        close();
    }

    function submit() {
        if (!calendar)
            return;
        const trimmed = hexField.text.trim().toLowerCase();
        if (!hexPattern.test(trimmed))
            return;
        const next = trimmed === (calendar.providerColor || "").toLowerCase() ? "" : trimmed;
        DankCalService.setCalendarColor(calendar.id, next);
        close();
    }

    title: I18n.tr("Calendar color", "calendar color dialog header")
    supportingText: hasOverride ? (calendar.providerColor ? I18n.tr("Synced color is %1. This override only changes it in Dank Calendar.", "calendar color dialog note showing provider color").arg(calendar.providerColor) : I18n.tr("This calendar has no synced color. This override only changes it in Dank Calendar.", "calendar color dialog note when provider has no color")) : ""
    onAccepted: submit()

    Row {
        spacing: Theme.spacingS

        Repeater {
            model: DankCalService.fallbackPalette

            Rectangle {
                required property string modelData

                width: Theme.chipIconSize
                height: Theme.chipIconSize
                radius: width / 2
                color: modelData
                border.width: hexField.text.toLowerCase() === modelData ? 2 : 0
                border.color: Theme.onSurface

                MouseArea {
                    anchors.fill: parent
                    onClicked: hexField.text = modelData
                }
            }
        }
    }

    DankTextField {
        id: hexField
        width: parent.width
        outlined: true
        labelText: I18n.tr("Hex color", "calendar color dialog hex input label")
        placeholderText: "#rrggbb"
        isError: text.length > 0 && !root.hexPattern.test(text.trim())
        supportingText: isError ? I18n.tr("Enter a hex color like #4287f5.", "calendar color dialog hex validation error") : ""
        onAccepted: root.submit()
        Keys.onReturnPressed: event => event.accepted = true
        Keys.onEnterPressed: event => event.accepted = true
    }

    actions: [
        DankButton {
            visible: root.hasOverride
            text: I18n.tr("Use synced color", "calendar color dialog button to revert to provider color")
            backgroundColor: "transparent"
            textColor: Theme.primary
            onClicked: root.useSyncedColor()
        },
        DankButton {
            text: I18n.tr("Cancel", "calendar color dialog button to cancel")
            backgroundColor: Theme.secondaryContainer
            textColor: Theme.onSecondaryContainer
            onClicked: root.close()
        },
        DankButton {
            text: I18n.tr("Save", "calendar color dialog button to save color")
            backgroundColor: Theme.primary
            textColor: Theme.primaryText
            onClicked: root.submit()
        }
    ]
}
