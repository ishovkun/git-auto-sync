import QtQuick
import QtQuick.Controls as QQC2
import QtQuick.Dialogs
import QtQuick.Layouts
import org.kde.kirigami as Kirigami
import org.kde.plasma.core as PlasmaCore
import org.kde.plasma.plasma5support as P5Support
import org.kde.plasma.plasmoid

PlasmoidItem {
    id: root

    property string daemonState: "unknown"
    property string daemonError: ""
    property var repos: []
    property string loadError: ""
    property bool refreshing: false
    property bool addingRepository: false
    property bool updatingAuthor: false
    property int pendingSyncs: 0
    property var commandKinds: ({})
    readonly property url gitIcon: Qt.resolvedUrl("../images/git-icon.svg")

    readonly property bool hasRepoError: {
        for (let i = 0; i < repos.length; ++i) {
            if (repos[i].synced && !repos[i].ok) {
                return true
            }
        }
        return false
    }
    readonly property string summary: loadError !== ""
        ? i18n("Status unavailable")
        : daemonState === "running"
            ? i18np("Watching %1 repository", "Watching %1 repositories", repos.length)
            : i18n("Daemon %1", daemonState)

    Plasmoid.icon: gitIcon
    Plasmoid.status: daemonState === "running"
        ? PlasmaCore.Types.ActiveStatus
        : PlasmaCore.Types.PassiveStatus
    toolTipMainText: i18n("Git Auto Sync")
    toolTipSubText: summary

    function shellQuote(value) {
        return "'" + String(value).replace(/'/g, "'\"'\"'") + "'"
    }

    function executableCommand() {
        const configured = Plasmoid.configuration.executable.trim()
        // plasmashell commonly starts with a system-only PATH and therefore
        // cannot see binaries installed by `go install` or into ~/.local/bin.
        const userPath = "PATH=\"$HOME/go/bin:$HOME/.local/bin:$PATH\" "
        return userPath + shellQuote(configured === "" ? "git-auto-sync" : configured)
    }

    function runCommand(command, kind) {
        commandKinds[command] = kind
        executableSource.connectSource(command)
    }

    function refresh() {
        if (refreshing) {
            return
        }
        refreshing = true
        runCommand(executableCommand() + " daemon status --json", "status")
    }

    function syncRepo(repoPath) {
        ++pendingSyncs
        let command = executableCommand() + " sync"
        const assignments = authorAssignments()
        for (let i = 0; i < assignments.length; ++i) {
            command += " --env " + shellQuote(assignments[i])
        }
        runCommand(command + " " + shellQuote(repoPath), "sync")
    }

    function addRepo(repoPath) {
        addingRepository = true
        loadError = ""
        let command = ""
        const assignments = authorAssignments()
        if (assignments.length > 0) {
            command = executableCommand() + " daemon env"
            for (let i = 0; i < assignments.length; ++i) {
                command += " " + shellQuote(assignments[i])
            }
            command += " && "
        }
        command += executableCommand() + " daemon add " + shellQuote(repoPath)
        runCommand(command, "add")
    }

    function authorAssignments() {
        const name = Plasmoid.configuration.authorName.trim()
        const email = Plasmoid.configuration.authorEmail.trim()
        if (name === "" || email === "") {
            return []
        }
        return [
            "GIT_AUTHOR_NAME=" + name,
            "GIT_AUTHOR_EMAIL=" + email,
            "GIT_COMMITTER_NAME=" + name,
            "GIT_COMMITTER_EMAIL=" + email
        ]
    }

    function updateDaemonAuthor() {
        const assignments = authorAssignments()
        if (assignments.length === 0 || updatingAuthor) {
            return
        }
        updatingAuthor = true
        let command = executableCommand() + " daemon env"
        for (let i = 0; i < assignments.length; ++i) {
            command += " " + shellQuote(assignments[i])
        }
        runCommand(command, "author")
    }

    function syncAll() {
        if (repos.length === 0 || pendingSyncs > 0) {
            return
        }
        for (let i = 0; i < repos.length; ++i) {
            syncRepo(repos[i].repo)
        }
    }

    function fileUrl(path) {
        return "file://" + String(path).split("/").map(encodeURIComponent).join("/")
    }

    function localPath(url) {
        const value = String(url)
        if (value.startsWith("file://")) {
            return decodeURIComponent(value.substring(7))
        }
        return decodeURIComponent(value)
    }

    function age(timestamp) {
        if (!timestamp) {
            return i18n("never")
        }
        const seconds = Math.max(0, Math.floor(Date.now() / 1000) - timestamp)
        if (seconds < 60) {
            return i18np("%1 second ago", "%1 seconds ago", seconds)
        }
        if (seconds < 3600) {
            const minutes = Math.floor(seconds / 60)
            return i18np("%1 minute ago", "%1 minutes ago", minutes)
        }
        if (seconds < 86400) {
            const hours = Math.floor(seconds / 3600)
            return i18np("%1 hour ago", "%1 hours ago", hours)
        }
        const days = Math.floor(seconds / 86400)
        return i18np("%1 day ago", "%1 days ago", days)
    }

    P5Support.DataSource {
        id: executableSource
        engine: "executable"
        connectedSources: []

        onNewData: function(sourceName, data) {
            disconnectSource(sourceName)
            const kind = root.commandKinds[sourceName]
            delete root.commandKinds[sourceName]

            if (kind === "status") {
                root.refreshing = false
                const exitCode = Number(data["exit code"])
                if (exitCode !== 0) {
                    root.loadError = String(data["stderr"] || i18n("git-auto-sync exited with code %1", exitCode)).trim()
                    return
                }
                try {
                    const snapshot = JSON.parse(String(data["stdout"]))
                    root.daemonState = snapshot.daemon || "unknown"
                    root.daemonError = snapshot.daemon_error || ""
                    root.repos = snapshot.repos || []
                    root.loadError = ""
                } catch (error) {
                    root.loadError = i18n("Could not parse git-auto-sync status: %1", error)
                }
            } else if (kind === "sync") {
                root.pendingSyncs = Math.max(0, root.pendingSyncs - 1)
                if (root.pendingSyncs === 0) {
                    root.refresh()
                }
            } else if (kind === "add") {
                root.addingRepository = false
                const exitCode = Number(data["exit code"])
                if (exitCode !== 0) {
                    root.loadError = String(data["stderr"]
                        || i18n("Could not add repository (exit code %1)", exitCode)).trim()
                } else {
                    root.refresh()
                }
            } else if (kind === "author") {
                root.updatingAuthor = false
                const exitCode = Number(data["exit code"])
                if (exitCode !== 0) {
                    root.loadError = String(data["stderr"]
                        || i18n("Could not update Git author (exit code %1)", exitCode)).trim()
                } else {
                    root.refresh()
                }
            }
        }
    }

    FolderDialog {
        id: repositoryPicker
        title: i18n("Choose a Git repository")
        onAccepted: root.addRepo(root.localPath(selectedFolder))
    }

    Timer {
        interval: Math.max(5, Plasmoid.configuration.refreshInterval) * 1000
        repeat: true
        running: true
        onTriggered: root.refresh()
    }

    Timer {
        id: authorUpdateTimer
        interval: 500
        onTriggered: root.updateDaemonAuthor()
    }

    Connections {
        target: Plasmoid.configuration
        function onAuthorNameChanged() {
            authorUpdateTimer.restart()
        }
        function onAuthorEmailChanged() {
            authorUpdateTimer.restart()
        }
    }

    Component.onCompleted: {
        refresh()
        authorUpdateTimer.start()
    }

    compactRepresentation: MouseArea {
        id: compact
        implicitWidth: Kirigami.Units.iconSizes.smallMedium
        implicitHeight: implicitWidth
        activeFocusOnTab: true
        Accessible.name: root.summary
        onClicked: root.expanded = !root.expanded

        Kirigami.Icon {
            anchors.centerIn: parent
            width: Math.max(1, Math.round(Math.min(compact.width, compact.height)
                * Plasmoid.configuration.iconScale / 100))
            height: width
            source: root.gitIcon
            isMask: true
            color: Kirigami.Theme.textColor
            active: compact.containsMouse
        }
    }

    fullRepresentation: Item {
        implicitWidth: Kirigami.Units.gridUnit * 24
        implicitHeight: Kirigami.Units.gridUnit * 24

        ColumnLayout {
            anchors.fill: parent
            anchors.margins: Kirigami.Units.largeSpacing
            spacing: Kirigami.Units.smallSpacing

            RowLayout {
                Layout.fillWidth: true

                Rectangle {
                    implicitWidth: Kirigami.Units.smallSpacing
                    implicitHeight: implicitWidth
                    radius: width / 2
                    color: root.daemonState === "running"
                        ? Kirigami.Theme.positiveTextColor
                        : Kirigami.Theme.negativeTextColor
                }

                QQC2.Label {
                    Layout.fillWidth: true
                    text: root.daemonState === "running"
                        ? i18n("Daemon running")
                        : i18n("Daemon %1", root.daemonState)
                    font.bold: true
                }

                QQC2.BusyIndicator {
                    visible: root.refreshing || root.pendingSyncs > 0 || root.updatingAuthor
                    running: visible
                    implicitWidth: Kirigami.Units.iconSizes.small
                    implicitHeight: implicitWidth
                }

                QQC2.ToolButton {
                    icon.name: "view-refresh"
                    text: i18n("Refresh")
                    display: QQC2.AbstractButton.IconOnly
                    enabled: !root.refreshing
                    onClicked: root.refresh()
                    QQC2.ToolTip.text: text
                    QQC2.ToolTip.visible: hovered
                }
            }

            Kirigami.InlineMessage {
                Layout.fillWidth: true
                visible: root.loadError !== "" || root.daemonError !== ""
                type: Kirigami.MessageType.Error
                text: root.loadError !== "" ? root.loadError : root.daemonError
            }

            Kirigami.Separator {
                Layout.fillWidth: true
            }

            QQC2.ScrollView {
                Layout.fillWidth: true
                Layout.fillHeight: true

                ListView {
                    id: repoList
                    clip: true
                    spacing: Kirigami.Units.smallSpacing
                    model: root.repos

                    delegate: QQC2.ItemDelegate {
                        required property var modelData
                        width: repoList.width
                        hoverEnabled: true
                        contentItem: RowLayout {
                            spacing: Kirigami.Units.smallSpacing

                            Rectangle {
                                implicitWidth: Kirigami.Units.smallSpacing
                                implicitHeight: implicitWidth
                                radius: width / 2
                                color: !modelData.synced
                                    ? Kirigami.Theme.neutralTextColor
                                    : modelData.ok
                                        ? Kirigami.Theme.positiveTextColor
                                        : Kirigami.Theme.negativeTextColor
                            }

                            ColumnLayout {
                                Layout.fillWidth: true
                                spacing: 0

                                QQC2.Label {
                                    Layout.fillWidth: true
                                    text: String(modelData.repo).split("/").pop()
                                    font.bold: true
                                    elide: Text.ElideRight
                                }

                                QQC2.Label {
                                    Layout.fillWidth: true
                                    text: !modelData.synced
                                        ? i18n("Not synced yet")
                                        : modelData.ok
                                            ? i18n("Synced %1", root.age(modelData.synced_at_unix))
                                            : i18n("Failed %1", root.age(modelData.synced_at_unix))
                                    color: modelData.synced && !modelData.ok
                                        ? Kirigami.Theme.negativeTextColor
                                        : Kirigami.Theme.disabledTextColor
                                    elide: Text.ElideRight
                                }

                                QQC2.Label {
                                    Layout.fillWidth: true
                                    visible: modelData.error !== undefined && modelData.error !== ""
                                    text: modelData.error || ""
                                    color: Kirigami.Theme.negativeTextColor
                                    wrapMode: Text.Wrap
                                    maximumLineCount: 3
                                    elide: Text.ElideRight
                                }
                            }

                            QQC2.ToolButton {
                                icon.name: "folder-open"
                                text: i18n("Open repository")
                                display: QQC2.AbstractButton.IconOnly
                                onClicked: Qt.openUrlExternally(root.fileUrl(modelData.repo))
                                QQC2.ToolTip.text: text
                                QQC2.ToolTip.visible: hovered
                            }

                            QQC2.ToolButton {
                                icon.name: "view-refresh"
                                text: i18n("Sync now")
                                display: QQC2.AbstractButton.IconOnly
                                enabled: root.pendingSyncs === 0
                                onClicked: root.syncRepo(modelData.repo)
                                QQC2.ToolTip.text: text
                                QQC2.ToolTip.visible: hovered
                            }
                        }
                    }

                    QQC2.Label {
                        anchors.centerIn: parent
                        visible: root.repos.length === 0 && root.loadError === ""
                        text: i18n("No repositories are being watched.")
                        color: Kirigami.Theme.disabledTextColor
                    }
                }
            }

            RowLayout {
                Layout.fillWidth: true

                QQC2.Button {
                    Layout.fillWidth: true
                    text: root.addingRepository ? i18n("Adding…") : i18n("Add repository…")
                    icon.name: "folder-add"
                    enabled: !root.addingRepository && root.pendingSyncs === 0
                    onClicked: repositoryPicker.open()
                }

                QQC2.Button {
                    Layout.fillWidth: true
                    text: root.pendingSyncs > 0 ? i18n("Syncing…") : i18n("Sync all now")
                    icon.name: "view-refresh"
                    enabled: root.repos.length > 0 && root.pendingSyncs === 0 && !root.addingRepository
                    onClicked: root.syncAll()
                }
            }
        }
    }
}
