import QtQuick
import QtQuick.Controls as QQC2
import QtQuick.Layouts
import org.kde.kirigami as Kirigami

Kirigami.FormLayout {
    property string title
    property alias cfg_executable: executableField.text
    property alias cfg_refreshInterval: refreshInterval.value
    property alias cfg_iconScale: iconScale.value
    property alias cfg_authorName: authorNameField.text
    property alias cfg_authorEmail: authorEmailField.text
    property string cfg_executableDefault
    property int cfg_refreshIntervalDefault
    property int cfg_iconScaleDefault
    property string cfg_authorNameDefault
    property string cfg_authorEmailDefault

    QQC2.TextField {
        id: executableField
        Kirigami.FormData.label: i18n("Executable:")
        placeholderText: "git-auto-sync"
    }

    QQC2.SpinBox {
        id: refreshInterval
        Kirigami.FormData.label: i18n("Refresh every:")
        from: 5
        to: 3600
        editable: true
        textFromValue: function(value) {
            return i18np("%1 second", "%1 seconds", value)
        }
        valueFromText: function(text) {
            return parseInt(text)
        }
    }

    QQC2.SpinBox {
        id: iconScale
        Kirigami.FormData.label: i18n("Panel icon scale:")
        from: 40
        to: 140
        stepSize: 5
        editable: true
        textFromValue: function(value) {
            return i18n("%1%", value)
        }
        valueFromText: function(text) {
            return Math.max(from, Math.min(to, parseInt(text)))
        }
    }

    Kirigami.Separator {
        Kirigami.FormData.isSection: true
        Kirigami.FormData.label: i18n("Commit identity")
    }

    QQC2.TextField {
        id: authorNameField
        Kirigami.FormData.label: i18n("Author name:")
        placeholderText: i18n("Your Name")
    }

    QQC2.TextField {
        id: authorEmailField
        Kirigami.FormData.label: i18n("Author email:")
        placeholderText: i18n("you@example.com")
        inputMethodHints: Qt.ImhEmailCharactersOnly
    }
}
