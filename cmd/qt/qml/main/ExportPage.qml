import QtQuick 2.13
import QtQuick.Controls 2.13
import QtQuick.Layouts 1.13
import QtQuick.Controls.Material 2.12
import QtQuick.Dialogs 1.3
import QtQuick.Shapes 1.13
import csc 1.2 as CSC

Page {
    id: page
    padding: 2 * CSC.Style.padding
    
    Material.accent: CSC.Style.primaryColor
    Material.foreground: CSC.Style.grey

    property bool chosen: false

    FileDialog {
        id: dialogSelect
        title: "Select file to export"
        folder: shortcuts.home
        selectExisting: true
        selectFolder: false

        onAccepted: {
            page.chosen = true
            exportModel.setProperty(0, "name", dialogSelect.fileUrl.toString())
        }
    }

    header: CSC.ProgressTracker {
        id: tracker
        visible: stack.currentIndex >= 2
        progressIndex: stack.currentIndex - 2
        model: ["Choose directory", "Export files", "Export complete"]
    }

    contentItem: StackLayout {
        id: stack
        currentIndex: QmlBridge.isProjectManager ? 1 : 0

        ColumnLayout {
            spacing: CSC.Style.padding

            Label {
                text: "<h1>Export is not possible</h1>"
                maximumLineCount: 1
            }

            Label {
                text: "Your need to be project manager to export files."
                font.pixelSize: 13
            }
        }

        FocusScope {
            focus: visible
            
            ColumnLayout {
                spacing: CSC.Style.padding
                width: stack.width

                Keys.onReturnPressed: continueButton.clicked() // Enter key
                Keys.onEnterPressed: continueButton.clicked()  // Numpad enter key

                Label {
                    text: "<h1>Select a destination folder for your export</h1>"
                    maximumLineCount: 1
                }

                Label {
                    text: "Your export will be sent to SD Connect. Please note that the folder name cannot be modified afterwards."
                    maximumLineCount: 1
                    font.pixelSize: 13
                }

                CSC.TextField {
                    id: nameField
                    titleText: "Folder name"
                    focus: true
                    Layout.preferredWidth: 400
                }

                CSC.Button {
                    id: continueButton
                    text: "Continue"
                    enabled: nameField.text != ""
                    Layout.alignment: Qt.AlignRight

                    onClicked: { 
                        exportModel.setProperty(0, "bucket", nameField.text)
                        stack.currentIndex = stack.currentIndex + 1 
                    }
                }
            }
        }

        ColumnLayout {
            spacing: CSC.Style.padding
            focus: visible

            DropArea {
                id: dropArea
                visible: !table.visible
                Layout.preferredHeight: dragColumn.height
                Layout.fillWidth: true

                Shape {
                    id: shape
                    anchors.fill: parent

                    ShapePath {
                        fillColor: "transparent"
                        strokeWidth: 3
                        strokeColor: dropArea.containsDrag ? CSC.Style.primaryColor : Qt.rgba(CSC.Style.primaryColor.r, CSC.Style.primaryColor.g, CSC.Style.primaryColor.b, 0.5)
                        strokeStyle: ShapePath.DashLine
                        dashPattern: [ 1, 3 ]
                        startX: 0; startY: 0
                        PathLine { x: shape.width; y: 0 }
                        PathLine { x: shape.width; y: shape.height }
                        PathLine { x: 0; y: shape.height }
                        PathLine { x: 0 ; y: 0 }
                    }
                }

                Column {
                    id: dragColumn
                    padding: 50
                    spacing: CSC.Style.padding
                    anchors.horizontalCenter: parent.horizontalCenter

                    Row {
                        id: dragRow
                        spacing: CSC.Style.padding
                        anchors.horizontalCenter: parent.horizontalCenter

                        Label {
                            text: "Drag and drop file or"
                            font.pixelSize: 15
                            font.weight: Font.DemiBold
                            anchors.verticalCenter: selectButton.verticalCenter
                        }

                        CSC.Button {
                            id: selectButton
                            text: "Select file"
                            outlined: true

                            onClicked: dialogSelect.visible = true
                        }
                    }

                    Label {
                        text: "If you wish to export multiple files, please create a tar/zip file." 
                        font.pixelSize: 14
                        anchors.horizontalCenter: dragRow.horizontalCenter
                    }
                }

                onDropped: {
                    if (!drop.hasUrls) {
                        popup.errorMessage = "Dropped item was not a file"
                        popup.open()
                        return
                    }

                    if (drop.urls.length != 1) {
                        popup.errorMessage = "Dropped too many items"
                        popup.open()
                        return
                    }
                    
                    if (QmlBridge.isFile(drop.urls[0])) {
                        page.chosen = true
                        exportModel.setProperty(0, "name", drop.urls[0])
                    } else {
                        popup.errorMessage = "Dropped item was not a file"
                        popup.open()
                    }
                }
            }

            CSC.Table {
                id: table
                visible: page.chosen
                hiddenHeader: true
                objectName: "files"
                Layout.fillWidth: true

                contentSource: fileLine
                footerSource: footerLine
                modelSource: ListModel {
                    id: exportModel

                    ListElement {
                        name: ""
                        bucket: ""
                    }
                }
            }

            RowLayout {
                spacing: CSC.Style.padding

                CSC.Button {
                    text: "Cancel"
                    outlined: true

                    onClicked: { 
                        stack.currentIndex = stack.currentIndex - 1
                        page.chosen = false
                    }
                }

                Item {
                    Layout.fillWidth: true
                }

                CSC.Button {
                    text: "Export"
                    enabled: page.chosen

                    onClicked: { QmlBridge.exportFile(exportModel.get(0).bucket, exportModel.get(0).name) }
                }
            }
        }
    }

    TextMetrics {
        id: textMetricsRemove
        text: "Remove"
        font.underline: true
        font.pixelSize: 14
        font.weight: Font.DemiBold
    }

    Component {
        id: footerLine

        RowLayout {
            Label {
                text: "File Name"
                Layout.preferredWidth: parent.width * 0.5
            }

            Label {
                text: "Target Folder"
                Layout.fillWidth: true
            }
        }
    }

    Component {
        id: fileLine

        RowLayout {
            property string name: modelData ? modelData.name : ""
            property string bucket: modelData ? modelData.bucket : ""

            Label {
                text: name.split('/').reverse()[0]
                elide: Text.ElideRight
                Layout.preferredWidth: parent.width * 0.5
            }

            Label {
                text: bucket
                elide: Text.ElideRight
                Layout.fillWidth: true
            }

            Label {
                text: textMetricsRemove.text
                font: textMetricsRemove.font
                color: CSC.Style.primaryColor                    
                
                MouseArea {
                    hoverEnabled: true
                    cursorShape: containsMouse ? Qt.PointingHandCursor : Qt.ArrowCursor
                    anchors.fill: parent

                    onClicked: page.chosen = false
                }
            }
        }
    }
}