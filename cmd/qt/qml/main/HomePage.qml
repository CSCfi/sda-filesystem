import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import Qt.labs.qmlmodels 1.0
import QtQuick.Dialogs 1.3
import csc 1.0 as CSC

Control {
    id: page
    padding: 20

    property Item topItem
    property real buttonPadding: 15

    CSC.Popup {
        id: popup
        parent: Overlay.overlay

        Component.onCompleted: leftMargin = page.mapToItem(topItem, 0, 0).x + margin

        Connections {
            target: ProjectModel
            onNoStorageWarning: {
                popup.isError = false
                popup.errorTextContent = count + " project(s) with no storage enabled"
                popup.open()
            }
        }
    }

    FileDialog {
        id: fileDialog
        title: "Choose or create a folder"
        folder: shortcuts.home
        selectExisting: false // TODO: Check this out 
        selectFolder: true
        onAccepted: {
            var mountError = QmlBridge.changeMountPoint(fileDialog.fileUrl)
            if (mountError) {
                popup.errorTextContent = mountError
                popup.open()
            }
        }
    }

    contentItem: GridLayout {
        id: pageGrid
        columns: 2
        columnSpacing: page.padding
        rowSpacing: page.padding

        ColumnLayout {
            id: dialogColumn
            spacing: page.padding
            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.maximumWidth: 600
            Layout.alignment: Qt.AlignTop

            Frame {
                id: acceptFrame
                Layout.fillWidth: true

                background: Rectangle {
                    color: CSC.Style.lightGreen
                }

                ColumnLayout {
                    width: parent.width

                    Text {
                        text: "<h3>Your SD Connect data will be available at this local directory:</h3>"
                        wrapMode: Text.WordWrap
                        Layout.fillWidth: true
                        Layout.preferredWidth: parent.implicitWidth
                    }

                    Rectangle {
                        id: mountField
                        radius: 5
                        color: CSC.Style.lightGreyBlue
                        border.width: 1
                        border.color: CSC.Style.lineGray
                        Layout.fillWidth: true
                        Layout.minimumWidth: 250
                        Layout.preferredHeight: childrenRect.height

                        Flickable {
                            clip: true
                            width: parent.width
                            height: mountText.height
                            contentWidth: mountText.width
                            boundsBehavior: Flickable.StopAtBounds

                            ScrollBar.horizontal: ScrollBar { interactive: false }
                            
                            Text {
                                id: mountText
                                text: QmlBridge.mountPoint
                                font.pointSize: 15
                                verticalAlignment: Text.AlignVCenter
                                maximumLineCount: 1
                                padding: 10
                            }
                        }
                    }

                    RowLayout {
                        spacing: 0
                        Layout.fillWidth: true
                        Layout.topMargin: 15

                        CSC.Button {
                            id: changeButton
                            text: "Change"
                            outlined: true
                            topInset: 0
                            bottomInset: 0
                            padding: page.buttonPadding
                            Layout.maximumWidth: implicitWidth + 2 * padding
                            Layout.fillWidth: true

                            onClicked: { popup.close(); fileDialog.visible = true }
                        }

                        Rectangle {
                            color: "transparent"
                            Layout.fillWidth: true
                            Layout.minimumWidth: page.padding
                        }

                        CSC.Button {
                            id: acceptButton
                            text: "OK"
                            topInset: 0
                            bottomInset: 0
                            enabled: mountText.text != ""
                            padding: page.buttonPadding
                            implicitWidth: state != "finished" ? changeButton.implicitWidth : implicitWidth
                            Layout.maximumWidth: implicitWidth + 2 * padding
                            Layout.minimumHeight: changeButton.implicitHeight
                            Layout.fillWidth: true

                            Material.accent: "white"

                            onClicked: {
                                if (state == "") {
                                    state = "loading"
                                    QmlBridge.loadFuse()
                                }
                            }

                            Connections {
                                target: QmlBridge
                                onFuseReady: acceptButton.state = "finished"
                            }

                            states: [
                                State {
                                    name: "loading";  
                                    PropertyChanges { target: acceptButton; text: ""; loading: true }
                                    PropertyChanges { target: acceptFrame; enabled: false }
                                },
                                State {
                                    name: "finished";
                                    PropertyChanges { target: openButton; enabled: true }
                                    PropertyChanges { target: changeButton; enabled: false }
                                    PropertyChanges { target: acceptButton; text: "Refresh"; enabled: false }
                                }
                            ]
                        }
                    }
                }
            }

            CSC.Button {
                id: openButton
                text: "Open Folder"
                padding: page.buttonPadding
                enabled: false
                outlined: true
                topInset: 0
                bottomInset: 0
                Layout.fillWidth: true
                Layout.minimumWidth: implicitWidth
                
                onClicked: QmlBridge.openFuse()
            }
        }

        Page {
            id: projectFrame
            implicitWidth: dialogColumn.implicitWidth
            rightPadding: 0
            leftPadding: 0
            Layout.fillHeight: true
            Layout.fillWidth: true

            property real rowHeight: 50
            property real viewPadding: 10

            background: Rectangle {
                color: CSC.Style.primaryColor
                radius: 10
            }

            header: Item {
                id: headerItem
                implicitHeight: childrenRect.height

                Rectangle {
                    id: headerLine
                    color: CSC.Style.lightGrey
                    height: 5
                    anchors.right: parent.right
                    anchors.left: parent.left
                    anchors.top: headerTitle.bottom
                }

                TextMetrics {
                    id: textMetricsLoaded
                    text: projectView.rows + "/" + projectView.rows + " loaded"
                    font: headerLoaded.font
                }

                Text {
                    id: headerTitle
                    text: "Projects"
                    color: "white"
                    height: projectFrame.rowHeight
                    maximumLineCount: 1
                    verticalAlignment: Text.AlignVCenter
                    font.pointSize: 0.6 * projectFrame.rowHeight
                    leftPadding: projectFrame.viewPadding
                }

                Text {
                    id: headerLoaded
                    text: loaded + "/" + projectView.rows + " loaded"
                    color: "white"
                    width: parent.width
                    maximumLineCount: 1
                    font.pointSize: 0.3 * projectFrame.rowHeight
                    horizontalAlignment: Text.AlignRight
                    rightPadding: projectFrame.viewPadding
                    leftPadding: projectFrame.viewPadding
                    anchors.baseline: headerTitle.baseline

                    property int loaded: ProjectModel.loadedProjects
                }

                states: [
                    State {
                        name: "dense"
                        when: headerItem.width < headerTitle.width + textMetricsLoaded.width + 3 * projectFrame.viewPadding
                        PropertyChanges { target: headerLoaded; horizontalAlignment: Text.AlignLeft }
                        AnchorChanges { target: headerLine; anchors.top: headerLoaded.bottom }
                        AnchorChanges { target: headerLoaded; anchors.baseline: undefined; anchors.top: headerTitle.bottom }
                    }
                ]
            }

            footer: Rectangle {
                implicitHeight: projectFrame.rowHeight + footerLine.height
                color: "transparent"

                Rectangle {
                    id: footerLine
                    color: CSC.Style.lightGrey
                    height: 5
                    anchors.right: parent.right
                    anchors.left: parent.left
                    anchors.top: parent.top
                }
            }

            TextMetrics {
                id: textMetrics100
                text: "100 %"
            }

            TableView {
                id: projectView
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                anchors.fill: parent

                Material.accent: CSC.Style.altGreen

                property bool ready: false
                property real numColumnMinWidth: textMetrics100.width + 2 * projectFrame.viewPadding
                property real nameColumnMaxWidth

                rowHeightProvider: function (row) { return projectFrame.rowHeight }
                columnWidthProvider: function (column) { return column == 0 ? -1 : 0 } // Some shenanigans so that we can figure out nameColumnMaxWidth

                model: ProjectModel
                delegate: chooser

                Component.onCompleted: {
                    forceLayout()
                    nameColumnMaxWidth = projectView.contentWidth
                    columnWidthProvider = function (column) { 
                        if (column == 0) {
                            return Math.min(nameColumnMaxWidth, projectView.width - numColumnMinWidth)
                        } else {
                            return Math.max(numColumnMinWidth, projectView.width - nameColumnMaxWidth)
                        }
                    }
                    ready = true
                }

                onWidthChanged: {
                    if (ready) { // <- Otherwise error
                        forceLayout()
                    }
                }

                ScrollBar.vertical: ScrollBar { }
            }

            DelegateChooser {
                id: chooser

                DelegateChoice {
                    column: 0
                    delegate: Rectangle {
                        implicitHeight: projectFrame.viewPadding
                        implicitWidth: projectNameText.width
                        color: row % 2 ? CSC.Style.blue : CSC.Style.darkBlue

                        Flickable {
                            clip: true
                            contentWidth: projectNameText.width
                            interactive: contentWidth > width
                            boundsBehavior: Flickable.StopAtBounds
                            anchors.fill: parent

                            ScrollIndicator.horizontal: ScrollIndicator { }
                            
                            Text {
                                id: projectNameText
                                text: projectName
                                font.pointSize: 15
                                font.weight: Font.Medium
                                verticalAlignment: Text.AlignVCenter
                                maximumLineCount: 1
                                padding: projectFrame.viewPadding
                                color: "white"
                                height: parent.height
                            }
                        }

                        Rectangle {
                            z: 2
                            anchors.fill: parent
                            color: noStorage ? CSC.Style.grey : "transparent"
                            opacity: 0.4
                        }
                    }
                }

                DelegateChoice {
                    column: 1
                    delegate: Rectangle {
                        implicitHeight: projectFrame.rowHeight
                        color: row % 2 ? CSC.Style.blue : CSC.Style.darkBlue

                        RowLayout {
                            anchors.fill: parent
                            anchors.rightMargin: projectFrame.viewPadding
                            anchors.leftMargin: projectFrame.viewPadding
                            property real value: (allContainers == -1) ? 0 : (allContainers == 0) ? 1 : loadedContainers / allContainers

                            CSC.ProgressBar {
                                id: progressbar
                                value: parent.value
                                Layout.fillWidth: true
                            }

                            Text {
                                id: percentValue
                                text: Math.round(parent.value * 100) + " %"
                                maximumLineCount: 1
                                color: "white"
                                Layout.minimumWidth: textMetrics100.width
                            }

                            onWidthChanged: {
                                progressbar.visible = (width > 100)
                            }
                        }

                        Rectangle {
                            z: 2
                            anchors.fill: parent
                            color: noStorage ? CSC.Style.grey : "transparent"
                            opacity: 0.4
                        }
                    }
                }
            }
        }

        states: [
            State {
                name: "dense"
                when: (dialogColumn.implicitWidth + page.padding) / pageGrid.width > 0.5
                PropertyChanges { target: pageGrid; columns: 1; rows: 2 }
                PropertyChanges { target: dialogColumn; Layout.maximumWidth: -1 }
                PropertyChanges { target: projectView; interactive: false }
                PropertyChanges { target: headerItem; state: "" }
                PropertyChanges { 
                    target: projectFrame
                    Layout.minimumHeight: projectView.contentHeight + implicitHeaderHeight + implicitFooterHeight
                }
            }
        ]
    }
}
