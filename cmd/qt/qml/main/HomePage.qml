import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import Qt.labs.qmlmodels 1.0
import csc 1.0 as CSC

Page {
    id: page
    padding: 20

    property int buttonPadding: 15

    GridLayout {
        id: pageGrid
        columns: 2
        columnSpacing: page.padding
        rowSpacing: page.padding
        anchors.fill: parent

        ColumnLayout {
            id: dialogColumn
            spacing: page.padding
            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.maximumWidth: 400
            Layout.alignment: Qt.AlignTop

            Frame {
                id: acceptFrame
                Layout.fillWidth: true

                background: Rectangle {
                    color: CSC.Style.lightGreen
                }

                ColumnLayout {
                    anchors.fill: parent

                    Text {
                        text: "<h3>FUSE will be mounted at:</h3>"
                        color: CSC.Style.grey
                    }

                    CSC.TextField {
                        id: mountField
                        text: QmlBridge.mountPoint
                        Layout.fillWidth: true
                    }

                    CSC.Button {
                        id: acceptButton
                        text: "Accept"
                        outlined: true
                        topInset: 0
                        bottomInset: 0
                        padding: page.buttonPadding
                        Layout.alignment: Qt.AlignRight
                        Layout.minimumWidth: implicitWidth
                        Layout.topMargin: 15

                        Component.onCompleted: Layout.preferredWidth = implicitWidth + 2 * page.buttonPadding

                        onClicked: {
                            if (acceptButton.state == "") {
                                QmlBridge.changeMountPoint(mountField.text)
                                acceptButton.state = "accepted"
                            } else {
                                acceptButton.state = ""
                            }
                        }

                        states: [
                            State {
                                name: "accepted"
                                PropertyChanges { target: acceptButton; text: "Change" }
                                PropertyChanges { target: mountField; enabled: false }
                                PropertyChanges { target: loadButton; enabled: true }
                            }
                        ]
                    }
                }
            }

            RowLayout {
                spacing: page.padding
                Layout.fillHeight: true
                Layout.fillWidth: true

                CSC.Button {
                    id: openButton
                    text: "Open FUSE"
                    padding: page.buttonPadding
                    enabled: false
                    topInset: 0
                    bottomInset: 0
                    Layout.fillWidth: true
                    Layout.minimumWidth: implicitWidth
                    
                    onClicked: QmlBridge.openFuse()
                }

                CSC.Button {
                    id: loadButton
                    text: "Load FUSE"
                    padding: page.buttonPadding
                    enabled: false
                    topInset: 0
                    bottomInset: 0
                    Layout.fillWidth: true
                    Layout.minimumWidth: implicitWidth

                    Material.accent: "white"

                    BusyIndicator {
                        id: busy
                        running: false
                        anchors.fill: parent
                        anchors.centerIn: parent
                        anchors.margins: 5
                    }

                    Connections {
                        target: QmlBridge
                        onFuseReady: loadButton.state = "finished"
                    }

                    onClicked: {
                        loadButton.state = "loading"
                        QmlBridge.loadFuse()
                    }

                    states: [
                        State {
                            name: "loading"; 
                            PropertyChanges { 
                                target: loadButton
                                text: ""
                                disableBackgound: CSC.Style.primaryColor
                                enabled: false 
                                Layout.minimumWidth: openButton.implicitWidth
                                Layout.minimumHeight: openButton.implicitHeight
                            }
                            PropertyChanges { target: acceptFrame; enabled: false }
                            PropertyChanges { target: busy; running: true }
                        },
                        State {
                            name: "finished"
                            PropertyChanges { target: openButton; enabled: true }
                            PropertyChanges { target: acceptFrame; enabled: false }
                            PropertyChanges { target: loadButton; text: "Refresh FUSE"; enabled: false }
                        }
                    ]			
                }
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
                    font.pixelSize: 0.6 * projectFrame.rowHeight
                    leftPadding: projectFrame.viewPadding
                }

                Text {
                    id: headerLoaded
                    text: loaded + "/" + projectView.rows + " loaded"
                    color: "white"
                    width: parent.width
                    maximumLineCount: 1
                    font.pixelSize: 0.3 * projectFrame.rowHeight
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
