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

    FileDialog {
    }

    header: CSC.ProgressTracker {
        id: tracker
        progressIndex: stack.currentIndex
        model: ["Choose directory", "Export files", "Export complete"]
    }

    contentItem: StackLayout {
        id: stack
        currentIndex: 0

        ColumnLayout {
            spacing: CSC.Style.padding

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
				placeholderText: "Folder name"
				focus: true
				Layout.preferredWidth: 350
			}

            CSC.Button {
                id: continueButton
                text: "Continue"
                enabled: nameField.text != ""
                onClicked: { stack.currentIndex = 1 }
            }
        }

        ColumnLayout {
            spacing: CSC.Style.padding

            DropArea {
                id: dropArea;
                Layout.preferredHeight: dragColumn.height
                Layout.fillWidth: true

                Shape {
                    id: shape
                    anchors.fill: parent

                    ShapePath {
                        fillColor: "transparent"
                        strokeWidth: 3
                        strokeColor: CSC.Style.primaryColor
                        strokeStyle: ShapePath.DashLine
                        dashPattern: [ 1, 4 ]
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

                            onClicked: {
                                
                            }
                        }
                    }

                    Label {
                        text: "If you wish to export multiple files, please create a tar/zip file" 
                        font.pixelSize: 14
                        anchors.horizontalCenter: dragRow.horizontalCenter
                    }
                }

                onEntered: {
                }

                onDropped: {
                }
            }

            CSC.Button {
                id: exportButton
                text: "Export"
                enabled: false
            }
        }
    }
}