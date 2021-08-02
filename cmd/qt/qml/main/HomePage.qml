import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import Qt.labs.qmlmodels 1.0
import csc 1.0 as CSC

Page {
    id: page
    property int gridSpacing: 20
    property int buttonPadding: 20
    implicitWidth: dialogColumn.implicitWidth + projectFrame.implicitWidth + 2 * gridSpacing  

    RowLayout {
        spacing: page.gridSpacing
        anchors.fill: parent
        anchors.margins: page.gridSpacing

        ColumnLayout {
            id: dialogColumn
            spacing: page.gridSpacing
            Layout.fillWidth: true
            Layout.preferredWidth: 1 // The two items in the above rowlayout are 1:1 (generally)
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
                    //spacing: page.gridSpacing

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
                        enabled: true
                        padding: page.buttonPadding
                        Layout.alignment: Qt.AlignRight
                        Layout.minimumWidth: implicitWidth
                        Layout.topMargin: 15

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
                spacing: page.gridSpacing
                Layout.fillHeight: true
                Layout.fillWidth: true

                CSC.Button {
                    id: openButton
                    text: "Open FUSE"
                    padding: page.buttonPadding
                    enabled: false
                    Layout.fillWidth: true
                    Layout.minimumWidth: implicitWidth
                    
                    onClicked: QmlBridge.openFuse()
                }

                CSC.Button {
                    id: loadButton
                    text: "Load FUSE"
                    padding: page.buttonPadding
                    enabled: false
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
            Layout.preferredWidth: 1

            property real rowHeight: 50
            property real viewPadding: 10

            background: Rectangle {
                color: CSC.Style.primaryColor
                radius: 10
            }

            header: Rectangle {
                implicitHeight: projectFrame.rowHeight
                color: "transparent"
                Text {
                    text: "Projects"
                    color: "white"
                    verticalAlignment: Text.AlignBottom
                    height: parent.height
                    maximumLineCount: 1
                    padding: 10
                    fontSizeMode: Text.VerticalFit
                    minimumPixelSize: 10
                    font.pixelSize: projectFrame.rowHeight
                }
            }

            footer: Rectangle {
                implicitHeight: projectFrame.rowHeight
                color: "transparent"
            }

            TextMetrics {
                id: textMetrics
                text: "100 %"
            }

            TableView {
                id: projectView
                clip: true
                anchors.fill: parent

                Material.accent: CSC.Style.altGreen

                property bool ready: false
                property real numColumnMinWidth: textMetrics.width + projectFrame.viewPadding
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

                ScrollBar.vertical: ScrollBar { interactive: false }
            }

            DelegateChooser {
                id: chooser

                DelegateChoice {
                    column: 0
                    delegate: Rectangle {
                        implicitHeight: projectFrame.viewPadding
                        implicitWidth: projectNameText.width
                        color: row % 2 ? CSC.Style.blue : CSC.Style.darkBlue

                        ScrollView {
                            clip: true
                            contentHeight: availableHeight
                            anchors.fill: parent

                            ScrollBar.horizontal.interactive: false
                            
                            Text {
                                id: projectNameText
                                text: "<h4>" + projectName + "</h4>"
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
                                Layout.minimumWidth: textMetrics.width
                            }

                            onWidthChanged: {
                                progressbar.visible = (width > 100)
                            }
                        }
                    }
                }
            }
        }
    }
}