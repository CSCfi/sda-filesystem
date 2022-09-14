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
        id: dialogSelect
        title: "Select file to export"
        folder: shortcuts.home
        selectExisting: true
        selectFolder: false

        onAccepted: { 
            console.log(dialogSelect.fileUrl)
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
        currentIndex: !LoginModel.loggedInToSDConnect ? 1 : (QmlBridge.isProjectManager ? 2 : 0)

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

        ColumnLayout {
            id: loginColumn
            spacing: CSC.Style.padding

            Keys.onReturnPressed: loginButton.clicked() // Enter key
            Keys.onEnterPressed: loginButton.clicked()  // Numpad enter key

            Label {
                text: "<h1>Please log in</h1>"
                maximumLineCount: 1
                color: CSC.Style.grey
            }

            Label {
                text: "You need to be logged in to the service using your CSC credentials to export files."
                font.pixelSize: 14
                color: CSC.Style.grey
            }

            /*Label {
                text: "Please log in with your CSC credentials"
                maximumLineCount: 1
                font.pixelSize: 13
            }*/

            CSC.TextField {
                id: usernameField
                focus: true
                placeholderText: "Username"
                Layout.preferredWidth: 350
			}

			CSC.TextField {
                id: passwordField
                placeholderText: "Password"
                errorText: "Please enter valid password"
                echoMode: TextInput.Password
                activeFocusOnTab: true
                extraPadding: true
                Layout.preferredWidth: 350
			}

            CSC.Button {
				id: loginButton
				text: "Login"

                Connections {
                    target: QmlBridge
                    enabled: window.loggedIn
                    onLogin401: {
                        passwordField.errorVisible = true
                        loginColumn.enabled = true
                        loginButton.loading = false

                        if (usernameField.text != "") {
                            passwordField.focus = true
                            passwordField.selectAll()
                        }
                    }
                    onLoginError: {
                        loginButton.loading = false
                        loginColumn.enabled = true
                        popup.errorMessage = message
                        popup.open()
					}
                }

				onClicked: {
					popup.close()
					loginColumn.enabled = false
					loginButton.loading = true
					passwordField.errorVisible = false
					QmlBridge.loginWithPassword(LoginModel.connectIdx, usernameField.text, passwordField.text)
				}
			}
        }

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
                onClicked: { stack.currentIndex = stack.currentIndex + 1 }
            }
        }

        ColumnLayout {
            spacing: CSC.Style.padding

            DropArea {
                id: dropArea
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
                    
                    for (var i = 0; i < drop.urls.length; i++) {
                        console.log(drop.urls[i])
                        /*if (QmlBridge.isFile(drag.urls[i])) {

                        }*/
                    }
                }
            }

            Row {
                spacing: CSC.Style.padding

                CSC.Button {
                    text: "Export"
                    enabled: false

                    onClicked: { }
                }

                CSC.Button {
                    text: "Cancel"
                    outlined: true

                    onClicked: { stack.currentIndex = stack.currentIndex - 1 }
                }
            }
        }
    }
}