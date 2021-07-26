import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import csc 1.0 as CSC

Page {
    id: page
    property int gridSpacing: 20
    property int buttonPadding: 20
    implicitWidth: dialogColumn.implicitWidth + projectView.implicitWidth + 2 * gridSpacing  

    RowLayout {
        spacing: page.gridSpacing
        anchors.fill: parent
        anchors.margins: page.gridSpacing

        ColumnLayout {
            id: dialogColumn
            Layout.fillHeight: true
            Layout.fillWidth: true
            Layout.preferredWidth: 1
            Layout.maximumWidth: 400

            Frame {
                id: acceptFrame
                Layout.fillWidth: true

                background: Rectangle {
                    color: CSC.Style.lightGreen
                }

                ColumnLayout {
                    anchors.fill: parent
                    spacing: page.gridSpacing

                    Text {
                        text: "<h3>FUSE will be mounted at:</h3>"
                    }

                    CSC.TextField {
                        id: mountField
                        text: QmlBridge.mountPoint
                        Layout.alignment: Qt.AlignLeft
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

                        onClicked: {
                            if (acceptButton.state == "") {
                                QmlBridge.changeMountPoint(mountField.text)
                                acceptButton.state = 'accepted'
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
                    Layout.alignment: Qt.AlignTop
                    Layout.minimumWidth: implicitWidth
                    
                    onClicked: QmlBridge.openFuse()
                }

                CSC.Button {
                    id: loadButton
                    text: "Load FUSE"
                    padding: page.buttonPadding
                    enabled: false
                    Layout.fillWidth: true
                    Layout.alignment: Qt.AlignTop
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

            // Dummy for alignment
            Rectangle {
                Layout.fillHeight: true
                color: "transparent"
            }
        }

        ListView {
            id: projectView
            interactive: false
            implicitWidth: dialogColumn.implicitWidth
            Layout.fillHeight: true
            Layout.fillWidth: true
            Layout.preferredWidth: 1

            Material.accent: CSC.Style.altGreen

            model: ProjectModel
            delegate: RowLayout {
                id: projectRow
                anchors.right: parent.right
                anchors.left: parent.left

                property real value: (containerCount == -1) ? 0 : (containerCount == 0) ? 1 : loadedContainers / containerCount

                Text {
                    text: "<h4>" + projectName + "</h4>"
                }
                ProgressBar {
                    value: projectRow.value
                }
                Text {
                    text: Math.round(projectRow.value * 100) + "%"
                }
            }
        }
    }
}