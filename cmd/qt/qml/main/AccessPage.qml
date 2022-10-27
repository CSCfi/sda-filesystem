import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import QtQuick.Dialogs 1.3
import csc 1.2 as CSC

Page {
    id: page
    padding: 2 * CSC.Style.padding
    contentHeight: stack.height

    Material.foreground: CSC.Style.grey

    FileDialog {
        id: dialogCreate
        title: "Choose or create a folder"
        folder: shortcuts.home
        selectExisting: false
        selectFolder: true
        onAccepted: {
            var mountError = QmlBridge.changeMountPoint(folder)
            if (mountError) {
                popup.errorMessage = mountError
                popup.open()
            }
        }
    }

    header: CSC.ProgressTracker {
        id: tracker
        progressIndex: stack.currentIndex
        model: ["Choose directory", "Prepare access", "Access ready"]
    }

    StackLayout {
        id: stack
        width: parent.width
        currentIndex: 0
        height: children[currentIndex].height

        ColumnLayout {
            spacing: CSC.Style.padding
            focus: visible
            Layout.preferredWidth: stack.width

            Keys.onReturnPressed: continueButton.clicked() // Enter key
            Keys.onEnterPressed: continueButton.clicked()  // Numpad enter key

            Label {
                text: "<h1>Choose directory</h1>"
                maximumLineCount: 1
            }

            Label {
                text: "Choose in which local directory your data will be available."
                maximumLineCount: 1
                font.pixelSize: 14
            }

            Row {
                spacing: CSC.Style.padding

                Rectangle {
                    radius: 5
                    border.width: 1
                    border.color: CSC.Style.grey
                    width: 400
                    height: childrenRect.height
                    anchors.verticalCenter: changeButton.verticalCenter

                    Flickable {
                        clip: true
                        width: parent.width
                        height: mountText.height
                        contentWidth: mountText.width
                        boundsBehavior: Flickable.StopAtBounds

                        ScrollBar.horizontal: ScrollBar { interactive: false }
                        
                        Label {
                            id: mountText
                            text: QmlBridge.mountPoint
                            font.pixelSize: 15
                            verticalAlignment: Text.AlignVCenter
                            maximumLineCount: 1
                            padding: 10
                        }
                    }
                }

                CSC.Button {
                    id: changeButton
                    text: "Change"
                    outlined: true

                    onClicked: { popup.close(); dialogCreate.visible = true }
                }
            }

            CSC.Button {
                id: continueButton
                text: "Continue"
                enabled: QmlBridge.mountPoint != ""
                Layout.alignment: Qt.AlignRight

                onClicked: if (enabled) { stack.currentIndex = 1; QmlBridge.loadFuse() }
            }
        }      

        ColumnLayout {
            id: accessLayout
            spacing: CSC.Style.padding
            focus: visible

            Keys.onReturnPressed: openButton.clicked() // Enter key
            Keys.onEnterPressed: openButton.clicked()  // Numpad enter key

            Label {
                id: headerText
                text: "<h1>Preparing access</h1>"
                maximumLineCount: 1
            }

            Label {
                text: "If you update the contents of these projects, please refresh access."
                visible: buttonRow.visible
                maximumLineCount: 1
                font.pixelSize: 14
            }

            RowLayout {
                id: buttonRow
                spacing: CSC.Style.padding
                visible: false
                Layout.alignment: Qt.AlignRight

                CSC.Button {
                    id: refreshButton
                    text: "Refresh"
                    outlined: true

                    Material.accent: "white"

                    onClicked: {
                        state = "loading"
                        var message = QmlBridge.refreshFuse()
                        if (message != "") {
                            createButton.state = ""
                            popup.errorMessage = message
                            popup.open()
                        }
                    }

                    Connections {
                        target: QmlBridge
                        onFuseReady: refreshButton.state = ""
                    }

                    states: [
                        State {
                            name: "loading";  
                            PropertyChanges { target: refreshButton; enabled: false }
                            PropertyChanges { target: openButton; enabled: false }
                            PropertyChanges { target: infoText; text: "Data Gateway is being refreshed" }
                        }
                    ]
                }

                CSC.Button {
                    id: openButton
                    text: "Open folder" 

                    onClicked: if (enabled) { QmlBridge.openFuse() }
                }
            }

            ColumnLayout {
                spacing: 0.5 * CSC.Style.padding

                Label {
                    id: infoText
                    text: "Please wait, this might take a few minutes."
                    maximumLineCount: 1
                    font.pixelSize: 14
                }

                CSC.ProgressBar {
                    id: progressbar
                    value: (ProjectModel.allContainers <= 0) ? 0 : ProjectModel.loadedContainers / ProjectModel.allContainers
                    Layout.fillWidth: true
                }

                Label {
                    text: Math.floor(progressbar.value * 100) + "% complete"
                    maximumLineCount: 1
                    font.pixelSize: 14
                }
            }

            CSC.Table {
                id: table
                modelSource: ProjectModel
                contentSource: projectLine
                footerSource: footerLine
                objectName: "projects"
                Layout.fillWidth: true
            }

            Connections {
                target: QmlBridge
                onFuseReady: {
                    tracker.progressIndex = 3
                    headerText.text = "<h1>Access ready</h1>"
                    infoText.text = "Data Gateway is ready to be used."
                    buttonRow.visible = true
                }
            }
        }
    }  

    TextMetrics {
        id: textMetrics100
        text: "100 %"
        font.pixelSize: 13
    }

    Component {
        id: footerLine

        RowLayout {
            Label {
                text: "Name"
                Layout.fillWidth: true
            }

            Label {
                text: "Location"
                Layout.preferredWidth: 150
            }

            Label {
                text: "Progress"
                Layout.maximumWidth: 200
                Layout.minimumWidth: 200
            }            
        }
    }

    Component {
        id: projectLine

        RowLayout {
            property string projectName: modelData ? modelData.projectName : ""
            property string repositoryName: modelData ? modelData.repositoryName : ""
            property int allContainers: modelData ? modelData.allContainers : -1
            property int loadedContainers: modelData ? modelData.loadedContainers : 0

            Label {
                text: projectName
                elide: Text.ElideRight
                Layout.fillWidth: true
            }

            Label {
                text: repositoryName
                Layout.preferredWidth: 150
            }

            RowLayout {
                id: loadingStatus
                Layout.maximumWidth: 200
                Layout.minimumWidth: 200

                property real value: (allContainers == -1) ? 0 : (allContainers == 0) ? 1 : loadedContainers / allContainers

                CSC.ProgressBar {
                    id: progressbar
                    value: parent.value
                    Layout.fillWidth: true
                }

                Label {
                    id: percentValue
                    text: Math.floor(parent.value * 100) + "%"
                    maximumLineCount: 1
                    font: textMetrics100.font
                    Layout.minimumWidth: textMetrics100.width
                }
            }
        }
    }  
}